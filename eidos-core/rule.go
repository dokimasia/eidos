// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"fmt"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Effect is the set of effects of a handler. The second parameter of the
// handler selects the role of its rule. A handler that takes an [*Emitter]
// generates, and one that takes a [*Stamper] annotates.
type Effect interface{ *Emitter | *Stamper }

// phaseOf returns the phase that the effect of a handler implies. A
// Stamper annotates and an Emitter generates.
func phaseOf[E Effect]() plugin.Phase {
	var zero E
	if _, stamps := any(zero).(*Stamper); stamps {
		return plugin.PhaseAnnotate
	}
	return plugin.PhaseGenerate
}

// effectFor returns the effect handle of the invocation, wired into the
// allocation of the match. The constraint admits exactly two types, so a
// failed assertion is a defect of the plumbing and not a condition of a
// run.
func effectFor[E Effect](rs *runState, m *match) E {
	var zero E
	if _, stamps := any(zero).(*Stamper); stamps {
		eff, held := any(stamperInto(m)).(E)
		if !held {
			panic("eidos: the stamper handle does not satisfy its own effect")
		}
		return eff
	}
	eff, held := any(rs.emitterFor(m)).(E)
	if !held {
		panic("eidos: the emitter handle does not satisfy its own effect")
	}
	return eff
}

// Rule is one trigger bound to one handler, with its gates. A rule is an
// opaque value. The On constructors make rules, the scoping wrappers
// annotate them, and a typed build of the [Builder] lowers them to the
// subscription records that the engine reads.
type Rule struct {
	// leaf is set on a trigger rule, and children on a wrapper.
	leaf   *leaf
	schema *directive.Schema
	// gate names a directive that the composition registers on its own,
	// which is a directive of the kernel. The wrapper gates on it without
	// a schema. gated marks a [Gated] wrapper, so a typed build checks its
	// name also when the name is empty.
	gate     directive.Name
	gated    bool
	preds    []Pred
	children []Rule
}

// leaf is one trigger. It has what runs, when it runs, and how an
// invocation calls the handler.
type leaf struct {
	kind   symbol.Kind
	phase  plugin.Phase
	graph  bool
	invoke func(inv invocation) error
}

// Directive gates rules on a validated directive, and the built plugin
// returns s for registration. One wrapper can gate many rules, and the
// schema registers once. A typed build panics on a name that two wrappers
// declare, so one wrapper gates every rule of a directive. On a rule that
// an emit triggers, the gate is the instance on the origin. A typed build
// sets an empty Kinds of s from the rules, as the documentation of
// [Builder] states. A typed build gives a schema with an empty Plugin the
// name of the plugin, and the workspace refuses a schema that names
// another plugin. A wrapper around no rules registers s and gates nothing,
// for a directive that a handler of another rule reads through the
// Directives method of [SubjectMatch]. Its schema states its Kinds unless
// it overrides, and a typed build panics otherwise. Directive allocates the
// copy of the schema and the list of rules, two allocations.
func Directive(s directive.Schema, rules ...Rule) Rule {
	return Rule{schema: &s, children: rules}
}

// Gated gates rules on a directive that another registrant registers,
// which is one of the directives of the kernel. The composition registers
// their schemas before the schema of any plugin, so a plugin that declares
// one again is refused as a duplicate. name is the canonical spelling of
// the schema. A typed build panics on a name that [directive.Kernel] does
// not return, because a plugin gates on its own directive through
// [Directive]. A rule under Gated runs once for each validated instance,
// as a rule under [Directive] does. Gated allocates the list of rules, one
// allocation.
func Gated(name directive.Name, rules ...Rule) Rule {
	return Rule{gate: name, gated: true, children: rules}
}

// Where gates rules on a predicate. Wrappers compose and predicates
// conjoin, a disjunction is two rules, and [Not] negates a predicate. On a
// rule that an emit triggers, a key gate evaluates against the origin, and
// a value gate against the emitted value. Where allocates the list of
// predicates and the list of rules, two allocations.
func Where(p Pred, rules ...Rule) Rule {
	return Rule{preds: []Pred{p}, children: rules}
}

// Pred is a declarative predicate. A key gate, such as [HasKey], reads a
// key of the fact store, and the subscription record of its rule contains
// the key as data. A value gate, such as [NameEquals], reads the value that
// triggers the rule, and adds no record. The dispatch runs the test of a
// predicate untracked, because the evaluation of a gate is routing. The
// zero Pred gates nothing, and a typed build refuses it.
type Pred struct {
	name meta.KeyName
	// test reports whether the predicate admits a subject. value is the
	// declaration or the emitted value that triggers the rule.
	test func(f *meta.Facts, subject symbol.Identity, value symbol.Symbol) bool
	// bind returns the id of the name of a named handle in a registry. For
	// a name that the registry does not contain under the value type of
	// the handle, it returns an error whose message contains the plugin p
	// and the key. It is nil for a registered handle.
	bind func(r *meta.Registry, p plugin.ID) (meta.KeyID, error)
	id   meta.KeyID
	// value marks a value gate, which reads no key.
	value bool
	// negated marks a predicate that [Not] negated an odd number of times.
	// The fact index cannot enumerate it.
	negated bool
}

