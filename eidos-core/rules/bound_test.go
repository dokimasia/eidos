// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The bound is what a handler holds, so its defaults and the calls
// that pass straight through to the language are pinned here.
func TestBound(t *testing.T) {
	t.Parallel()

	t.Run("NewBound", func(t *testing.T) {
		t.Parallel()

		t.Run("stands in the absent value for a missing source", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(nil, rules.View{}, nil)
			assert.True(t, rules.IsAbsent(b.Source()), "no source is the absent one")
			assert.Equal(t, b.Lang(), symbol.Lang(""), "for the zero language")
			assert.True(t, b.View().IsZero(), "over the view it was given")
		})
	})

	t.Run("passes through", func(t *testing.T) {
		t.Parallel()

		t.Run("the language's own returns, refusing on the zero view", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			zero, held := b.ZeroValue(builtin(intSpelling))
			assert.True(t, held, "a zero derives")
			assert.Equal(t, zero, emit.Literal(emit.LiteralInt, "0"), "as the language spells it")
			lit, held := b.LiteralFor(nil, builtin(strSpelling), "x")
			assert.True(t, held, "a literal derives")
			assert.Equal(t, lit, emit.Literal(emit.LiteralString, "x"), "as text")
			assert.Equal(t, b.TypeName("Builder", "Row"), "RowBuilder", "the join is the language's")

			none := rules.NewBound(rulestest.Scripted(), rules.View{}, nil)
			_, held = none.ZeroValue(builtin(intSpelling))
			assert.False(t, held, "no view, no zero")
			_, held = none.LiteralFor(nil, builtin(strSpelling), "x")
			assert.False(t, held, "and no literal")
			_, held = b.ZeroValue(nil)
			assert.False(t, held, "nor for no reference")
		})
	})
}
