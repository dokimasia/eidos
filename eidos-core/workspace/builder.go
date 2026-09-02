// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Config carries what the composition populates from: option
// values per plugin, keyed by plugin name, then option key. The
// typed struct is the contract a config file maps onto; no file
// format lives here.
type Config struct {
	Options map[string]map[string]any
}

// Plan is one write side: a value, never a plugin. Build validates
// it whole, so a plan that survives names only registered things.
type Plan struct {
	// Name keys the plan's store in the run's report and must be
	// unique across the composition.
	Name string
	// Scope filters what the plan's generators see; nil admits
	// everything.
	Scope store.Scope
	// Generators run in bucket order within the plan, whatever
	// order they are listed in.
	Generators []plugin.Generator
	// Backend renders the plan's settled store: exactly one per
	// plan, its target registered.
	Backend plugin.Backend
	// Layout derives the path a rendered file takes in the output
	// tree, from its owning package and its target-spelled name;
	// nil takes the convention, the package path and the name
	// joined by a slash, and the name alone for a plan file. A
	// composition whose tree is laid out otherwise states its own.
	Layout func(pkg symbol.Identity, name string) string
}

// Builder collects a composition. Every method appends or sets
// data; nothing validates until [Builder.Build] runs the steps,
// which is what lets one error carry every fault.
type Builder struct {
	annotators []plugin.Annotator
	plans      []Plan
	targets    []plugin.Target
	sink       output.Sink
	brand      output.Brand
	keys       []func(r *meta.Registry) error
	ignored    []directive.Name
	config     Config
}

// New returns an empty builder.
func New() *Builder {
	return &Builder{}
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

// Output declares where a run's rendered files go: the sink that
// stages and commits them, and the brand the output contract
// stamps each file's frame with.
//
// A composition declaring none stops after the settle, and its
// plans' emit stores are the run's whole product. That is the
// difference between a composition that answers what it would
// write and one that writes it.
func (b *Builder) Output(sink output.Sink, brand output.Brand) *Builder {
	b.sink, b.brand = sink, brand
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
// The registrations run at Build, in declaration order.
func (b *Builder) Keys(register ...func(r *meta.Registry) error) *Builder {
	b.keys = append(b.keys, register...)
	return b
}

// Config hands over the values options populate from.
func (b *Builder) Config(c Config) *Builder {
	b.config = c
	return b
}

// Ignore opts the composition out of reporting unclaimed
// directives under these spellings: a foreign tool's carriers
// living in the same comments. A full name ignores one directive;
// a plugin prefix ending in its colon, "k8s:", ignores every
// directive under it. A spelling a registered schema claims is a
// Build fault, because silencing a registered directive would hide
// its validation.
func (b *Builder) Ignore(names ...directive.Name) *Builder {
	b.ignored = append(b.ignored, names...)
	return b
}

// Build runs every step and returns the immutable workspace, or
// one error joining every fault it found. Each step runs even when
// an earlier one found faults, except where a fault empties a
// following check for that one item, so the fault list is complete.
func (b *Builder) Build() (*Workspace, error) {
	roster, byName, faults := b.assemble()
	keys, dirs, targets, rerr := b.register(roster)
	faults = append(faults, rerr...)
	ann, gens, lerr := lower(roster)
	faults = append(faults, lerr...)
	faults = append(faults, configure(roster, byName, b.config)...)
	plans, perr := compilePlans(b.plans, gens, targets)
	faults = append(faults, perr...)
	if b.sink != nil {
		faults = append(faults, stampable(plans, b.brand)...)
	}
	if len(faults) > 0 {
		return nil, errors.Join(faults...)
	}
	return &Workspace{
		keys:       keys,
		directives: dirs,
		annotate:   ann,
		plans:      plans,
		sink:       b.sink,
		brand:      b.brand,
	}, nil
}
