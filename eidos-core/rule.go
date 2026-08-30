// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import (
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Effect is the set of handler effect surfaces. The handler's
// second parameter picks the plugin's role: a handler taking
// [*Emitter] generates, one taking [*Stamper] annotates.
type Effect interface{ *Emitter | *Stamper }

// Rule is one trigger bound to one handler, with its gates. Rules
// are opaque values: the On constructors make them and the scoping
// wrappers annotate them, and [Builder.Build] lowers them to the
// subscription records the engine reads.
type Rule struct {
	// leaf is set on a trigger rule; children on a wrapper.
	leaf     *leaf
	schema   *directive.Schema
	preds    []Pred
	children []Rule
}

// leaf is one trigger: what fires, when, and how an invocation
// reaches the handler.
type leaf struct {
	kind   symbol.Kind
	phase  plugin.Phase
	graph  bool
	invoke func(inv invocation) error
}

// Directive gates rules on a validated directive and carries the
// schema for registration. One wrapper may gate many rules and the
// schema registers once; a name carried by two wrappers is refused
// at Build, so one wrapper gates them all. On an emit-triggered
// rule the gate is the origin's instance.
func Directive(s directive.Schema, rules ...Rule) Rule {
	return Rule{schema: &s, children: rules}
}

// Where gates rules on stamped facts. Wrappers compose and
// predicates conjoin; a disjunction is two rules. On an
// emit-triggered rule the predicate evaluates against the origin.
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
}

// HasKey admits a subject on which k reads present.
func HasKey[T meta.FactValue](k meta.Key[T]) Pred {
	return Pred{
		id:   k.ID(),
		name: k.Name(),
		test: func(f *meta.Facts, subject symbol.Identity) bool {
			_, held := meta.Get(f, subject, k)
			return held
		},
	}
}

// Equatable is the comparable half of the fact vocabulary; a list
// value has no equality gate.
type Equatable interface {
	meta.FactValue
	comparable
}

// KeyEquals admits a subject whose winning value for k equals v.
func KeyEquals[T Equatable](k meta.Key[T], v T) Pred {
	return Pred{
		id:   k.ID(),
		name: k.Name(),
		test: func(f *meta.Facts, subject symbol.Identity) bool {
			got, held := meta.Get(f, subject, k)
			return held && got == v
		},
	}
}
