// Copyright Dokimasia B.V. 2026
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

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand every fixture unit reads its carriers under.
const brand = string(frontendtest.Brand)

// The fixture crate's manifest names the package with a hyphen. Beside
// it are the crate name Cargo spells from it, the library root, and the
// directory a nested copy of the crate is in.
const (
	manifestPath = "Cargo.toml"
	manifestSrc  = "[package]\nname = \"demo-crate\"\n"
	crateName    = "demo_crate"
	libRoot      = "src/lib.rs"
	publicStruct = "pub struct A;\n"
	nestedDir    = "crates/demo"
)

// The canonical scale every layer benches at: 1000 packages of 10 files
// of 20 declarations, 200k declarations in all. A Rust package is a
// crate, whose library root declares its ten modules.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// parseAllocs is one parse of the canonical corpus. With the collector
// off a parse allocates 2,381,003 times: the lowering's nodes, texts,
// stamps and type references, the manifests' reads, and one tree handle
// per file. Tree-sitter builds its trees in C memory, which the count
// does not see. The collections that run during a parse add more: 10
// fresh processes counted up to 10 more. The ceiling allows 32 more.
const parseAllocs = 2_381_003 + 32

// treeReader is the partition's recorded door over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r treeReader) Read(path string) ([]byte, error) { return r.tree.ReadFile(path) }

