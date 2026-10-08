// Copyright ThesmOS B.V. 2026
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

// The names of the described composition: the workspace, the scope and
// the routing of its server plan.
const (
	describedName      = "platform"
	describedPattern   = "svc/..."
	describedDir       = "gen"
	describedFamilyDir = "gen/mirror"
	describedFile      = "mirror.go"
)

// A description lists the composition as Build compiled it, and runs
// nothing.
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
				Annotate: []workspace.Component{{Name: "early", Bucket: 1}, {Name: "late", Bucket: 2}},
				Plans: []workspace.PlanDescription{
					{
						Name:      "bindings",
						DependsOn: []string{"server"},
						Generate:  []workspace.Component{{Name: "bindings-mirror", Bucket: 1}},
						Backend:   workspace.Component{Name: "bindings-printer"},
						Target:    "fixture",
					},
					{
						Name:     "server",
						Sources:  workspace.Sources{Packages: []string{describedPattern}},
						Generate: []workspace.Component{{Name: "server-mirror", Bucket: 2}},
						Backend:  workspace.Component{Name: "server-printer"},
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

		t.Run("returns an empty name for a composition that leaves the name to the ledger", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, built(t, valid()).Describe().Name, "", "the composition names no workspace")
		})

		t.Run("returns a description that shares no memory with the workspace", func(t *testing.T) {
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

// described returns the composition that the description cases describe: a
// named workspace with a frontend, two annotators that their priorities
// order against their declaration, two plans that commit in the opposite
// order of their declaration, the second scoped and routed, and a check
// that reads the second plan.
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
