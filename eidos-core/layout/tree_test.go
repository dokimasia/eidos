// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture's toolchain modules.
const (
	acmeModule  = "example.com/acme"
	cacheModule = "example.com/acme/cache"
	depModule   = "example.com/dep"
)

// The ceilings of one read of the source tree.
const (
	// residentsAllocs is one index of the fixture's four directories:
	// 2 for the map Residents returns, and the backing array.
	residentsAllocs = 3
	// canonicalResidentsAllocs is one index of the canonical corpus's
	// 1,000 directories: 6 for the map Residents returns, the backing
	// array, and 20 for the map that counts each directory's files as
	// it grows to 1,000 entries.
	canonicalResidentsAllocs = 27
	// modulesAllocs is one list of the fixture's two modules: the list
	// alone, because the set of two modules is on the stack.
	modulesAllocs = 1
)

// A plan writes its files into the source tree, and a run reads that
// tree once: which package each directory's files declare, and which
// modules the load resolved.
func TestTree(t *testing.T) {
	t.Parallel()

	t.Run("Residents", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each directory's files with the package that declares them", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, layout.Residents(graph(t)), map[string][]plugin.Resident{
				"svc/store":   {resident(rowFile, storePkg, "store"), resident(storeFile, storePkg, "store")},
				"svc/cache":   {resident(cacheFile, cachePkg, "cache")},
				"svc/multi/a": {resident(aceFile, multiPkg, "multi")},
				"svc/multi/z": {resident(zedFile, multiPkg, "multi")},
			}, "the dependency store's file is absent")
		})

		t.Run("returns two packages of one directory sorted by file", func(t *testing.T) {
			t.Parallel()

			const external = "svc/store_test"
			const externalFile = "svc/store/store_test.go"
			g := coretest.Frozen(t,
				sourcePackage(storePkg, "store", source{storeFile, storeID}),
				sourcePackage(external, "store_test",
					source{externalFile, coretest.ID(external, "StoreTest", symbol.KindStruct)}),
			)
			assert.Equal(t, layout.Residents(g)["svc/store"], []plugin.Resident{
				resident(storeFile, storePkg, "store"),
				resident(externalFile, external, "store_test"),
			}, "each file names the package it declares")
		})

		t.Run("returns each directory's slice capped at its length", func(t *testing.T) {
			t.Parallel()

			residents := layout.Residents(graph(t))
			for dir, rs := range residents {
				expect.Equal(t, cap(rs), len(rs), "an append to "+dir+" copies the slice")
			}
		})
	})

	t.Run("Modules", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			moduled map[string][2]string
			want    []plugin.Module
		}{
			{
				name: "returns the modules innermost root first",
				moduled: map[string][2]string{
					storePkg: {acmeModule, "."},
					cachePkg: {cacheModule, cachePkg},
				},
				want: []plugin.Module{
					{Lang: coretest.Lang, Path: cacheModule, Root: cachePkg},
					{Lang: coretest.Lang, Path: acmeModule, Root: "."},
				},
			},
			{
				name: "returns a module two packages declare once",
				moduled: map[string][2]string{
					storePkg: {acmeModule, "."},
					multiPkg: {acmeModule, "."},
				},
				want: []plugin.Module{{Lang: coretest.Lang, Path: acmeModule, Root: "."}},
			},
			{
				name: "returns the modules at one depth in root order",
				moduled: map[string][2]string{
					storePkg: {acmeModule, storePkg},
					cachePkg: {cacheModule, cachePkg},
				},
				want: []plugin.Module{
					{Lang: coretest.Lang, Path: cacheModule, Root: cachePkg},
					{Lang: coretest.Lang, Path: acmeModule, Root: storePkg},
				},
			},
			{
				name: "returns the modules at one root in path order",
				moduled: map[string][2]string{
					storePkg: {cacheModule, "."},
					cachePkg: {acmeModule, "."},
				},
				want: []plugin.Module{
					{Lang: coretest.Lang, Path: acmeModule, Root: "."},
					{Lang: coretest.Lang, Path: cacheModule, Root: "."},
				},
			},
			{
				name:    "returns no module for a module path without a root",
				moduled: map[string][2]string{storePkg: {acmeModule, ""}},
			},
			{
				name:    "returns no module for a root without a module path",
				moduled: map[string][2]string{storePkg: {"", "."}},
			},
			{
				name:    "returns no module rooted in a dependency store",
				moduled: map[string][2]string{depPkg: {depModule, "gomod://example.com/dep@v1.0.0"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f, k := facts(t, tt.moduled)
				got := layout.Modules(graph(t), f, k)
				if tt.want == nil {
					assert.Nil(t, got, "a tree without a module returns nil")
					return
				}
				assert.Equal(t, got, tt.want, "the modules")
			})
		}
	})
}

