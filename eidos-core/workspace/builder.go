// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
)

// Config is what the composition populates from: option values per
// plugin, keyed by plugin name, then option key. The typed struct is
// the contract a config file maps onto, and this package defines no
// file format.
type Config struct {
	Options map[string]map[string]any
}

// Plan is one write side: a value, never a plugin. Build validates
// it whole, so a plan that Build accepts names only registered things.
type Plan struct {
	// Name keys the plan's store in the run's report and must be
	// unique across the composition.
	Name string
	// Sources scopes what the plan's generators see. The zero value
	// admits every package.
	Sources Sources
	// DependsOn names the plans whose exports the plan's generators
	// read. The plan generates after each of them has rendered, and
	// commits only where each of them commits. Build refuses a name the
	// composition does not declare, the plan's own name, a name listed
	// twice, and a cycle, naming every plan in it.
	DependsOn []string
	// Generators run in bucket order within the plan, whatever
	// order they are listed in.
	Generators []plugin.Generator
	// Backend renders the plan's settled store: exactly one per
	// plan, its target registered.
	Backend plugin.Backend
	// Layout routes the plan's declarations to files: its policy,
	// its output directory, its import base and its refinements per
	// generator and per family. The zero value writes every file
	// beside its source. Build validates it against the families the
	// plan's generators declare.
	Layout layout.Config
}

// Builder collects a composition. Every method appends or sets data.
// Nothing validates until [Builder.Build] runs the steps, which lets
// one error join every fault.
type Builder struct {
	frontends  []plugin.Frontend
	annotators []plugin.Annotator
	plans      []Plan
	checks     []plugin.WorkspaceCheck
	targets    []plugin.Target
	brand      output.Brand
	open       func() (output.Sink, error)
	ledger     func() (ledger.Ledger, error)
	id         string
	workers    int
	keys       []func(r *meta.Registry) error
	rules      []rules.SourceRules
	ignored    []directive.Name
	config     Config
	memo       Memo
}

// New returns an empty builder.
func New() *Builder {
	return &Builder{}
}

// Brand declares the composition's brand, which Build requires: the
// name the source's carriers open with, the name every stamped
// file's trailer claims ownership under, and the name a load
// refuses the workspace's own outputs by. It is a lowercase letter,
// then lowercase letters, digits and hyphens.
func (b *Builder) Brand(brand output.Brand) *Builder {
	b.brand = brand
	return b
}

// Frontends registers the frontends a run loads its tree with, in
// load order. Build refuses a nil frontend, a frontend without a
// declared version, because every unit key folds the version, an
// empty name, a name another frontend of the composition already has,
// and a frontend named after a kernel phase. A language's frontend and
// backend may share the language's name, as the Go satellite's both
// report under golang.
func (b *Builder) Frontends(fs ...plugin.Frontend) *Builder {
	b.frontends = append(b.frontends, fs...)
	return b
}

// Annotators registers the read side's stamping plugins.
func (b *Builder) Annotators(as ...plugin.Annotator) *Builder {
	b.annotators = append(b.annotators, as...)
	return b
}

// Plans registers the write sides.
func (b *Builder) Plans(ps ...Plan) *Builder {
	b.plans = append(b.plans, ps...)
	return b
}

// Checks registers the workspace checks Close runs, in registration
// order. Build refuses a nil check, a check whose name another plugin
// of the composition has, and a check that reads a plan the
// composition does not declare or names one plan twice. A check takes
// options, keys and capabilities the way any plugin of the composition
// does.
func (b *Builder) Checks(cs ...plugin.WorkspaceCheck) *Builder {
	b.checks = append(b.checks, cs...)
	return b
}

// Output declares where a run's rendered files go: open returns a
// fresh sink or an error, and a run calls it once for each plan it
// stages, after the render, so each plan stages into a sink of its
// own. A run calls it from the plans' goroutines, so open is safe for
// concurrent use. A run whose open returns neither fails. A plan that
// reports an Error, and every plan of a run whose shared phases
// reported one, opens nothing, and the previous generation of its
// files remains in place.
//
// A composition declaring no output stops after the settle, and its
// plans' emit stores are the run's whole product: a composition
// that computes what it would write, and writes nothing.
func (b *Builder) Output(open func() (output.Sink, error)) *Builder {
	b.open = open
	return b
}

// Ledger declares where a run reads the previous run's record and
// writes its own: open returns a fresh ledger, and every run of a
// composition declaring output calls it once before it loads. A
// composition declaring no ledger runs as if no run had ever
// committed: it removes nothing, and records nothing.
func (b *Builder) Ledger(open func() (ledger.Ledger, error)) *Builder {
	b.ledger = open
	return b
}

// Workspace names the workspace in its manifest. Empty leaves the name
// to the ledger, and the disk ledger records the base name of the
// workspace root.
func (b *Builder) Workspace(id string) *Builder {
	b.id = id
	return b
}

