// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The routed files and the residents that the cases place.
const (
	modelsPath = "src/shop/models.py"
	initPath   = "src/shop/__init__.py"
	ordersPath = "src/shop/orders.py"
	modelsName = "models"
	shopModels = "shop/models"
)

// moduleAllocs is the module of a file beside a resident: the package of
// the directory joined with the stem of the file.
const moduleAllocs = 1

// The packages of the residents of the directory src/shop.
var (
	shopPackage   = symbol.Identity{Lang: python.Lang, Package: "shop", Kind: symbol.KindPackage}
	ordersPackage = symbol.Identity{Lang: python.Lang, Package: "shop/orders", Kind: symbol.KindPackage}
)

// A Python file is a module of its own. The package of a routed file is
// its module, which the residents of its directory, the import base of the
// plan or its path decide.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			give     plugin.Placement
			wantPath string
			wantName string
		}{
			{
				name: "returns the package of the __init__.py of the directory joined with the stem",
				give: plugin.Placement{
					Path: modelsPath, Residents: []plugin.Resident{{File: initPath, Pkg: shopPackage}},
				},
				wantPath: shopModels, wantName: modelsName,
			},
			{
				name: "returns the parent of the module of a resident joined with the stem",
				give: plugin.Placement{
					Path: modelsPath, Residents: []plugin.Resident{{File: ordersPath, Pkg: ordersPackage}},
				},
				wantPath: shopModels, wantName: modelsName,
			},
			{
				name: "returns the package of the __init__.py before the parent of another module",
				give: plugin.Placement{
					Path: modelsPath,
					Residents: []plugin.Resident{
						{File: "src/shop/Base.py", Pkg: symbol.Identity{Lang: python.Lang, Package: "legacy/Base"}},
						{File: initPath, Pkg: shopPackage},
					},
				},
				wantPath: shopModels, wantName: modelsName,
			},
			{
				name: "returns the stem beside a module at the root of the imports",
				give: plugin.Placement{
					Path: "src/models.py",
					Residents: []plugin.Resident{
						{File: "src/app.py", Pkg: symbol.Identity{Lang: python.Lang, Package: "app"}},
					},
				},
				wantPath: modelsName, wantName: modelsName,
			},
			{
				name: "returns the package of the directory for its __init__.py",
				give: plugin.Placement{
					Path: initPath, Residents: []plugin.Resident{{File: ordersPath, Pkg: ordersPackage}},
				},
				wantPath: "shop", wantName: "shop",
			},
			{
				name: "returns the module of the stem of a stub file",
				give: plugin.Placement{
					Path: "src/shop/models.pyi", Residents: []plugin.Resident{{File: ordersPath, Pkg: ordersPackage}},
				},
				wantPath: shopModels, wantName: modelsName,
			},
			{
				name: "returns the path of a file beside a resident of another language",
				give: plugin.Placement{
					Path: modelsPath,
					Residents: []plugin.Resident{{
						File: "src/shop/main.go", Pkg: symbol.Identity{Lang: "golang", Package: "example.com/shop"},
					}},
				},
				wantPath: "src/shop/models", wantName: modelsName,
			},
			{
				name:     "returns the import base joined with the path below the output directory",
				give:     plugin.Placement{Path: "out/py/shop/models.py", ImportBase: "acme/gen", BaseDir: "out/py"},
				wantPath: "acme/gen/shop/models", wantName: modelsName,
			},
			{
				name:     "returns the path below the output directory under the import base of the root",
				give:     plugin.Placement{Path: "out/py/shop/models.py", ImportBase: ".", BaseDir: "out/py"},
				wantPath: shopModels, wantName: modelsName,
			},
			{
				name:     "returns the package of the import base for its __init__.py",
				give:     plugin.Placement{Path: "out/py/shop/__init__.py", ImportBase: ".", BaseDir: "out/py"},
				wantPath: "shop", wantName: "shop",
			},
			{
				name:     "returns the import base joined with the path without an output directory",
				give:     plugin.Placement{Path: "shop/models.py", ImportBase: "acme"},
				wantPath: "acme/shop/models", wantName: modelsName,
			},
			{
				name:     "returns the path of a file outside the output directory",
				give:     plugin.Placement{Path: "lib/models.py", ImportBase: "acme", BaseDir: "out/py"},
				wantPath: "lib/models", wantName: modelsName,
			},
			{
				name: "returns the path of a file without residents",
				give: plugin.Placement{Path: "shop/models.py"}, wantPath: shopModels, wantName: modelsName,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(tt.give)
				assert.NoError(t, err, "the module derives")
				assert.Equal(t, got, symbol.Identity{
					Lang: python.Lang, Package: tt.wantPath, Name: tt.wantName, Kind: symbol.KindPackage,
				}, "the module of the file")
			})
		}

		refusals := []struct {
			name string
			give string
			want string
		}{
			{
				name: "returns an error for a directory that is not an identifier",
				give: "my-app/models.py", want: "my-app",
			},
			{
				name: "returns an error for a directory named after a keyword",
				give: "class/models.py", want: "class",
			},
			{
				name: "returns an error for a stem that starts with a digit",
				give: "shop/2fa.py", want: "2fa",
			},
			{
				name: "returns an error for the __init__.py of the root of the imports",
				give: "__init__.py", want: "__init__.py",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Package(plugin.Placement{Path: tt.give})
				assert.HasError(t, err, "no import names the module")
				assert.Contains(t, err.Error(), tt.want, "the error names the part")
			})
		}
	})
}

// Package allocates the module path that it joins, and nothing for the
// __init__.py of a directory whose package a resident names. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestPackageAllocs(t *testing.T) {
	residents := []plugin.Resident{{File: initPath, Pkg: shopPackage}}
	for _, tt := range []struct {
		path   string
		allocs uint64
		want   string
	}{
		{path: modelsPath, allocs: moduleAllocs, want: shopModels},
		{path: initPath, want: "shop"},
	} {
		placement := plugin.Placement{Path: tt.path, Residents: residents}
		var (
			got symbol.Identity
			err error
		)
		assert.MaxAllocs(t, func() { got, err = spell.Package(placement) }, tt.allocs,
			"Package of "+tt.path+" allocates within its ceiling")
		assert.NoError(t, err, "Package derives the module of "+tt.path)
		assert.Equal(t, got.Package, tt.want, "Package returns the module of "+tt.path)
	}
}

// BenchmarkPackage measures the module that a backend names once per routed
// file.
func BenchmarkPackage(b *testing.B) {
	b.Run("Package", func(b *testing.B) {
		placement := plugin.Placement{
			Path:      modelsPath,
			Residents: []plugin.Resident{{File: initPath, Pkg: shopPackage}},
		}
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
		assert.Equal(b, got.Package, shopModels, "Package returns the module of the file")
	})
}
