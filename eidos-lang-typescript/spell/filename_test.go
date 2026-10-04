// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// filenameAllocs is a per-source unit's filename whose parts are kebab
// case already: the list of its parts, their join, and the name with
// its extension.
const filenameAllocs = 3

// Files are addressed by name, so a change of spelling orphans the files
// an earlier run wrote. Every spelling of a unit a plan admits is
// pinned.
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
				name: "joins the stem to the word of a per-source unit with a dot",
				give: storeUnit(),
				want: "store.stub.ts",
			},
			{
				name: "returns a Pascal-case stem in kebab case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/UserStore.java", Word: "stub"},
				want: "user-store.stub.ts",
			},
			{
				name: "returns the tag as a qualifier of its own",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc"},
				want: "store.stub.grpc.ts",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "stub"},
				want: "stub.ts",
			},
			{
				name: "returns a test file for a per-package unit of the test companion",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "suite", Tag: "test"},
				want: "suite.test.ts",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
				want: "registry.ts",
			},
			{
				name: "returns an acronym in the word in kebab case",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
				want: "http-client.ts",
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
	assert.Equal(t, got, "store.stub.ts", "Filename spells the unit")
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
		assert.Equal(b, got, "store.stub.ts", "Filename spells the unit")
	})
}

// storeUnit returns the stub family's unit of svc/store.go.
func storeUnit() plugin.Unit {
	return plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"}
}
