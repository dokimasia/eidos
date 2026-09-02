// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"io/fs"
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Builder accumulates a plugin's declaration. Everything on it is
// data; only the handlers inside rules and the key registrations
// are functions. Build freezes it, and a Builder is not reused
// afterwards.
type Builder struct {
	name     plugin.ID
	version  string
	outputs  []plugin.Output
	priority map[plugin.Role]int
	provides []plugin.Capability
	requires []plugin.Capability
	options  any
	keys     []func(r *meta.Registry) error
	trees    []targetTree
	rules    []Rule
}

// targetTree is one declared template tree, keyed by target.
type targetTree struct {
	target plugin.Target
	tree   fs.FS
}

// NewPlugin starts a plugin declaration.
func NewPlugin(name plugin.ID) *Builder {
	return &Builder{name: name, priority: map[plugin.Role]int{}}
}

// Version sets the version the composition fingerprint folds in:
// bump it with any change to what the plugin produces.
func (b *Builder) Version(v string) *Builder {
	b.version = v
	return b
}

// Output declares one file family; repeatable, one declaration per
// family.
func (b *Builder) Output(o plugin.Output) *Builder {
	b.outputs = append(b.outputs, o)
	return b
}

// Priority sets one role's priority.
func (b *Builder) Priority(r plugin.Role, p int) *Builder {
	b.priority[r] = p
	return b
}

// Provides declares the capability labels this plugin stands
// behind.
func (b *Builder) Provides(caps ...plugin.Capability) *Builder {
	b.provides = append(b.provides, caps...)
	return b
}

// Requires declares the capabilities this plugin runs after.
func (b *Builder) Requires(caps ...plugin.Capability) *Builder {
	b.requires = append(b.requires, caps...)
	return b
}

// Options declares the plugin's tagged options struct: a pointer
// whose constructed values are the defaults. The composition
// populates it; Build only carries it.
func (b *Builder) Options(cfg any) *Builder {
	b.options = cfg
	return b
}

// Keys declares the registration the plugin performs at
// composition: claim the namespace, register the keys, keep the
// typed handles. The built value returns it through
// [plugin.KeyProvider]; repeated declarations run in order.
func (b *Builder) Keys(register func(r *meta.Registry) error) *Builder {
	b.keys = append(b.keys, register)
	return b
}

// Templates declares the plugin's template tree for one target;
// repeatable, one tree per target. The built value returns it
// through [plugin.TemplateProvider]: the tree a render pass
// resolves this plugin's template references in.
func (b *Builder) Templates(t plugin.Target, tree fs.FS) *Builder {
	b.trees = append(b.trees, targetTree{target: t, tree: tree})
	return b
}

// Handle registers rules, in declaration order.
func (b *Builder) Handle(rules ...Rule) *Builder {
	b.rules = append(b.rules, rules...)
	return b
}

// Build freezes the declaration and returns the lowered plugin.
//
// The dynamic type implements the roles the rules imply and no
// others: Stamper rules make an Annotator, Emitter rules a
// Generator, a mixed set both. It also implements
// [plugin.Subscribed] and the provider interfaces, which return
// what was declared and empty where nothing was.
//
// Build panics on a declaration defect: an empty name, no rules, a
// duplicate output tag, an empty output word, a zero cardinality,
// an empty capability label, a nil, zero-target or duplicate
// template tree, a directive name carried by two wrappers, a rule
// gating on two directives, a gate wrapped around a graph rule, a
// zero predicate. A wrong declaration is a bug in
// the plugin's own constructor and panics on the first Build in any
// test, before a run exists; composition faults stay collected
// errors where the workspace composes.
func (b *Builder) Build() plugin.Plugin {
	if b.name == "" {
		panic("eidos: NewPlugin with an empty name")
	}
	name := string(b.name)
	if len(b.rules) == 0 {
		panic("eidos: " + name + " declares no rules")
	}
	outByTag := make(map[Tag]plugin.Output, len(b.outputs))
	for _, o := range b.outputs {
		if o.Per == 0 {
			panic("eidos: " + name + " declares a family with no cardinality")
		}
		if o.Word == "" {
			panic("eidos: " + name + " declares a family with no word")
		}
		if _, taken := outByTag[Tag(o.Tag)]; taken {
			panic("eidos: " + name + " declares output tag " +
				strconv.Quote(o.Tag) + " twice")
		}
		outByTag[Tag(o.Tag)] = o
	}
	for _, c := range slices.Concat(b.provides, b.requires) {
		if c == "" {
			panic("eidos: " + name + " declares an empty capability label")
		}
	}
	for _, register := range b.keys {
		if register == nil {
			panic("eidos: " + name + " declares a nil key registration")
		}
	}
	trees := make(map[plugin.Target]fs.FS, len(b.trees))
	for _, tt := range b.trees {
		if tt.target == "" {
			panic("eidos: " + name + " declares a template tree for the zero target")
		}
		if tt.tree == nil {
			panic("eidos: " + name + " declares a nil template tree")
		}
		if _, taken := trees[tt.target]; taken {
			panic("eidos: " + name + " declares the " +
				strconv.Quote(string(tt.target)) + " template tree twice")
		}
		trees[tt.target] = tt.tree
	}

	var rules []flatRule
	flatten(name, b.rules, nil, nil, &rules)

	schemas := collectSchemas(name, rules)
	base := &built{
		name:     b.name,
		version:  b.version,
		outputs:  slices.Clone(b.outputs),
		outByTag: outByTag,
		priority: b.priority,
		provides: slices.Clone(b.provides),
		requires: slices.Clone(b.requires),
		options:  b.options,
		keys:     slices.Clone(b.keys),
		trees:    trees,
		schemas:  schemas,
		rules:    rules,
		subs:     subscriptionsFor(rules),
	}

	annotates, generates := false, false
	for _, fr := range rules {
		switch fr.phase {
		case plugin.PhaseAnnotate:
			annotates = true
		case plugin.PhaseGenerate, plugin.PhaseEmit:
			generates = true
		}
	}
	switch {
	case annotates && generates:
		return &builtDual{built: base}
	case annotates:
		return &builtAnnotator{built: base}
	default:
		return &builtGenerator{built: base}
	}
}

