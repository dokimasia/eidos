// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"text/template"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Builder collects the declaration of a plugin. The declaration is data,
// except the handlers of the rules, the key registrations and the template
// helpers, which are functions. Every method returns the builder. Each of
// [Builder.Generator], [Builder.Annotator] and [Builder.Dual] lowers the
// declaration to one role, and a builder is not reused after it.
//
// # Lowering
//
// The built plugin implements its role, [plugin.Subscribed],
// [plugin.KeyBinder] and the provider interfaces. Each provider returns
// what the plugin declares, and nothing where the plugin declares nothing.
//
// [plugin.DirectiveProvider] returns the schema of each [Directive]
// wrapper in the order of the declaration, a wrapper around no rules
// included. A typed build sets an empty Kinds of a schema to the kinds of
// the rules under the wrapper, in the order of [symbol.Kind]. For an
// override schema, it also adds the kinds of the rules of the plugin that
// no directive gates. An [OnGraph] rule has no subject, so it adds no
// kind. A typed build leaves the field empty where one of those rules is
// an [OnEmit] rule, because the origin of an emitted value can have any
// kind.
//
// # Declaration defects
//
// A typed build panics on a defect of the declaration, because a wrong
// declaration is a bug in the constructor of the plugin. The panic occurs
// at the first build in any test, before a run exists. The workspace
// returns the faults of a composition as errors. The defects are:
//
//   - an empty name, no rules, and a [Where] or [Gated] wrapper around no
//     rules
//   - a [Directive] wrapper around no rules whose schema states no Kinds
//     and does not override
//   - a duplicate output tag, an empty output word, a zero cardinality,
//     and a family template that is not a slash-separated path inside a
//     tree
//   - an empty capability label and a nil key registration
//   - a capability of [Builder.ProvidesIn] in a role other than the
//     annotator role and the generator role, or in a role that the typed
//     build lacks
//   - a directive name in two wrappers, and explicit Kinds that leave out
//     the kind of a rule under the wrapper other than an [OnEmit] rule
//   - a [Gated] name that is not a kernel directive, a rule gated on two
//     directives, and a gate around a graph rule
//   - a zero predicate and a gate on the zero key
//   - a nil tree or helper map, two trees or one helper name twice at one
//     level, a helper that text/template refuses, and the zero option
//   - a [Builder.For] that names the zero target, names a target twice or
//     declares no option
//
// A gate on a named handle binds when the workspace builds, through
// [plugin.KeyBinder].
//
// # Concurrency
//
// A Builder is not safe for concurrent use. The constructor of a plugin
// declares it on one goroutine.
//
// # Allocation contract
//
// [NewPlugin] allocates the builder and its map of priorities. A method
// that appends to a list of the declaration allocates the list on its
// first call and grows it on a later one, and a method that sets one
// value allocates nothing. A typed build allocates in proportion to the
// declaration. It allocates the table of output families, the lowered
// rules and the predicates that each inherits, the built plugin with its
// copies of the declared lists, the schemas with the kinds that it
// derives, the subscriptions, and the role wrapper. A declaration of one
// family, a graph rule and two emit rules under one fact gate allocates 13
// times.
type Builder struct {
	name     plugin.ID
	version  string
	outputs  []plugin.Output
	priority map[plugin.Role]int
	provides []plugin.Capability
	scoped   []scopedCapability
	requires []plugin.Capability
	after    []plugin.Capability
	options  any
	keys     []func(r *meta.Registry) error
	defaults []TargetOption
	layers   []targetLayer
	rules    []Rule
}

// targetLayer is one [Builder.For] declaration. It has the target and its
// options, in declaration order.
type targetLayer struct {
	target plugin.Target
	opts   []TargetOption
}

// scopedCapability is one capability of a [Builder.ProvidesIn]
// declaration, with the role that provides it.
type scopedCapability struct {
	role  plugin.Role
	label plugin.Capability
}

// Dual is a plugin with an annotator half and a generator half. Each half
// runs at the priority of its own role.
type Dual interface {
	plugin.Annotator
	plugin.Generator
}

// NewPlugin starts the declaration of a plugin named name. It allocates
// the builder and its map of priorities, two allocations.
func NewPlugin(name plugin.ID) *Builder {
	return &Builder{name: name, priority: map[plugin.Role]int{}}
}

// Version sets the version that the composition fingerprint includes.
// Raise it with every change to what the plugin produces. Version
// allocates nothing.
func (b *Builder) Version(v string) *Builder {
	b.version = v
	return b
}

