// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit_test

import (
	"testing"

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
			if got := slot.Len(); got != 1 {
				t.Fatalf("Len() = %d, want 1", got)
			}
		})

		t.Run("keeps insertion order across calls", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append(1, 2)
			slot.Append(3)

			want := []int{1, 2, 3}
			got := slot.Items()
			if len(got) != len(want) {
				t.Fatalf("Items() = %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("Items() = %v, want %v", got, want)
				}
			}
		})

		t.Run("accepts no values without changing the slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append()
			if got := slot.Len(); got != 0 {
				t.Fatalf("Len() = %d, want 0", got)
			}
		})
	})

	t.Run("Items", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero slot answers nothing", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			if got := slot.Items(); len(got) != 0 {
				t.Fatalf("Items() = %v, want empty", got)
			}
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
			if got := slot.Len(); got != 5 {
				t.Fatalf("Len() = %d, want 5", got)
			}
		})
	})
}
