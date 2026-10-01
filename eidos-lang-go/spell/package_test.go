// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The fixture's modules and the package paths below them.
const (
	acmeModule = "example.com/acme"
	svcModule  = "example.com/acme/svc"
	genBase    = "example.com/gen"
	storePath  = "example.com/acme/svc/store"
	// testPath and testName are the store directory's external test
	// package, under the path the frontend loads it at.
	testPath = storePath + golang.TestSuffix
	testName = "store" + golang.TestSuffix
)

// declared returns the package a loaded directory's files declare.
func declared(importPath, name string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: importPath, Name: name, Kind: symbol.KindPackage}
}

// origin returns a unit's package, an identity without a clause name.
func origin(importPath string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: importPath, Kind: symbol.KindPackage}
}

// acme returns the module at the tree's root.
func acme() plugin.Module { return plugin.Module{Lang: golang.Lang, Path: acmeModule, Root: "."} }

// storeResidents returns the store directory's files: the package's
// source and a file of its external test package.
func storeResidents() []plugin.Resident {
	return []plugin.Resident{
		{File: "svc/store/store.go", Pkg: declared(storePath, "store")},
		{File: "svc/store/store_test.go", Pkg: declared(testPath, testName)},
	}
}

// A Go file declares its directory's package, so a routed file joins
// the package the frontend loads from that directory.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Placement
			want symbol.Identity
		}{
			{
				name: "returns the package the directory's non-test file declares",
				give: plugin.Placement{
					Path:      "svc/store/store_stub.go",
					Residents: []plugin.Resident{{File: "svc/store/store.go", Pkg: declared(storePath, "store")}},
				},
				want: declared(storePath, "store"),
			},
			{
				name: "returns the main package of a command's directory",
				give: plugin.Placement{
					Path: "cmd/tool/flags_gen.go",
					Residents: []plugin.Resident{
						{File: "cmd/tool/main.go", Pkg: declared(acmeModule+"/cmd/tool", "main")},
					},
				},
				want: declared(acmeModule+"/cmd/tool", "main"),
			},
			{
				name: "returns the external test package for a test file of its origin",
				give: plugin.Placement{
					Path: "svc/store/stub_test.go", Origin: origin(testPath), Residents: storeResidents(),
				},
				want: declared(testPath, testName),
			},
			{
				name: "returns the directory's package for a test file of the package's origin",
				give: plugin.Placement{
					Path: "svc/store/store_stub_test.go", Origin: origin(storePath), Residents: storeResidents(),
				},
				want: declared(storePath, "store"),
			},
			{
				name: "returns the directory's package for a test file whose origin no resident declares",
				give: plugin.Placement{
					Path:      "svc/store/store_stub_test.go",
					Origin:    origin(svcModule + "/user"),
					Residents: storeResidents(),
				},
				want: declared(storePath, "store"),
			},
			{
				name: "returns the directory's package for a non-test file of an external test package",
				give: plugin.Placement{
					Path: "svc/store/stub.go", Origin: origin(testPath), Residents: storeResidents(),
				},
				want: declared(storePath, "store"),
			},
			{
				name: "returns the directory's Go package for a test file of another language's origin",
				give: plugin.Placement{
					Path:   "svc/store/store_stub_test.go",
					Origin: symbol.Identity{Lang: "protobuf", Package: "acme.store", Kind: symbol.KindPackage},
					Residents: []plugin.Resident{
						{File: "svc/store/store.go", Pkg: declared(storePath, "store")},
						{
							File: "svc/store/store.proto",
							Pkg:  symbol.Identity{Lang: "protobuf", Package: "acme.store", Kind: symbol.KindPackage},
						},
					},
				},
				want: declared(storePath, "store"),
			},
			{
				name: "returns the module's import path for a directory without a Go package",
				give: plugin.Placement{
					Path: "svc/store/store_stub.go",
					Residents: []plugin.Resident{{
						File: "svc/store/store.proto",
						Pkg:  symbol.Identity{Lang: "protobuf", Package: "acme.store", Kind: symbol.KindPackage},
					}},
					Modules: []plugin.Module{acme()},
				},
				want: declared(acmeModule+"/svc/store", "store"),
			},
			{
				name: "returns the innermost module's import path",
				give: plugin.Placement{
					Path:    "svc/gen/stub.go",
					Modules: []plugin.Module{{Lang: golang.Lang, Path: svcModule, Root: "svc"}, acme()},
				},
				want: declared(svcModule+"/gen", "gen"),
			},
			{
				name: "returns the import base's path for a directory outside every Go module",
				give: plugin.Placement{
					Path:       "gen/svc/store/stub.go",
					Modules:    []plugin.Module{{Lang: "rust", Path: "crate", Root: "."}},
					ImportBase: genBase, BaseDir: "gen",
				},
				want: declared(genBase+"/svc/store", "store"),
			},
			{
				name: "returns the import base for a file in the base directory",
				give: plugin.Placement{Path: "gen/stub.go", ImportBase: genBase, BaseDir: "gen"},
				want: declared(genBase, "gen"),
			},
			{
				name: "returns the import base joined with the directory for an output directory at the tree's root",
				give: plugin.Placement{Path: "svc/stub.go", ImportBase: genBase},
				want: declared(genBase+"/svc", "svc"),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(tt.give)
				assert.NoError(t, err, "the package derives")
				assert.Equal(t, got, tt.want, "the package")
			})
		}

		refusals := []struct {
			name string
			give plugin.Placement
		}{
			{
				name: "returns an error for a directory outside every module without an import base",
				give: plugin.Placement{Path: "gen/stub.go"},
			},
			{
				name: "returns an error for a directory outside the import base's directory",
				give: plugin.Placement{Path: "other/stub.go", ImportBase: genBase, BaseDir: "gen"},
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Package(tt.give)
				assert.HasError(t, err, "no package derives")
				assert.Contains(t, err.Error(), "import base", "the error names what the plan can state")
			})
		}
	})
}
