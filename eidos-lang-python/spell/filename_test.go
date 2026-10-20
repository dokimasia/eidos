// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The routing key of the source file and the family word of the store unit
// that the allocation test and the benchmark spell.
const (
	storeKey  = "svc/store.go"
	stubWord  = "stub"
	storeFile = "store_stub.py"
)

// filenameAllocs is the filename of a per-source unit whose parts are snake
// case already: the list of its parts, their join, and the name with its
// extension.
const filenameAllocs = 3

// Files are addressed by name, so a change of spelling orphans the files
// that an earlier run wrote. Every spelling of a unit that a plan admits is
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
				name: "joins the stem to the word of a per-source unit with an underscore",
				give: plugin.Unit{Per: plugin.PerSource, Key: storeKey, Word: stubWord}, want: storeFile,
			},
			{
				name: "returns a stem in CapWords in snake case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/UserStore.java", Word: stubWord},
				want: "user_store_stub.py",
			},
			{
				name: "returns the tag as a part of its own",
				give: plugin.Unit{Per: plugin.PerSource, Key: storeKey, Word: stubWord, Tag: "grpc"},
				want: "store_stub_grpc.py",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: stubWord}, want: "stub.py",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"}, want: "registry.py",
			},
			{
				name: "returns an acronym in the word in snake case",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"}, want: "http_client.py",
			},
			{
				name: "returns the stem of a module of source unchanged",
				give: plugin.Unit{Per: plugin.PerSource, Key: "shop/order_items.py"}, want: "order_items.py",
			},
			{
				name: "returns the module of a package directory unchanged",
				give: plugin.Unit{Per: plugin.PerSource, Key: "shop/__init__.py"}, want: "__init__.py",
			},
			{
				name: "returns the leading underscore of a module unchanged",
				give: plugin.Unit{Per: plugin.PerSource, Key: "shop/_impl.py"}, want: "_impl.py",
			},
			{
				name: "returns the trailing underscore of a module unchanged",
				give: plugin.Unit{Per: plugin.PerSource, Key: "shop/list_.py"}, want: "list_.py",
			},
			{
				name: "returns a keyword with a trailing underscore",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "class"}, want: "class_.py",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spell.Filename(tt.give), tt.want, "the filename")
			})
		}
	})

	t.Run("StubFilename", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the filename with the extension .pyi", func(t *testing.T) {
			t.Parallel()

			got := spell.StubFilename(plugin.Unit{Per: plugin.PerSource, Key: "shop/models.py"})
			assert.Equal(t, got, "models.pyi", "the filename of the stub file")
		})
	})
}

// Filename allocates its parts, their join and the name. The ordinary run,
// which runs no benchmark, checks that ceiling here.
func TestFilenameAllocs(t *testing.T) {
	unit := plugin.Unit{Per: plugin.PerSource, Key: storeKey, Word: stubWord}
	var got string
	assert.MaxAllocs(t, func() { got = spell.Filename(unit) }, filenameAllocs,
		"Filename allocates the parts, their join and the name")
	assert.Equal(t, got, storeFile, "Filename spells the unit")
}

// BenchmarkFilename measures the filename that a backend spells once per
// unit.
func BenchmarkFilename(b *testing.B) {
	b.Run("Filename", func(b *testing.B) {
		unit := plugin.Unit{Per: plugin.PerSource, Key: storeKey, Word: stubWord}
		c := bench.Start(b).MaxAllocs(filenameAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = spell.Filename(unit)
		}
		assert.Equal(b, got, storeFile, "Filename spells the unit")
	})
}
