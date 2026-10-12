// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand under which every fixture unit reads its carriers.
const brand = string(frontendtest.Brand)

// The cases parse a fixture file and a file beside it, under a package
// clause and the package path of the clause. The Maven project file above
// them states the module, and most cases declare the public class.
const (
	srcFile     = "src/main/java/com/acme/A.java"
	siblingFile = "src/main/java/com/acme/B.java"
	pkgClause   = "package com.acme;\n\n"
	pkgPath     = "com/acme"
	pomFile     = "pom.xml"
	pomSrc      = "<project><groupId>com.acme</groupId><artifactId>store</artifactId></project>"
	publicClass = "public class A {}\n"
)

// The module that pomSrc states, the module root of a pom.xml at the root
// of the workspace, and the directory of a nested module.
const (
	storeModule = "com.acme:store"
	rootModule  = "."
	nestedRoot  = "store"
)

// The SHA-1 record that the unit of a Maven JAR shares, and its text.
const (
	jarRecord  = jarMember + ".sha1"
	recordText = "0  lib.jar\n"
)

// The canonical scale of every benchmark: 1000 packages of 10 files of 20
// declarations, 200,000 declarations in all.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// parseAllocs is the budget of one parse of the canonical corpus of
// 200,000 declarations. One run counted 1,820,009 allocations: the nodes,
// texts, stamps and type references of the lowering, and one tree handle
// per file. Tree-sitter builds its trees in C memory, which the count does
// not see. The budget is about 5% above the count.
const parseAllocs = 1_910_000

// treeReader is the recorded door of the partition over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns the bytes of one file.
func (r treeReader) Read(path string) ([]byte, error) { return r.tree.ReadFile(path) }

// A parse turns the files of one directory into the packages of the unit.
// The cases pin its contract over the module identity and the failures of
// the unit.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps a package with the module that its pom.xml states", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, pomTree(pomSrc), srcFile, plugin.DepthFull)
			assert.Equal(t, stampsOf(gb, meta.ModuleKey), []any{storeModule}, "the group and the artifact")
		})

		t.Run("stamps a package with the directory of its pom.xml as the module root", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, pomTree(pomSrc), srcFile, plugin.DepthFull)
			assert.Equal(t, stampsOf(gb, meta.ModuleRootKey), []any{rootModule}, "the root of the workspace")
		})

		t.Run("stamps the directory of a nested pom.xml as the module root", func(t *testing.T) {
			t.Parallel()

			nested := path.Join(nestedRoot, srcFile)
			gb, _ := parsedTree(t, fstest.MapFS{
				path.Join(nestedRoot, pomFile): {Data: []byte(pomSrc)},
				nested:                         {Data: []byte(pkgClause + publicClass)},
			}, nested, plugin.DepthFull)
			assert.Equal(t, stampsOf(gb, meta.ModuleRootKey), []any{nestedRoot}, "the directory of the module")
		})

		t.Run("stamps a package once for the files of one unit", func(t *testing.T) {
			t.Parallel()

			tree := pomTree(pomSrc)
			tree[siblingFile] = &fstest.MapFile{Data: []byte(pkgClause + "public class B {}\n")}
			gb, _ := parsedTree(t, tree, srcFile, plugin.DepthFull)
			assert.Length(t, stampsOf(gb, meta.ModuleKey), 1, "one stamp per package")
		})

		t.Run("stamps no module for a unit that no pom.xml governs", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+publicClass)
			assert.Empty(t, stampsOf(gb, meta.ModuleKey), "the unit states no module")
		})

		t.Run("reports BadPOM for a pom.xml that does not parse", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, pomTree("<project>"), srcFile, plugin.DepthFull)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadPOM}, "the fault of the project file reports")
			assert.Equal(t, found[0].Severity, diag.SeverityWarning, "the finding is a warning")
		})

		t.Run("loads the files under a pom.xml that does not parse", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, pomTree("<project>"), srcFile, plugin.DepthFull)
			named[*node.Struct](t, fileIn(t, gb, pkgPath).Decls, "A")
		})

		t.Run("reports BadPOM for a pom.xml that does not read", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			tree := fstest.MapFS{srcFile: {Data: []byte(pkgClause + publicClass)}}
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: srcFile, Shared: []string{pomFile}}}, tree,
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
			assert.Equal(t, codesOf(slices.Collect(sink.All())), []diag.Code{frontend.BadPOM},
				"the project file of the unit does not read")
		})

		t.Run("reads no shared input but a pom.xml as the project file", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			tree := fstest.MapFS{jarMember: {Data: mustRead(t, libJAR)}, jarRecord: {Data: []byte(recordText)}}
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: jarMember, Shared: []string{jarRecord}}}, tree,
				plugin.DepthSignatures, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
			assert.Empty(t, slices.Collect(sink.All()), "the SHA-1 record of a Maven JAR states no module")
		})

		t.Run("returns the error of the context for a done context", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: srcFile}},
				fstest.MapFS{srcFile: {Data: []byte(publicClass)}}, plugin.DepthFull, f.Syntax(), brand,
				diag.NewSink(), f.Name())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return f.Parse(ctx, u) },
				"a cancelled load parses nothing")
		})

		t.Run("returns the error of the read for a member outside the tree", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: srcFile}}, fstest.MapFS{}, plugin.DepthFull,
				f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u), "a member that does not read fails the unit")
		})
	})
}

