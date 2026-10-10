// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// depthKey is a policy key of the fixture target that no backend
// declares.
const depthKey plugin.PolicyKey = "tsx.depth"

// undocumented is a backend of the fixture target that declares the
// width policy without documentation, which the backend kit refuses at
// its own Build.
type undocumented struct{ fakeBackend }

// Policies returns the width policy without its documentation.
func (undocumented) Policies() []plugin.PolicySpec {
	spec := widthPolicy
	spec.Doc = ""
	return []plugin.PolicySpec{spec}
}

// Build resolves one lowering policy for each plan, so a spoke reads a
// choice for every key that its backend declares. The config selects a
// choice for every plan or for one plan, and Build refuses a selection
// that no backend can read.
func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("resolvePolicies", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves the default of each policy for a plan without a selection", func(t *testing.T) {
			t.Parallel()

			readers := readPolicies(t, workspace.Config{})
			assert.Equal(t, readers[0].policy.Choice(widthKey), narrow, "the plan reads the default")
		})

		t.Run("resolves the choice that the config selects for every plan", func(t *testing.T) {
			t.Parallel()

			readers := readPolicies(t, workspace.Config{Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: wide}})
			expect.Equal(t, readers[0].policy.Choice(widthKey), wide, "the first plan reads the selected choice")
			expect.Equal(t, readers[1].policy.Choice(widthKey), wide, "the second plan reads the selected choice")
		})

		t.Run("resolves the choice that a plan selects over the config's", func(t *testing.T) {
			t.Parallel()

			readers := readPolicies(t, workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: wide},
				Plans: map[string]workspace.PlanConfig{
					clientPlan: {Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: full}},
				},
			})
			expect.Equal(t, readers[0].policy.Choice(widthKey), full, "the plan reads its own selection")
			expect.Equal(t, readers[1].policy.Choice(widthKey), wide, "the other plan reads the config's selection")
		})

		t.Run("returns an error with the declared keys for a key that no backend declares", func(t *testing.T) {
			t.Parallel()

			b, _ := policyPlans(t)
			_, err := b.Config(workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{depthKey: "deep"},
			}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "the config selects policy tsx.depth, which no plan's backend declares: "+
				"the backends declare tsx.width", "the error lists the declared keys")
		})

		t.Run("returns an error for a selection where no backend declares a policy", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Config(workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: wide},
			}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(),
				"the config selects policy tsx.width, and no plan's backend declares a policy",
				"the error is about the missing policies")
		})

		t.Run("returns an error with the plan for a selection that its backend does not declare", func(t *testing.T) {
			t.Parallel()

			b, _ := policyPlans(t)
			_, err := b.Config(workspace.Config{Plans: map[string]workspace.PlanConfig{
				clientPlan: {Policies: map[plugin.PolicyKey]plugin.Choice{depthKey: "deep"}},
			}}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `plan "client": plugin: target tsx does not declare the policy tsx.depth`,
				"the error contains the plan and the key")
			assert.Contains(t, err.Error(), "its policies are tsx.width", "the error lists the backend's keys")
		})

		t.Run("returns an error with the choices for a choice outside them", func(t *testing.T) {
			t.Parallel()

			b, _ := policyPlans(t)
			_, err := b.Config(workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: "huge"},
			}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `policy tsx.width takes one of narrow, wide, full, not "huge"`,
				"the error lists the choices")
		})

		t.Run("returns one error for a defective policy of a backend of two plans", func(t *testing.T) {
			t.Parallel()

			backend := undocumented{fakeBackend{name: clientBackend, target: lowerTarget}}
			_, err := workspace.New().
				Brand(fixtureBrand).
				Targets(lowerTarget).
				Plans(
					workspace.Plan{Name: "first", Generators: []plugin.Generator{mirror("first")}, Backend: backend},
					workspace.Plan{Name: "second", Generators: []plugin.Generator{mirror("second")}, Backend: backend},
				).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "backend tsx-printer declares a defective policy",
				"the error contains the backend")
			assert.Equal(t, strings.Count(err.Error(), "declares a defective policy"), 1,
				"the error contains the defect once")
		})
	})
}

// policyPlans returns a composition of two plans, the client plan and
// the server plan, which render through one printer of the fixture
// target that declares the width policy. It also returns the generator
// of each plan, a context reader, in the order of the plans.
func policyPlans(tb assert.TB) (*workspace.Builder, []*contextReader) {
	tb.Helper()

	readers := []*contextReader{{name: "client-reader"}, {name: "server-reader"}}
	printer := lowerBackend(tb, clientBackend, lowerTarget, camel, widthPolicy)
	return workspace.New().
		Brand(fixtureBrand).
		Targets(lowerTarget).
		Plans(
			workspace.Plan{Name: clientPlan, Generators: []plugin.Generator{readers[0]}, Backend: printer},
			workspace.Plan{Name: "server", Generators: []plugin.Generator{readers[1]}, Backend: printer},
		), readers
}

// readPolicies runs the policy composition under the config, and returns
// the context readers of the client plan and the server plan, in that
// order.
func readPolicies(t *testing.T, cfg workspace.Config) []*contextReader {
	t.Helper()

	b, readers := policyPlans(t)
	cleanRun(t, built(t, b.Config(cfg)), routedIn(t, coretest.StorePath))
	return readers
}
