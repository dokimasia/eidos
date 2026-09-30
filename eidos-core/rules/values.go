// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// aliasDepth bounds the alias chain an authored value's shape
// follows, so an alias cycle in a broken source ends.
const aliasDepth = 8

// Sample is one value of a type a generated check writes, or the
// reason none could be derived.
type Sample struct {
	Value   emit.Value
	Refusal Refusal
}

// OK reports whether the sample has a derived value: no refusal,
// and a value with a kind.
func (s Sample) OK() bool { return s.Refusal == RefusedNone && !s.Value.IsZero() }

// Of returns a sample with a value.
func Of(v emit.Value) Sample { return Sample{Value: v} }

// Refused returns a sample with a refusal and no value.
func Refused(why Refusal) Sample { return Sample{Refusal: why} }

// Pair returns a literal sample and its alternate, both of one
// kind.
func Pair(k emit.LiteralKind, sample, alternate string) (Sample, Sample) {
	return Of(emit.Literal(k, sample)), Of(emit.Literal(k, alternate))
}

// NumberPair returns a numeric sample and its alternate, both of one
// kind and written for a number type of one width in bits, as
// [emit.Number] states it.
func NumberPair(k emit.LiteralKind, sample, alternate string, bits int) (Sample, Sample) {
	return Of(emit.Number(k, sample, bits)), Of(emit.Number(k, alternate, bits))
}

// RefusedPair returns one refusal as both halves, for a type that
// admits no pair.
func RefusedPair(why Refusal) (Sample, Sample) { return Refused(why), Refused(why) }

// Lift returns a derived sample with its value wrapped, such as an
// element placed in a composite. A sample without a value returns
// unchanged, so the wrapped part keeps its reason.
func Lift(s Sample, wrap func(emit.Value) emit.Value) Sample {
	if !s.OK() {
		return s
	}
	return Of(wrap(s.Value))
}

// Complete returns a pair from the halves an author stated and the
// halves a language derived. A stated half is kept. A half no author
// stated takes the derived value of its own position where that
// value differs from the stated half, the other derived value where
// that one differs, and a [RefusedNoLiteral] refusal where neither
// does. The pair is then two distinct values. With no half stated,
// the derived pair returns as it is.
func Complete(sample, alternate, derived, derivedAlternate Sample) (Sample, Sample) {
	switch {
	case sample.OK() && alternate.OK():
		return sample, alternate
	case sample.OK():
		return sample, differing(sample, derivedAlternate, derived)
	case alternate.OK():
		return differing(alternate, derived, derivedAlternate), alternate
	default:
		return derived, derivedAlternate
	}
}

// differing returns the derived candidate that pairs with a stated
// half: first, unless it has the stated value, then second, and
// [RefusedNoLiteral] where neither differs. A refused first
// candidate returns as it is, with its reason.
func differing(stated, first, second Sample) Sample {
	if !first.OK() || !sameLiteral(stated.Value, first.Value) {
		return first
	}
	if second.OK() && !sameLiteral(stated.Value, second.Value) {
		return second
	}
	return Refused(RefusedNoLiteral)
}

// Refusal names why a sample has no value. Only [RefusedNoLiteral]
// is a fact about the type. The rest describe an input the caller
// can fix.
type Refusal uint8

const (
	// RefusedNone means the sample has a derived value.
	RefusedNone Refusal = iota
	// RefusedNoView means the projection received a zero view.
	RefusedNoView
	// RefusedNoRules means the composition registers no rules for
	// the language.
	RefusedNoRules
	// RefusedNoLiteral means the type admits no distinguishable
	// value.
	RefusedNoLiteral
	// RefusedUnresolved means a named type the view does not
	// contain.
	RefusedUnresolved
	// RefusedDepth means a self-referential type past the walk's
	// budget.
	RefusedDepth
)

// FirstRefusal returns the first refusal among samples, and
// [RefusedNoLiteral] where none states one. A value built from
// several derived parts refuses with it when a part has no value.
func FirstRefusal(samples ...Sample) Refusal {
	for _, s := range samples {
		if s.Refusal != RefusedNone {
			return s.Refusal
		}
	}
	return RefusedNoLiteral
}

// String returns the refusal's spelling.
func (r Refusal) String() string {
	switch r {
	case RefusedNone:
		return "none"
	case RefusedNoView:
		return "no-view"
	case RefusedNoRules:
		return "no-rules"
	case RefusedNoLiteral:
		return "no-literal"
	case RefusedUnresolved:
		return "unresolved"
	case RefusedDepth:
		return "depth"
	default:
		return strconv.Itoa(int(r))
	}
}