// flatten lowers the rule tree to leaves, accumulating the gates
// wrappers carried and refusing the shapes that gate nothing.
func flatten(
	name string, rules []Rule, preds []Pred, schema *directive.Schema,
	out *[]flatRule,
) {
	for _, r := range rules {
		held := preds
		if len(r.preds) > 0 {
			held = slices.Concat(preds, r.preds)
		}
		sch := schema
		if r.schema != nil {
			if sch != nil {
				panic("eidos: " + name + " gates one rule on two directives")
			}
			sch = r.schema
		}
		if r.leaf == nil {
			flatten(name, r.children, held, sch, out)
			continue
		}
		for _, p := range held {
			if p.test == nil {
				panic("eidos: " + name + " gates on a zero predicate")
			}
			if p.id == 0 {
				panic("eidos: " + name + " gates on an unregistered key;" +
					" a gate reads its key when the rule is declared")
			}
		}
		if r.leaf.graph && (sch != nil || len(held) > 0) {
			panic("eidos: " + name +
				" gates a graph rule, which has no subject to gate on")
		}
		*out = append(*out, flatRule{
			ordinal: plugin.RuleID(len(*out)),
			kind:    r.leaf.kind,
			phase:   r.leaf.phase,
			graph:   r.leaf.graph,
			schema:  sch,
			preds:   held,
			invoke:  r.leaf.invoke,
		})
	}
}

// flatRule is one lowered rule: the trigger, its accumulated gates,
// and its stable ordinal in declaration order.
type flatRule struct {
	ordinal plugin.RuleID
	kind    symbol.Kind
	phase   plugin.Phase
	graph   bool
	schema  *directive.Schema
	preds   []Pred
	invoke  func(inv invocation) error
}

// collectSchemas gathers the schemas the wrappers carried, one per
// name: many rules under one wrapper share one schema, and a name
// carried by two wrappers is refused, so one wrapper gates them
// all.
func collectSchemas(name string, rules []flatRule) []directive.Schema {
	byName := map[directive.Name]*directive.Schema{}
	var out []directive.Schema
	for _, fr := range rules {
		if fr.schema == nil {
			continue
		}
		canonical := fr.schema.Canonical()
		if held, taken := byName[canonical]; taken {
			if held != fr.schema {
				panic("eidos: " + name + " carries directive " +
					string(canonical) + " in two wrappers; one wrapper gates them all")
			}
			continue
		}
		byName[canonical] = fr.schema
		out = append(out, *fr.schema)
	}
	return out
}

// subscriptionsFor lowers the gates to records: one per rule, and
// one per gated key where a rule holds several.
func subscriptionsFor(rules []flatRule) []plugin.Subscription {
	var subs []plugin.Subscription
	for _, fr := range rules {
		var gate directive.Name
		if fr.schema != nil {
			gate = fr.schema.Canonical()
		}
		if len(fr.preds) == 0 {
			subs = append(subs, plugin.Subscription{
				Rule: fr.ordinal, Kind: fr.kind, Directive: gate, Phase: fr.phase,
			})
			continue
		}
		for _, p := range fr.preds {
			subs = append(subs, plugin.Subscription{
				Rule: fr.ordinal, Kind: fr.kind, Directive: gate,
				FactKey: p.id, Phase: fr.phase,
			})
		}
	}
	return subs
}