// Output declares one file family, and a plugin calls it once for each
// family. A first call allocates the list of families, and a later call
// allocates only to grow it.
func (b *Builder) Output(o plugin.Output) *Builder {
	b.outputs = append(b.outputs, o)
	return b
}

// Priority sets the priority of the role r, which is 0 where the plugin
// does not set it. A priority places the plugin against every plugin of
// the role, and [Builder.Requires] and [Builder.After] order it against
// one provider. A first call allocates the first group of the map of
// priorities.
func (b *Builder) Priority(r plugin.Role, p int) *Builder {
	b.priority[r] = p
	return b
}

// Provides declares the capabilities that the plugin provides in each of
// its roles. A plugin that requires one of them, or runs after one of
// them, runs after this plugin. A first call allocates the list of labels,
// and a later call allocates only to grow it.
func (b *Builder) Provides(caps ...plugin.Capability) *Builder {
	b.provides = append(b.provides, caps...)
	return b
}

// ProvidesIn declares capabilities that the plugin provides in the role r
// alone. Such a capability orders the plugins of r alone. A plugin with
// both roles provides a capability of its annotator half this way. A
// generator that runs after the capability then has no order against the
// generator half, so the generator half can require that generator. A
// first call allocates the list of these capabilities, and a later call
// allocates only to grow it.
func (b *Builder) ProvidesIn(r plugin.Role, caps ...plugin.Capability) *Builder {
	b.scoped = slices.Grow(b.scoped, len(caps))
	for _, c := range caps {
		b.scoped = append(b.scoped, scopedCapability{role: r, label: c})
	}
	return b
}

// Requires declares the capabilities that the plugin requires. The
// workspace refuses a composition without a provider of each of them, and
// runs the providers before the plugin. In a plugin with both roles, a
// capability that a generator provides orders the generator half alone,
// because every annotator runs before the generators. A first call
// allocates the list of labels, and a later call allocates only to grow
// it.
func (b *Builder) Requires(caps ...plugin.Capability) *Builder {
	b.requires = append(b.requires, caps...)
	return b
}

// After declares the capabilities whose providers run before the plugin
// where the composition contains them. The workspace requires no provider
// of them. In a plugin with both roles, a capability that a generator
// provides orders the generator half alone, because every annotator runs
// before the generators. A first call allocates the list of labels, and a
// later call allocates only to grow it.
func (b *Builder) After(caps ...plugin.Capability) *Builder {
	b.after = append(b.after, caps...)
	return b
}

// Options declares the tagged options struct of the plugin, a pointer
// whose constructed values are the defaults. The workspace writes the
// values of the config into it, and a typed build keeps the pointer.
// Options allocates nothing.
func (b *Builder) Options(cfg any) *Builder {
	b.options = cfg
	return b
}

// Keys declares a registration that the plugin runs when the workspace
// composes. The registration claims the namespace of the plugin,
// registers its keys and keeps the typed handles. The built plugin runs
// every registration through [plugin.KeyProvider], in declaration order.
// A first call allocates the list of registrations, and a later call
// allocates only to grow it.
func (b *Builder) Keys(register func(r *meta.Registry) error) *Builder {
	b.keys = append(b.keys, register)
	return b
}

// Templates declares the template tree of the plugin. A plan resolves the
// template references of the plugin in this tree, for every target
// without a tree of its own under [Builder.For]. The built plugin returns
// the tree through [plugin.TemplateProvider]. A typed build panics on a
// nil tree and on a second tree. A first call of Templates or
// [Builder.Funcs] allocates the list of options of the plugin level, and a
// later call allocates only to grow it.
func (b *Builder) Templates(tree fs.FS) *Builder {
	b.defaults = append(b.defaults, Templates(tree))
	return b
}

// Funcs declares helpers that the templates of the plugin call in the
// plans of every target, beside the shared vocabulary of the backend. A
// plugin can call Funcs more than once. A typed build panics on a name
// declared twice, on a nil map and on a function that text/template
// refuses. A helper that works on a type takes the spelling that the
// shared vocabulary returns in the template, because the vocabulary
// records the imports of the file when it spells a type. A first call of
// Funcs or [Builder.Templates] allocates the list of options of the plugin
// level, and a later call allocates only to grow it.
func (b *Builder) Funcs(fm template.FuncMap) *Builder {
	b.defaults = append(b.defaults, Funcs(fm))
	return b
}

// For declares the presentation of the target t over the declarations of
// the plugin level. [Templates] replaces the tree of the plugin level,
// [Funcs] adds or replaces helpers, and [Overrides] replaces names in the
// shared vocabulary of the target. A plugin that declares trees through
// For alone serves those targets alone, and the workspace refuses a plan
// of another target. A typed build panics on the zero target, on a target
// declared twice and on a For without options. Each call copies its
// options, one allocation, and a first call allocates the list of
// targets.
func (b *Builder) For(t plugin.Target, opts ...TargetOption) *Builder {
	b.layers = append(b.layers, targetLayer{target: t, opts: slices.Clone(opts)})
	return b
}