// sameLiteral reports whether two values are literals of one kind
// with one text. Only a literal has a literal kind, so a literal
// kind shared with a literal makes b one too.
func sameLiteral(a, b emit.Value) bool {
	return a.Kind == emit.ValueLiteral && a.Literal == b.Literal && a.Text == b.Text
}

// EmitRef restates a node reference in the emit model, structure,
// arguments, target and package included, so a value's Type is what
// a backend spells and imports. A nil reference returns nil.
func EmitRef(ref *node.TypeRef) *emit.TypeRef {
	if ref == nil {
		return nil
	}
	out := &emit.TypeRef{
		Spelling: ref.Spelling,
		Target:   ref.Target,
		Package:  ref.Package,
		Form:     ref.Form,
		Split:    ref.Split,
		Length:   ref.Length,
		Variance: ref.Variance,
	}
	if len(ref.Elems) > 0 {
		out.Elems = make([]*emit.TypeRef, 0, len(ref.Elems))
		for _, child := range ref.Elems {
			out.Elems = append(out.Elems, EmitRef(child))
		}
	}
	if len(ref.Args) > 0 {
		out.Args = make([]*emit.TypeRef, 0, len(ref.Args))
		for _, arg := range ref.Args {
			out.Args = append(out.Args, EmitRef(arg))
		}
	}
	return out
}

// samplesOf returns the halves an author stated, read through
// [View.Authored], and asks the language to derive only where a half
// is missing.
func (b Bound) samplesOf(subject symbol.Identity, ref *node.TypeRef, hint string) (Sample, Sample) {
	if b.view.IsZero() {
		return RefusedPair(RefusedNoView)
	}
	sample, alternate := b.view.Authored(b.source, subject, ref)
	if sample.OK() && alternate.OK() {
		return sample, alternate
	}
	derived, derivedAlternate := b.source.SamplesOf(ref, hint, b.view)
	return Complete(sample, alternate, derived, derivedAlternate)
}

// valueShape returns the shape an authored value of a type lifts by.
// It is the fold's shape, with a reference to an alias followed to
// the type the alias names. A defined type therefore takes the
// literals of the type it is defined over, and an alias without a
// target folds to Opaque. A chain longer than aliasDepth returns the
// shape it stopped at.
func (b Bound) valueShape(ref *node.TypeRef) TypeShape {
	shape := b.typeOf(ref)
	for range aliasDepth {
		if shape.Form != symbol.FormReference {
			return shape
		}
		decl, held := b.view.Lookup(shape.Ref)
		alias, is := decl.(*node.Alias)
		if !held || !is {
			return shape
		}
		shape = b.typeOf(alias.Target)
	}
	return shape
}

// witnesses reads the authored witness of each parameter first and
// asks the language's generics capability to derive the rest. The
// list is whole or nil, because an entry point instantiates every
// parameter at once.
func (b Bound) witnesses(params []*node.TypeParam) []*node.TypeRef {
	if len(params) == 0 || b.view.IsZero() {
		return nil
	}
	generics, held := b.source.(GenericsRules)
	out := make([]*node.TypeRef, 0, len(params))
	for _, p := range params {
		if p == nil {
			return nil
		}
		if id, authored := Fact(b.view, p.ID, b.view.Kernel.Witness); authored {
			out = append(out, b.witnessRef(id))
			continue
		}
		if !held {
			return nil
		}
		ref, derived := generics.Derive(p, b.view)
		if !derived || ref == nil {
			return nil
		}
		out = append(out, ref)
	}
	return out
}

// witnessRef lifts an authored witness identity into a reference
// spelled by its bare name. A witness the graph declares is the
// reference's target. One the graph does not declare, a type outside
// the workspace, keeps its package beside the name and no target, so
// a backend imports it and the language's builtin table classifies
// it by both. A builtin has neither. The lookup records a read, so a
// declaration arriving later changes what the witness is.
func (b Bound) witnessRef(id symbol.Identity) *node.TypeRef {
	ref := &node.TypeRef{Spelling: id.Name}
	if id.Package == "" {
		return ref
	}
	if _, declared := b.view.Lookup(id); declared {
		ref.Target = id
		return ref
	}
	ref.Package = id.Package
	return ref
}
