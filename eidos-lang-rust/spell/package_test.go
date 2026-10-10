// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// stubPath is the routed file every case names the module of.
const stubPath = "acme/src/store_stub.rs"

// moduleAllocs is a file's module: the directory's module joined with
// the file's stem.
const moduleAllocs = 1

// A Rust file is a module of the module its directory belongs to. A
// routed file's package joins that module with the file's stem.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    []plugin.Resident
			modules []plugin.Module
			want    string
		}{
			{
				name:    "returns the crate root's module joined with the stem",
				give:    []plugin.Resident{{File: "acme/src/lib.rs", Pkg: module("acme")}},
				modules: []plugin.Module{crate("acme", "acme")},
				want:    "acme/store_stub",
			},
			{
				name:    "returns the module of a crate of its own joined with the stem",
				give:    []plugin.Resident{{File: "acme/src/tool.rs", Pkg: module("acme/src/tool")}},
				modules: []plugin.Module{crate("acme/src/tool", "acme/src")},
				want:    "acme/src/tool/store_stub",
			},
			{
				name: "returns a mod.rs's module joined with the stem",
				give: []plugin.Resident{{File: "acme/src/mod.rs", Pkg: module("acme/svc")}},
				want: "acme/svc/store_stub",
			},
			{
				name: "returns a file module's parent joined with the stem",
				give: []plugin.Resident{{File: "acme/src/store.rs", Pkg: module("acme/svc/store")}},
				want: "acme/svc/store_stub",
			},
			{
				name:    "returns a file module's parent where another language's module has its path",
				give:    []plugin.Resident{{File: "acme/src/store.rs", Pkg: module("acme/svc/store")}},
				modules: []plugin.Module{{Lang: "golang", Path: "acme/svc/store", Root: "acme"}},
				want:    "acme/svc/store_stub",
			},
			{
				name: "returns the directory's own module before a file module's parent",
				give: []plugin.Resident{
					{File: "acme/src/row.rs", Pkg: module("acme/svc/row")},
					{File: "acme/src/mod.rs", Pkg: module("acme/svc")},
				},
				want: "acme/svc/store_stub",
			},
			{
				name: "returns the first file module's parent for a directory without a module file",
				give: []plugin.Resident{
					{File: "acme/src/row.rs", Pkg: module("acme/svc/row")},
					{File: "acme/src/store.rs", Pkg: module("acme/svc/store")},
				},
				want: "acme/svc/store_stub",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(plugin.Placement{Path: stubPath, Residents: tt.give, Modules: tt.modules})
				assert.NoError(t, err, "the module derives")
				assert.Equal(t, got, stubModule(tt.want), "the file's module")
			})
		}

		refusals := []struct {
			name string
			give []plugin.Resident
		}{
			{name: "returns an error for a directory without a Rust file"},
			{
				name: "returns an error for a directory of another language's files",
				give: []plugin.Resident{{
					File: "acme/src/store.go",
					Pkg:  symbol.Identity{Lang: "golang", Package: "example.com/acme", Kind: symbol.KindPackage},
				}},
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Package(plugin.Placement{Path: stubPath, Residents: tt.give})
				assert.HasError(t, err, "no module derives")
			})
		}
	})
}

// Package allocates the module path it joins in the ordinary run, which
// runs no benchmark.
func TestPackageAllocs(t *testing.T) {
	placement := modPlacement()
	var (
		got symbol.Identity
		err error
	)
	assert.MaxAllocs(t, func() { got, err = spell.Package(placement) }, moduleAllocs,
		"Package allocates the joined module path")
	assert.NoError(t, err, "Package derives the module")
	assert.Equal(t, got, stubModule("acme/svc/store_stub"), "Package returns the file's module")
}

// BenchmarkPackage measures the module a backend names once per routed
// file.
func BenchmarkPackage(b *testing.B) {
	b.Run("Package", func(b *testing.B) {
		placement := modPlacement()
		c := bench.Start(b).MaxAllocs(moduleAllocs)
		defer c.End()
		var (
			got symbol.Identity
			err error
		)
		for c.Loop() {
			got, err = spell.Package(placement)
		}
		assert.NoError(b, err, "Package derives the module")
		assert.Equal(b, got, stubModule("acme/svc/store_stub"), "Package returns the file's module")
	})
}

// module returns the package of a loaded Rust module.
func module(path string) symbol.Identity {
	return symbol.Identity{Lang: rust.Lang, Package: path, Kind: symbol.KindPackage}
}

// crate returns a Rust crate the load resolved, declared in a directory.
func crate(name, dir string) plugin.Module {
	return plugin.Module{Lang: rust.Lang, Path: name, Root: dir}
}

// stubModule returns the module of the routed file under a path.
func stubModule(path string) symbol.Identity {
	return symbol.Identity{Lang: rust.Lang, Package: path, Name: "store_stub", Kind: symbol.KindPackage}
}

// modPlacement returns the routed file in a directory whose mod.rs
// declares acme/svc.
func modPlacement() plugin.Placement {
	return plugin.Placement{
		Path:      stubPath,
		Residents: []plugin.Resident{{File: "acme/src/mod.rs", Pkg: module("acme/svc")}},
	}
}