// BenchmarkParse drives the parse at the canonical scale: every unit of the
// scaled corpus through Parse into a builder of its own. The partition
// runs once before the loop, so the number measures the tree-sitter parse
// and the lowering alone, and the benchmark fails above parseAllocs. Only
// -bench checks the budget. One parse takes about 2.6 s on four cores, so
// the 101 calls of an allocation check would take more than four minutes.
func BenchmarkParse(b *testing.B) {
	tree := scaledJava()
	f := frontend.New(nil)
	units, err := f.Partition(b.Context(), claimedIn(tree), treeReader{tree})
	assert.NoError(b, err, "the corpus partitions")
	c := bench.Start(b).MaxAllocs(parseAllocs)
	defer c.End()
	for c.Loop() {
		for _, unit := range units {
			u := plugin.NewSourceUnit(unit, tree, plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			err = cmp.Or(err, f.Parse(b.Context(), u))
		}
	}
	assert.NoError(b, err, "the corpus parses")
}

// scaledJava returns the canonical corpus as Java source: a directory per
// package of files of four classes, whose first field has a class of the
// package before it as its type, and sixteen interfaces of one constant.
func scaledJava() fstest.MapFS {
	tree := fstest.MapFS{}
	for p := range benchPackages {
		prev := (p + benchPackages - 1) % benchPackages
		for f := range benchFiles {
			var b strings.Builder
			fmt.Fprintf(&b, "package p%d;\n\nimport p%d.T%d_%d_0;\n\n", p, prev, prev, f)
			for d := range benchDecls {
				if d < 4 {
					fmt.Fprintf(&b, "class T%d_%d_%d {\n    T%d_%d_0 f0;\n    int f1;\n}\n\n", p, f, d, prev, f)
				} else {
					fmt.Fprintf(&b, "interface I%d_%d_%d {\n    int C = %d;\n}\n\n", p, f, d, d)
				}
			}
			tree[fmt.Sprintf("p%d/F%d.java", p, f)] = &fstest.MapFile{Data: []byte(b.String())}
		}
	}
	return tree
}

// pomTree returns a tree of a pom.xml with a source and the fixture file,
// which declares the public class A in com.acme.
func pomTree(src string) fstest.MapFS {
	return fstest.MapFS{
		pomFile: {Data: []byte(src)},
		srcFile: {Data: []byte(pkgClause + publicClass)},
	}
}

// parsedTree partitions the Java files of a tree through the frontend and
// parses the unit that has a member, at a depth. It returns the builder of
// the unit and the findings that the parse reported.
func parsedTree(
	tb testing.TB, tree fstest.MapFS, member string, depth plugin.Depth,
) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	f := frontend.New(nil)
	units, err := f.Partition(tb.Context(), claimedIn(tree), treeReader{tree})
	assert.NoError(tb, err, "the fixture partitions")
	for _, unit := range units {
		if !slices.ContainsFunc(unit, func(ref plugin.SourceRef) bool { return ref.Path == member }) {
			continue
		}
		sink := diag.NewSink()
		u := plugin.NewSourceUnit(unit, tree, depth, f.Syntax(), brand, sink, f.Name())
		assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
		return u.Graph(), slices.Collect(sink.All())
	}
	tb.Fatalf("no unit has the member %s", member)
	return nil, nil
}

// parsedSource parses one file at src/main/java/com/acme/A.java at full
// depth.
func parsedSource(tb testing.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{srcFile: {Data: []byte(src)}}, srcFile, plugin.DepthFull)
}

// declsOf parses a body below the package clause of com.acme and returns
// the declarations of the file.
func declsOf(tb testing.TB, body string) node.Symbols {
	tb.Helper()

	gb, _ := parsedSource(tb, pkgClause+body)
	return fileIn(tb, gb, pkgPath).Decls
}

// shallowDecls parses a body below the package clause of com.acme at
// signature depth and returns the declarations of the file.
func shallowDecls(tb testing.TB, body string) node.Symbols {
	tb.Helper()

	gb, _ := parsedTree(tb, fstest.MapFS{srcFile: {Data: []byte(pkgClause + body)}}, srcFile,
		plugin.DepthSignatures)
	return fileIn(tb, gb, pkgPath).Decls
}

// packageIn returns the package of a path in a builder.
func packageIn(tb testing.TB, gb *plugin.GraphBuilder, pkg string) *node.Package {
	tb.Helper()

	for _, p := range gb.Packages() {
		if p.ID.Package == pkg {
			return p
		}
	}
	tb.Fatalf("the unit declares no package %q", pkg)
	return nil
}

// fileIn returns the first File node of the package of a path in a
// builder.
func fileIn(tb testing.TB, gb *plugin.GraphBuilder, pkg string) *node.File {
	tb.Helper()

	p := packageIn(tb, gb, pkg)
	assert.NotEmpty(tb, p.Files, "the package has a File node")
	return p.Files[0]
}

// named returns the declaration of a name among declarations, of the type
// that the case expects.
func named[T symbol.Symbol](tb testing.TB, decls node.Symbols, name string) T {
	tb.Helper()

	for _, d := range decls {
		if decl, is := d.(T); is && nameOf(d) == name {
			return decl
		}
	}
	var zero T
	tb.Fatalf("no %T has the name %s", zero, name)
	return zero
}

// nameOf returns the written name of a declaration.
func nameOf(s symbol.Symbol) string {
	switch d := s.(type) {
	case *node.Struct:
		return d.Name
	case *node.Interface:
		return d.Name
	case *node.Enum:
		return d.Name
	default:
		return ""
	}
}

// codesOf returns the codes of findings, in report order.
func codesOf(found []diag.Diag) []diag.Code {
	out := make([]diag.Code, 0, len(found))
	for _, d := range found {
		out = append(out, d.Code)
	}
	return out
}

// stampsOf returns the values that a builder stamped under a key, in
// record order.
func stampsOf(gb *plugin.GraphBuilder, key meta.KeyName) []any {
	var out []any
	for _, rec := range gb.StampRecords() {
		if rec.Stamp.Key == key {
			out = append(out, rec.Stamp.Value)
		}
	}
	return out
}
