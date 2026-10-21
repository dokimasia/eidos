// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	tsgrammar "go.dokimi.dev/eidos/lang/treesitter/typescript"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand every fixture unit reads its carriers under.
const brand = string(frontendtest.Brand)

// The cases parse a module file under the package that its path names,
// the declaration file of that package, and a TSX file, beside the root
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

// The root package.json of the module cases, the name of its package, and
// the first comment that a generator writes.
const (
	packageFile     = "package.json"
	shopModule      = "@acme/shop"
	generatedHeader = "// " + typescript.GeneratedMark + " by protoc-gen-ts\n"
)

// descriptorName is an interface that ES5 declares without declare, so
// only the file of the library makes it ambient.
const descriptorName = "PropertyDescriptor"

// The canonical scale every layer benches at: 1000 packages of 10 files
// of 20 declarations, 200k declarations in all. A TypeScript unit is one
// file, so the corpus is 10,000 units.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// parseAllocs is the budget of one parse of the canonical corpus of
// 200,000 declarations in 10,000 files. One run counted 1,710,009
// allocations: the nodes, the texts, the stamps and the type references of
// the lowering, the source and the sink of each unit, and one tree handle
// for each file. Tree-sitter builds its trees in C memory, which the count
// does not see. The budget is about 5% above the count.
const parseAllocs = 1_800_000

// treeReader is the partition's recorded door over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r treeReader) Read(path string) ([]byte, error) { return r.tree.ReadFile(path) }

// endingContext is a context that ends after its first check. Err returns
// nil once and [context.Canceled] after that, so of two parses that check it
// the first proceeds and the second ends. It has no deadline, no done
// channel and no values.
type endingContext struct {
	checks atomic.Int32
}

// Deadline reports that the context has no deadline.
func (*endingContext) Deadline() (time.Time, bool) { return time.Time{}, false }

// Done returns nil, the done channel of a context that no one cancels.
func (*endingContext) Done() <-chan struct{} { return nil }

// Err returns nil at the first call and [context.Canceled] at every later
// call.
func (c *endingContext) Err() error {
	if c.checks.Add(1) == 1 {
		return nil
	}
	return context.Canceled
}