// Handle registers rules, in declaration order. A first call allocates
// the list of rules, and a later call allocates only to grow it.
func (b *Builder) Handle(rules ...Rule) *Builder {
	b.rules = append(b.rules, rules...)
	return b
}

// Generator lowers the declaration to a generator, as the documentation
// of [Builder] states. It panics where a rule stamps facts, because a
// plugin with such a rule is an annotator or a dual plugin. It also panics
// on a capability of the annotator role, which only a dual plugin has.
func (b *Builder) Generator() plugin.Generator {
	base, annotates, _ := b.lower()
	if annotates {
		panic("eidos: " + string(b.name) + " declares a rule that stamps facts, so build it with Annotator or Dual")
	}
	if slices.ContainsFunc(b.scoped, func(s scopedCapability) bool { return s.role == plugin.RoleAnnotator }) {
		panic("eidos: " + string(b.name) + " provides a capability in the annotator role, so build it with Dual")
	}
	return &builtGenerator{built: base}
}

// Annotator lowers the declaration to an annotator, as the documentation
// of [Builder] states. It panics where a rule emits, because a plugin
// with such a rule is a generator or a dual plugin. It also panics on a
// capability of the generator role, which only a dual plugin has.
func (b *Builder) Annotator() plugin.Annotator {
	base, _, generates := b.lower()
	if generates {
		panic("eidos: " + string(b.name) + " declares a rule that emits, so build it with Generator or Dual")
	}
	if slices.ContainsFunc(b.scoped, func(s scopedCapability) bool { return s.role == plugin.RoleGenerator }) {
		panic("eidos: " + string(b.name) + " provides a capability in the generator role, so build it with Dual")
	}
	return &builtAnnotator{built: base}
}

// Dual lowers the declaration to a plugin that annotates and generates, as
// the documentation of [Builder] states. It panics unless the declaration
// has a rule that stamps facts and a rule that emits.
func (b *Builder) Dual() Dual {
	base, annotates, generates := b.lower()
	if !annotates {
		panic("eidos: " + string(b.name) + " declares no rule that stamps facts, so build it with Generator")
	}
	if !generates {
		panic("eidos: " + string(b.name) + " declares no rule that emits, so build it with Annotator")
	}
	return &builtDual{built: base}
}

// lower freezes the declaration and returns the lowered plugin. It also
// reports whether a rule stamps facts and whether a rule emits. lower
// panics on a defect of the declaration, as the documentation of
// [Builder] states.
func (b *Builder) lower() (base *built, annotates, generates bool) {
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
		if o.Template != "" && !fs.ValidPath(o.Template) {
			panic("eidos: " + name + " declares the family template " + strconv.Quote(o.Template) +
				", which is no slash-separated path inside a tree")
		}
		if _, taken := outByTag[Tag(o.Tag)]; taken {
			panic("eidos: " + name + " declares output tag " +
				strconv.Quote(o.Tag) + " twice")
		}
		outByTag[Tag(o.Tag)] = o
	}
	for _, c := range slices.Concat(b.provides, b.requires, b.after) {
		if c == "" {
			panic("eidos: " + name + " declares an empty capability label")
		}
	}
	for _, s := range b.scoped {
		if s.role != plugin.RoleAnnotator && s.role != plugin.RoleGenerator {
			panic("eidos: " + name + " provides capability " + strconv.Quote(string(s.label)) + " in role " +
				s.role.String() + ", which is no role of a plugin")
		}
		if s.label == "" {
			panic("eidos: " + name + " declares an empty capability label")
		}
	}
	for _, register := range b.keys {
		if register == nil {
			panic("eidos: " + name + " declares a nil key registration")
		}
	}
	pres := resolvePresentation(name, b.defaults, b.layers)

	var (
		rules    []flatRule
		wrappers []*directive.Schema
	)
	flatten(name, b.rules, nil, nil, "", &rules, &wrappers)

	schemas := collectSchemas(name, wrappers, rules)
	base = &built{
		name:     b.name,
		version:  b.version,
		outputs:  slices.Clone(b.outputs),
		outByTag: outByTag,
		priority: maps.Clone(b.priority),
		provides: rolesOf(b.provides, b.scoped),
		requires: slices.Clone(b.requires),
		after:    slices.Clone(b.after),
		options:  b.options,
		keys:     slices.Clone(b.keys),
		pres:     pres,
		schemas:  schemas,
		rules:    rules,
		subs:     subscriptionsFor(rules),
	}
	for _, fr := range rules {
		switch fr.phase {
		case plugin.PhaseAnnotate:
			annotates = true
		case plugin.PhaseGenerate, plugin.PhaseEmit:
			generates = true
		}
	}
	return base, annotates, generates
}

