// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
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

// brand is the brand every fixture unit reads its carriers under.
const brand = string(frontendtest.Brand)

// The cases parse a fixture file and a file beside it, under a package
// clause and the package path the clause names. The Maven project file
// above them states the module, and most cases declare the public class.
const (
	srcFile     = "src/main/java/com/acme/A.java"
	siblingFile = "src/main/java/com/acme/B.java"
	pkgClause   = "package com.acme;\n\n"
	pkgPath     = "com/acme"
	pomFile     = "pom.xml"
	pomSrc      = "<project><groupId>com.acme</groupId><artifactId>store</artifactId></project>"
	publicClass = "public class A {}\n"
)

// The module pomSrc states, the module root of a pom.xml at the
// workspace root, and the directory of a nested module.
const (
	storeModule = "com.acme:store"
	rootModule  = "."
	nestedRoot  = "store"
)

// The SHA-1 record a Maven JAR's unit shares, and its text.
const (
	jarRecord  = jarMember + ".sha1"
	recordText = "0  lib.jar\n"
)

// The canonical scale every layer benches at: 1000 packages of 10 files
// of 20 declarations, 200k declarations in all.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// parseAllocs is one parse of the canonical corpus. With the collector
// off a parse allocates 1,829,001 times: the lowering's nodes, texts,
// stamps and type references, and one tree handle per file. Tree-sitter
// builds its trees in C memory, which the count does not see. The
// collections that run during a parse add more: 10 fresh processes
// counted 1 to 10 more. The ceiling allows 32 more.
const parseAllocs = 1_829_001 + 32

// treeReader is the partition's recorded door over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r treeReader) Read(path string) ([]byte, error) { return r.tree.ReadFile(path) }

// A parse turns one directory's files into the unit's packages, so its
// contract over the module identity and the unit's failures is pinned.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps a package with the module its pom.xml states", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, pomTree(pomSrc), srcFile, plugin.DepthFull)
			assert.Equal(t, stampsOf(gb, meta.ModuleKey), []any{storeModule}, "the group and the artifact")
		})

		t.Run("stamps a package with its pom.xml's directory as the module root", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, pomTree(pomSrc), srcFile, plugin.DepthFull)
			assert.Equal(t, stampsOf(gb, meta.ModuleRootKey), []any{rootModule}, "the workspace root")
		})

		t.Run("stamps the directory of a nested pom.xml as the module root", func(t *testing.T) {
			t.Parallel()

			nested := path.Join(nestedRoot, srcFile)
			gb, _ := parsedTree(t, fstest.MapFS{
				path.Join(nestedRoot, pomFile): {Data: []byte(pomSrc)},
				nested:                         {Data: []byte(pkgClause + publicClass)},
			}, nested, plugin.DepthFull)
			assert.Equal(t, stampsOf(gb, meta.ModuleRootKey), []any{nestedRoot}, "the module's own directory")
		})

		t.Run("stamps a package once for the files of one unit", func(t *testing.T) {
			t.Parallel()

			tree := pomTree(pomSrc)
			tree[siblingFile] = &fstest.MapFile{Data: []byte(pkgClause + "public class B {}\n")}
			gb, _ := parsedTree(t, tree, srcFile, plugin.DepthFull)
			assert.Length(t, stampsOf(gb, meta.ModuleKey), 1, "one stamp per package")
		})

		t.Run("stamps no module for a unit no pom.xml governs", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+publicClass)
			assert.Empty(t, stampsOf(gb, meta.ModuleKey), "the unit states no module")
		})

		t.Run("reports BadPOM for a pom.xml that does not parse", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, pomTree("<project>"), srcFile, plugin.DepthFull)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadPOM}, "the project file's fault reports")
			assert.Equal(t, found[0].Severity, diag.SeverityWarning, "as a warning")
		})

		t.Run("loads the files of a pom.xml that does not parse", func(t *testing.T) {
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
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
			assert.Equal(t, codesOf(slices.Collect(sink.All())), []diag.Code{frontend.BadPOM},
				"the project file it declares does not read")
		})

		t.Run("reads no shared input but a pom.xml as the project file", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			tree := fstest.MapFS{jarMember: {Data: mustRead(t, libJAR)}, jarRecord: {Data: []byte(recordText)}}
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: jarMember, Shared: []string{jarRecord}}}, tree,
				plugin.DepthSignatures, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
			assert.Empty(t, slices.Collect(sink.All()), "a Maven JAR's SHA-1 record states no module")
		})

		t.Run("returns the context's error for a done context", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: srcFile}},
				fstest.MapFS{srcFile: {Data: []byte(publicClass)}}, plugin.DepthFull, f.Syntax(), brand,
				diag.NewSink(), f.Name())
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			assert.ErrorIs(t, f.Parse(ctx, u), context.Canceled, "a cancelled load parses nothing")
		})

		t.Run("returns the read's error for a member outside the tree", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: srcFile}}, fstest.MapFS{}, plugin.DepthFull,
				f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(context.Background(), u), "a member that does not read fails the unit")
		})
	})
}

