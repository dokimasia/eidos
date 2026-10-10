// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// filenameAllocs is a per-source unit's filename whose parts are snake
// case already: the list of its parts, their join, and the name with
// its extension.
const filenameAllocs = 3

// A Rust filename is a module name, so a change of spelling breaks every
// mod declaration that names it. Every spelling of a unit a plan admits
// is pinned.
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
				name: "joins the stem to the word of a per-source unit",
				give: storeUnit(),
				want: "store_stub.rs",
			},
			{
				name: "returns a Pascal-case stem in snake case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/UserStore.java", Word: "stub"},
				want: "user_store_stub.rs",
			},
			{
				name: "returns the tag after the word",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc"},
				want: "store_stub_grpc.rs",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "stub"},
				want: "stub.rs",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
				want: "registry.rs",
			},
			{
				name: "returns an acronym in the word as one name",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
				want: "http_client.rs",
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

// Filename allocates its parts, their join and the name. The ordinary
// run, which runs no benchmark, checks that ceiling here.
func TestFilenameAllocs(t *testing.T) {
	unit := storeUnit()
	var got string
	assert.MaxAllocs(t, func() { got = spell.Filename(unit) }, filenameAllocs,
		"Filename allocates the parts, their join and the name")
	assert.Equal(t, got, "store_stub.rs", "Filename spells the unit")
}

// BenchmarkFilename measures the filename a backend spells once per
// unit.
func BenchmarkFilename(b *testing.B) {
	b.Run("Filename", func(b *testing.B) {
		unit := storeUnit()
		c := bench.Start(b).MaxAllocs(filenameAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = spell.Filename(unit)
		}
		assert.Equal(b, got, "store_stub.rs", "Filename spells the unit")
	})
}

// storeUnit returns the stub family's unit of svc/store.go.
func storeUnit() plugin.Unit {
	return plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"}
}
