// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The routed files of the cases, and the output directory and the import
// base of a plan.
const (
	schemaPath = "svc/store_schema.proto"
	outputDir  = "gen/proto"
	importBase = "acme/api"
)

// The allocations of a package that Package derives.
const (
	// derivedAllocs is the package of a path without an import base: the
	// package that the directories derive.
	derivedAllocs = 1
	// basedAllocs is the package of a path below the output directory of
	// an import base: the joined import path and the package that its
	// directories derive.
	basedAllocs = 2
)

// A protobuf file is a package of its own, whose path is the import path
// of the file. The clause comes from the header, from a protobuf origin,
// or from the directories of the import path, in that order. The files of
// one clause declare their names in one namespace.
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
				name: "returns the workspace path of a plan without an import base",
				give: plugin.Placement{Path: schemaPath},
				want: pkg(schemaPath, "svc"),
			},
			{
				name: "returns the import base joined with the path below the output directory",
				give: plugin.Placement{
					Path:       outputDir + "/svc/store.proto",
					ImportBase: importBase,
					BaseDir:    outputDir,
				},
				want: pkg("acme/api/svc/store.proto", "acme.api.svc"),
			},
			{
				name: "returns the path below the output directory under an import base of a dot",
				give: plugin.Placement{Path: outputDir + "/svc/store.proto", ImportBase: ".", BaseDir: outputDir},
				want: pkg("svc/store.proto", "svc"),
			},
			{
				name: "returns the import base as the package of a file in the output directory",
				give: plugin.Placement{Path: outputDir + "/store.proto", ImportBase: importBase, BaseDir: outputDir},
				want: pkg("acme/api/store.proto", "acme.api"),
			},
			{
				name: "returns the import base joined with the path below the root of the tree",
				give: plugin.Placement{Path: "svc/store.proto", ImportBase: importBase},
				want: pkg("acme/api/svc/store.proto", "acme.api.svc"),
			},
			{
				name: "returns the workspace path of a file outside the output directory",
				give: plugin.Placement{Path: "other/store.proto", ImportBase: importBase, BaseDir: outputDir},
				want: pkg("other/store.proto", "other"),
			},
			{
				name: "returns the directories of the import path in snake case",
				give: plugin.Placement{Path: "Svc/RowStore/store.proto"},
				want: pkg("Svc/RowStore/store.proto", "svc.row_store"),
			},
			{
				name: "returns no package for a file at the root of the imports",
				give: plugin.Placement{Path: "store.proto"},
				want: pkg("store.proto", ""),
			},
			{
				name: "returns no package for a file in the output directory of an import base of a dot",
				give: plugin.Placement{Path: outputDir + "/store.proto", ImportBase: ".", BaseDir: outputDir},
				want: pkg("store.proto", ""),
			},
			{
				name: "returns the package that the header marker states",
				give: plugin.Placement{Path: schemaPath, Header: symbol.Annotations{packageMarker("acme.store.v1")}},
				want: pkg(schemaPath, "acme.store.v1"),
			},
			{
				name: "returns the package that the header marker states beside the syntax marker",
				give: plugin.Placement{
					Path: schemaPath,
					Header: symbol.Annotations{
						{Name: string(protobuf.SyntaxMarker), Args: []string{protobuf.Proto3}},
						packageMarker("acme.store.v1"),
					},
				},
				want: pkg(schemaPath, "acme.store.v1"),
			},
			{
				name: "returns one package for a header that states it twice",
				give: plugin.Placement{
					Path: schemaPath, Header: symbol.Annotations{packageMarker("acme.v1"), packageMarker("acme.v1")},
				},
				want: pkg(schemaPath, "acme.v1"),
			},
			{
				name: "returns the package of a protobuf origin",
				give: plugin.Placement{Path: schemaPath, Origin: origin(protobuf.Lang, "svc.store")},
				want: pkg(schemaPath, "svc.store"),
			},
			{
				name: "returns no package for a protobuf origin without a package",
				give: plugin.Placement{Path: schemaPath, Origin: origin(protobuf.Lang, "")},
				want: pkg(schemaPath, ""),
			},
			{
				name: "returns the header package before the package of a protobuf origin",
				give: plugin.Placement{
					Path: schemaPath, Origin: origin(protobuf.Lang, "svc.store"),
					Header: symbol.Annotations{packageMarker("acme.store")},
				},
				want: pkg(schemaPath, "acme.store"),
			},
			{
				name: "returns the directories of the import path for an origin of another language",
				give: plugin.Placement{Path: schemaPath, Origin: origin(sourceLang, "example.com/svc")},
				want: pkg(schemaPath, "svc"),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(tt.give)
				assert.NoError(t, err, "the path derives a package")
				assert.Equal(t, got, tt.want, "the package of the file")
			})
		}

		refusals := []struct {
			name string
			give plugin.Placement
			want []string
		}{
			{
				name: "returns an error for a header that states two packages",
				give: plugin.Placement{
					Path: schemaPath, Header: symbol.Annotations{packageMarker("acme.v1"), packageMarker("acme.v2")},
				},
				want: []string{schemaPath, "acme.v1", "acme.v2"},
			},
			{
				name: "returns an error for a package marker without an argument",
				give: plugin.Placement{Path: schemaPath, Header: symbol.Annotations{packageMarker()}},
				want: []string{schemaPath, "0"},
			},
			{
				name: "returns an error for a package marker of two arguments",
				give: plugin.Placement{Path: schemaPath, Header: symbol.Annotations{packageMarker("acme", "v1")}},
				want: []string{schemaPath, "2"},
			},
			{
				name: "returns an error for a package that is not a dotted name",
				give: plugin.Placement{Path: schemaPath, Header: symbol.Annotations{packageMarker("acme..v1")}},
				want: []string{schemaPath, "acme..v1"},
			},
			{
				name: "returns an error for a directory that is not an identifier",
				give: plugin.Placement{Path: "2026/store.proto"},
				want: []string{"2026/store.proto", `"2026"`},
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(tt.give)
				assert.HasError(t, err, "the file derives no package")
				for _, part := range tt.want {
					assert.Contains(t, err.Error(), part, "the error names "+part)
				}
				assert.Equal(t, got, symbol.Identity{}, "a refusal returns no package")
			})
		}
	})

	t.Run("NameScope", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want symbol.Identity
		}{
			{
				name: "returns the protobuf package of a file",
				give: pkg(schemaPath, "acme.v1"),
				want: symbol.Identity{Lang: protobuf.Lang, Package: "acme.v1", Kind: symbol.KindPackage},
			},
			{
				name: "returns the protobuf package of another file of that package",
				give: pkg("svc/other.proto", "acme.v1"),
				want: symbol.Identity{Lang: protobuf.Lang, Package: "acme.v1", Kind: symbol.KindPackage},
			},
			{
				name: "returns the namespace of the files without a package",
				give: pkg("store.proto", ""),
				want: symbol.Identity{Lang: protobuf.Lang, Kind: symbol.KindPackage},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spell.NameScope(tt.give), tt.want, "the namespace of the names of the file")
			})
		}
	})
}

