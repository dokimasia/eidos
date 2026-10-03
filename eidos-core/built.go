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
// returns, and the rules the role methods dispatch. The role
// methods are on the wrapper types, so a type assertion returns
// exactly the roles the rules imply.
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
	pres     presentation
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

// Templates implements [plugin.TemplateProvider]: the tree
// [Builder.For] declared for the target, else the plugin-level
// tree, and false where neither was declared.
func (b *built) Templates(t plugin.Target) (fs.FS, bool) {
	if tree, own := b.pres.trees[t]; own {
		return tree, true
	}
	return b.pres.tree, b.pres.tree != nil
}

// TemplateTargets implements [plugin.TemplateProvider]: the targets
// [Builder.For] declared a tree for, sorted, and nil where none.
func (b *built) TemplateTargets() []plugin.Target { return b.pres.targets }

// TemplateFuncs implements [plugin.TemplateProvider]: for a target
// [Builder.For] declared, the plugin-level helpers with the
// target's helpers and overrides layered over them; for any other
// target, the plugin-level helpers. Nil where none was declared.
func (b *built) TemplateFuncs(t plugin.Target) template.FuncMap {
	if fm, own := b.pres.layered[t]; own {
		return fm
	}
	return b.pres.funcs
}

// Overrides implements [plugin.TemplateProvider]: the shared names
// the target's overrides replace, sorted, and nil where none.
func (b *built) Overrides(t plugin.Target) []string { return b.pres.overrides[t] }

// Directives returns the schemas the Directive wrappers declared,
// for registration at composition.
func (b *built) Directives() []directive.Schema { return b.schemas }

// Subscriptions returns the gate tuples as data: what every rule
// watches, one record per gated key.
func (b *built) Subscriptions() []plugin.Subscription { return b.subs }

// annotate dispatches the annotate-phase rules on up to the context's
// workers, restricted to the context's selection where it has one, then
// reports the buffered findings in canonical match order and hands the
// context's journal its records. The stamps arrive in the fact store as
// they are made, because the store ranks claims by their rule, subject
// and instance and not by their arrival.
func annotate(b *built, ctx *plugin.AnnotatorContext) error {
	c := newPhaseCall(b, ctx.Index, ctx.Facts, ctx.Sink, nil, ctx.Plugin, ctx.Bucket, ctx.Rules, ctx.Kernel,
		ctx.Workers)
	c.journal = ctx.Journal
	c.restrict(ctx.Select)
	if err := c.run(plugin.PhaseAnnotate); err != nil {
		return err
	}
	c.applyEffects()
	c.deliver()
	return nil
}

// generate dispatches the generate- and emit-phase rules in
// declaration order, each on up to the context's workers and restricted
// to the context's selection where it has one, then applies the
// buffered effects in canonical match order and flushes the
// accumulators into the plan's store after every rule ran, which keeps
// a plugin's own emit invisible to its own emit rules. It hands the
// context's journal its records once the flush has added the units.
// Every handler reads the context's exports through its match.
func generate(b *built, ctx *plugin.GeneratorContext) error {
	c := newPhaseCall(b, ctx.Index, ctx.Facts, ctx.Sink, ctx.Emit, ctx.Plugin, ctx.Bucket, ctx.Rules, ctx.Kernel,
		ctx.Workers)
	c.exports = ctx.Exports
	c.journal = ctx.Journal
	c.restrict(ctx.Select)
	if err := c.run(plugin.PhaseGenerate, plugin.PhaseEmit); err != nil {
		return err
	}
	c.applyEffects()
	c.keyHosts()
	if err := c.flush(ctx.Emit); err != nil {
		return err
	}
	c.deliver()
	return nil
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

// builtDual is a lowered plugin with both roles.
type builtDual struct{ *built }

// Annotate implements [plugin.Annotator].
func (b *builtDual) Annotate(ctx *plugin.AnnotatorContext) error {
	return annotate(b.built, ctx)
}

// Generate implements [plugin.Generator].
func (b *builtDual) Generate(ctx *plugin.GeneratorContext) error {
	return generate(b.built, ctx)
}
