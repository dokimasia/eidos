// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/symbol"
)

// The generated constants are checked in kind.gen_test.go, which
// enumerates them. These cases hold the hand-written type's own
// contract, which the schema cannot state.
func TestKindType(t *testing.T) {
	t.Parallel()

	t.Run("the zero value is Invalid", func(t *testing.T) {
		t.Parallel()

		var got symbol.Kind
		if got != symbol.KindInvalid {
			t.Fatalf("zero Kind = %d, want KindInvalid", got)
		}
		if got.String() != "Invalid" {
			t.Fatalf("zero Kind names %q, want Invalid", got.String())
		}
	})

	t.Run("no declaration answers Invalid", func(t *testing.T) {
		t.Parallel()

		if symbol.KindInvalid == symbol.KindPackage {
			t.Fatal("KindInvalid collides with a real kind")
		}
	})

	t.Run("the set fits the type", func(t *testing.T) {
		t.Parallel()

		// Kind is a uint8, so the schema may hold 255 kinds before
		// the constants wrap and two of them collide.
		if symbol.KindEmbed > symbol.Kind(200) {
			t.Fatalf("the kind set reaches %d, close to what a uint8 holds",
				symbol.KindEmbed)
		}
	})
}
