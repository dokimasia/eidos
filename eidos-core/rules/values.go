// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// aliasDepth bounds the alias chain that the shape of an authored value,
// [Unaliased] and the member walk follow, so an alias cycle in a broken
// source ends.
const aliasDepth = 8

// Sample is one value of a type that a generated check writes, or the
// reason why no value derives.
//
// # Allocation contract
//
// The constructors and combinators of a Sample return it by value and
// allocate nothing of their own.
type Sample struct {
	Value   emit.Value // the derived value, and zero for a refused sample
	Refusal Refusal    // the reason why no value derived, and RefusedNone for a sample with a value
}

// OK reports whether the sample has no refusal and a value with a kind.
// It allocates nothing.
func (s Sample) OK() bool { return s.Refusal == RefusedNone && !s.Value.IsZero() }

// Of returns a sample with a value. It allocates nothing.
func Of(v emit.Value) Sample { return Sample{Value: v} }

// Refused returns a sample with a refusal and no value. It allocates
// nothing.
func Refused(why Refusal) Sample { return Sample{Refusal: why} }

// Pair returns a literal sample and its alternate, both of one kind.
// It allocates nothing.
func Pair(k emit.LiteralKind, sample, alternate string) (Sample, Sample) {
	return Of(emit.Literal(k, sample)), Of(emit.Literal(k, alternate))
}

// NumberPair returns a numeric sample and its alternate, both of one
// kind and written for a number type of one width in bits, as
// [emit.Number] states it. It allocates nothing.
func NumberPair(k emit.LiteralKind, sample, alternate string, bits int) (Sample, Sample) {
	return Of(emit.Number(k, sample, bits)), Of(emit.Number(k, alternate, bits))
}

// RefusedPair returns one refusal as both halves, for a type that
// admits no pair. It allocates nothing.
func RefusedPair(why Refusal) (Sample, Sample) { return Refused(why), Refused(why) }

// Lift returns a derived sample with its value wrapped, such as an
// element placed in a composite. A sample without a value returns
// unchanged, so the wrapped part keeps its reason. It allocates what
// wrap allocates.
func Lift(s Sample, wrap func(emit.Value) emit.Value) Sample {
	if !s.OK() {
		return s
	}
	return Of(wrap(s.Value))
}

// Complete returns a pair from the halves that an author stated and the
// halves that a language derived. A half is stated where it has a value
// or a refusal, and Complete keeps a stated half. The caller then receives
// the refusal of an authored text instead of a derived value. A half that
// no author stated takes the first of these values that applies:
//   - the derived value of its own position, where that value differs
//     from the stated half
//   - the other derived value, where that value differs
//   - a [RefusedNoLiteral] refusal, where neither differs
//
// The completed half then differs from the stated half. Where no half is
// stated, Complete returns the derived pair unchanged. It allocates
// nothing.
func Complete(sample, alternate, derived, derivedAlternate Sample) (Sample, Sample) {
	sampled := sample.OK() || sample.Refusal != RefusedNone
	alternated := alternate.OK() || alternate.Refusal != RefusedNone
	switch {
	case sampled && alternated:
		return sample, alternate
	case sampled:
		return sample, differing(sample, derivedAlternate, derived)
	case alternated:
		return differing(alternate, derived, derivedAlternate), alternate
	default:
		return derived, derivedAlternate
	}
}

// differing returns the derived candidate that pairs with a stated half.
// It returns first unless first has the stated value, then second, and
// [RefusedNoLiteral] where neither differs. It returns a refused first
// candidate unchanged, with its reason.
func differing(stated, first, second Sample) Sample {
	if !first.OK() || !sameLiteral(stated.Value, first.Value) {
		return first
	}
	if second.OK() && !sameLiteral(stated.Value, second.Value) {
		return second
	}
	return Refused(RefusedNoLiteral)
}

// Refusal is the reason why a sample has no value. Only
// [RefusedNoLiteral] is a fact about the type. The others describe an
// input that the caller can fix.
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
	// RefusedUnresolved means a named type that the view does not
	// contain.
	RefusedUnresolved
	// RefusedDepth means a self-referential type past the budget of the
	// walk.
	RefusedDepth
	// RefusedAuthored means an authored text that the rules of its
	// language do not read as a value of a boolean or a scalar type.
	RefusedAuthored
)

