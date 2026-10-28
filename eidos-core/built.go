// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"errors"
	"io/fs"
	"slices"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// built is a lowered declaration. It has the data that every provider
// returns, and the rules that the role methods dispatch. The role methods
// are on the wrapper types, so a type assertion finds exactly the role of
// the typed build.
type built struct {
	name     plugin.ID
	version  string
	outputs  []plugin.Output
	outByTag map[Tag]plugin.Output
	priority map[plugin.Role]int
	provides map[plugin.Role][]plugin.Capability
	requires []plugin.Capability
	after    []plugin.Capability
	options  any
	keys     []func(r *meta.Registry) error
	pres     presentation
	schemas  []directive.Schema
	rules    []flatRule
	subs     []plugin.Subscription
	// removed is the function that [Builder.Removed] declares, and nil
	// where the plugin declares none.
	removed func(plugin.RemovedLine) (diag.Diag, bool)
}

// Name returns the name of the plugin.
func (b *built) Name() plugin.ID { return b.name }

// Version returns the declared version, and "" where the plugin declares
// none.
func (b *built) Version() string { return b.version }

// Outputs returns the declared file families, in declaration order.
func (b *built) Outputs() []plugin.Output { return b.outputs }

// Priority returns the declared priority of the role r, and zero where the
// plugin declares none.
func (b *built) Priority(r plugin.Role) int { return b.priority[r] }

// Provides returns the capabilities that the plugin provides in the role
// r, and nil for a role in which it provides none.
func (b *built) Provides(r plugin.Role) []plugin.Capability { return b.provides[r] }

// Requires returns the capabilities that the plugin requires.
func (b *built) Requires() []plugin.Capability { return b.requires }

// After returns the capabilities whose providers run before the plugin
// where the composition contains them.
func (b *built) After() []plugin.Capability { return b.after }

// Options returns the declared options struct, which is the pointer that
// the plugin constructed, with its defaults. It returns nil where the
// plugin declares none.
func (b *built) Options() any { return b.options }

