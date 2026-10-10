// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/symbol"
)

// Each modifier's zero value is a contract: a frontend that has not
// returned must not have silently returned something definite.
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
				"the zero Variance is the variance of every Go and Rust type parameter")
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

		t.Run("distinguishes VariadicPositional from VariadicKeyword", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, symbol.VariadicPositional, symbol.VariadicKeyword,
				"Python *args and **kwargs spell differently")
		})
	})

	t.Run("Structural", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.TypeForm
			want bool
		}{
			{name: "reports true for FormNamed", give: symbol.FormNamed, want: true},
			{name: "reports true for FormInline", give: symbol.FormInline, want: true},
			{name: "reports false for FormScalar", give: symbol.FormScalar, want: false},
			{name: "reports false for FormOpaque", give: symbol.FormOpaque, want: false},
			{name: "reports false for FormDynamic", give: symbol.FormDynamic, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Structural(), tt.want, "whether a frontend sets the form")
			})
		}
	})
}

// Structural allocates nothing in the ordinary run, which runs no
// benchmark.
func TestModifierAllocs(t *testing.T) {
	form := symbol.FormInline
	var structural bool
	assert.MaxAllocs(t, func() { structural = form.Structural() }, 0, "Structural allocates nothing")
	assert.True(t, structural, "Structural reports true for FormInline")
}

// BenchmarkModifier measures the one method of the modifiers.
func BenchmarkModifier(b *testing.B) {
	b.Run("Structural", func(b *testing.B) {
		form := symbol.FormInline
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = form.Structural()
		}
		assert.True(b, got, "FormInline is a structural form")
	})
}
