// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Sample is one value of a type a generated check writes, or the
// reason none could be derived.
type Sample struct {
	Value   emit.Value
	Refusal Refusal
}

// OK reports whether a value was derived: no refusal, and a value
// with a kind.
func (s Sample) OK() bool { return s.Refusal == RefusedNone && !s.Value.IsZero() }

// Of returns a sample carrying a value.
func Of(v emit.Value) Sample { return Sample{Value: v} }

// Refused returns a sample carrying a refusal.
func Refused(why Refusal) Sample { return Sample{Refusal: why} }

// Refusal says why a sample carries no value. Only
// [RefusedNoLiteral] is a fact about the type; the rest describe an
// input the caller can fix.
type Refusal uint8

const (
	// RefusedNone means a value was derived.
	RefusedNone Refusal = iota
	// RefusedNoView means the projection was handed a zero view.
	RefusedNoView
	// RefusedNoRules means the composition registers no rules for
	// the language.
	RefusedNoRules
	// RefusedNoLiteral means the type admits no distinguishable
	// value.
	RefusedNoLiteral
	// RefusedUnresolved means a named type the view does not hold.
	RefusedUnresolved
	// RefusedDepth means a self-referential type past the walk's
	// budget.
	RefusedDepth
)

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

// samplesOf reads the authored values first, on the declaration
// carrying the type and then on the declaration the type names,
// and asks the language to derive only what neither stated. Each
// half reads independently. A derived half paired with an authored
// one must differ from it, so the pair is two distinct values.
func (b Bound) samplesOf(subject symbol.Identity, ref *node.TypeRef, hint string) (Sample, Sample) {
	if b.view.IsZero() {
		return Refused(RefusedNoView), Refused(RefusedNoView)
	}
	sample, alternate := b.authored(subject)
	if ref != nil && !ref.Target.IsZero() && (!sample.OK() || !alternate.OK()) {
		typeSample, typeAlternate := b.authored(ref.Target)
		if !sample.OK() {
			sample = typeSample
		}
		if !alternate.OK() {
			alternate = typeAlternate
		}
	}
	if sample.OK() && alternate.OK() {
		return sample, alternate
	}
	derived, derivedAlternate := b.source.SamplesOf(ref, hint, b.view)
	switch {
	case !sample.OK() && !alternate.OK():
		return derived, derivedAlternate
	case !sample.OK():
		sample = differing(alternate, derived, derivedAlternate)
	default:
		alternate = differing(sample, derivedAlternate, derived)
	}
	return sample, alternate
}

// differing returns the derived candidate that pairs with an
// authored half: first, unless it equals the authored value, then
// second, and [RefusedNoLiteral] where neither differs. A refused
// first candidate returns as it is, keeping its reason.
func differing(authored, first, second Sample) Sample {
	if !first.OK() || !sameLiteral(authored.Value, first.Value) {
		return first
	}
	if second.OK() && !sameLiteral(authored.Value, second.Value) {
		return second
	}
	return Refused(RefusedNoLiteral)
}

// sameLiteral reports whether two values are literals with one
// text, whatever their kinds: an authored value is raw text, so
// its text is what compares. An authored string's raw text keeps
// its quotes, so it never equals a derived string.
func sameLiteral(a, b emit.Value) bool {
	return a.Kind == emit.ValueLiteral && b.Kind == emit.ValueLiteral && a.Text == b.Text
}

// authored reads the two authored values stamped on a declaration.
// An authored value is text in the source language, so it arrives
// as a raw literal tagged with the language that wrote it, which
// is what lets a target refuse another language's text.
func (b Bound) authored(id symbol.Identity) (Sample, Sample) {
	var sample, alternate Sample
	lang := b.source.Lang()
	if text, held := Fact(b.view, id, b.view.Kernel.Sample); held {
		sample = Of(emit.Raw(lang, text))
	}
	if text, held := Fact(b.view, id, b.view.Kernel.Alternate); held {
		alternate = Of(emit.Raw(lang, text))
	}
	return sample, alternate
}

// witnesses reads the authored witness per parameter first and
// asks the language's generics capability to derive the rest; the
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
			out = append(out, witnessRef(id))
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

// witnessRef lifts an authored witness identity into a reference:
// the bare name as its spelling, the identity as its target where
// the witness names a declaration, and a builtin's spelling alone.
func witnessRef(id symbol.Identity) *node.TypeRef {
	ref := &node.TypeRef{Spelling: id.Name}
	if id.Package != "" {
		ref.Target = id
	}
	return ref
}

// EmitRef restates a node reference in the emit model, structure,
// arguments and target included, so a value's Type is what a
// backend spells and qualifies. A nil reference stays nil.
func EmitRef(ref *node.TypeRef) *emit.TypeRef {
	if ref == nil {
		return nil
	}
	out := &emit.TypeRef{
		Spelling: ref.Spelling,
		Target:   ref.Target,
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
