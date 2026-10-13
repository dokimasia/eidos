// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// The names of the described composition, which are the name of the
// workspace and the scope and routing of its server plan.
const (
	describedName      = "platform"
	describedPattern   = "svc/..."
	describedDir       = "gen"
	describedFamilyDir = "gen/mirror"
	describedFile      = "mirror.go"
)

// TestDescribe tests the description of a composition. A description
// lists the composition as Build compiled it, and runs nothing.
func TestDescribe(t *testing.T) {
	t.Parallel()

	t.Run("Describe", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the composition as data", func(t *testing.T) {
			t.Parallel()

			w := built(t, described())
			want := workspace.Description{
				Brand:       fixtureBrand,
				Name:        describedName,
				Fingerprint: w.Fingerprint(),
				Frontends: []workspace.Component{
					{Name: frontendtest.ScriptedID, Version: frontendtest.NewScripted().Version()},
				},
				Annotate: []workspace.Component{
					{Name: fixtureLowering, Turn: 1}, {Name: "early", Turn: 2}, {Name: "late", Turn: 3},
				},
				Plans: []workspace.PlanDescription{
					{
						Name:      "bindings",
						DependsOn: []string{"server"},
						Generate:  []workspace.Component{{Name: "bindings-mirror", Turn: 1}},
						Backend:   workspace.Component{Name: "fixture" + printerSuffix},
						Target:    "fixture",
					},
					{
						Name:     "server",
						Sources:  workspace.Sources{Packages: []string{describedPattern}},
						Generate: []workspace.Component{{Name: "server-mirror", Turn: 2}},
						Backend:  workspace.Component{Name: "fixture" + printerSuffix},
						Target:   "fixture",
						Layout: layout.Config{
							Policy:  layout.PolicyCentralised,
							Dir:     describedDir,
							Plugins: map[plugin.ID]layout.Refinement{"server-mirror": {File: describedFile}},
							Families: map[layout.Family]layout.Refinement{
								{Plugin: "server-mirror"}: {Dir: describedFamilyDir},
							},
						},
					},
				},
				Order:  []string{"server", "bindings"},
				Checks: []workspace.CheckDescription{{Name: "agree", Reads: []string{"server"}}},
			}
			assert.Equal(t, w.Describe(), want, "the description lists the composition", assert.EquateEmpty())
		})

		t.Run("returns the choice of each policy that a plan's backend declares", func(t *testing.T) {
			t.Parallel()

			b, _ := policyPlans(t)
			plans := built(t, b.Config(workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: wide},
				Plans: map[string]workspace.PlanConfig{
					clientPlan: {Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: full}},
				},
			})).Describe().Plans
			assert.Length(t, plans, 2, "the composition has the client plan and the server plan")
			expect.Equal(t, plans[0].Policies, map[plugin.PolicyKey]plugin.Choice{widthKey: full},
				"the client plan has its own selection")
			expect.Equal(t, plans[1].Policies, map[plugin.PolicyKey]plugin.Choice{widthKey: wide},
				"the server plan has the selection of the config")
		})

		t.Run("returns a plan that the config disables apart from the plans", func(t *testing.T) {
			t.Parallel()

			d := built(t, described().Config(workspace.Config{Plans: map[string]workspace.PlanConfig{
				"bindings": {Disabled: true},
			}})).Describe()
			assert.Equal(t, d.Disabled, []workspace.PlanDescription{{
				Name:     "bindings",
				Generate: []workspace.Component{{Name: "bindings-mirror"}},
				Backend:  workspace.Component{Name: "fixture" + printerSuffix},
				Target:   "fixture",
			}}, "the disabled plan has its generators without a turn, its backend and its target")
		})

		t.Run("returns an empty name for a composition that leaves the name to the ledger", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, built(t, valid()).Describe().Name, "", "the composition does not set a workspace name")
		})

		t.Run("returns a description that does not share memory with the workspace", func(t *testing.T) {
			t.Parallel()

			w := built(t, described())
			first, second := w.Describe(), w.Describe()
			expect.NotEqual(t, first.Fingerprint, second.Fingerprint,
				"each description has a fingerprint of its own", assert.ByIdentity())
			expect.NotEqual(t, first.Plans[1].Sources.Packages, second.Plans[1].Sources.Packages,
				"each description has patterns of its own", assert.ByIdentity())
			expect.NotEqual(t, first.Plans[1].Layout.Plugins, second.Plans[1].Layout.Plugins,
				"each description has generator refinements of its own", assert.ByIdentity())
			expect.NotEqual(t, first.Plans[1].Layout.Families, second.Plans[1].Layout.Families,
				"each description has family refinements of its own", assert.ByIdentity())
		})
	})
}

// described returns the composition of the description cases. It is a
// named workspace with a frontend and two annotators, whose priorities
// order them against their declaration. Its two plans commit in the
// opposite order of their declaration, and the second has a scope and a
// routing. A check reads the second plan.
func described() *workspace.Builder {
	server := planTo("server", "fixture", mirror("server-mirror"))
	server.Sources = workspace.Sources{Packages: []string{describedPattern}}
	server.Layout = layout.Config{
		Policy:   layout.PolicyCentralised,
		Dir:      describedDir,
		Plugins:  map[plugin.ID]layout.Refinement{"server-mirror": {File: describedFile}},
		Families: map[layout.Family]layout.Refinement{{Plugin: "server-mirror"}: {Dir: describedFamilyDir}},
	}
	bindings := planTo("bindings", "fixture", mirror("bindings-mirror"))
	bindings.DependsOn = []string{"server"}
	return workspace.New().
		Brand(fixtureBrand).
		Workspace(describedName).
		Frontends(frontendtest.NewScripted()).
		Annotators(stamperAt("late", 2, nil, nil, quiet), stamperAt("early", 1, nil, nil, quiet)).
		Targets("fixture").
		Plans(bindings, server).
		Checks(&recordingCheck{name: "agree", reads: []string{"server"}})
}
