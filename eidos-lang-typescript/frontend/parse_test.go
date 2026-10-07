// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand every fixture unit reads its carriers under.
const brand = string(frontendtest.Brand)

// The cases parse a module file under the package its path names, the
// declaration file of that package, and a TSX file, beside the root
// tsconfig and a tsconfig that does not parse.
const (
	aFile       = "src/a.ts"
	aPackage    = "src/a"
	aDeclFile   = "src/a.d.ts"
	tsxFile     = "src/view.tsx"
	tsxPackage  = "src/view"
	rootConfig  = "tsconfig.json"
	badConfig   = "{ \"compilerOptions\": "
	exportClass = "export class A {}\n"
)

// The canonical scale every layer benches at: 1000 packages of 10 files
// of 20 declarations, 200k declarations in all. A TypeScript unit is one
// file, so the corpus is 10,000 units.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// parseAllocs is one parse of the canonical corpus. With the collector
// off a parse allocates 2,400,001 times: the lowering's nodes, texts,
// stamps and type references, each unit's source and sink, and one tree
// handle per file. Tree-sitter builds its trees in C memory, which the
// count does not see. The collections that run during a parse add more:
// 10 fresh processes counted up to 9 more. The ceiling allows 32 more.
const parseAllocs = 2_400_001 + 32

// treeReader is the partition's recorded door over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r treeReader) Read(path string) ([]byte, error) { return r.tree.ReadFile(path) }

// A parse turns one file into the unit's packages, so its contract
// over the tree, the grammar and the configuration is pinned.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a module's declarations into the package its path names", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, exportClass)
			assert.Length(t, fileIn(t, gb, aPackage).Decls, 1, "the class is in src/a")
		})

		t.Run("parses a .tsx file with the TSX grammar", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{tsxFile: {Data: []byte(
				"export const View = () => <div className=\"v\" />;\n",
			)}}, tsxFile, plugin.DepthFull)
			assert.Empty(t, found, "the JSX parses")
			assert.Length(t, fileIn(t, gb, tsxPackage).Decls, 1, "the constant is declared")
		})

		t.Run("reports BadConfig for a tsconfig that does not parse", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, fstest.MapFS{
				rootConfig: {Data: []byte(badConfig)},
				aFile:      {Data: []byte(exportClass)},
			}, aFile, plugin.DepthFull)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadConfig}, "the configuration's fault reports")
		})

		t.Run("reports BadConfig for a governing configuration that does not read", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			tree := fstest.MapFS{aFile: {Data: []byte(exportClass)}}
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: aFile, Shared: []string{rootConfig}}}, tree,
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
			assert.Equal(t, codesOf(slices.Collect(sink.All())), []diag.Code{frontend.BadConfig},
				"the configuration it declares does not read")
		})

		t.Run("returns the context's error for a done context", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			tree := fstest.MapFS{aFile: {Data: []byte(exportClass)}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: aFile}}, tree, plugin.DepthFull,
				f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return f.Parse(ctx, u) },
				"a cancelled load parses nothing")
		})

		t.Run("returns the read's error for a member outside the tree", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: aFile}}, fstest.MapFS{}, plugin.DepthFull,
				f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u), "a member that does not read fails the unit")
		})
	})
}

