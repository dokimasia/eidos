// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
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
}