// HasKey admits a subject on which k reads present.
//
// HasKey reads the key when the rule declares the gate, because the
// subscription record contains it as data. A handle that a composition
// assigns later is still zero at this point, and the gate would watch
// nothing, so a typed build panics on it. A handler can read such a handle
// through its closure, and a gate cannot. A handle from [meta.Named] has
// the name of a key of another registrant and no id. The workspace binds
// the gate when it builds, and refuses a name that the registry does not
// contain under T. HasKey allocates the closure of the test over the key,
// one allocation, and for a named handle the closure that binds it, two
// allocations.
func HasKey[T meta.FactValue](k meta.Key[T]) Pred {
	return Pred{
		id:   k.ID(),
		name: k.Name(),
		test: func(f *meta.Facts, subject symbol.Identity, _ symbol.Symbol) bool {
			_, held := meta.Get(f, subject, k)
			return held
		},
		bind: bindName(k),
	}
}

// Equatable is the comparable half of the fact vocabulary. A list value
// has no equality gate.
type Equatable interface {
	meta.FactValue
	comparable
}

// KeyEquals admits a subject on which the value that the arbitration
// selects for k equals v. KeyEquals reads the key when the rule declares
// the gate, and a named handle binds when the workspace builds, as
// [HasKey] states. KeyEquals allocates the closure of the test over the
// key and the value, one allocation, and for a named handle the closure
// that binds it, two allocations.
func KeyEquals[T Equatable](k meta.Key[T], v T) Pred {
	return Pred{
		id:   k.ID(),
		name: k.Name(),
		test: func(f *meta.Facts, subject symbol.Identity, _ symbol.Symbol) bool {
			got, held := meta.Get(f, subject, k)
			return held && got == v
		},
		bind: bindName(k),
	}
}

// Not admits a subject that p does not admit. For [HasKey], that is a
// subject on which the key reads absent. For [KeyEquals], it is also a
// subject with another value. The fact index cannot enumerate a negated
// gate, so a rule whose key gates are all negated enumerates every subject
// of its kind. A negated key gate reads its key, so the subscription record
// of its rule contains the key. The zero Pred remains zero under Not, and a
// typed build refuses it. Not allocates the closure of the test, one
// allocation.
func Not(p Pred) Pred {
	if p.test == nil {
		return p
	}
	test := p.test
	p.test = func(f *meta.Facts, subject symbol.Identity, value symbol.Symbol) bool {
		return !test(f, subject, value)
	}
	p.negated = !p.negated
	return p
}

// NameEquals admits a subject whose value has the name: the declaration
// that a node trigger runs on, or the emitted value that an [OnEmit]
// trigger runs on. For the empty name it returns the zero Pred, which a
// typed build refuses. NameEquals allocates the closure of the test over
// the name, one allocation.
func NameEquals(name string) Pred {
	if name == "" {
		return Pred{}
	}
	return Pred{
		value: true,
		test: func(_ *meta.Facts, _ symbol.Identity, value symbol.Symbol) bool {
			if decl, declared := value.(node.Declaration); declared {
				return decl.Identity().Name == name
			}
			got, named := emit.NameOf(value)
			return named && got == name
		},
	}
}

// HasSlot admits an emitted function or method whose body declares the
// named slot. It does not admit a node declaration, because the node
// model does not record a body. The prologue and the epilogue are on every
// body, and an owner cannot declare a slot under their names. So for
// either name, and for the empty name, HasSlot returns the zero Pred,
// which a typed build refuses. HasSlot allocates the closure of the test
// over the name, one allocation.
func HasSlot(name string) Pred {
	if name == "" || name == emit.SlotPrologue || name == emit.SlotEpilogue {
		return Pred{}
	}
	return Pred{
		value: true,
		test: func(_ *meta.Facts, _ symbol.Identity, value symbol.Symbol) bool {
			var body *emit.Body
			switch v := value.(type) {
			case *emit.Function:
				body = &v.Body
			case *emit.Method:
				body = &v.Body
			default:
				return false
			}
			_, declared := body.Slot(name)
			return declared
		},
	}
}

// bindName returns the bind of a predicate on k. It returns nil for a
// registered handle and for the zero Key, and for a named handle a
// function that returns the id of its name in a registry. The function
// returns an error for a name that the registry does not contain, and for
// a name that the registry contains under another value type than T. The
// message of the error contains the plugin of the gate and the key.
func bindName[T meta.FactValue](k meta.Key[T]) func(r *meta.Registry, p plugin.ID) (meta.KeyID, error) {
	if k.ID() != 0 || k.Name() == "" {
		return nil
	}
	return func(r *meta.Registry, p plugin.ID) (meta.KeyID, error) {
		if bound, registered := meta.Lookup[T](r, k.Name()); registered {
			return bound.ID(), nil
		}
		if _, registered := r.Resolve(k.Name()); registered {
			var want T
			return 0, fmt.Errorf(
				"eidos: %s gates on %s, and the registration of the key has another value type than %T",
				p,
				k.Name(),
				want,
			)
		}
		return 0, fmt.Errorf("eidos: %s gates on %s, and nothing registers the key", p, k.Name())
	}
}
