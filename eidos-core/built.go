// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"errors"
	"io/fs"
	"text/template"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// built is a lowered declaration: the data every provider surface
// returns, and the rules the role methods dispatch. The role methods
// live on the wrapper types, so a type assertion returns exactly
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
	trees    map[plugin.Target]fs.FS
	schemas  []directive.Schema
	rules    []flatRule
	subs     []plugin.Subscription
}

// Name returns the plugin's one identity.
func (b *built) Name() plugin.ID { return b.name }

// Version returns the declared version, "" where none was.
func (b *built) Version() string { return b.version }

// Outputs returns the declared file families, in declaration order.
func (b *built) Outputs() []plugin.Output { return b.outputs }

// Priority returns the declared priority for one role, zero
// where none was declared.
func (b *built) Priority(r plugin.Role) int { return b.priority[r] }

// Provides returns the declared capability labels.
func (b *built) Provides() []plugin.Capability { return b.provides }

// Requires returns the required capability labels.
func (b *built) Requires() []plugin.Capability { return b.requires }

// Options returns the declared options struct: the same pointer the
// plugin constructed, defaults intact. Nil where none was declared.
func (b *built) Options() any { return b.options }

// Keys implements [plugin.KeyProvider]: the declared registrations
// run in order and their faults join, so the composition reads
// every fault at once.
func (b *built) Keys(r *meta.Registry) error {
	var errs []error
	for _, register := range b.keys {
		if err := register(r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Templates implements [plugin.TemplateProvider]: the declared
// tree for one target, absent where none was declared.
func (b *built) Templates(t plugin.Target) (fs.FS, bool) {
	tree, held := b.trees[t]
	return tree, held
}

// TemplateFuncs implements [plugin.TemplateProvider]. The facade
// carries no helper declaration: a plugin whose templates need
// helpers implements the provider directly.
func (*built) TemplateFuncs(plugin.Target) template.FuncMap { return nil }

// Overrides implements [plugin.TemplateProvider]. The facade
// carries no override declaration, so a facade plugin never
// replaces a shared vocabulary name.
func (*built) Overrides() []string { return nil }

// Directives returns the schemas the Directive wrappers carried,
// for registration at composition.
func (b *built) Directives() []directive.Schema { return b.schemas }

// Subscriptions returns the gate tuples as data: what every rule
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

// builtDual is a lowered plugin holding both roles.
type builtDual struct{ *built }

// Annotate implements [plugin.Annotator].
func (b *builtDual) Annotate(ctx *plugin.AnnotatorContext) error {
	return annotate(b.built, ctx)
}

// Generate implements [plugin.Generator].
func (b *builtDual) Generate(ctx *plugin.GeneratorContext) error {
	return generate(b.built, ctx)
}