// A parse walks one crate target's module tree into the unit's packages,
// so its contract over the tree, the manifest and the options is pinned.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a crate root's items into the package its crate name names", func(t *testing.T) {
			t.Parallel()

			named[*node.Struct](t, declsOf(t, publicStruct), "A")
		})

		t.Run("walks a file module a mod item names into the package below its parent", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{
				libRoot: "pub mod store;\n", "src/store.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "the module tree is linked")
			named[*node.Struct](t, fileIn(t, gb, crateName+"/store").Decls, "A")
		})

		t.Run("walks a directory module's mod.rs", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot: "pub mod store;\n", "src/store/mod.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			named[*node.Struct](t, fileIn(t, gb, crateName+"/store").Decls, "A")
		})

		t.Run("walks a file module of a non-root file in the directory named after it", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{
				libRoot: "pub mod store;\n", "src/store.rs": "pub mod table;\n", "src/store/table.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "the module tree is linked")
			named[*node.Struct](t, fileIn(t, gb, crateName+"/store/table").Decls, "A")
		})

		t.Run("walks the file a path attribute names relative to the declaring file", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{
				libRoot: "#[path = \"other.rs\"]\npub mod x;\n", "src/other.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "the module tree is linked")
			named[*node.Struct](t, fileIn(t, gb, crateName+"/x").Decls, "A")
		})

		t.Run("walks a path attribute inside an inline module relative to its directory", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{
				libRoot: "pub mod a {\n    #[path = \"c.rs\"]\n    pub mod b;\n}\n", "src/a/c.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "the module tree is linked")
			named[*node.Struct](t, fileIn(t, gb, crateName+"/a/b").Decls, "A")
		})

		t.Run("walks a file once when two mod items name it", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot:    "#[path = \"a.rs\"]\npub mod a;\n#[path = \"a.rs\"]\npub mod b;\n",
				"src/a.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Length(t, fileIn(t, gb, crateName+"/a").Decls, 1, "the first mod item walks the file")
			assert.Empty(t, packageIn(t, gb, crateName+"/b").Files, "the second walks nothing")
		})

		t.Run("reports UnlinkedFile for a member no mod item names", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{
				libRoot: publicStruct, "src/orphan.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnlinkedFile}, "the orphan reports")
			assert.Equal(t, found[0].Severity, diag.SeverityInfo, "as information")
			named[*node.Struct](t, fileIn(t, gb, crateName+"/orphan").Decls, "A")
		})

		t.Run("loads an unlinked mod.rs beside the crate root as the crate root's module", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{libRoot: "", "src/mod.rs": publicStruct}),
				libRoot, plugin.DepthFull, nil)
			assert.Length(t, packageIn(t, gb, crateName).Files, 2, "the root's directory is the crate root's module")
		})

		t.Run("loads an unlinked mod.rs under its directory's module path", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot: publicStruct, "src/x/mod.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			named[*node.Struct](t, fileIn(t, gb, crateName+"/x").Decls, "A")
		})

		t.Run("reports ExcludedFile for every file of a module a cfg predicate keeps out",
			func(t *testing.T) {
				t.Parallel()

				gb, found := parsedTree(t, crateTree(map[string]string{
					libRoot:              "#[cfg(feature = \"x\")]\nmod gated;\n",
					"src/gated.rs":       "mod inner;\n",
					"src/gated/inner.rs": publicStruct,
				}), libRoot, plugin.DepthFull, nil)
				assert.Equal(t, codesOf(found), []diag.Code{frontend.ExcludedFile, frontend.ExcludedFile},
					"each member of the module reports")
				assert.Equal(t, found[0].Pos.File, "src/gated.rs", "the module's own file first")
				assert.Equal(t, found[0].Severity, diag.SeverityInfo, "as information")
				assert.Equal(t, packagesOf(gb), []string{crateName}, "and no member of it loads")
			})

		t.Run("reports ExcludedFile for the members of an inline module a cfg predicate keeps out",
			func(t *testing.T) {
				t.Parallel()

				gb, found := parsedTree(t, crateTree(map[string]string{
					libRoot:              "#[cfg(feature = \"x\")]\nmod gated {\n    mod inner;\n}\n",
					"src/gated/inner.rs": publicStruct,
				}), libRoot, plugin.DepthFull, nil)
				assert.Equal(t, codesOf(found), []diag.Code{frontend.ExcludedFile}, "the member below it reports")
				assert.Equal(t, packagesOf(gb), []string{crateName}, "and does not load")
			})

		t.Run("leaves out a module whose file is not a member", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{libRoot: "pub mod absent;\n"}),
				libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "a module without a file reports nothing")
			assert.Empty(t, packageIn(t, gb, crateName+"/absent").Files, "and contributes no File node")
		})

		t.Run("lowers no item of a file whose inner cfg predicate is false", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{
				libRoot:              "mod gated;\n",
				"src/gated.rs":       "#![cfg(feature = \"x\")]\nmod inner;\npub struct G;\n",
				"src/gated/inner.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.ExcludedFile}, "the member below the file reports")
			assert.Empty(t, fileIn(t, gb, crateName+"/gated").Decls, "the file declares nothing")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"feature = \"x\""}},
				"and its File node records the predicate")
		})

		t.Run("reports ExcludedFile for a member below a crate root whose inner cfg predicate is false",
			func(t *testing.T) {
				t.Parallel()

				gb, found := parsedTree(t, crateTree(map[string]string{
					libRoot: "#![cfg(feature = \"x\")]\n", "src/orphan.rs": publicStruct,
				}), libRoot, plugin.DepthFull, nil)
				assert.Equal(t, codesOf(found), []diag.Code{frontend.ExcludedFile}, "the orphan reports")
				assert.Equal(t, packagesOf(gb), []string{crateName}, "and does not load")
			})

		t.Run("reports ExcludedFile for every member of a crate rooted at the workspace root whose inner cfg "+
			"predicate is false", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{
				manifestPath:    {Data: []byte(manifestSrc + "[lib]\npath = \"lib.rs\"\n")},
				"lib.rs":        {Data: []byte("#![cfg(feature = \"x\")]\n")},
				"src/orphan.rs": {Data: []byte(publicStruct)},
			}, "lib.rs", plugin.DepthFull, nil)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.ExcludedFile},
				"the root's directory is the workspace root, above every member")
			assert.Equal(t, packagesOf(gb), []string{crateName}, "and no member loads")
		})

		t.Run("stamps every file of an integration test rust.test", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{"tests/it.rs": publicStruct}),
				"tests/it.rs", plugin.DepthFull, nil)
			assert.Contains(t, stampKeys(gb, fileIn(t, gb, "it")), rust.TestKey, "the test target's file")
		})

		t.Run("stamps every file of a shared module under the tests directory rust.test", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{"tests/common/mod.rs": publicStruct}),
				"tests/common/mod.rs", plugin.DepthFull, nil)
			file := fileIn(t, gb, crateName+"/tests/common")
			assert.Contains(t, stampKeys(gb, file), rust.TestKey, "the shared module's file")
		})

		t.Run("stamps no file of the library rust.test", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, publicStruct)
			assert.Empty(t, stampsOf(gb, string(rust.TestKey)), "a library file is not a test file")
		})

		nested := fstest.MapFS{
			nestedDir + "/" + manifestPath: {Data: []byte(manifestSrc)},
			nestedDir + "/" + libRoot:      {Data: []byte("pub mod store;\n")},
			nestedDir + "/src/store.rs":    {Data: []byte(publicStruct)},
		}
		loose := fstest.MapFS{"scripts/tool.rs": {Data: []byte(publicStruct)}}
		virtual := fstest.MapFS{manifestPath: {Data: []byte("[workspace]\n")}, libRoot: {Data: []byte(publicStruct)}}
		broken := fstest.MapFS{manifestPath: {Data: []byte("[package\n")}, libRoot: {Data: []byte(publicStruct)}}
		modules := []struct {
			name string
			tree fstest.MapFS
			root string
			key  meta.KeyName
			want []any
		}{
			{
				name: "stamps every package of a crate gen.module with the crate's name",
				tree: nested, root: nestedDir + "/" + libRoot, key: meta.ModuleKey,
				want: []any{crateName, crateName},
			},
			{
				name: "stamps every package of a crate gen.moduleRoot with its manifest's directory",
				tree: nested, root: nestedDir + "/" + libRoot, key: meta.ModuleRootKey,
				want: []any{nestedDir, nestedDir},
			},
			{
				name: "stamps a crate of its own gen.module with its path",
				tree: loose, root: "scripts/tool.rs", key: meta.ModuleKey,
				want: []any{"scripts/tool"},
			},
			{
				name: "stamps a crate of its own gen.moduleRoot with its directory",
				tree: loose, root: "scripts/tool.rs", key: meta.ModuleRootKey,
				want: []any{"scripts"},
			},
			{
				name: "stamps a crate of a manifest that names no package gen.moduleRoot with its root's directory",
				tree: virtual, root: libRoot, key: meta.ModuleRootKey,
				want: []any{"src"},
			},
			{
				name: "stamps a crate of a manifest that does not parse gen.moduleRoot with its root's directory",
				tree: broken, root: libRoot, key: meta.ModuleRootKey,
				want: []any{"src"},
			},
		}
		for _, tt := range modules {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedTree(t, tt.tree, tt.root, plugin.DepthFull, nil)
				assert.Equal(t, stampsOf(gb, string(tt.key)), tt.want, "one stamp per package")
			})
		}

		t.Run("names a binary target after the package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{"src/main.rs": publicStruct}),
				"src/main.rs", plugin.DepthFull, nil)
			named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
		})

		t.Run("names a library after the name its manifest gives it", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{
				manifestPath: {Data: []byte(manifestSrc + "[lib]\nname = \"core_lib\"\n")},
				libRoot:      {Data: []byte(publicStruct)},
			}, libRoot, plugin.DepthFull, nil)
			named[*node.Struct](t, fileIn(t, gb, "core_lib").Decls, "A")
		})

		t.Run("loads a file without a manifest as a crate named for its path", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{"scripts/tool.rs": {Data: []byte(publicStruct)}},
				"scripts/tool.rs", plugin.DepthFull, nil)
			named[*node.Struct](t, fileIn(t, gb, "scripts/tool").Decls, "A")
		})

		t.Run("loads a file of a manifest that names no package as a crate named for its path", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{
				manifestPath: {Data: []byte("[workspace]\n")}, libRoot: {Data: []byte(publicStruct)},
			}, libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "a virtual manifest reports nothing")
			named[*node.Struct](t, fileIn(t, gb, "src/lib").Decls, "A")
		})

		t.Run("reports BadManifest for a manifest that does not parse", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{
				manifestPath: {Data: []byte("[package\n")}, libRoot: {Data: []byte(publicStruct)},
			}, libRoot, plugin.DepthFull, nil)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadManifest}, "the manifest's fault reports")
			assert.Equal(t, found[0].Severity, diag.SeverityWarning, "as a warning")
			named[*node.Struct](t, fileIn(t, gb, "src/lib").Decls, "A")
		})

		t.Run("reports BadManifest for a manifest that does not read", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			tree := fstest.MapFS{libRoot: {Data: []byte(publicStruct)}}
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: libRoot, Shared: []string{manifestPath}}}, tree,
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
			assert.Equal(t, codesOf(slices.Collect(sink.All())), []diag.Code{frontend.BadManifest},
				"the manifest it declares does not read")
		})

		t.Run("returns the context's error for a done context", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: libRoot}}, crateTree(map[string]string{
				libRoot: publicStruct,
			}), plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return f.Parse(ctx, u) },
				"a cancelled load parses nothing")
		})

		t.Run("returns the read's error for a crate root outside the tree", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: libRoot}}, fstest.MapFS{}, plugin.DepthFull,
				f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u), "a member that does not read fails the unit")
		})

		t.Run("returns the read's error for a module file outside the tree", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			tree := fstest.MapFS{libRoot: {Data: []byte("mod a;\nmod b;\n")}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: libRoot}, {Path: "src/a.rs"}, {Path: "src/b.rs"}},
				tree, plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u), "a module that does not read fails the unit")
		})
	})
}

