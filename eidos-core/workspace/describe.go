// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// Component is one plugin of a composition in a [Description].
type Component struct {
	// Name is the name of the plugin.
	Name plugin.ID
	// Version is the version that the plugin declares. It is empty for a
	// plugin that declares no version.
	Version string
	// Bucket is the position of an annotator or a generator in the schedule
	// of its role, counted from one. Every claim and every unit of the plugin
	// records this bucket. Bucket is zero for a frontend, a backend and a
	// check.
	Bucket int
}

// component returns the description of one plugin: its name, the version
// that it declares when it implements [plugin.Versioned], and its bucket.
func component(name plugin.ID, run any, bucket int) Component {
	c := Component{Name: name, Bucket: bucket}
	if v, versioned := run.(plugin.Versioned); versioned {
		c.Version = v.Version()
	}
	return c
}

// PlanDescription is one plan of a composition in a [Description], with the
// refinements of the config applied.
type PlanDescription struct {
	// Name is the name of the plan, and Sources is the scope of its
	// generators.
	Name    string
	Sources Sources
	// DependsOn lists the plans whose exports the plan reads, in the order
	// that the plan declares them.
	DependsOn []string
	// Generate lists the generators of the plan in bucket order.
	Generate []Component
	// Backend is the backend of the plan, and Target is the target that the
	// backend renders.
	Backend Component
	Target  plugin.Target
	// Layout is the routing configuration of the plan.
	Layout layout.Config
}

// CheckDescription is one workspace check in a [Description].
type CheckDescription struct {
	Component
	// Reads lists the plans that the check reads, in composition order. For a
	// check whose Reads method returns nil, it lists every plan.
	Reads []string
}

// Description is a composition as data. [Workspace.Describe] returns it,
// and a command prints it without running the composition. A Description
// shares no memory with the workspace, so a caller may change it.
type Description struct {
	// Brand is the brand of the composition. Name is the name of the
	// workspace, and it is empty when the composition lets the ledger name
	// the workspace.
	Brand output.Brand
	Name  string
	// Fingerprint is the fingerprint of the composition, which
	// [Workspace.Fingerprint] also returns.
	Fingerprint []byte
	// Frontends lists the frontends in load order.
	Frontends []Component
	// Annotate lists the annotators in bucket order.
	Annotate []Component
	// Plans lists the plans in composition order.
	Plans []PlanDescription
	// Order lists the names of the plans in commit order, in which every plan
	// comes after the plans that it depends on.
	Order []string
	// Checks lists the workspace checks in registration order, which is the
	// order in which Close runs them.
	Checks []CheckDescription
}

// Describe returns the composition as data. The description contains the
// brand, the name of the workspace, the fingerprint, the frontends, the
// annotate schedule, the commit order and the checks. For each plan, it
// contains the sources, the dependencies, the generate schedule, the
// backend and the layout. Describe runs nothing, reads nothing outside the
// workspace and cannot fail. Each call returns a new description. Describe
// is safe to call concurrently with every other method of the workspace.
func (w *Workspace) Describe() Description {
	d := Description{
		Brand:       w.brand,
		Name:        w.id,
		Fingerprint: slices.Clone(w.fingerprint),
		Frontends:   make([]Component, 0, len(w.frontends)),
		Annotate:    make([]Component, 0, len(w.annotate)),
		Plans:       make([]PlanDescription, 0, len(w.plans)),
		Order:       make([]string, 0, len(w.order)),
		Checks:      make([]CheckDescription, 0, len(w.checks)),
	}
	for _, f := range w.frontends {
		d.Frontends = append(d.Frontends, component(f.Name(), f, 0))
	}
	for _, a := range w.annotate {
		d.Annotate = append(d.Annotate, component(a.name, a.run, a.bucket))
	}
	for i := range w.plans {
		p := &w.plans[i]
		plan := PlanDescription{
			Name:      p.name,
			Sources:   p.sources,
			DependsOn: make([]string, 0, len(p.deps)),
			Generate:  make([]Component, 0, len(p.entries)),
			Backend:   component(p.backend.Name(), p.backend, 0),
			Target:    p.backend.Target(),
			Layout:    p.routing,
		}
		plan.Sources.Packages = slices.Clone(p.sources.Packages)
		plan.Layout.Plugins = maps.Clone(p.routing.Plugins)
		plan.Layout.Families = maps.Clone(p.routing.Families)
		for _, dep := range p.deps {
			plan.DependsOn = append(plan.DependsOn, w.plans[dep].name)
		}
		for _, g := range p.entries {
			plan.Generate = append(plan.Generate, component(g.name, g.run, g.bucket))
		}
		d.Plans = append(d.Plans, plan)
	}
	for _, i := range w.order {
		d.Order = append(d.Order, w.plans[i].name)
	}
	for _, c := range w.checks {
		check := CheckDescription{Component: component(c.name, c.run, 0), Reads: make([]string, 0, len(c.reads))}
		for _, i := range c.reads {
			check.Reads = append(check.Reads, w.plans[i].name)
		}
		d.Checks = append(d.Checks, check)
	}
	return d
}
