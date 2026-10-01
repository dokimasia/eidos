// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"

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

// The source tree a plan's files land in is read once per run: which
// package each directory's files declare, and which modules the load
// resolved.
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
				name:    "returns no module for a package with a module path and no root",
				moduled: map[string][2]string{storePkg: {acmeModule, ""}},
			},
			{
				name:    "returns no module for a package with a root and no module path",
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
					assert.Empty(t, got, "no module")
					return
				}
				assert.Equal(t, got, tt.want, "the modules")
			})
		}
	})
}