// Parallel lets one phase call run up to workers invocations at once:
// an annotator's or a generator's matches inside its bucket. Zero and
// one dispatch sequentially, which is the default, and Build refuses
// a negative count. The output does not depend on the count: the
// placements, slot appends and findings of a phase call apply in
// canonical match order, and the fact store ranks stamps by that
// order. Plans run in parallel whatever the count.
func (b *Builder) Parallel(workers int) *Builder {
	b.workers = workers
	return b
}

// Targets declares the target names this composition recognises,
// which is what a plan's backend resolves against.
func (b *Builder) Targets(ts ...plugin.Target) *Builder {
	b.targets = append(b.targets, ts...)
	return b
}

// Keys registers the composition's own metadata keys, beyond what the
// plugins' own providers register: a consumer's keys, a fixture's.
// The registrations run at Build, in declaration order, through the
// composition's handle on the registry.
func (b *Builder) Keys(register ...func(r *meta.Registry) error) *Builder {
	b.keys = append(b.keys, register...)
	return b
}

// Rules registers the language rules the kernel's walks run
// under, one value per language. A run binds a subject's rules by
// its language, and a language none registered for binds the absent
// rules and warns. Two values for one language are a Build fault.
func (b *Builder) Rules(rs ...rules.SourceRules) *Builder {
	b.rules = append(b.rules, rs...)
	return b
}

// Config hands over the values options populate from.
func (b *Builder) Config(c Config) *Builder {
	b.config = c
	return b
}

// Ignore opts the composition out of reporting unclaimed directives
// under these spellings: directives written under the brand for a
// plugin the composition does not include. A full name ignores one
// directive. A plugin prefix ending in its colon, "legacy:",
// ignores every directive under it. A spelling a registered schema
// claims is a Build fault, because silencing a registered directive
// would hide its validation.
func (b *Builder) Ignore(names ...directive.Name) *Builder {
	b.ignored = append(b.ignored, names...)
	return b
}

// Build runs every step and returns the immutable workspace, or
// one error joining every fault it found. Each step runs even when
// an earlier one found faults, except where a fault empties a
// following check for that one item, so the fault list is complete.
// A composition without a valid brand is a fault, because the load
// reads carriers and the output contract stamps under it.
//
// The configure step writes the config's values into the options
// struct each plugin declared, so a plugin instance belongs to one
// workspace: building one instance into two workspaces leaves both
// reading the second one's values. The fingerprint is taken at
// Build, over the values the config left.
func (b *Builder) Build() (*Workspace, error) {
	faults := b.brandFaults()
	if b.workers < 0 {
		faults = append(faults, fmt.Errorf(
			"workspace: Parallel(%d) is negative, and a phase call runs at least one invocation at a time",
			b.workers,
		))
	}
	if b.memo.Limit < 0 {
		faults = append(faults, fmt.Errorf(
			"workspace: the memo's limit %d is negative, and zero keeps no memo", b.memo.Limit,
		))
	}
	faults = append(faults, frontendFaults(b.frontends)...)
	roster, byName, afaults := b.assemble()
	faults = append(faults, afaults...)
	reg, rerr := b.register(roster)
	faults = append(faults, rerr...)
	ann, gens, lerr := lower(roster)
	faults = append(faults, lerr...)
	options, cerr := configure(roster, byName, b.config)
	faults = append(faults, cerr...)
	plans, perr := compilePlans(b.plans, gens, reg.targets, languages(b.frontends, reg.rules))
	faults = append(faults, perr...)
	order, oerr := orderPlans(plans)
	faults = append(faults, oerr...)
	checks, kerr := compileChecks(b.checks, plans)
	faults = append(faults, kerr...)
	if b.open != nil && b.brand.Valid() {
		faults = append(faults, stampable(plans, b.brand)...)
	}
	if len(faults) > 0 {
		return nil, errors.Join(faults...)
	}
	return &Workspace{
		keys:        reg.keys,
		kernel:      reg.kernel,
		directives:  reg.directives,
		rules:       reg.rules,
		annotate:    ann,
		plans:       plans,
		order:       order,
		checks:      checks,
		frontends:   slices.Clone(b.frontends),
		contracts:   contractsOf(reg.keys),
		open:        b.open,
		ledger:      b.ledger,
		id:          b.id,
		workers:     b.workers,
		brand:       b.brand,
		memo:        b.memo,
		fingerprint: fingerprintOf(ann, plans, checks, options),
	}, nil
}

// brandFaults is the brand step: a composition declares one brand,
// spelled the way [output.Brand.Valid] requires.
func (b *Builder) brandFaults() []error {
	switch {
	case b.brand == "":
		return []error{errors.New(
			"workspace: the composition declares no brand, and carriers and outputs are read and written under it",
		)}
	case !b.brand.Valid():
		return []error{fmt.Errorf(
			"workspace: %q is not a brand: a lowercase letter, then lowercase letters, digits and hyphens",
			string(b.brand),
		)}
	}
	return nil
}