// rolesOf returns the capabilities of each role of a declaration. The list
// of a role has the capabilities of provides, then the capabilities of
// scoped in that role. rolesOf returns nil for a declaration without
// capabilities, so a typed build of such a declaration allocates nothing
// for them.
func rolesOf(provides []plugin.Capability, scoped []scopedCapability) map[plugin.Role][]plugin.Capability {
	if len(provides) == 0 && len(scoped) == 0 {
		return nil
	}
	roles := map[plugin.Role][]plugin.Capability{
		plugin.RoleAnnotator: slices.Clone(provides),
		plugin.RoleGenerator: slices.Clone(provides),
	}
	for _, s := range scoped {
		roles[s.role] = append(roles[s.role], s.label)
	}
	return roles
}

// flatten lowers the rule tree to its leaves. It collects the gates of the
// wrappers on the way down, and panics on a [Where] or a [Gated] wrapper
// around no rules. A gate on a directive lowers to the canonical spelling of
// the directive, once for each rule, so the dispatch compares names and
// builds none. A schema without a plugin lowers to a copy with the name of
// the plugin, which the rules under its wrapper share. flatten appends the
// schema of each [Directive] wrapper to wrappers in the order of the
// declaration, a wrapper around no rules included.
func flatten(
	name string, rules []Rule, preds []Pred, schema *directive.Schema,
	gate directive.Name, out *[]flatRule, wrappers *[]*directive.Schema,
) {
	for _, r := range rules {
		held := preds
		if len(r.preds) > 0 {
			held = slices.Concat(preds, r.preds)
		}
		sch, on := schema, gate
		if r.schema != nil || r.gated {
			if sch != nil || on != "" {
				panic("eidos: " + name + " gates one rule on two directives")
			}
			if r.gated && !kernelDirective(r.gate) {
				panic("eidos: " + name + " gates on " + strconv.Quote(string(r.gate)) +
					", which is not a kernel directive")
			}
			sch, on = r.schema, r.gate
			if r.schema != nil {
				if r.schema.Plugin == "" {
					owned := *r.schema
					owned.Plugin = name
					sch = &owned
				}
				on = sch.Canonical()
				*wrappers = append(*wrappers, sch)
			}
		}
		if r.leaf == nil {
			if len(r.children) == 0 && r.schema == nil {
				panic("eidos: " + name + " declares a wrapper around no rules")
			}
			flatten(name, r.children, held, sch, on, out, wrappers)
			continue
		}
		for _, p := range held {
			if p.test == nil {
				panic("eidos: " + name + " gates on a zero predicate")
			}
			if !p.value && p.id == 0 && p.bind == nil {
				panic("eidos: " + name + " gates on the zero key;" +
					" a gate takes a registered handle when the rule is declared, or a named handle")
			}
		}
		if r.leaf.graph && (sch != nil || on != "" || len(held) > 0) {
			panic("eidos: " + name +
				" gates a graph rule, which has no subject to gate on")
		}
		*out = append(*out, flatRule{
			ordinal:   plugin.RuleID(len(*out)),
			kind:      r.leaf.kind,
			phase:     r.leaf.phase,
			graph:     r.leaf.graph,
			schema:    sch,
			gate:      on,
			spellings: spellingsOf(sch, on),
			preds:     held,
			indexed:   slices.IndexFunc(held, func(p Pred) bool { return !p.value && !p.negated }),
			invoke:    r.leaf.invoke,
		})
	}
}

// flatRule is one lowered rule. It has the trigger, the gates that it
// inherits, and an ordinal that is stable in declaration order.
type flatRule struct {
	ordinal plugin.RuleID
	kind    symbol.Kind
	phase   plugin.Phase
	graph   bool
	schema  *directive.Schema
	// gate is the canonical spelling of the directive that gates the rule,
	// which is its own schema or the directive of a Gated wrapper. It is
	// empty on a rule that no directive gates.
	gate directive.Name
	// spellings are the spellings under which the index can contain the
	// gating directive, and the dispatch enumerates them. It is nil on a
	// rule that no directive gates.
	spellings []directive.Name
	preds     []Pred
	// indexed is the position in preds of the first key gate that Not did
	// not negate, whose key the fact index enumerates. It is -1 where the
	// rule has no such gate.
	indexed int
	invoke  func(inv invocation) error
}

