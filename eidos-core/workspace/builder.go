// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package workspace

import (
	"errors"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
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
	// Backend is carried and validated: exactly one per plan, its
	// target registered. Nothing invokes it at this scope.
	Backend plugin.Backend
}

// Builder collects a composition. Every method appends or sets
// data; nothing validates until [Builder.Build] runs the steps,
// which is what lets one error carry every fault.
type Builder struct {
	annotators []plugin.Annotator
	plans      []Plan
	targets    []plugin.Target
	keys       []func(r *meta.Registry) error
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
	if len(faults) > 0 {
		return nil, errors.Join(faults...)
	}
	return &Workspace{
		keys:       keys,
		directives: dirs,
		annotate:   ann,
		plans:      plans,
	}, nil
}