// Package derives the package of a path in the ordinary run, which runs no
// benchmark, within its ceilings, and NameScope allocates nothing.
func TestPackageAllocs(t *testing.T) {
	for _, tt := range packageCalls() {
		var (
			got symbol.Identity
			err error
		)
		assert.MaxAllocs(t, func() { got, err = spell.Package(tt.give) }, tt.allocs,
			"Package of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Package derives the package of "+tt.name)
		assert.Equal(t, got.Name, tt.want, "Package returns the clause of "+tt.name)
	}
	file := pkg(schemaPath, "acme.v1")
	var scope symbol.Identity
	assert.MaxAllocs(t, func() { scope = spell.NameScope(file) }, 0, "NameScope allocates nothing")
	assert.Equal(t, scope.Package, file.Name, "NameScope returns the protobuf package")
}

// BenchmarkPackage measures the package a backend names once per routed
// file.
func BenchmarkPackage(b *testing.B) {
	b.Run("Package", func(b *testing.B) {
		for _, tt := range packageCalls() {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got symbol.Identity
					err error
				)
				for c.Loop() {
					got, err = spell.Package(tt.give)
				}
				assert.NoError(b, err, "Package derives the package of "+tt.name)
				assert.Equal(b, got.Name, tt.want, "Package returns the clause of "+tt.name)
			})
		}
	})

	b.Run("NameScope", func(b *testing.B) {
		file := pkg(schemaPath, "acme.v1")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var scope symbol.Identity
		for c.Loop() {
			scope = spell.NameScope(file)
		}
		assert.Equal(b, scope.Package, file.Name, "NameScope returns the protobuf package")
	})
}

// packageCall is one placement that the allocation test and the benchmark
// measure: the placement, its ceiling and the clause that it returns.
type packageCall struct {
	name   string
	give   plugin.Placement
	allocs uint64
	want   string
}

// packageCalls returns the placements that the allocation test and the
// benchmark measure: a header package, a path without an import base and
// a path below the output directory of an import base.
func packageCalls() []packageCall {
	return []packageCall{
		{
			name: "a header package", allocs: 0, want: "acme.v1",
			give: plugin.Placement{Path: schemaPath, Header: symbol.Annotations{packageMarker("acme.v1")}},
		},
		{name: "a path", allocs: derivedAllocs, want: "svc", give: plugin.Placement{Path: schemaPath}},
		{
			name: "a path below an import base", allocs: basedAllocs, want: "acme.api.svc",
			give: plugin.Placement{Path: outputDir + "/svc/store.proto", ImportBase: importBase, BaseDir: outputDir},
		},
	}
}

// pkg returns the package identity of a routed file with an import path
// and a clause.
func pkg(importPath, clause string) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: importPath, Name: clause, Kind: symbol.KindPackage}
}

// origin returns the package of a file's first unit in a language.
func origin(lang symbol.Lang, path string) symbol.Identity {
	return symbol.Identity{Lang: lang, Package: path, Kind: symbol.KindPackage}
}

// packageMarker returns the header marker of a package with its
// arguments.
func packageMarker(args ...string) symbol.Annotation {
	return symbol.Annotation{Name: string(protobuf.PackageMarker), Args: args}
}