// Value returns nil for every key.
func (*endingContext) Value(any) any { return nil }

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

		forms := []struct {
			name       string
			giveFile   string
			giveSource string
		}{
			{
				name:     "parses an array of an import type without a syntax error",
				giveFile: aFile, giveSource: "export type Nodes = import('./lib').Node[];\n",
			},
			{
				name:     "parses typeof import() as a type argument of a call without a syntax error",
				giveFile: aFile, giveSource: "export const m = load<typeof import('./lib')>();\n",
			},
			{
				name:     "parses a bare & in JSX text without a syntax error",
				giveFile: tsxFile, giveSource: "export const View = () => <p>a & b</p>;\n",
			},
			{
				name:     "parses abstract as the name of a property without a syntax error",
				giveFile: aFile, giveSource: "export interface I {\n  abstract: string;\n}\n",
			},
			{
				name:     "parses a keyword as the label of a tuple element without a syntax error",
				giveFile: aFile, giveSource: "export type T = [symbol: string];\n",
			},
			{
				name:     "parses a call signature on the line after a type without a syntax error",
				giveFile: aFile, giveSource: "export interface S {\n  next: S\n  <T>(value: T): T\n}\n",
			},
		}
		for _, tt := range forms {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, found := parsedTree(t, fstest.MapFS{tt.giveFile: {Data: []byte(tt.giveSource)}}, tt.giveFile,
					plugin.DepthFull)
				assert.Empty(t, found, "the grammar parses the form")
			})
		}

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

		t.Run("stamps the name of the nearest package.json as the module of every package", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{
				packageFile: {Data: []byte(`{"name": "` + shopModule + `"}`)},
				aFile:       {Data: []byte(exportClass)},
			}, aFile, plugin.DepthFull)
			assert.Empty(t, found, "the manifest parses")
			pkg := packageIn(t, gb, aPackage)
			module, stamped := stampOn(gb, pkg, meta.ModuleKey)
			assert.True(t, stamped, "the package has a module")
			expect.Equal(t, module, any(shopModule), "the module is the name of the manifest")
			root, _ := stampOn(gb, pkg, meta.ModuleRootKey)
			expect.Equal(t, root, any("."), "the root is the directory of the manifest")
		})

		t.Run("stamps no module for a package.json without a name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{
				packageFile: {Data: []byte("{}")},
				aFile:       {Data: []byte(exportClass)},
			}, aFile, plugin.DepthFull)
			_, stamped := stampOn(gb, packageIn(t, gb, aPackage), meta.ModuleKey)
			assert.False(t, stamped, "a package without a name is in no module")
		})

		t.Run("reports BadPackage for a package.json that does not parse", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, fstest.MapFS{
				packageFile: {Data: []byte("{")},
				aFile:       {Data: []byte(exportClass)},
			}, aFile, plugin.DepthFull)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadPackage}, "the fault of the manifest reports")
		})

		t.Run("stamps typescript.generated on a file whose first comment has the mark", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, generatedHeader+exportClass)
			marker, stamped := stampOn(gb, fileIn(t, gb, aPackage), typescript.GeneratedKey)
			assert.True(t, stamped, "the file has the mark of a generator")
			assert.Equal(t, marker, any(strings.TrimSuffix(generatedHeader, "\n")),
				"the stamp is the comment as written")
		})

		t.Run("stamps nothing on a file whose first comment lacks the mark", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "// The shop module.\n"+generatedHeader+exportClass)
			_, stamped := stampOn(gb, fileIn(t, gb, aPackage), typescript.GeneratedKey)
			assert.False(t, stamped, "the frontend reads the mark in the first comment alone")
		})

		t.Run("stamps nothing on a file whose first comment follows a statement", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, exportClass+generatedHeader)
			_, stamped := stampOn(gb, fileIn(t, gb, aPackage), typescript.GeneratedKey)
			assert.False(t, stamped, "the mark of a generator opens a file")
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

		t.Run("declares the scripts of the members of a library into the package of the library", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedLibrary(t, withLibrary(fstest.MapFS{}), defaultES5, defaultDOM)
			assert.Empty(t, found, "the library reports nothing")
			pkg := packageIn(t, gb, defaultPackage)
			assert.Length(t, pkg.Files, 2, "each member contributes a File node")
			assert.Equal(t, pkg.Name, "", "the package of a library has no name, as the global package")
			assert.Length(t, packageIn(t, gb, defaultPackage+"/Intl").Files, 1, "a namespace is below the library")
		})

		t.Run("merges an interface that two members of a library declare", func(t *testing.T) {
			t.Parallel()

			library := fstest.MapFS{
				defaultRoot + "/" + es5Root: {Data: []byte("interface Error {\n  message: string;\n}\n")},
				defaultRoot + "/" + domRoot: {Data: []byte("interface Error {\n  stack?: string;\n}\n")},
			}
			tree := storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: library}}
			gb, _ := parsedLibrary(t, tree, defaultES5, defaultDOM)
			pkg := packageIn(t, gb, defaultPackage)
			assert.Length(t, pkg.Files[0].Decls, 1, "the first member declares Error")
			assert.Empty(t, pkg.Files[1].Decls, "the second member merges into it")
			assert.Length(t, pkg.Files[0].Decls[0].(*node.Interface).Fields, 2, "Error has the fields of both")
		})

		t.Run("declares the global block of a module of a library into the package of the library", func(t *testing.T) {
			t.Parallel()

			module := plugin.StorePath(typescript.LibraryStore, defaultRoot+"/lib.es2025.iterator.d.ts")
			library := fstest.MapFS{
				defaultRoot + "/lib.es2025.iterator.d.ts": {Data: []byte(
					"export {};\ndeclare class Hidden {}\ndeclare global {\n  interface IteratorObject {}\n}\n",
				)},
			}
			tree := storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: library}}
			gb, _ := parsedLibrary(t, tree, module)
			global := packageIn(t, gb, defaultPackage)
			assert.Length(t, global.Files, 1, "the global block declares into the library")
			assert.Equal(t, nameOf(global.Files[0].Decls[0]), "IteratorObject", "the interface of the global block")
		})

		t.Run("stamps typescript.ambient on a declaration of a fragment of the library", func(t *testing.T) {
			t.Parallel()

			fragment := plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+es5Root+"#"+descriptorName)
			gb, _ := parsedLibrary(t, storeTree{fstest.MapFS{}, realStores(t)}, fragment)
			iface := named[*node.Interface](t, packageIn(t, gb, defaultPackage).Files[0].Decls, descriptorName)
			ambient, stamped := stampOn(gb, iface, typescript.AmbientKey)
			assert.True(t, stamped, "an interface of the library without declare is implemented elsewhere")
			assert.Equal(t, ambient, any(true), "the stamp is set")
		})

		t.Run("does not report a syntax error of a member of a library", func(t *testing.T) {
			t.Parallel()

			library := fstest.MapFS{defaultRoot + "/" + es5Root: {Data: []byte("interface Error {\n  message: ;\n}\n")}}
			tree := storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: library}}
			_, found := parsedLibrary(t, tree, defaultES5)
			assert.Empty(t, found, "the author of the workspace cannot change the library")
		})

		t.Run("returns the read's error for a member of a library outside the store", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: defaultES5}}, storeTree{fstest.MapFS{}, nil},
				plugin.DepthSignatures, f.Syntax(), brand, diag.NewSink(), f.Name())
			err := f.Parse(t.Context(), u)
			assert.ErrorIs(t, err, plugin.ErrStoreAbsent, "a member that does not read fails the unit")
		})

		t.Run("returns the context's error for a library under a done context", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: defaultES5}, {Path: defaultDOM}},
				withLibrary(fstest.MapFS{}), plugin.DepthSignatures, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return f.Parse(ctx, u) },
				"a cancelled load parses no library")
		})

		t.Run("returns the context's error for a library whose context ends during the parse", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: defaultInput}, {Path: defaultES5}},
				withLibrary(fstest.MapFS{}), plugin.DepthSignatures, f.Syntax(), brand, diag.NewSink(), f.Name())
			err := f.Parse(&endingContext{}, u)
			assert.ErrorIs(t, err, context.Canceled, "the parse of the second member ends with the context")
		})

		t.Run("parses every file of the embedded library without a syntax error", func(t *testing.T) {
			t.Parallel()

			entries, err := fs.ReadDir(library.Files, library.Dir)
			assert.NoError(t, err, "the embed lists its files")
			var got []string
			for _, e := range entries {
				src, err := fs.ReadFile(library.Files, path.Join(library.Dir, e.Name()))
				assert.NoError(t, err, "the embed reads "+e.Name())
				tree, err := tsgrammar.TypeScript.Parse(t.Context(), e.Name(), src)
				assert.NoError(t, err, "the grammar parses "+e.Name())
				if problems := tree.Problems(); problems != nil {
					for at := range problems {
						got = append(got, at.String())
					}
				}
				tree.Close()
			}
			assert.Empty(t, got, "the grammar parses each construct of the library")
		})

		t.Run("lowers the units of the default library as the whole library lowers", func(t *testing.T) {
			t.Parallel()

			golden, err := os.ReadFile(filepath.Join("testdata", defaultOrderFile))
			assert.NoError(t, err, "the golden order reads")
			tree := storeTree{fstest.MapFS{}, realStores(t)}
			order := strings.Fields(string(golden))
			whole := make([]string, 0, len(order))
			names := map[string]bool{}
			for _, file := range order {
				whole = append(whole, plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+file))
				for _, s := range library.Index[file].Statements {
					names[s.Name] = true
				}
			}
			round := &plugin.DependencyRound{Number: 1}
			for _, name := range slices.Sorted(maps.Keys(names)) {
				round.Uses = append(round.Uses, id(defaultPackage, name))
			}
			units, err := runRound(t, roundReader{tree}, round)
			assert.NoError(t, err, "the round places the units of the names")
			got := map[string][]string{}
			for _, members := range membersOf(units) {
				gb, _ := parsedLibrary(t, tree, members...)
				declsWithoutPositions(t, gb, got)
			}
			gb, _ := parsedLibrary(t, tree, whole...)
			want := map[string][]string{}
			declsWithoutPositions(t, gb, want)
			for _, decls := range got {
				slices.Sort(decls)
			}
			for _, decls := range want {
				slices.Sort(decls)
			}
			assert.Equal(t, got, want, "the units declare what the whole library declares, in each package")
		})
	})
}

