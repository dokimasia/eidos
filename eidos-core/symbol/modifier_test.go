// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol_test

import (
	"testing"

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
			if got != symbol.VisibilityUnknown {
				t.Fatalf("zero Visibility = %d, want VisibilityUnknown", got)
			}
			if symbol.VisibilityUnknown == symbol.VisibilityPublic {
				t.Fatal("VisibilityUnknown equals VisibilityPublic: " +
					"an unanswered visibility would read as public")
			}
		})
	})

	t.Run("Level", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value is Instance", func(t *testing.T) {
			t.Parallel()

			var got symbol.Level
			if got != symbol.LevelInstance {
				t.Fatalf("zero Level = %d, want LevelInstance", got)
			}
		})
	})

	t.Run("Variance", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value is Invariant", func(t *testing.T) {
			t.Parallel()

			var got symbol.Variance
			if got != symbol.VarianceInvariant {
				t.Fatalf("zero Variance = %d, want VarianceInvariant", got)
			}
		})
	})

	t.Run("Mutability", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value claims nothing", func(t *testing.T) {
			t.Parallel()

			var got symbol.Mutability
			if got != symbol.MutabilityUnknown {
				t.Fatalf("zero Mutability = %d, want MutabilityUnknown", got)
			}
			if symbol.MutabilityUnknown == symbol.MutabilityImmutable {
				t.Fatal("MutabilityUnknown equals MutabilityImmutable: " +
					"a language without the distinction would read as immutable")
			}
		})
	})

	t.Run("Variadic", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value takes one argument", func(t *testing.T) {
			t.Parallel()

			var got symbol.Variadic
			if got != symbol.VariadicNone {
				t.Fatalf("zero Variadic = %d, want VariadicNone", got)
			}
		})

		t.Run("positional and keyword stay distinct", func(t *testing.T) {
			t.Parallel()

			if symbol.VariadicPositional == symbol.VariadicKeyword {
				t.Fatal("the two variadic forms collapsed: Python *args and " +
					"**kwargs would spell the same")
			}
		})
	})
}