// spellingsOf returns the spellings under which the index can contain a
// gating directive. A schema has its canonical spelling, and also its bare
// spelling where the two differ. A kernel gate has one spelling, and a
// rule that no directive gates has none.
func spellingsOf(schema *directive.Schema, gate directive.Name) []directive.Name {
	switch {
	case gate == "":
		return nil
	case schema == nil || gate == schema.Name:
		return []directive.Name{gate}
	default:
		return []directive.Name{gate, schema.Name}
	}
}

// kernelDirective reports whether name is the canonical spelling of a
// directive that [directive.Kernel] returns. A [Gated] wrapper can name
// only those directives.
func kernelDirective(name directive.Name) bool {
	return slices.ContainsFunc(directive.Kernel(), func(s directive.Schema) bool {
		return s.Canonical() == name
	})
}

// collectSchemas returns the schemas of the wrappers in the order of the
// declaration, with the kinds that [kinded] sets. The rules under one
// wrapper share its schema. collectSchemas panics on a name that two
// wrappers declare, because one wrapper gates every rule of a directive. It
// also panics on a wrapper around no rules whose schema states no kinds and
// does not override, because no rule gives the kinds of the declarations
// that an instance may be written on.
func collectSchemas(name string, wrappers []*directive.Schema, rules []flatRule) []directive.Schema {
	declared := map[directive.Name]struct{}{}
	out := make([]directive.Schema, 0, len(wrappers))
	for _, w := range wrappers {
		canonical := w.Canonical()
		if _, taken := declared[canonical]; taken {
			panic("eidos: " + name + " declares directive " +
				string(canonical) + " in two wrappers; one wrapper gates them all")
		}
		declared[canonical] = struct{}{}
		gates := slices.ContainsFunc(rules, func(fr flatRule) bool { return fr.schema == w })
		if !gates && len(w.Kinds) == 0 && !w.Overrides {
			panic("eidos: " + name + " declares directive " + string(canonical) + " around no rules and without kinds")
		}
		out = append(out, kinded(name, w, rules))
	}
	return out
}

// kinded returns a copy of the schema of one wrapper with the kinds of the
// declarations that an instance can be written on. The rules that an
// instance can affect are the rules under the wrapper. For an override
// schema, they also include every rule of the plugin that no directive
// gates, except a graph rule.
//
// kinded keeps an explicit Kinds. It panics where the explicit kinds leave
// out the kind of a rule under the wrapper that a declaration triggers. It
// sets an empty Kinds to the kinds of the affected rules, in the order of
// [symbol.Kind]. It leaves the field empty where an affected rule runs on
// emitted values, because the origin of an emitted value can have any
// kind. kinded allocates the derived list, one allocation for up to eight
// kinds.
func kinded(name string, wrapper *directive.Schema, rules []flatRule) directive.Schema {
	s := *wrapper
	var kinds []symbol.Kind
	emits := false
	for _, fr := range rules {
		under := fr.schema == wrapper
		bare := s.Overrides && fr.gate == "" && !fr.graph
		if !under && !bare {
			continue
		}
		if fr.phase == plugin.PhaseEmit {
			emits = true
			continue
		}
		if len(s.Kinds) > 0 {
			if under && !slices.Contains(s.Kinds, fr.kind) {
				panic("eidos: " + name + " handles " + fr.kind.String() + " under directive " +
					string(fr.gate) + ", and the kinds of the directive leave it out")
			}
			continue
		}
		if !slices.Contains(kinds, fr.kind) {
			kinds = append(kinds, fr.kind)
		}
	}
	if len(s.Kinds) == 0 && !emits {
		slices.Sort(kinds)
		s.Kinds = kinds
	}
	return s
}

// subscriptionsFor lowers the gates to subscriptions. It returns one
// record for each key gate of a rule, negated or not, and one record
// without a key for a rule whose gates read no key.
func subscriptionsFor(rules []flatRule) []plugin.Subscription {
	var subs []plugin.Subscription
	for _, fr := range rules {
		keyed := false
		for _, p := range fr.preds {
			if p.value {
				continue
			}
			keyed = true
			subs = append(subs, plugin.Subscription{
				Rule: fr.ordinal, Kind: fr.kind, Directive: fr.gate,
				FactKey: p.id, Phase: fr.phase,
			})
		}
		if !keyed {
			subs = append(subs, plugin.Subscription{
				Rule: fr.ordinal, Kind: fr.kind, Directive: fr.gate, Phase: fr.phase,
			})
		}
	}
	return subs
}