// positionField matches the position of a symbol in the JSON of the node
// model.
var positionField = regexp.MustCompile(`"pos":\{[^{}]*\}`)

// declsWithoutPositions adds the JSON of each declaration of a builder to
// the list of its package in decls, without the position of any symbol.
func declsWithoutPositions(tb testing.TB, gb *plugin.GraphBuilder, decls map[string][]string) {
	tb.Helper()

	for _, p := range gb.Packages() {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				b, err := node.EncodeJSON(d)
				assert.NoError(tb, err, "the declaration encodes")
				decls[p.ID.Package] = append(decls[p.ID.Package], string(positionField.ReplaceAll(b, nil)))
			}
		}
	}
}

// parsedLibrary parses a unit of the members of a library at signature
// depth, the depth of a dependency unit, over a tree and its stores. It
// returns the builder of the unit and the findings of the parse.
func parsedLibrary(tb testing.TB, tree storeTree, members ...string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	refs := make([]plugin.SourceRef, 0, len(members))
	for _, m := range members {
		refs = append(refs, plugin.SourceRef{Path: m})
	}
	f := frontend.New()
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(refs, tree, plugin.DepthSignatures, f.Syntax(), brand, sink, f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the library parses")
	return u.Graph(), slices.Collect(sink.All())
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

// packageIn returns the package of a path that a builder declares.
func packageIn(tb testing.TB, gb *plugin.GraphBuilder, pkg string) *node.Package {
	tb.Helper()

	pkgs := gb.Packages()
	at := slices.IndexFunc(pkgs, func(p *node.Package) bool { return p.ID.Package == pkg })
	assert.NotEqual(tb, at, -1, "the unit declares the package "+pkg)
	return pkgs[at]
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