// The reads of the tree allocate their index and their list, over the
// fixture's tree and over the canonical corpus, in the ordinary run,
// which runs no benchmark.
func TestTreeAllocs(t *testing.T) {
	g := graph(t)
	var residents map[string][]plugin.Resident
	assert.MaxAllocs(t, func() { residents = layout.Residents(g) }, residentsAllocs,
		"Residents allocates its map and one backing array")
	assert.Length(t, residents, 4, "Residents indexes every workspace directory")
	f, k := facts(t, twoModules())
	var modules []plugin.Module
	assert.MaxAllocs(t, func() { modules = layout.Modules(g, f, k) }, modulesAllocs,
		"Modules allocates its list")
	assert.Length(t, modules, 2, "Modules lists both modules")

	corpus, _ := benchTree(t, benchPackages)
	assert.MaxAllocs(t, func() { residents = layout.Residents(corpus) }, canonicalResidentsAllocs,
		"Residents allocates its map, its backing array and the count of each directory's files")
	assert.Length(t, residents, benchPackages, "Residents indexes one directory per package")
	none, keys := facts(t, nil)
	assert.MaxAllocs(t, func() { modules = layout.Modules(corpus, none, keys) }, 0,
		"Modules allocates nothing for a tree without a module")
	assert.Empty(t, modules, "Modules lists no module")
}

// BenchmarkTree measures the reads of the source tree that a plan writes
// its files into, over the fixture's tree and over the canonical corpus.
func BenchmarkTree(b *testing.B) {
	b.Run("Residents", func(b *testing.B) {
		b.Run("the fixture's tree", func(b *testing.B) {
			g := graph(b)
			c := bench.Start(b).MaxAllocs(residentsAllocs)
			defer c.End()
			var got map[string][]plugin.Resident
			for c.Loop() {
				got = layout.Residents(g)
			}
			assert.Length(b, got, 4, "Residents indexes every workspace directory")
		})

		b.Run("the canonical corpus of 10,000 files", func(b *testing.B) {
			g, _ := benchTree(b, benchPackages)
			c := bench.Start(b).MaxAllocs(canonicalResidentsAllocs)
			defer c.End()
			var got map[string][]plugin.Resident
			for c.Loop() {
				got = layout.Residents(g)
			}
			assert.Length(b, got, benchPackages, "Residents indexes one directory per package")
		})
	})

	b.Run("Modules", func(b *testing.B) {
		b.Run("the fixture's two modules", func(b *testing.B) {
			g := graph(b)
			f, k := facts(b, twoModules())
			c := bench.Start(b).MaxAllocs(modulesAllocs)
			defer c.End()
			var got []plugin.Module
			for c.Loop() {
				got = layout.Modules(g, f, k)
			}
			assert.Length(b, got, 2, "Modules lists both modules")
		})

		b.Run("the canonical corpus without a module", func(b *testing.B) {
			g, _ := benchTree(b, benchPackages)
			f, k := facts(b, nil)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got []plugin.Module
			for c.Loop() {
				got = layout.Modules(g, f, k)
			}
			assert.Empty(b, got, "Modules lists no module")
		})
	})
}

// twoModules states the fixture's two nested modules.
func twoModules() map[string][2]string {
	return map[string][2]string{
		storePkg: {acmeModule, "."},
		cachePkg: {cacheModule, cachePkg},
	}
}

// facts returns a fact store over the kernel's keys, with each
// package's module path and root stamped where moduled names them.
func facts(tb testing.TB, moduled map[string][2]string) (*meta.Facts, meta.KernelKeys) {
	tb.Helper()

	reg := meta.NewRegistry()
	k, err := meta.Kernel(reg)
	assert.NoError(tb, err, "the kernel's keys register")
	reg.Seal()
	f := meta.NewFacts(reg)
	for pkg, module := range moduled {
		claim := meta.Claim{Subject: coretest.PackageID(pkg), Authority: meta.AuthorityPlugin, Plugin: "golang"}
		if module[0] != "" {
			assert.NoError(tb, meta.Stamp(f, k.Module, module[0], claim), "the module path stamps")
		}
		if module[1] != "" {
			assert.NoError(tb, meta.Stamp(f, k.ModuleRoot, module[1], claim), "the module root stamps")
		}
	}
	return f, k
}

// resident returns the record of one source file of a fixture package.
func resident(file, pkg, name string) plugin.Resident {
	id := coretest.PackageID(pkg)
	id.Name = name
	return plugin.Resident{File: file, Pkg: id}
}
