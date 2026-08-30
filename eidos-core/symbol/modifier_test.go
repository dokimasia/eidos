// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol"
)

// Each modifier's zero value is a contract: a frontend that has not
// answered must not have silently answered something definite.
func TestModifier(t *testing.T) {
	t.Parallel()

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value claims nothing", func(t *testing.T) {
			t.Parallel()

			var got symbol.Visibility
			assert.Equal(t, got, symbol.VisibilityUnknown,
				"an unanswered visibility claims nothing")
			assert.NotEqual(t, symbol.VisibilityUnknown, symbol.VisibilityPublic,
				"so it cannot read as public")
		})
	})

	t.Run("Level", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value is Instance", func(t *testing.T) {
			t.Parallel()

			var got symbol.Level
			assert.Equal(t, got, symbol.LevelInstance,
				"the zero Level is the common case everywhere")
		})
	})

	t.Run("Variance", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value is Invariant", func(t *testing.T) {
			t.Parallel()

			var got symbol.Variance
			assert.Equal(t, got, symbol.VarianceInvariant,
				"the zero Variance is what Go and Rust always answer")
		})
	})

	t.Run("Mutability", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value claims nothing", func(t *testing.T) {
			t.Parallel()

			var got symbol.Mutability
			assert.Equal(t, got, symbol.MutabilityUnknown,
				"an unanswered mutability claims nothing")
			assert.NotEqual(t, symbol.MutabilityUnknown, symbol.MutabilityImmutable,
				"so a language without the distinction cannot read as immutable")
		})
	})

	t.Run("Variadic", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value takes one argument", func(t *testing.T) {
			t.Parallel()

			var got symbol.Variadic
			assert.Equal(t, got, symbol.VariadicNone,
				"the zero Variadic takes exactly one argument")
		})

		t.Run("positional and keyword stay distinct", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, symbol.VariadicPositional, symbol.VariadicKeyword,
				"Python *args and **kwargs spell differently")
		})
	})
}
