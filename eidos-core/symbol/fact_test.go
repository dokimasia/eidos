// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol"
)

// The generated constants are checked in fact.gen_test.go, which
// enumerates them. These cases hold the hand-written type's own
// contract, which the schema cannot state.
func TestFactType(t *testing.T) {
	t.Parallel()

	t.Run("the zero value is FactInvalid", func(t *testing.T) {
		t.Parallel()

		var got symbol.Fact
		assert.Equal(t, got, symbol.FactInvalid, "the zero Fact is the invalid one")
		assert.False(t, slices.Contains(symbol.Facts(), got),
			"and no declaration states it, so the vocabulary leaves it out")
	})
}
