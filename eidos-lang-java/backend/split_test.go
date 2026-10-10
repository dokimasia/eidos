// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The split cases name the routing key and family word of the unit it
// reshapes, and the declarations the unit contains.
const (
	typesKey    = "svc/types.src"
	splitWord   = "gen"
	storeType   = "Store"
	phaseName   = "Phase"
	limitName   = "Limit"
	limitValue  = "8"
	fixtureLang = symbol.Lang("fixture")
	fixturePkg  = "svc"
)

// splitAllocs is a unit of two types and a constant: the list of units,
// each type's list of its origin, and the list of the constant.
const splitAllocs = 1 + 2 + 1

// A Java file declares one public type, so the split is what makes
// the naming's file-per-type rule reachable at all.
func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("Split", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one unit per file-level type", func(t *testing.T) {
			t.Parallel()

			row, store := declOf(rowName), declOf(storeType)
			out := backend.Split(typesUnit(row, store))
			assert.Length(t, out, 2, "two types, two units")
			assert.Equal(t, out[0].Decls, []symbol.Symbol{row}, "the first type")
			assert.Equal(t, out[1].Decls, []symbol.Symbol{store}, "the second")
		})

		t.Run("keeps the routing key of the unit it splits", func(t *testing.T) {
			t.Parallel()

			out := backend.Split(typesUnit(declOf(rowName), declOf(storeType)))
			assert.Equal(t, out[0].Key, typesKey, "the source derivation survives")
		})

		t.Run("narrows each split unit's provenance to its own type", func(t *testing.T) {
			t.Parallel()

			row := declOf(rowName)
			out := backend.Split(typesUnit(row, declOf(storeType)))
			assert.Equal(t, out[0].Origins, []symbol.Identity{row.Origin}, "the split unit's own type")
		})

		t.Run("returns an enum in a unit of its own", func(t *testing.T) {
			t.Parallel()

			phase := &emit.Enum{Name: phaseName}
			out := backend.Split(typesUnit(declOf(rowName), phase, limit()))
			assert.Length(t, out, 3, "an enum is a file-level type like a class")
			assert.Equal(t, out[1].Decls, []symbol.Symbol{phase}, "alone in its unit")
		})

		t.Run("returns the other declarations in one unit after the types", func(t *testing.T) {
			t.Parallel()

			odd := limit()
			out := backend.Split(typesUnit(declOf(rowName), odd))
			assert.Length(t, out, 2, "the type and the remainder")
			assert.Equal(t, out[1].Decls, []symbol.Symbol{odd},
				"the remainder, whole, for the render to report under the refused kind")
		})

		t.Run("keeps the routing key of the unit of other declarations", func(t *testing.T) {
			t.Parallel()

			out := backend.Split(typesUnit(declOf(rowName), limit()))
			assert.Equal(t, out[1].Key, typesKey, "the original key")
		})

		t.Run("returns a unit without a type whole", func(t *testing.T) {
			t.Parallel()

			u := typesUnit(limit())
			out := backend.Split(u)
			assert.Length(t, out, 1, "one unit in, one unit out")
			assert.Equal(t, out[0].Decls, u.Decls, "untouched")
		})

		t.Run("returns type units without spare capacity", func(t *testing.T) {
			t.Parallel()

			out := backend.Split(typesUnit(declOf(rowName), declOf(storeType)))
			assert.Equal(t, cap(out[0].Decls), 1, "an append to a unit copies it, so the next type stays as it is")
		})
	})
}

// A split allocates its units and their provenance. The ordinary run,
// which runs no benchmark, checks that ceiling here.
func TestSplitAllocs(t *testing.T) {
	checkAllocs(t, splitCalls())
}

// BenchmarkSplit measures the reshaping the settle runs over every
// unit.
func BenchmarkSplit(b *testing.B) {
	benchCalls(b, splitCalls())
}

// splitCalls returns a call of Split over a unit of two types and a
// constant.
func splitCalls() []allocCall {
	u := typesUnit(declOf(rowName), declOf(storeType), limit())
	var out []plugin.Unit
	return []allocCall{
		{
			name: "Split", allocs: splitAllocs,
			call:  func() { out = backend.Split(u) },
			check: func(tb assert.TB) { assert.Length(tb, out, 3, "Split returns two types and the rest") },
		},
	}
}

// declOf returns an emit type with an origin, so the split's
// provenance narrowing has something to keep.
func declOf(name string) *emit.Struct {
	return &emit.Struct{
		Origin: symbol.Identity{
			Lang: fixtureLang, Package: fixturePkg, Name: name, Kind: symbol.KindStruct,
		},
		Name: name,
	}
}

// typesUnit returns a per-source unit of the given declarations
// under the fixture's routing key.
func typesUnit(decls ...symbol.Symbol) plugin.Unit {
	u := plugin.Unit{
		Plugin: splitWord, Per: plugin.PerSource, Word: splitWord,
		Key: typesKey, Decls: decls,
	}
	for _, d := range decls {
		if id, held := emit.OriginOf(d); held && !id.IsZero() {
			u.Origins = append(u.Origins, id)
		}
	}
	return u
}

// limit returns the fixture's constant, a kind Java declares no
// file of its own for.
func limit() *emit.Constant {
	return &emit.Constant{Name: limitName, Value: limitValue}
}
