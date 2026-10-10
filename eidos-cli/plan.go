// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// The events of plan, and the roles of a component event.
const (
	eventComponent = "component"
	eventPlan      = "plan"
	roleFrontend   = "frontend"
	roleAnnotator  = "annotator"
	roleCheck      = "check"
)

// component is the JSON form of one plugin of a composition. Bucket is 0
// for a plugin outside a schedule, such as a frontend.
type component struct {
	Role    string   `json:"role,omitempty"`
	Name    string   `json:"name"`
	Version string   `json:"version,omitempty"`
	Bucket  int      `json:"bucket,omitempty"`
	Reads   []string `json:"reads,omitempty"`
}

// sources is the JSON form of the sources of a plan.
type sources struct {
	Lang     string   `json:"lang,omitempty"`
	Packages []string `json:"packages,omitempty"`
	Module   string   `json:"module,omitempty"`
}

// layoutOf is the JSON form of the layout of a plan.
type layoutOf struct {
	Policy     string `json:"policy"`
	Dir        string `json:"dir,omitempty"`
	ImportBase string `json:"importBase,omitempty"`
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

// planCommand returns the command plan, which prints the composition of
// each workspace without running it.
func planCommand(compose Compose) *kernel {
	return &kernel{
		name:     "plan",
		synopsis: "Prints the composition of each workspace without running it.",
		compose:  compose,
		define: func(*flag.FlagSet) runner {
			return func(_ context.Context, x *invocation) int { return x.plan() }
		},
	}
}

// plan renders the description of each workspace. It renders a component
// event for each frontend, annotator and check, and a plan event for each
// plan.
//
// plan returns [StatusUsage] for a config error, and [StatusOK] otherwise.
func (x *invocation) plan() int {
	found, err := open(x.stdio, x.flags, x.k.compose)
	if err != nil {
		x.r.Error(err)
		return StatusUsage
	}
	for _, m := range found.members {
		if m.Name != "" {
			x.r.Workspace(m)
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
	if c.Bucket > 0 {
		text = fmt.Sprintf("%s %d %s", role, c.Bucket, c.Name)
	}
	if c.Version != "" {
		text += " " + c.Version
	}
	if len(reads) > 0 {
		text += ": reads " + strings.Join(reads, ", ")
	}
	x.r.Event(eventComponent, &component{
		Role: role, Name: string(c.Name), Version: c.Version, Bucket: c.Bucket, Reads: reads,
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
		generate = append(generate, component{Name: string(g.Name), Version: g.Version, Bucket: g.Bucket})
		line := fmt.Sprintf("  generator %d %s", g.Bucket, g.Name)
		if g.Version != "" {
			line += " " + g.Version
		}
		lines = append(lines, line)
	}
	lines = append(lines, "  layout: "+fields(
		"policy", p.Layout.Policy.String(), "dir", p.Layout.Dir, "import base", p.Layout.ImportBase,
	))
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
			Policy: p.Layout.Policy.String(), Dir: p.Layout.Dir, ImportBase: p.Layout.ImportBase,
		},
		Policies: p.Policies,
	}, strings.Join(lines, "\n"))
}
