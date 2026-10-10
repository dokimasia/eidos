// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of a filename.
const (
	// typeFilenameAllocs is a lone type's filename: the name with its
	// extension.
	typeFilenameAllocs = 1
	// partsFilenameAllocs is a per-source unit's fallback filename: the
	// list of its parts, their join, their Pascal-case conversion, and
	// the name with its extension.
	partsFilenameAllocs = 4
)

// Java names a file after the public type it declares, so a change of
// spelling renames a type as well as a file. Every spelling of a unit a
// plan admits is pinned.
func TestFilename(t *testing.T) {
	t.Parallel()

	t.Run("Filename", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Unit
			want string
		}{
			{
				name: "names the file after a lone struct",
				give: rowUnit(),
				want: "Row.java",
			},
			{
				name: "returns a lone enum's name",
				give: plugin.Unit{
					Per: plugin.PerSource, Key: "svc/types.src", Word: "gen",
					Decls: []symbol.Symbol{&emit.Enum{Name: "Phase"}},
				},
				want: "Phase.java",
			},
			{
				name: "returns a lone interface's name",
				give: plugin.Unit{
					Per: plugin.PerSource, Key: "svc/types.src", Word: "gen",
					Decls: []symbol.Symbol{&emit.Interface{Name: "Store"}},
				},
				want: "Store.java",
			},
			{
				name: "joins the stem to the word of a unit whose lone declaration is no type",
				give: plugin.Unit{
					Per: plugin.PerSource, Key: "svc/store.go", Word: "stub",
					Decls: []symbol.Symbol{&emit.Function{Name: "open"}},
				},
				want: "StoreStub.java",
			},
			{
				name: "joins the stem to the word of a per-source unit as one type name",
				give: storeUnit(),
				want: "StoreStub.java",
			},
			{
				name: "returns a snake stem in the type name's case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/user_profile.ts", Word: "stub"},
				want: "UserProfileStub.java",
			},
			{
				name: "returns the tag after the word",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc"},
				want: "StoreStubGrpc.java",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "registry"},
				want: "Registry.java",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
				want: "Registry.java",
			},
			{
				name: "returns an acronym run in the word as written",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
				want: "HTTPClient.java",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spell.Filename(tt.give), tt.want, "the filename")
			})
		}
	})
}

// Filename allocates a lone type's name with its extension, and the
// fallback its parts, their join and their conversion too. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestFilenameAllocs(t *testing.T) {
	typed, parts := rowUnit(), storeUnit()
	var got string
	assert.MaxAllocs(t, func() { got = spell.Filename(typed) }, typeFilenameAllocs,
		"Filename allocates the type's name with its extension")
	assert.Equal(t, got, "Row.java", "Filename spells the type")
	assert.MaxAllocs(t, func() { got = spell.Filename(parts) }, partsFilenameAllocs,
		"Filename allocates the parts, their join, their conversion and the name")
	assert.Equal(t, got, "StoreStub.java", "Filename spells the parts")
}

// BenchmarkFilename measures the filename a backend spells once per
// unit.
func BenchmarkFilename(b *testing.B) {
	units := []struct {
		name   string
		give   plugin.Unit
		allocs uint64
		want   string
	}{
		{name: "a unit of one type", give: rowUnit(), allocs: typeFilenameAllocs, want: "Row.java"},
		{
			name: "a unit without a lone type", give: storeUnit(),
			allocs: partsFilenameAllocs, want: "StoreStub.java",
		},
	}
	b.Run("Filename", func(b *testing.B) {
		for _, tt := range units {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var got string
				for c.Loop() {
					got = spell.Filename(tt.give)
				}
				assert.Equal(b, got, tt.want, "Filename spells the unit")
			})
		}
	})
}

// rowUnit returns a unit that declares the struct Row alone.
func rowUnit() plugin.Unit {
	return plugin.Unit{
		Per: plugin.PerSource, Key: "svc/types.src", Word: "gen",
		Decls: []symbol.Symbol{&emit.Struct{Name: "Row"}},
	}
}

// storeUnit returns the stub family's unit of svc/store.go, which
// declares no lone type.
func storeUnit() plugin.Unit {
	return plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"}
}