// BenchmarkParse drives the parse at the canonical scale: every unit of
// the scaled corpus through Parse into a builder of its own, the
// partition run once before the loop, so the number measures the
// tree-sitter parse and the lowering alone, and fails above
// parseAllocs. Only -bench checks the ceiling. One parse takes about
// 2.1 s on four cores, and the 101 calls of an allocation check would
// take more than three minutes.
func BenchmarkParse(b *testing.B) {
	tree := scaledTypeScript()
	f := frontend.New()
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

// scaledTypeScript returns the canonical corpus as TypeScript modules: a
// directory per package of modules of four interfaces, whose first field
// is an interface the package before it declares, and sixteen constants.
func scaledTypeScript() fstest.MapFS {
	tree := fstest.MapFS{}
	for p := range benchPackages {
		prev := (p + benchPackages - 1) % benchPackages
		for f := range benchFiles {
			var b strings.Builder
			fmt.Fprintf(&b, "import { T%d_0 as Prev } from '../p%d/f%d';\n\n", f, prev, f)
			for d := range benchDecls {
				if d < 4 {
					fmt.Fprintf(&b, "export interface T%d_%d {\n  f0: Prev;\n  f1: number;\n}\n\n", f, d)
				} else {
					fmt.Fprintf(&b, "export const c%d_%d = %d;\n\n", f, d, d)
				}
			}
			tree[fmt.Sprintf("p%d/f%d.ts", p, f)] = &fstest.MapFile{Data: []byte(b.String())}
		}
	}
	return tree
}

// claimedIn returns every file of a tree as the selection claims it, in
// path order.
func claimedIn(tree fstest.MapFS) []plugin.SourceRef {
	claimed := make([]plugin.SourceRef, 0, len(tree))
	for p := range tree {
		claimed = append(claimed, plugin.SourceRef{Path: p})
	}
	slices.SortFunc(claimed, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	return claimed
}

// parsedTree partitions a tree through the frontend and parses the unit
// of one member at a depth. It returns the unit's builder and the
// findings the parse reported.
func parsedTree(
	tb testing.TB, tree fstest.MapFS, member string, depth plugin.Depth,
) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	f := frontend.New()
	units, err := f.Partition(tb.Context(), claimedIn(tree), treeReader{tree})
	assert.NoError(tb, err, "the fixture partitions")
	at := slices.IndexFunc(units, func(unit []plugin.SourceRef) bool { return unit[0].Path == member })
	assert.NotEqual(tb, at, -1, "a unit has the member "+member)
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(units[at], tree, depth, f.Syntax(), brand, sink, f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	return u.Graph(), slices.Collect(sink.All())
}

// parsedSource parses one module's source at src/a.ts, at full depth.
func parsedSource(tb testing.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{aFile: {Data: []byte(src)}}, aFile, plugin.DepthFull)
}

// declsOf parses one module's source and returns the declarations of
// its file package.
func declsOf(tb testing.TB, src string) node.Symbols {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	return fileIn(tb, gb, aPackage).Decls
}

// fileIn returns the File node a builder's package of a path has.
func fileIn(tb testing.TB, gb *plugin.GraphBuilder, pkg string) *node.File {
	tb.Helper()

	pkgs := gb.Packages()
	at := slices.IndexFunc(pkgs, func(p *node.Package) bool { return p.ID.Package == pkg })
	assert.NotEqual(tb, at, -1, "the unit declares the package "+pkg)
	assert.Length(tb, pkgs[at].Files, 1, "the package has the file's one File node")
	return pkgs[at].Files[0]
}

// named returns the declaration of a name among declarations, of the
// type the case expects.
func named[T symbol.Symbol](tb testing.TB, decls node.Symbols, name string) T {
	tb.Helper()

	at := slices.IndexFunc(decls, func(d symbol.Symbol) bool {
		_, is := d.(T)
		return is && nameOf(d) == name
	})
	var zero T
	assert.NotEqual(tb, at, -1, fmt.Sprintf("a %T is named %s", zero, name))
	return decls[at].(T)
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
	case *node.Alias:
		return d.Name
	case *node.Function:
		return d.Name
	case *node.Constant:
		return d.Name
	case *node.Variable:
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

// stampOn returns the value a builder stamped on a subject under a key,
// and reports whether it stamped one.
func stampOn(gb *plugin.GraphBuilder, subject symbol.Symbol, key meta.KeyName) (any, bool) {
	for _, r := range gb.StampRecords() {
		if r.Subject == subject && r.Stamp.Key == key {
			return r.Stamp.Value, true
		}
	}
	return nil, false
}