// FirstRefusal returns the first refusal among samples, and
// [RefusedNoLiteral] where no sample has one. A value built from derived
// parts refuses with it when a part has no value. FirstRefusal keeps no
// sample, so the argument list of the caller does not escape, and a call
// allocates nothing.
func FirstRefusal(samples ...Sample) Refusal {
	for _, s := range samples {
		if s.Refusal != RefusedNone {
			return s.Refusal
		}
	}
	return RefusedNoLiteral
}

// String returns the spelling of the refusal, and the decimal number of
// an undeclared refusal. It allocates nothing for a declared refusal.
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
	case RefusedAuthored:
		return "authored"
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

// EmitRef restates a node reference in the emit model, with its
// structure, its facts, its arguments, its target and its package, so the
// Type of a value is the reference that a backend spells and imports. The
// facts are the variance, the asynchrony and the mutability. A nil
// reference returns nil.
//
// # Allocation contract
//
// EmitRef allocates each restated reference, and the list of children
// and the list of arguments of each that has them. A reference without
// children or arguments allocates one.
func EmitRef(ref *node.TypeRef) *emit.TypeRef {
	if ref == nil {
		return nil
	}
	out := &emit.TypeRef{
		Spelling:   ref.Spelling,
		Target:     ref.Target,
		Package:    ref.Package,
		Form:       ref.Form,
		Split:      ref.Split,
		Length:     ref.Length,
		Variance:   ref.Variance,
		Async:      ref.Async,
		Mutability: ref.Mutability,
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

// samplesOf returns the halves that an author stated, which
// [View.Authored] reads, and a refused stated half too. It asks the
// language to derive only where a half is not stated.
func (b Bound) samplesOf(subject symbol.Identity, ref *node.TypeRef, hint string) (Sample, Sample) {
	if b.view.IsZero() {
		return RefusedPair(RefusedNoView)
	}
	sample, alternate := b.view.Authored(b.source, subject, ref)
	sampled := sample.OK() || sample.Refusal != RefusedNone
	if sampled && (alternate.OK() || alternate.Refusal != RefusedNone) {
		return sample, alternate
	}
	derived, derivedAlternate := b.source.DeriveSamples(ref, hint, b.view)
	return Complete(sample, alternate, derived, derivedAlternate)
}

// authored returns a text that an author stated as a sample of a type. The
// rules of the language of by, the declaration that states the text, read
// it first in the scope of by. Where they do not read it, the lift depends
// on the shape of the type. A text type lifts it to a string literal, or to
// the value that the [TextRules] of those rules give it, and a boolean or a
// scalar refuses with [RefusedAuthored]. Every other form lifts it to raw
// text in the language of by.
func (b Bound) authored(by symbol.Identity, ref *node.TypeRef, text string) Sample {
	r := b.rulesOf(by.Lang)
	if value, read := r.LiteralFor(b.view.ScopeOf(by), ref, text, b.view); read {
		return Of(value)
	}
	switch b.valueShape(ref).Form {
	case symbol.FormText:
		if texts, converts := r.(TextRules); converts {
			return Of(texts.Text(ref, text, b.view))
		}
		return Of(emit.Literal(emit.LiteralString, text))
	case symbol.FormBool, symbol.FormScalar:
		return Refused(RefusedAuthored)
	default:
		return Of(emit.Raw(by.Lang, text))
	}
}

// valueShape returns the shape by which an authored value of a type
// lifts. It is the shape of the fold, with a reference to an alias
// followed to the target of the alias. A defined type takes the literals
// of the type over which it is defined, and an alias without a target
// folds to Opaque. A chain longer than aliasDepth returns the shape at
// which it stopped.
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

// witnesses reads the authored witness of each parameter first, and asks
// the generics capability of the language to derive the rest. The list
// is whole or nil, because an entry point instantiates every parameter at
// once.
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
		if id, authored := Fact(b.view, p.ID, b.view.kernel.Witness); authored {
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

// witnessRef lifts an authored witness identity into a reference whose
// spelling is the bare name. A witness that the graph declares is the
// target of the reference. A witness that the graph does not declare is a
// type outside the workspace. Its reference keeps the package beside the
// name and has no target, so a backend imports it and the builtin table
// of the language classifies it by both. A builtin has neither. The
// lookup records a read, so a declaration that arrives later changes the
// witness. It allocates the reference.
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
