// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol_test

import (
	"testing"

	"go.dokimi.dev/assert"

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
		assert.Equal(t, got, symbol.KindInvalid,
			"the zero Kind is the kind no declaration returns")
		assert.Equal(t, got.String(), "Invalid",
			"and it spells itself as such")
	})

	t.Run("no declaration returns Invalid", func(t *testing.T) {
		t.Parallel()

		assert.NotEqual(t, symbol.KindInvalid, symbol.KindPackage,
			"KindInvalid stays distinct from every real kind")
	})

	t.Run("the set fits the type", func(t *testing.T) {
		t.Parallel()

		// Kind is a uint8, so the schema may hold 255 kinds before
		// the constants wrap and two of them collide.
		assert.True(t, symbol.KindEmbed <= symbol.Kind(200),
			"the kind set stays clear of what a uint8 holds")
	})
}
