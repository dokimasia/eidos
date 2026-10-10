// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
)

// The plans table lists each plan that did not commit in the run that
// recorded the generation, so the next warm run runs the plan whole.
func TestPlans(t *testing.T) {
	t.Parallel()

	t.Run("PhaseState", func(t *testing.T) {
		t.Parallel()

		t.Run("Pending", func(t *testing.T) {
			t.Parallel()

			t.Run("reports true for a plan that did not commit", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), []string{failedPlan}, func(*state.Recorder) {})
				pending, err := g.Phases(t.Context()).Pending(failedPlan)
				assert.NoError(t, err, "the plans table reads")
				assert.True(t, pending, "the plan's work is pending")
			})

			t.Run("reports false for a plan that committed", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), []string{failedPlan}, func(*state.Recorder) {})
				pending, err := g.Phases(t.Context()).Pending(recordedPlan)
				assert.NoError(t, err, "the plans table reads")
				assert.False(t, pending, "the plan committed")
			})

			t.Run("reports false for a plan that committed after a run it did not commit in", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(*state.Recorder) {})
				g := recordedPhases(t, l, emptyFacts(), nil, func(*state.Recorder) {})
				pending, err := g.Phases(t.Context()).Pending(failedPlan)
				assert.NoError(t, err, "the plans table reads")
				assert.False(t, pending, "the later commit removes the plan's row")
			})

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(*state.Recorder) {})
				damageRun(t, l, func(int) int { return 3 })
				_, err := g.Phases(t.Context()).Pending(failedPlan)
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged block is found")
			})
		})
	})
}

// A lookup of the plans table allocates nothing in the ordinary run, which
// runs no benchmark, once the run reader keeps the block it reads. The
// check runs alone, because the count includes every goroutine's
// allocations.
func TestPlansAllocs(t *testing.T) {
	s := recordedPhases(t, ledger.NewMem(), emptyFacts(), []string{failedPlan}, func(*state.Recorder) {}).
		Phases(t.Context())
	pending := false
	var err error
	assert.MaxAllocs(t, func() { pending, err = s.Pending(failedPlan) }, 0, "Pending allocates nothing")
	assert.NoError(t, err, "the plans table reads")
	assert.True(t, pending, "the plan's work is pending")
}

// BenchmarkPlans measures a lookup of the plans table, after one lookup
// that the warm-up runs.
func BenchmarkPlans(b *testing.B) {
	s := recordedPhases(b, ledger.NewMem(), emptyFacts(), []string{failedPlan}, func(*state.Recorder) {}).
		Phases(b.Context())

	b.Run("PhaseState", func(b *testing.B) {
		b.Run("Pending", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			var (
				pending bool
				err     error
			)
			for c.Loop() {
				pending, err = s.Pending(failedPlan)
			}
			assert.NoError(b, err, "the plans table reads")
			assert.True(b, pending, "Pending reports the plan's work")
		})
	})
}
