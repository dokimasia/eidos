// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

func TestSlot(t *testing.T) {
	t.Parallel()

	t.Run("Append", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero slot accepts values", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append(1)
			assert.Equal(t, slot.Len(), 1, "the zero slot accepts values")
		})

		t.Run("keeps insertion order across calls", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append(1, 2)
			slot.Append(3)

			assert.Equal(t, slot.Items(), []int{1, 2, 3},
				"insertion order holds across calls")
		})

		t.Run("accepts no values without changing the slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append()
			assert.Equal(t, slot.Len(), 0, "appending nothing changes nothing")
		})
	})

	t.Run("Items", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero slot returns nothing", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			assert.Empty(t, slot.Items(), "the zero slot holds nothing")
		})
	})

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("counts what was appended", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[string]
			for i := range 5 {
				slot.Append(string(rune('a' + i)))
			}
			assert.Equal(t, slot.Len(), 5, "Len counts what was appended")
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("separates an untouched slot from a filled one", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			assert.True(t, slot.IsZero(),
				"an untouched slot holds nothing, which is what lets an encoder omit it")
			slot.Append(1)
			assert.False(t, slot.IsZero(), "and a slot holding a value is not omitted")
		})
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("encodes a concrete slot as the array of its contents", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[string]
			slot.Append("a", "b")

			encoded, err := json.Marshal(slot)
			assert.NoError(t, err, "the slot encodes")
			assert.Equal(t, string(encoded), `["a","b"]`,
				"as the array of its contents, from the element type's own tags")
		})
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips a concrete slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[string]
			slot.Append("a", "b")
			encoded, err := json.Marshal(slot)
			assert.NoError(t, err, "the slot encodes")

			var decoded emit.Slot[string]
			assert.NoError(t, json.Unmarshal(encoded, &decoded), "and decodes")
			assert.Equal(t, decoded.Items(), []string{"a", "b"},
				"returning the contents in insertion order")
		})

		t.Run("round-trips a declaration slot as each element's own kind", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[symbol.Symbol]
			slot.Append(&emit.Alias{Name: "RowID"}, &emit.Constant{Name: "Version"})
			encoded, err := json.Marshal(slot)
			assert.NoError(t, err, "the declaration slot encodes")

			var decoded emit.Slot[symbol.Symbol]
			assert.NoError(t, json.Unmarshal(encoded, &decoded), "and decodes")
			assert.Equal(t, decoded.Len(), 2, "returning what it held")
			alias, isAlias := decoded.Items()[0].(*emit.Alias)
			assert.True(t, isAlias,
				"each element decodes as the kind it encoded, which is what the "+
					"kind discriminator is for")
			assert.Equal(t, alias.Name, "RowID", "carrying the fields it wrote")
			constant, isConstant := decoded.Items()[1].(*emit.Constant)
			assert.True(t, isConstant, "and a second kind decodes as its own")
			assert.Equal(t, constant.Name, "Version", "carrying its fields too")
		})

		t.Run("refuses malformed JSON into a concrete slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[string]
			slot.Append("held")
			err := slot.UnmarshalJSON([]byte(`["a",`))
			assert.HasError(t, err, "malformed input is refused")
			assert.HasPrefix(t, err.Error(), "emit: ", "under the package prefix")
			assert.Equal(t, slot.Items(), []string{"held"},
				"and the slot keeps what it held rather than zeroing")
		})

		t.Run("refuses malformed JSON into a declaration slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[symbol.Symbol]
			slot.Append(&emit.Alias{Name: "RowID"})
			err := slot.UnmarshalJSON([]byte(`[{"kind":`))
			assert.HasError(t, err, "malformed input is refused before the elements are allocated")
			assert.HasPrefix(t, err.Error(), "emit: ", "under the package prefix")
			assert.Equal(t, slot.Len(), 1,
				"and the slot keeps what it held rather than zeroing")
		})
	})
}
