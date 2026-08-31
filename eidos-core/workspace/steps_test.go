// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// runOrdering composes the annotators over the one-struct fixture
// and runs, so each role's handler runs exactly once, in schedule
// order.
func runOrdering(t *testing.T, anns ...plugin.Annotator) {
	t.Helper()

	w, err := workspace.New().
		Annotators(anns...).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirror("mirror"))).
		Build()
	assert.NoError(t, err, "the ordering fixture composes")
	g, _ := alpha(t)
	_, err = w.Run(t.Context(), g)
	assert.NoError(t, err, "the ordering fixture runs")
}

// The steps' output is the schedule, and the schedule is
// observable twice over: the order handlers run in, and the bucket
// number every claim carries.
func TestSteps(t *testing.T) {
	t.Parallel()

	t.Run("priority places the roles", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		runOrdering(t,
			ordered("late", 5, nil, nil, &calls),
			ordered("early", 1, nil, nil, &calls),
		)
		assert.Equal(t, calls, []plugin.ID{"early", "late"},
			"the lower priority runs first, whatever the declaration order")
	})

	t.Run("capabilities order inside one priority", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		runOrdering(t,
			ordered("needs", 1, nil, caps("cap"), &calls),
			ordered("gives", 1, caps("cap"), nil, &calls),
		)
		assert.Equal(t, calls, []plugin.ID{"gives", "needs"},
			"the provider runs before its requirer, whatever the names say")
	})

	t.Run("the name breaks the tie", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		runOrdering(t,
			ordered("beta", 1, nil, nil, &calls),
			ordered("alph", 1, nil, nil, &calls),
		)
		assert.Equal(t, calls, []plugin.ID{"alph", "beta"},
			"names order what priorities and capabilities leave open")
	})

	t.Run("the bucket number is the schedule position", func(t *testing.T) {
		t.Parallel()

		var rank meta.Key[string]
		register := func(r *meta.Registry) error {
			if err := r.ClaimNamespace("order", "the test"); err != nil {
				return err
			}
			k, err := meta.Register[string](r, meta.KeySpec{
				Name: "order.rank", Doc: "which role stamped first",
			})
			rank = k
			return err
		}
		stampRank := func(name plugin.ID, pri int) plugin.Annotator {
			return stamperAt(name, pri, nil, nil,
				func(m *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, rank, string(name))
					return nil
				})
		}
		w, err := workspace.New().
			Keys(register).
			Annotators(stampRank("second", 2), stampRank("first", 1)).
			Targets("fixture").
			Plans(planTo("plan", "fixture", mirror("mirror"))).
			Build()
		assert.NoError(t, err, "the bucket fixture composes")
		g, s := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "the bucket fixture runs")

		got, held := meta.Get(report.Facts, s.Identity(), rank)
		assert.True(t, held, "the fact is stamped")
		assert.Equal(t, got, "first", "the earlier bucket wins the rank")
		buckets := map[plugin.ID]int{}
		for view := range report.Facts.Claims(s.Identity(), rank.ID()) {
			buckets[view.Claim.Plugin] = view.Claim.Bucket
		}
		assert.Equal(t, buckets, map[plugin.ID]int{"first": 1, "second": 2},
			"each claim carries its plugin's schedule position")
	})

	t.Run("generators order within the plan", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		placed := func(name plugin.ID, pri int) plugin.Generator {
			p, held := eidos.NewPlugin(name).
				Priority(plugin.RoleGenerator, pri).
				Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
				Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
					calls = append(calls, name)
					return nil
				})).Build().(plugin.Generator)
			assert.True(t, held, "an emitter rule lowers to the generator role")
			return p
		}
		w, err := workspace.New().
			Targets("fixture").
			Plans(workspace.Plan{
				Name:       "plan",
				Generators: []plugin.Generator{placed("late", 9), placed("early", 3)},
				Backend:    fakeBackend{name: "printer", target: "fixture"},
			}).
			Build()
		assert.NoError(t, err, "the plan fixture composes")
		g, _ := alpha(t)
		_, err = w.Run(t.Context(), g)
		assert.NoError(t, err, "the plan fixture runs")
		assert.Equal(t, calls, []plugin.ID{"early", "late"},
			"the plan's generators run in bucket order, not list order")
	})
}
