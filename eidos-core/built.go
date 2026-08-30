// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import (
	"errors"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// built is a lowered declaration: the data every provider surface
// answers, and the rules the role methods dispatch. The role seats
// live on the wrapper types, so a type assertion answers exactly
// the roles the rules imply.
type built struct {
	name     plugin.ID
	version  string
	outputs  []plugin.Output
	outByTag map[Tag]plugin.Output
	priority map[plugin.Role]int
	provides []plugin.Capability
	requires []plugin.Capability
	options  any
	keys     []func(r *meta.Registry) error
	schemas  []directive.Schema
	rules    []flatRule
	subs     []plugin.Subscription
}

// Name answers the plugin's one identity.
func (b *built) Name() plugin.ID { return b.name }

// Version answers the declared version, "" where none was.
func (b *built) Version() string { return b.version }

// Outputs answers the declared file families, in declaration order.
func (b *built) Outputs() []plugin.Output { return b.outputs }

// Priority answers the declared priority for one role seat, zero
// where none was declared.
func (b *built) Priority(r plugin.Role) int { return b.priority[r] }

// Provides answers the declared capability labels.
func (b *built) Provides() []plugin.Capability { return b.provides }

// Requires answers the required capability labels.
func (b *built) Requires() []plugin.Capability { return b.requires }

// Options answers the declared options struct: the same pointer the
// plugin constructed, defaults intact. Nil where none was declared.
func (b *built) Options() any { return b.options }

// Keys implements [plugin.KeyProvider]: the declared registrations
// run in order and their faults join, so the composition reads the
// whole bill.
func (b *built) Keys(r *meta.Registry) error {
	var errs []error
	for _, register := range b.keys {
		if err := register(r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Directives answers the schemas the Directive wrappers carried,
// for registration at composition.
func (b *built) Directives() []directive.Schema { return b.schemas }

// Subscriptions answers the gate tuples as data: what every rule
// watches, one record per gated key.
func (b *built) Subscriptions() []plugin.Subscription { return b.subs }

// annotate dispatches the annotate-phase rules.
func annotate(b *built, ctx *plugin.AnnotatorContext) error {
	rs := newRunState(b, ctx.Index, ctx.Facts, ctx.Sink, nil, ctx.Plugin, ctx.Bucket)
	return rs.run(plugin.PhaseAnnotate)
}

// generate dispatches the generate- and emit-phase rules in
// declaration order, then flushes the accumulators into the plan's
// store — after every rule ran, which is what keeps a plugin's own
// emit invisible to its own emit rules.
func generate(b *built, ctx *plugin.GeneratorContext) error {
	rs := newRunState(b, ctx.Index, ctx.Facts, ctx.Sink, ctx.Emit, ctx.Plugin, ctx.Bucket)
	if err := rs.run(plugin.PhaseGenerate, plugin.PhaseEmit); err != nil {
		return err
	}
	return rs.flush(ctx.Emit)
}

// builtAnnotator is a lowered plugin whose rules stamp only.
type builtAnnotator struct{ *built }

// Annotate implements [plugin.Annotator].
func (b *builtAnnotator) Annotate(ctx *plugin.AnnotatorContext) error {
	return annotate(b.built, ctx)
}

// builtGenerator is a lowered plugin whose rules emit only.
type builtGenerator struct{ *built }

// Generate implements [plugin.Generator].
func (b *builtGenerator) Generate(ctx *plugin.GeneratorContext) error {
	return generate(b.built, ctx)
}

// builtDual is a lowered plugin holding both seats.
type builtDual struct{ *built }

// Annotate implements [plugin.Annotator].
func (b *builtDual) Annotate(ctx *plugin.AnnotatorContext) error {
	return annotate(b.built, ctx)
}

// Generate implements [plugin.Generator].
func (b *builtDual) Generate(ctx *plugin.GeneratorContext) error {
	return generate(b.built, ctx)
}
