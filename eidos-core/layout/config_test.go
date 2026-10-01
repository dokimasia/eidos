// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
)

// The plan every configuration case checks under, and the generator no
// plan contains.
const (
	planName           = "services"
	ghost    plugin.ID = "ghost"
)

// sourceFamilies returns generators whose families are all written
// beside their sources: the families no zero configuration refuses.
func sourceFamilies() map[plugin.ID][]plugin.Output {
	all := families()
	return map[plugin.ID][]plugin.Output{
		stubgen: all[stubgen][:2],
		docgen:  all[docgen],
	}
}

// fault returns a fault's text under the fixture plan.
func fault(text string) string { return `layout: plan "services": ` + text }

// A configuration's contradictions are Build faults, so a misspelled
// generator or tag never waits for a run.
func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    layout.Config
			outputs map[plugin.ID][]plugin.Output
			want    []string
		}{
			{
				name:    "returns no fault for the zero configuration of families beside their sources",
				outputs: sourceFamilies(),
			},
			{
				name:    "returns no fault for a plan directory a per-plan family reads",
				give:    layout.Config{Dir: genDir},
				outputs: families(),
			},
			{
				name: "returns no fault for a generator directory a centralised family reads",
				give: layout.Config{
					Plugins: refine(stubgen, layout.Refinement{Policy: layout.PolicyCentralised, Dir: genDir}),
				},
				outputs: sourceFamilies(),
			},
			{
				name:    "returns a fault for a plan policy outside the vocabulary",
				give:    layout.Config{Policy: layout.Policy(7)},
				outputs: sourceFamilies(),
				want:    []string{fault("Policy(7) is not a layout policy")},
			},
			{
				name:    "returns a fault for an output directory that leaves the workspace root",
				give:    layout.Config{Dir: "../gen"},
				outputs: sourceFamilies(),
				want:    []string{fault(`the output directory "../gen" is not a workspace-relative directory`)},
			},
			{
				name:    "returns a fault for an output directory with a backslash",
				give:    layout.Config{Dir: `gen\out`},
				outputs: sourceFamilies(),
				want:    []string{fault(`the output directory "gen\\out" is not a workspace-relative directory`)},
			},
			{
				name:    "returns a fault for an import base that is not a package path",
				give:    layout.Config{ImportBase: "/example.com/gen"},
				outputs: sourceFamilies(),
				want: []string{
					fault(`the import base "/example.com/gen" is not a slash-separated package path`),
				},
			},
			{
				name:    "returns a fault for a generator refinement of a generator outside the plan",
				give:    layout.Config{Plugins: refine(ghost, layout.Refinement{})},
				outputs: sourceFamilies(),
				want:    []string{fault("a refinement names ghost, which is no generator of the plan")},
			},
			{
				name:    "returns a fault for a generator refinement's policy outside the vocabulary",
				give:    layout.Config{Plugins: refine(stubgen, layout.Refinement{Policy: layout.Policy(9)})},
				outputs: sourceFamilies(),
				want:    []string{fault("stubgen refines to Policy(9), which is not a layout policy")},
			},
			{
				name:    "returns a fault for a generator refinement's filename of two path elements",
				give:    layout.Config{Plugins: refine(stubgen, layout.Refinement{File: "a/b.go"})},
				outputs: sourceFamilies(),
				want:    []string{fault(`stubgen refines its filename to "a/b.go", which is not one path element`)},
			},
			{
				name: "returns a fault for a family refinement of a generator outside the plan",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: ghost}: {},
				}},
				outputs: sourceFamilies(),
				want:    []string{fault("a refinement names ghost, which is no generator of the plan")},
			},
			{
				name: "returns a fault for a family refinement of a tag its generator does not declare",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: "bogus"}: {},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`a refinement names family "bogus" of stubgen, which stubgen does not declare`),
				},
			},
			{
				name: "returns a fault for a family refinement's directory that leaves the workspace root",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: tagTest}: {Policy: layout.PolicyCentralised, Dir: "../x"},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`family "test" of stubgen refines its directory to "../x", ` +
						"which is not a workspace-relative directory"),
				},
			},
			{
				name:    "returns a fault for a per-plan family without a directory",
				outputs: families(),
				want: []string{
					fault(`family "plan" of stubgen writes under a directory, and the plan states none`),
				},
			},
			{
				name:    "returns a fault per family for a centralised policy without a directory",
				give:    layout.Config{Policy: layout.PolicyCentralised},
				outputs: sourceFamilies(),
				want: []string{
					fault(`family "" of docgen writes under a directory, and the plan states none`),
					fault(`family "" of stubgen writes under a directory, and the plan states none`),
					fault(`family "test" of stubgen writes under a directory, and the plan states none`),
				},
			},
			{
				name: "returns a fault for a family directory its policy leaves unread",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: tagTest}: {Dir: "x"},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`family "test" of stubgen is written beside its source, and its directory "x" goes unread`),
				},
			},
			{
				name:    "returns a fault for a generator directory no family reads",
				give:    layout.Config{Plugins: refine(stubgen, layout.Refinement{Dir: "x"})},
				outputs: sourceFamilies(),
				want: []string{
					fault(`no family of stubgen reads the generator's directory "x": ` +
						"each is written beside its source or under a directory of its own"),
				},
			},
			{
				name: "returns the faults of family refinements in plugin then tag order",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: "zeta"}:  {},
					{Plugin: stubgen, Tag: "alpha"}: {},
					{Plugin: docgen, Tag: "zeta"}:   {},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`a refinement names family "zeta" of docgen, which docgen does not declare`),
					fault(`a refinement names family "alpha" of stubgen, which stubgen does not declare`),
					fault(`a refinement names family "zeta" of stubgen, which stubgen does not declare`),
				},
			},
			{
				name: "returns the faults of the plan, the refinements and the families in order",
				give: layout.Config{
					Policy:  layout.Policy(7),
					Plugins: refine(ghost, layout.Refinement{}),
					Families: map[layout.Family]layout.Refinement{
						{Plugin: stubgen, Tag: "bogus"}: {},
					},
				},
				outputs: families(),
				want: []string{
					fault("Policy(7) is not a layout policy"),
					fault("a refinement names ghost, which is no generator of the plan"),
					fault(`a refinement names family "bogus" of stubgen, which stubgen does not declare`),
					fault(`family "plan" of stubgen writes under a directory, and the plan states none`),
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := []string{}
				for _, err := range tt.give.Check(planName, tt.outputs) {
					got = append(got, err.Error())
				}
				want := tt.want
				if want == nil {
					want = []string{}
				}
				assert.Equal(t, got, want, "the faults")
			})
		}
	})
}

// refine returns a generator refinement map of one entry.
func refine(p plugin.ID, r layout.Refinement) map[plugin.ID]layout.Refinement {
	return map[plugin.ID]layout.Refinement{p: r}
}
