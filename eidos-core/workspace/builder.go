// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Config is what the composition populates from: option values per
// plugin, keyed by plugin name, then option key. The typed struct is
// the contract a config file maps onto, and this package defines no
// file format.
type Config struct {
	Options map[string]map[string]any
}

// Plan is one write side: a value, never a plugin. Build validates
// it whole, so a plan that survives names only registered things.
type Plan struct {
	// Name keys the plan's store in the run's report and must be
	// unique across the composition.
	Name string
	// Scope filters what the plan's generators see, and nil admits
	// everything.
	Scope store.Scope
	// Generators run in bucket order within the plan, whatever
	// order they are listed in.
	Generators []plugin.Generator
	// Backend renders the plan's settled store: exactly one per
	// plan, its target registered.
	Backend plugin.Backend
	// Layout derives the path a rendered file takes in the output
	// tree, from its package and its target-spelled name. Nil
	// takes the convention: the package path and the name joined by
	// a slash, and the name alone for a plan file. A composition
	// whose tree is laid out otherwise states its own.
	Layout func(pkg symbol.Identity, name string) string
}

// Builder collects a composition. Every method appends or sets data.
// Nothing validates until [Builder.Build] runs the steps, which lets
// one error join every fault.
type Builder struct {
	annotators []plugin.Annotator
	plans      []Plan
	targets    []plugin.Target
	brand      output.Brand
	open       func() (output.Sink, error)
	keys       []func(r *meta.Registry) error
	rules      []rules.SourceRules
	ignored    []directive.Name
	config     Config
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

// Output declares where a run's rendered files go: open returns a
// fresh sink or an error, and every run that commits calls it once,
// after the render, so each run stages into a sink of its own. A run
// whose open returns neither fails. A run that reports an Error
// opens nothing, and the previous generation of files remains in
// place.
//
// A composition declaring no output stops after the settle, and its
// plans' emit stores are the run's whole product: a composition
// that computes what it would write, and writes nothing.
func (b *Builder) Output(open func() (output.Sink, error)) *Builder {
	b.open = open
	return b
}

// Targets declares the target names this composition recognises,
// which is what a plan's backend resolves against.
func (b *Builder) Targets(ts ...plugin.Target) *Builder {
	b.targets = append(b.targets, ts...)
	return b
}

// Keys registers composition-owned metadata keys, beyond what the
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
	roster, byName, afaults := b.assemble()
	faults = append(faults, afaults...)
	reg, rerr := b.register(roster)
	faults = append(faults, rerr...)
	ann, gens, lerr := lower(roster)
	faults = append(faults, lerr...)
	options, cerr := configure(roster, byName, b.config)
	faults = append(faults, cerr...)
	plans, perr := compilePlans(b.plans, gens, reg.targets)
	faults = append(faults, perr...)
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
		open:        b.open,
		brand:       b.brand,
		fingerprint: fingerprintOf(ann, plans, options),
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