// BenchmarkParse drives the parse at the canonical scale: every unit of
// the scaled corpus through Parse into a builder of its own, the
// partition run once before the loop, so the number measures the
// tree-sitter parse and the lowering alone, and fails above
// parseAllocs. Only -bench checks the ceiling. One parse takes about
// 1.7 s on four cores, and the 101 calls of an allocation check would
// take nearly three minutes.
func BenchmarkParse(b *testing.B) {
	tree := scaledRust()
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

// scaledRust returns the canonical corpus as Rust crates: a crate per
// package whose library root declares ten modules of four structs,
// whose first field is a struct the crate before it declares, and
// sixteen constants.
func scaledRust() fstest.MapFS {
	tree := fstest.MapFS{}
	for p := range benchPackages {
		prev := (p + benchPackages - 1) % benchPackages
		tree[fmt.Sprintf("p%d/Cargo.toml", p)] = &fstest.MapFile{
			Data: fmt.Appendf(nil, "[package]\nname = \"p%d\"\n", p),
		}
		var root strings.Builder
		for f := range benchFiles {
			fmt.Fprintf(&root, "pub mod f%d;\n", f)
			var b strings.Builder
			fmt.Fprintf(&b, "use p%d::f%d::T%d_0 as Prev;\n\n", prev, f, f)
			for d := range benchDecls {
				if d < 4 {
					fmt.Fprintf(&b, "pub struct T%d_%d {\n    pub f0: Prev,\n    pub f1: i32,\n}\n\n", f, d)
				} else {
					fmt.Fprintf(&b, "pub const C%d_%d: i32 = %d;\n\n", f, d, d)
				}
			}
			tree[fmt.Sprintf("p%d/src/f%d.rs", p, f)] = &fstest.MapFile{Data: []byte(b.String())}
		}
		tree[fmt.Sprintf("p%d/src/lib.rs", p)] = &fstest.MapFile{Data: []byte(root.String())}
	}
	return tree
}

// crateTree returns a tree of the fixture crate's manifest and the
// given files, each path to its source.
func crateTree(files map[string]string) fstest.MapFS {
	tree := fstest.MapFS{manifestPath: {Data: []byte(manifestSrc)}}
	for p, src := range files {
		tree[p] = &fstest.MapFile{Data: []byte(src)}
	}
	return tree
}

// claimedIn returns the Rust files of a tree as the selection claims
// them, in path order.
func claimedIn(tree fstest.MapFS) []plugin.SourceRef {
	var claimed []plugin.SourceRef
	for p := range tree {
		if strings.HasSuffix(p, rust.Extension) {
			claimed = append(claimed, plugin.SourceRef{Path: p})
		}
	}
	slices.SortFunc(claimed, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	return claimed
}

// parsedTree partitions a tree's Rust files through a frontend of the
// options and parses the unit whose first member is root, at a depth. It
// returns the unit's builder and the findings the parse reported.
func parsedTree(
	tb testing.TB, tree fstest.MapFS, root string, depth plugin.Depth, opts *frontend.Options,
) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	f := frontend.New(opts)
	units, err := f.Partition(tb.Context(), claimedIn(tree), treeReader{tree})
	assert.NoError(tb, err, "the fixture partitions")
	at := slices.IndexFunc(units, func(unit []plugin.SourceRef) bool { return unit[0].Path == root })
	assert.NotEqual(tb, at, -1, "a unit has the root "+root)
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(units[at], tree, depth, f.Syntax(), brand, sink, f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	return u.Graph(), slices.Collect(sink.All())
}

// parsedSource parses the fixture crate whose library root is src, at
// full depth.
func parsedSource(tb testing.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, crateTree(map[string]string{libRoot: src}), libRoot, plugin.DepthFull, nil)
}

// declsOf parses the fixture crate whose library root is src and
// returns the declarations of the crate root's package.
func declsOf(tb testing.TB, src string) node.Symbols {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	return fileIn(tb, gb, crateName).Decls
}

// packageIn returns a builder's package of a path.
func packageIn(tb testing.TB, gb *plugin.GraphBuilder, pkg string) *node.Package {
	tb.Helper()

	pkgs := gb.Packages()
	at := slices.IndexFunc(pkgs, func(p *node.Package) bool { return p.ID.Package == pkg })
	assert.NotEqual(tb, at, -1, "the unit declares the package "+pkg)
	return pkgs[at]
}

// fileIn returns the one File node a builder's package of a path has.
func fileIn(tb testing.TB, gb *plugin.GraphBuilder, pkg string) *node.File {
	tb.Helper()

	p := packageIn(tb, gb, pkg)
	assert.Length(tb, p.Files, 1, "the package has the file's one File node")
	return p.Files[0]
}

// packagesOf returns the paths of a builder's packages, in first-touch
// order.
func packagesOf(gb *plugin.GraphBuilder) []string {
	pkgs := gb.Packages()
	out := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, p.ID.Package)
	}
	return out
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
	case *node.Sum:
		return d.Name
	case *node.Alias:
		return d.Name
	case *node.Function:
		return d.Name
	case *node.Method:
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

// stampsOf returns the values a builder stamped under a key, in record
// order.
func stampsOf(gb *plugin.GraphBuilder, key string) []any {
	var out []any
	for _, rec := range gb.StampRecords() {
		if string(rec.Stamp.Key) == key {
			out = append(out, rec.Stamp.Value)
		}
	}
	return out
}

// stampKeys returns the keys a builder stamped on a subject, in record
// order.
func stampKeys(gb *plugin.GraphBuilder, subject symbol.Symbol) []meta.KeyName {
	var out []meta.KeyName
	for _, rec := range gb.StampRecords() {
		if rec.Subject == subject {
			out = append(out, rec.Stamp.Key)
		}
	}
	return out
}
