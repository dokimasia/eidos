// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
)

// A capability is declared by satisfying and found by asserting, so
// the assertion shape and the vocabulary's spellings are pinned.
func TestOptional(t *testing.T) {
	t.Parallel()

	t.Run("assertion", func(t *testing.T) {
		t.Parallel()

		t.Run("finds what a language satisfies and nothing else", func(t *testing.T) {
			t.Parallel()

			source := rulestest.Scripted()
			_, generic := source.(rules.GenericsRules)
			assert.True(t, generic, "the scripted language reasons about generics")
			_, enums := source.(rules.EnumRules)
			assert.False(t, enums, "and declares no enumerations")
			_, enums = rules.Absent("x").(rules.EnumRules)
			assert.False(t, enums, "the absent value satisfies nothing optional")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the vocabulary and numbers the rest", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.EnumValue.String(), "value", "an enum form")
			assert.Equal(t, rules.EnumForm(9).String(), "9", "and an undeclared one")
			assert.Equal(t, rules.OwnBorrowMut.String(), "borrow-mut", "an ownership")
			assert.Equal(t, rules.Ownership(9).String(), "9", "and an undeclared one")
		})
	})
}
