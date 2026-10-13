// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// The name and the help text of the command plan.
const (
	planName = "plan"
	planHelp = "Prints the composition of each workspace without running it."
)

// The events of plan, and the roles of a component event.
const (
	eventComponent = "component"
	eventPlan      = "plan"
	roleFrontend   = "frontend"
	roleAnnotator  = "annotator"
	roleCheck      = "check"
)

// component is the JSON form of one plugin of a composition. Turn is 0
// for a plugin outside a schedule, such as a frontend.
type component struct {
	Role    string   `json:"role,omitempty"`
	Name    string   `json:"name"`
	Version string   `json:"version,omitempty"`
	Turn    int      `json:"turn,omitempty"`
	Reads   []string `json:"reads,omitempty"`
}

// sources is the JSON form of the sources of a plan.
type sources struct {
	Lang     string   `json:"lang,omitempty"`
	Packages []string `json:"packages,omitempty"`
	Module   string   `json:"module,omitempty"`
}

// layoutOf is the JSON form of the layout of a plan. Generators has the
// refinements of the generators and of their families, in the form of the
// config file.
type layoutOf struct {
	Policy     string                   `json:"policy"`
	Dir        string                   `json:"dir,omitempty"`
	ImportBase string                   `json:"importBase,omitempty"`
	Generators map[string]*refinementOf `json:"generators,omitempty"`
}

// refinementOf is the JSON form of the refinement of a generator or of a
// family. A field that the refinement leaves unset has no key, and only the
// refinement of a generator has families.
type refinementOf struct {
	Policy   string                   `json:"policy,omitempty"`
	Dir      string                   `json:"dir,omitempty"`
	File     string                   `json:"file,omitempty"`
	Families map[string]*refinementOf `json:"families,omitempty"`
}

// planOf is the JSON form of one plan of a composition. Order is the place
// of the plan in the commit order, and the first plan has 1. Policies has
// the choice of each lowering policy of the backend.
type planOf struct {
	Name      string                             `json:"name"`
	Order     int                                `json:"order"`
	Sources   sources                            `json:"sources"`
	DependsOn []string                           `json:"dependsOn,omitempty"`
	Generate  []component                        `json:"generate"`
	Backend   component                          `json:"backend"`
	Target    string                             `json:"target"`
	Layout    layoutOf                           `json:"layout"`
	Policies  map[plugin.PolicyKey]plugin.Choice `json:"policies,omitempty"`
}

// disabledOf is the JSON form of a plan that the config disables, with the
// generators that it declares, its backend and its target. Disabled is
// always true.
type disabledOf struct {
	Name     string      `json:"name"`
	Disabled bool        `json:"disabled"`
	Generate []component `json:"generate"`
	Backend  component   `json:"backend"`
	Target   string      `json:"target"`
}

// plan is the handler of plan, which prints the composition of each
// workspace without running it.
func (k kernels) plan(ctx context.Context, run *gocli.Run[Global, struct{}]) error {
	return execute(ctx, k.compose, run, nil, func(_ context.Context, x *invocation) int { return x.plan() })
}

// plan renders the description of each workspace. It renders a component
// event for each frontend, annotator and check, and a plan event for each
// plan and for each plan that the config disables.
//
// plan returns [StatusUsage] for a config error, and [StatusOK] otherwise.
func (x *invocation) plan() int {
	found, err := open(x.stdio, x.global, x.compose, nil)
	if err != nil {
		x.r.Error(err)
		return StatusUsage
	}
	for _, m := range found.members {
		if m.Name != "" {
			x.r.Member(m)
		}
		d := m.Workspace.Describe()
		for _, c := range d.Frontends {
			x.component(roleFrontend, c, nil)
		}
		for _, c := range d.Annotate {
			x.component(roleAnnotator, c, nil)
		}
		for _, p := range d.Plans {
			x.planEvent(p, slices.Index(d.Order, p.Name)+1)
		}
		for _, p := range d.Disabled {
			x.disabledEvent(p)
		}
		for _, c := range d.Checks {
			x.component(roleCheck, c.Component, c.Reads)
		}
	}
	return StatusOK
}

// component renders one plugin of a composition in the role role. reads
// are the plans that a check reads.
func (x *invocation) component(role string, c workspace.Component, reads []string) {
	text := role + " " + string(c.Name)
	if c.Turn > 0 {
		text = fmt.Sprintf("%s %d %s", role, c.Turn, c.Name)
	}
	if c.Version != "" {
		text += " " + c.Version
	}
	if len(reads) > 0 {
		text += ": reads " + strings.Join(reads, ", ")
	}
	x.r.Event(eventComponent, &component{
		Role: role, Name: string(c.Name), Version: c.Version, Turn: c.Turn, Reads: reads,
	}, text)
}