// BenchmarkParse drives the parse at the canonical scale: every unit of
// the scaled corpus through Parse into a builder of its own, the
// partition run once before the loop, so the number measures the
// tree-sitter parse and the lowering alone, and fails above
// parseAllocs. Only -bench checks the ceiling. One parse takes about
// 2.4 s on four cores, and the 101 calls of an allocation check would
// take four minutes.
func BenchmarkParse(b *testing.B) {
	tree := scaledJava()
	f := frontend.New(nil)
	units, err := f.Partition(context.Background(), claimedIn(tree), treeReader{tree})
	if err != nil {
		b.Fatalf("the corpus partitions: %v", err)
	}
	c := bench.Start(b).MaxAllocs(parseAllocs)
	defer c.End()
	for c.Loop() {
		for _, unit := range units {
			u := plugin.NewSourceUnit(unit, tree, plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			if err := f.Parse(context.Background(), u); err != nil {
				b.Fatalf("the corpus parses: %v", err)
			}
		}
	}
}

// scaledJava returns the canonical corpus as Java source: a directory
// per package of files of four classes, whose first field is a class the
// package before it declares, and sixteen interfaces of one constant.
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

// pomTree returns a tree of a pom.xml of a source and the fixture file,
// which declares the public class A in com.acme.
func pomTree(src string) fstest.MapFS {
	return fstest.MapFS{
		pomFile: {Data: []byte(src)},
		srcFile: {Data: []byte(pkgClause + publicClass)},
	}
}

// parsedTree partitions a tree's Java files through the frontend and
// parses the unit that has a member, at a depth. It returns the unit's
// builder and the findings the parse reported.
func parsedTree(
	tb assert.TB, tree fstest.MapFS, member string, depth plugin.Depth,
) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	f := frontend.New(nil)
	units, err := f.Partition(context.Background(), claimedIn(tree), treeReader{tree})
	assert.NoError(tb, err, "the fixture partitions")
	for _, unit := range units {
		if !slices.ContainsFunc(unit, func(ref plugin.SourceRef) bool { return ref.Path == member }) {
			continue
		}
		sink := diag.NewSink()
		u := plugin.NewSourceUnit(unit, tree, depth, f.Syntax(), brand, sink, f.Name())
		assert.NoError(tb, f.Parse(context.Background(), u), "the unit parses")
		return u.Graph(), slices.Collect(sink.All())
	}
	tb.Fatalf("no unit has the member %s", member)
	return nil, nil
}

// parsedSource parses one file at src/main/java/com/acme/A.java, at full
// depth.
func parsedSource(tb assert.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{srcFile: {Data: []byte(src)}}, srcFile, plugin.DepthFull)
}

// declsOf parses a body below the package clause of com.acme and
// returns the file's declarations.
func declsOf(tb assert.TB, body string) node.Symbols {
	tb.Helper()

	gb, _ := parsedSource(tb, pkgClause+body)
	return fileIn(tb, gb, pkgPath).Decls
}

// shallowDecls parses a body below the package clause of com.acme at
// signature depth and returns the file's declarations.
func shallowDecls(tb assert.TB, body string) node.Symbols {
	tb.Helper()

	gb, _ := parsedTree(tb, fstest.MapFS{srcFile: {Data: []byte(pkgClause + body)}}, srcFile,
		plugin.DepthSignatures)
	return fileIn(tb, gb, pkgPath).Decls
}

// packageIn returns a builder's package of a path.
func packageIn(tb assert.TB, gb *plugin.GraphBuilder, pkg string) *node.Package {
	tb.Helper()

	for _, p := range gb.Packages() {
		if p.ID.Package == pkg {
			return p
		}
	}
	tb.Fatalf("the unit declares no package %q", pkg)
	return nil
}

// fileIn returns the first File node a builder's package of a path has.
func fileIn(tb assert.TB, gb *plugin.GraphBuilder, pkg string) *node.File {
	tb.Helper()

	p := packageIn(tb, gb, pkg)
	assert.NotEmpty(tb, p.Files, "the package has a File node")
	return p.Files[0]
}

// named returns the declaration of a name among declarations, of the
// type the case expects.
func named[T symbol.Symbol](tb assert.TB, decls node.Symbols, name string) T {
	tb.Helper()

	for _, d := range decls {
		if decl, is := d.(T); is && nameOf(d) == name {
			return decl
		}
	}
	var zero T
	tb.Fatalf("no %T is named %s", zero, name)
	return zero
}

// nameOf returns a declaration's written name.
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

// stampsOf returns the values a builder stamped under a key, in record
// order.
func stampsOf(gb *plugin.GraphBuilder, key meta.KeyName) []any {
	var out []any
	for _, rec := range gb.StampRecords() {
		if rec.Stamp.Key == key {
			out = append(out, rec.Stamp.Value)
		}
	}
	return out
}