// Keys implements [plugin.KeyProvider]. It runs the declared registrations
// in order and joins their faults, so the composition reports every fault
// at once.
func (b *built) Keys(r *meta.Registry) error {
	var errs []error
	for _, register := range b.keys {
		if err := register(r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Templates implements [plugin.TemplateProvider]. It returns the tree that
// [Builder.For] declares for the target t, or else the tree of the plugin
// level, and false where the plugin declares neither.
func (b *built) Templates(t plugin.Target) (fs.FS, bool) {
	if tree, own := b.pres.trees[t]; own {
		return tree, true
	}
	return b.pres.tree, b.pres.tree != nil
}

// TemplateTargets implements [plugin.TemplateProvider]. It returns the
// sorted targets for which [Builder.For] declares a tree, and nil where
// the plugin declares none.
func (b *built) TemplateTargets() []plugin.Target { return b.pres.targets }

// TemplateFuncs implements [plugin.TemplateProvider]. For a target that
// [Builder.For] declares, it returns the helpers of the plugin level with
// the helpers and overrides of the target over them. For any other target,
// it returns the helpers of the plugin level. It returns nil where the
// plugin declares none.
func (b *built) TemplateFuncs(t plugin.Target) template.FuncMap {
	if fm, own := b.pres.layered[t]; own {
		return fm
	}
	return b.pres.funcs
}

// Overrides implements [plugin.TemplateProvider]. It returns the sorted
// shared names that the overrides of the target t replace, and nil where
// the target has none.
func (b *built) Overrides(t plugin.Target) []string { return b.pres.overrides[t] }

// Removed implements [plugin.LineSetReader] through the function that
// [Builder.Removed] declares. It reports false for every line where the
// plugin declares none.
func (b *built) Removed(r plugin.RemovedLine) (diag.Diag, bool) {
	if b.removed == nil {
		return diag.Diag{}, false
	}
	return b.removed(r)
}

// Directives returns the schemas that the Directive wrappers declare, for
// registration when the workspace composes.
func (b *built) Directives() []directive.Schema { return b.schemas }

// Subscriptions returns the gates as data. Each record states what one
// rule watches, with one record for each fact gate.
func (b *built) Subscriptions() []plugin.Subscription { return b.subs }

// bind returns the plugin with each gate on a named handle bound to the id
// of its name in r, and b itself where b has no named handle. The bound
// plugin has rules and subscriptions of its own, and shares everything
// else with b, so b does not change. bind returns one error for each name
// that r does not contain under the value type of its handle, and the gate
// on such a name remains unbound.
func (b *built) bind(r *meta.Registry) (*built, error) {
	named := func(p Pred) bool { return p.bind != nil }
	if !slices.ContainsFunc(b.rules, func(fr flatRule) bool { return slices.ContainsFunc(fr.preds, named) }) {
		return b, nil
	}
	bound := *b
	bound.rules = slices.Clone(b.rules)
	var errs []error
	refused := map[meta.KeyName]bool{}
	for i := range bound.rules {
		fr := &bound.rules[i]
		if !slices.ContainsFunc(fr.preds, named) {
			continue
		}
		fr.preds = slices.Clone(fr.preds)
		for j := range fr.preds {
			p := &fr.preds[j]
			if p.bind == nil {
				continue
			}
			id, err := p.bind(r, b.name)
			if err != nil {
				if !refused[p.name] {
					refused[p.name] = true
					errs = append(errs, err)
				}
				continue
			}
			p.id = id
		}
	}
	bound.subs = subscriptionsFor(bound.rules)
	return &bound, errors.Join(errs...)
}

// annotate dispatches the rules of the annotate phase on up to the workers
// of the context, restricted to the selection of the context where it has
// one. It then reports the buffered findings in canonical match order, and
// passes the records to the journal of the context. The stamps arrive in
// the fact store as the handlers make them, because the store ranks claims
// by their rule, subject and instance and not by their arrival. The state
// of the call returns to the shared pool when the call returns.
func annotate(b *built, ctx *plugin.AnnotatorContext) error {
	c := newPhaseCall(b, ctx.Index, ctx.Facts, ctx.Sink, nil, ctx.Plugin, ctx.Turn, ctx.Rules, ctx.Kernel,
		ctx.Workers)
	defer c.release()
	c.journal = ctx.Journal
	c.restrict(ctx.Select)
	if err := c.run(plugin.PhaseAnnotate); err != nil {
		return err
	}
	c.applyEffects()
	c.deliver()
	return nil
}

// generate dispatches the rules of the generate and emit phases in
// declaration order, each on up to the workers of the context, restricted
// to the selection of the context where it has one. After every rule ran,
// it applies the buffered effects in canonical match order and flushes the
// accumulators into the store of the plan, so the emit rules of a plugin
// do not see its own emit. It passes the records to the journal of the
// context once the flush has added the units. A handler reads the exports
// of the context through its match, and translates a type with the target,
// the spoke and the policy of the context. It previews a route with the
// placer of the context. The state of the call returns to the shared pool
// when the call returns.
func generate(b *built, ctx *plugin.GeneratorContext) error {
	c := newPhaseCall(b, ctx.Index, ctx.Facts, ctx.Sink, ctx.Emit, ctx.Plugin, ctx.Turn, ctx.Rules, ctx.Kernel,
		ctx.Workers)
	defer c.release()
	c.exports = ctx.Exports
	c.target, c.types, c.policy, c.placer = ctx.Target, ctx.Types, ctx.Policy, ctx.Placer
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

// builtAnnotator is a lowered plugin whose rules stamp facts.
type builtAnnotator struct{ *built }

// Annotate implements [plugin.Annotator].
func (b *builtAnnotator) Annotate(ctx *plugin.AnnotatorContext) error {
	return annotate(b.built, ctx)
}

// BindKeys implements [plugin.KeyBinder]. It returns an annotator with
// the bound gates, and b itself where b has no gate on a named handle.
func (b *builtAnnotator) BindKeys(r *meta.Registry) (plugin.Plugin, error) {
	bound, err := b.bind(r)
	if bound == b.built {
		return b, err
	}
	return &builtAnnotator{built: bound}, err
}

// builtGenerator is a lowered plugin whose rules emit.
type builtGenerator struct{ *built }

// Generate implements [plugin.Generator].
func (b *builtGenerator) Generate(ctx *plugin.GeneratorContext) error {
	return generate(b.built, ctx)
}

// BindKeys implements [plugin.KeyBinder]. It returns a generator with the
// bound gates, and b itself where b has no gate on a named handle.
func (b *builtGenerator) BindKeys(r *meta.Registry) (plugin.Plugin, error) {
	bound, err := b.bind(r)
	if bound == b.built {
		return b, err
	}
	return &builtGenerator{built: bound}, err
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

// BindKeys implements [plugin.KeyBinder]. It returns a dual plugin with
// the bound gates, and b itself where b has no gate on a named handle.
func (b *builtDual) BindKeys(r *meta.Registry) (plugin.Plugin, error) {
	bound, err := b.bind(r)
	if bound == b.built {
		return b, err
	}
	return &builtDual{built: bound}, err
}
