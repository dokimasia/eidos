// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// splitAllocs is a split of a unit with an inside origin: the list of the
// parts, and the declarations and the origins of each part.
const splitAllocs = 5

// The structs of the split cases: an exported one, whose check tests its
// package from outside, and an unexported one, whose check tests it from
// inside.
var (
	exportedID   = symbol.Identity{Lang: "golang", Package: storePath, Name: "Store", Kind: symbol.KindStruct}
	unexportedID = symbol.Identity{Lang: "golang", Package: storePath, Name: "entry", Kind: symbol.KindStruct}
)

// Go compiles an external test package apart from the package under
// test, so the checks that test from inside take a file of their own.
func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("Split", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a unit without an inside origin as its one part", func(t *testing.T) {
			t.Parallel()

			u := checkUnit()
			u.Inside = nil
			assert.Equal(t, spell.Split(u), []plugin.Unit{u}, "the unit is its one part")
		})

		t.Run("returns the checks that test from outside before the checks that test from inside", func(t *testing.T) {
			t.Parallel()

			parts := spell.Split(checkUnit())
			assert.Length(t, parts, 2, "the unit splits into two parts")
			expect.Equal(t, names(parts[0].Decls), []string{"CheckStore", "CheckZone"},
				"the first part tests from outside, in the order of the unit")
			expect.Equal(t, names(parts[1].Decls), []string{"Checkentry"}, "the second part tests from inside")
		})

		t.Run("narrows the provenance of each part to the origins of its checks", func(t *testing.T) {
			t.Parallel()

			parts := spell.Split(checkUnit())
			assert.Length(t, parts, 2, "the unit splits into two parts")
			expect.Equal(t, parts[0].Origins, []symbol.Identity{exportedID}, "the outside part derives from Store")
			expect.Equal(t, parts[1].Origins, []symbol.Identity{unexportedID}, "the inside part derives from entry")
		})

		t.Run("keeps the inside origins in the inside part alone", func(t *testing.T) {
			t.Parallel()

			parts := spell.Split(checkUnit())
			assert.Length(t, parts, 2, "the unit splits into two parts")
			expect.Nil(t, parts[0].Inside, "the outside part has no inside origin")
			expect.Equal(t, parts[1].Inside, []symbol.Identity{unexportedID}, "the inside part keeps entry")
		})

		t.Run("returns the inside part alone for a unit whose every check tests from inside", func(t *testing.T) {
			t.Parallel()

			u := checkUnit()
			u.Decls, u.Inside = u.Decls[:2], []symbol.Identity{exportedID, unexportedID}
			parts := spell.Split(u)
			assert.Length(t, parts, 1, "every check tests from inside")
			assert.Length(t, parts[0].Decls, 2, "the part keeps every check")
		})

		t.Run("returns the outside part alone for an inside origin without a check", func(t *testing.T) {
			t.Parallel()

			u := checkUnit()
			u.Inside = []symbol.Identity{{Lang: "golang", Package: storePath, Name: "gone", Kind: symbol.KindStruct}}
			parts := spell.Split(u)
			assert.Length(t, parts, 1, "no check tests from inside")
			assert.Nil(t, parts[0].Inside, "the part has no inside origin")
		})

		t.Run("returns no provenance for a part whose checks have no origin", func(t *testing.T) {
			t.Parallel()

			u := checkUnit()
			u.Decls = u.Decls[1:]
			parts := spell.Split(u)
			assert.Length(t, parts, 2, "the unit splits into two parts")
			assert.Nil(t, parts[0].Origins, "the check of the zone has no origin")
		})

		t.Run("names the file of the inside part apart", func(t *testing.T) {
			t.Parallel()

			parts := spell.Split(checkUnit())
			assert.Length(t, parts, 2, "the unit splits into two parts")
			expect.Equal(t, spell.Filename(parts[0]), "store_check_test.go", "the outside part has the test file")
			expect.Equal(t, spell.Filename(parts[1]), "store_check_internal_test.go",
				"the inside part has the internal test file")
		})
	})
}

// Split allocates the parts of a unit with an inside origin. The ordinary
// run, which runs no benchmark, checks that ceiling here.
func TestSplitAllocs(t *testing.T) {
	u := checkUnit()
	var parts []plugin.Unit
	assert.MaxAllocs(t, func() { parts = spell.Split(u) }, splitAllocs,
		"Split allocates the parts with their declarations and origins")
	assert.Length(t, parts, 2, "Split returns two parts")
}

// BenchmarkSplit measures the split that the layout asks for once per
// routed unit.
func BenchmarkSplit(b *testing.B) {
	b.Run("Split", func(b *testing.B) {
		u := checkUnit()
		c := bench.Start(b).MaxAllocs(splitAllocs)
		defer c.End()
		var parts []plugin.Unit
		for c.Loop() {
			parts = spell.Split(u)
		}
		assert.Length(b, parts, 2, "Split returns two parts")
	})
}

// checkUnit returns the unit of an external test family of svc/store.go
// with a check of Store, of entry and of a zone without an origin, whose
// check of entry tests from inside.
func checkUnit() plugin.Unit {
	return plugin.Unit{
		Per: plugin.PerSource, Key: "svc/store.go", Word: "check", Tag: "test",
		Decls: []symbol.Symbol{
			&emit.Struct{Origin: exportedID, Name: "CheckStore"},
			&emit.Struct{Origin: unexportedID, Name: "Checkentry"},
			&emit.Struct{Name: "CheckZone"},
		},
		Origins: []symbol.Identity{exportedID, unexportedID},
		Inside:  []symbol.Identity{unexportedID},
	}
}

// names returns the declared names of decls, in their order.
func names(decls []symbol.Symbol) []string {
	out := make([]string, 0, len(decls))
	for _, d := range decls {
		out = append(out, emit.DeclaredName(d))
	}
	return out
}
