// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"fmt"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Effect is the set of handler effect surfaces. The handler's
// second parameter picks the plugin's role: a handler taking
// [*Emitter] generates, one taking [*Stamper] annotates.
type Effect interface{ *Emitter | *Stamper }

// phaseOf returns the phase a handler's effect implies: a Stamper
// annotates and an Emitter generates.
func phaseOf[E Effect]() plugin.Phase {
	var zero E
	if _, stamps := any(zero).(*Stamper); stamps {
		return plugin.PhaseAnnotate
	}
	return plugin.PhaseGenerate
}

// effectFor returns the invocation's effect handle, wired into the
// match's own allocation. The constraint admits exactly two types,
// so a failed assertion is a plumbing defect rather than a run
// condition.
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

// Rule is one trigger bound to one handler, with its gates. Rules
// are opaque values: the On constructors make them and the scoping
// wrappers annotate them, and [Builder.Build] lowers them to the
// subscription records the engine reads.
type Rule struct {
	// leaf is set on a trigger rule; children on a wrapper.
	leaf   *leaf
	schema *directive.Schema
	// gate names a directive the composition registers on its
	// own, the kernel's, which the wrapper gates on without
	// carrying a schema. gated marks a [Gated] wrapper, so Build
	// checks its name even when the name is empty.
	gate     directive.Name
	gated    bool
	preds    []Pred
	children []Rule
}

// leaf is one trigger: what runs, when, and how an invocation
// reaches the handler.
type leaf struct {
	kind   symbol.Kind
	phase  plugin.Phase
	graph  bool
	invoke func(inv invocation) error
}

// Directive gates rules on a validated directive and carries the
// schema for registration. One wrapper may gate many rules and the
// schema registers once; a name two wrappers carry is refused at
// Build, so one wrapper gates them all. On an emit-triggered rule the
// gate is the origin's instance. Directive allocates the copy of the
// schema and the list of rules, two allocations.
func Directive(s directive.Schema, rules ...Rule) Rule {
	return Rule{schema: &s, children: rules}
}

// Gated gates rules on a directive registered by someone else: one
// of the kernel's, whose schema the composition registers before
// any plugin's, so a plugin carrying it again would be refused as
// a duplicate. The name is the schema's canonical spelling. Build
// panics on a name [directive.Kernel] does not return, because a
// plugin gates on its own directive through [Directive]. A rule
// under it runs once per validated instance like one under
// [Directive]. Gated allocates the list of rules, one allocation.
func Gated(name directive.Name, rules ...Rule) Rule {
	return Rule{gate: name, gated: true, children: rules}
}

// Where gates rules on stamped facts. Wrappers compose and
// predicates conjoin; a disjunction is two rules. On an
// emit-triggered rule the predicate evaluates against the origin.
// Where allocates the list of predicates and the list of rules, two
// allocations.
func Where(p Pred, rules ...Rule) Rule {
	return Rule{preds: []Pred{p}, children: rules}
}

// Pred is a declarative fact predicate: its key is data, which is
// what the subscription record carries, and its test runs at
// dispatch, untracked, because gate evaluation is routing. The
// zero Pred gates nothing and is refused at Build.
type Pred struct {
	id   meta.KeyID
	name meta.KeyName
	test func(f *meta.Facts, subject symbol.Identity) bool
	// bind returns the id that a named handle's name has in a registry.
	// For a name that the registry does not contain under the handle's
	// value type, it returns an error whose message contains the plugin p
	// and the key. It is nil for a registered handle.
	bind func(r *meta.Registry, p plugin.ID) (meta.KeyID, error)
}

// HasKey admits a subject on which k reads present.
//
// The key is read here, when the gate is declared, because the
// subscription record carries it as data. A handle a composition
// assigns later is still zero at this point and the gate would
// watch nothing, so Build panics on it: a handler may read such a
// handle through its closure, a gate may not. A handle from
// [meta.Named] has the name of another registrant's key and no id. The
// workspace binds the gate when it builds, and refuses a name that the
// registry does not contain under T. HasKey allocates the test's closure
// over the key, one allocation, and for a named handle the closure that
// binds it, two allocations.
func HasKey[T meta.FactValue](k meta.Key[T]) Pred {
	return Pred{
		id:   k.ID(),
		name: k.Name(),
		test: func(f *meta.Facts, subject symbol.Identity) bool {
			_, held := meta.Get(f, subject, k)
			return held
		},
		bind: bindName(k),
	}
}

// Equatable is the comparable half of the fact vocabulary; a list
// value has no equality gate.
type Equatable interface {
	meta.FactValue
	comparable
}

// KeyEquals admits a subject on which the value arbitration selects
// for k equals v. The key is read when the gate is declared, and a named
// handle binds when the workspace builds, as [HasKey] states. KeyEquals
// allocates the test's closure over the key and the value, one
// allocation, and for a named handle the closure that binds it, two
// allocations.
func KeyEquals[T Equatable](k meta.Key[T], v T) Pred {
	return Pred{
		id:   k.ID(),
		name: k.Name(),
		test: func(f *meta.Facts, subject symbol.Identity) bool {
			got, held := meta.Get(f, subject, k)
			return held && got == v
		},
		bind: bindName(k),
	}
}

// bindName returns the bind of a predicate on k: nil for a registered
// handle and for the zero Key, and for a named handle a function that
// returns the id of its name in a registry. The function returns an
// error for a name that the registry does not contain, and for a name
// that the registry contains under another value type than T. The message
// of the error contains the plugin of the gate and the key.
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