// planEvent renders one plan of a composition. order is the place of the
// plan in the commit order.
func (x *invocation) planEvent(p workspace.PlanDescription, order int) {
	generate := make([]component, 0, len(p.Generate))
	sourced := fields(
		"language", string(p.Sources.Lang),
		"packages", strings.Join(p.Sources.Packages, " "),
		"module", p.Sources.Module,
	)
	lines := []string{
		fmt.Sprintf("plan %s: target %s, backend %s, commit order %d", p.Name, p.Target, p.Backend.Name, order),
		"  sources: " + cmp.Or(sourced, "every package"),
	}
	if len(p.DependsOn) > 0 {
		lines = append(lines, "  depends on: "+strings.Join(p.DependsOn, ", "))
	}
	for _, g := range p.Generate {
		generate = append(generate, component{Name: string(g.Name), Version: g.Version, Turn: g.Turn})
		line := fmt.Sprintf("  generator %d %s", g.Turn, g.Name)
		if g.Version != "" {
			line += " " + g.Version
		}
		lines = append(lines, line)
	}
	lines = append(lines, "  layout: "+fields(
		"policy", p.Layout.Policy.String(), "dir", p.Layout.Dir, "import base", p.Layout.ImportBase,
	))
	refined := map[string]*refinementOf{}
	for _, name := range slices.Sorted(maps.Keys(p.Layout.Plugins)) {
		if p.Layout.Plugins[name] == (layout.Refinement{}) {
			continue
		}
		r := refinementOfLayout(p.Layout.Plugins[name])
		refined[string(name)] = r
		lines = append(lines, fmt.Sprintf("  refine generator %s: %s", name,
			fields("policy", r.Policy, "dir", r.Dir, "file", r.File)))
	}
	for _, f := range slices.SortedFunc(maps.Keys(p.Layout.Families), func(a, b layout.Family) int {
		return cmp.Or(cmp.Compare(a.Plugin, b.Plugin), cmp.Compare(a.Tag, b.Tag))
	}) {
		if p.Layout.Families[f] == (layout.Refinement{}) {
			continue
		}
		r := refinementOfLayout(p.Layout.Families[f])
		g := refined[string(f.Plugin)]
		if g == nil {
			g = &refinementOf{}
			refined[string(f.Plugin)] = g
		}
		if g.Families == nil {
			g.Families = map[string]*refinementOf{}
		}
		g.Families[f.Tag] = r
		lines = append(lines, fmt.Sprintf("  refine family %q of %s: %s", f.Tag, f.Plugin,
			fields("policy", r.Policy, "dir", r.Dir, "file", r.File)))
	}
	if len(p.Policies) > 0 {
		pairs := make([]string, 0, 2*len(p.Policies))
		for _, k := range slices.Sorted(maps.Keys(p.Policies)) {
			pairs = append(pairs, string(k), string(p.Policies[k]))
		}
		lines = append(lines, "  policies: "+fields(pairs...))
	}
	x.r.Event(eventPlan, &planOf{
		Name:  p.Name,
		Order: order,
		Sources: sources{
			Lang: string(p.Sources.Lang), Packages: p.Sources.Packages, Module: p.Sources.Module,
		},
		DependsOn: p.DependsOn,
		Generate:  generate,
		Backend:   component{Name: string(p.Backend.Name), Version: p.Backend.Version},
		Target:    string(p.Target),
		Layout: layoutOf{
			Policy: p.Layout.Policy.String(), Dir: p.Layout.Dir, ImportBase: p.Layout.ImportBase, Generators: refined,
		},
		Policies: p.Policies,
	}, strings.Join(lines, "\n"))
}

// disabledEvent renders one plan that the config disables. Text output
// writes a line with its target and its backend, and a line for each
// generator that it declares.
func (x *invocation) disabledEvent(p workspace.PlanDescription) {
	generate := make([]component, 0, len(p.Generate))
	lines := make([]string, 0, 1+len(p.Generate))
	lines = append(lines, fmt.Sprintf("plan %s: disabled by the config file, target %s, backend %s",
		p.Name, p.Target, p.Backend.Name))
	for _, g := range p.Generate {
		generate = append(generate, component{Name: string(g.Name), Version: g.Version})
		lines = append(lines, "  generator "+strings.TrimSpace(string(g.Name)+" "+g.Version))
	}
	x.r.Event(eventPlan, &disabledOf{
		Name: p.Name, Disabled: true, Generate: generate,
		Backend: component{Name: string(p.Backend.Name), Version: p.Backend.Version}, Target: string(p.Target),
	}, strings.Join(lines, "\n"))
}

// refinementOfLayout returns the JSON form of the fields that a refinement
// sets. The policy inherit is a field that the refinement leaves unset.
func refinementOfLayout(r layout.Refinement) *refinementOf {
	out := &refinementOf{Dir: r.Dir, File: r.File}
	if r.Policy != layout.PolicyInherit {
		out.Policy = r.Policy.String()
	}
	return out
}
