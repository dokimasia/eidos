// Copyright ThesmOS B.V. 2026
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

// declOf returns an emit type carrying an origin, so the split's
// provenance narrowing has something to keep.
func declOf(name string) *emit.Struct {
	return &emit.Struct{
		Origin: symbol.Identity{
			Lang: "fixture", Package: "svc", Name: name, Kind: symbol.KindStruct,
		},
		Name: name,
	}
}

// A Java file holds one public type, so the split is what makes
// the naming's file-per-type rule reachable at all.
func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("returns one unit per file-level type", func(t *testing.T) {
		t.Parallel()

		row, store := declOf("Row"), declOf("Store")
		u := plugin.Unit{
			Plugin: "gen", Per: plugin.PerSource, Word: "gen",
			Key:     "svc/types.src",
			Decls:   []symbol.Symbol{row, store},
			Origins: []symbol.Identity{row.Origin, store.Origin},
		}
		out := backend.Split(u)
		assert.Equal(t, len(out), 2, "two types, two units")
		assert.Equal(t, out[0].Decls, []symbol.Symbol{row}, "the first type")
		assert.Equal(t, out[1].Decls, []symbol.Symbol{store}, "the second")
		assert.Equal(t, out[0].Key, u.Key,
			"the routing key survives, carrying the source derivation")
		assert.Equal(t, out[0].Origins, []symbol.Identity{row.Origin},
			"provenance narrows to the split unit's own type")
	})

	t.Run("splits an enum into a file of its own", func(t *testing.T) {
		t.Parallel()

		row := declOf("Row")
		phase := &emit.Enum{Name: "Phase"}
		odd := &emit.Constant{Name: "Limit", Value: "8"}
		u := plugin.Unit{
			Plugin: "gen", Per: plugin.PerSource, Word: "gen",
			Key:   "svc/types.src",
			Decls: []symbol.Symbol{row, phase, odd},
		}
		out := backend.Split(u)
		assert.Equal(t, len(out), 3, "an enum is a file-level type like a class")
		assert.Equal(t, out[1].Decls, []symbol.Symbol{phase}, "alone in its unit")
		assert.Equal(t, out[2].Decls, []symbol.Symbol{odd}, "apart from the remainder")
	})

	t.Run("keeps everything else together under the original key", func(t *testing.T) {
		t.Parallel()

		row := declOf("Row")
		odd := &emit.Constant{Name: "Limit", Value: "8"}
		u := plugin.Unit{
			Plugin: "gen", Per: plugin.PerSource, Word: "gen",
			Key:   "svc/types.src",
			Decls: []symbol.Symbol{row, odd},
		}
		out := backend.Split(u)
		assert.Equal(t, len(out), 2, "the type and the remainder")
		assert.Equal(t, out[1].Decls, []symbol.Symbol{odd},
			"the remainder stays whole, for the render to report")
		assert.Equal(t, out[1].Key, u.Key, "under the original key")
	})

	t.Run("keeps a typeless unit whole", func(t *testing.T) {
		t.Parallel()

		odd := &emit.Constant{Name: "Limit", Value: "8"}
		u := plugin.Unit{
			Plugin: "gen", Per: plugin.PerPlan, Word: "gen",
			Decls: []symbol.Symbol{odd},
		}
		out := backend.Split(u)
		assert.Equal(t, len(out), 1, "one unit in, one unit out")
		assert.Equal(t, out[0].Decls, u.Decls, "untouched")
	})
}
