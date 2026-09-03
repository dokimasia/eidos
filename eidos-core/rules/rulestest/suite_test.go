// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
)

// The suite is the projections' conformance bar, so it must hold
// the scripted language, and each check's own failure path must be
// reachable through a broken language.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunRulesSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("holds the scripted language to every check", func(t *testing.T) {
			t.Parallel()
			rulestest.RunRulesSuite(t, setup)
		})
	})

	t.Run("AssertDistinctSamples", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a language deriving one value twice", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a pair that cannot tell a subject apart", func(tb assert.TB) {
				rulestest.AssertDistinctSamples(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return same{rulestest.Scripted()}, f
				})
			})
			assert.Contains(t, msg, "differ", "the rejection names the property")
		})
	})

	t.Run("AssertRefusesWithReason", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a refusal without a reason", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a sample that refuses in silence", func(tb assert.TB) {
				rulestest.AssertRefusesWithReason(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return silent{rulestest.Scripted()}, f
				})
			})
			assert.Contains(t, msg, "refusal", "the rejection names what is missing")
		})
	})

	t.Run("AssertTotal", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fold that returns a name", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a leaf that is not a leaf", func(tb assert.TB) {
				rulestest.AssertTotal(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return naming{rulestest.Scripted()}, f
				})
			})
			assert.Contains(t, msg, "leaf", "the rejection names the rule")
		})
	})

	t.Run("AssertRecorded", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture with no graph", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "nothing to read", func(tb assert.TB) {
				rulestest.AssertRecorded(tb, func(assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					return rulestest.Scripted(), &rulestest.Fixture{}
				})
			})
			assert.Contains(t, msg, "no graph", "the rejection names what the setup owes")
		})
	})
}

// same derives one value for both halves.
type same struct {
	rules.SourceRules
}

// SamplesOf returns the first half twice.
func (s same) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	sample, _ := s.SourceRules.SamplesOf(ref, hint, v)
	return sample, sample
}

// silent refuses without a reason.
type silent struct {
	rules.SourceRules
}

// SamplesOf returns two empty samples carrying no refusal.
func (silent) SamplesOf(*node.TypeRef, string, rules.View) (rules.Sample, rules.Sample) {
	return rules.Sample{}, rules.Sample{}
}

// naming folds a builtin to the named form, which no leaf is.
type naming struct {
	rules.SourceRules
}

// Builtin returns the name itself.
func (naming) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	return rules.TypeShape{Spelling: ref.Spelling}
}

// ZeroValue keeps the scripted language's, so the wrapper stays a
// full SourceRules.
func (n naming) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return n.SourceRules.ZeroValue(ref, v)
}
