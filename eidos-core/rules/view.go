// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// View is what a projection reads through: the invocation's tracked
// declaration reader, the run's arbitrated facts and the read set both
// record into, and the kernel's own keys for the authored values the
// walks read first. The workspace builds one per invocation.
//
// The zero View reads nothing. A read of an untracked graph would record
// no edge, so a projection handed the zero View refuses.
//
// # Concurrency
//
// A View is not safe for concurrent use. The read set it records into is
// not.
//
// # Allocation contract
//
// A read allocates only to record a new edge in the read set.
// [View.Authored] states what it allocates to lift a stated value.
type View struct {
	Decls  *store.Reader   // the tracked declaration reader; nil in the zero View
	Facts  *meta.Facts     // the run's arbitrated facts; nil reads no fact
	Reads  meta.Recorder   // the read set a fact read records into; nil reads untracked
	Kernel meta.KernelKeys // the kernel's keys for authored values
}

// IsZero reports whether the view reads nothing. It allocates nothing.
func (v View) IsZero() bool { return v.Decls == nil }

// Lookup returns one declaration by identity, and records the read. The
// zero view and the zero identity return nothing. It allocates what the
// read set allocates to record a new edge.
func (v View) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	if v.Decls == nil || id.IsZero() {
		return nil, false
	}
	return v.Decls.Lookup(id)
}

// PackageOf returns the package a declaration is in, and records the
// read. The zero view and the zero identity return nothing. It allocates
// what the read set allocates to record a new edge.
func (v View) PackageOf(id symbol.Identity) (*node.Package, bool) {
	if v.Decls == nil || id.IsZero() {
		return nil, false
	}
	return v.Decls.PackageOf(id)
}

// Authored returns the two values an author stated for a value of a
// type. Each half reads the declaration that has the type first,
// then the declaration the type names. The two halves resolve
// independently. A half neither declaration states is the zero
// [Sample], for the caller to derive. A zero subject and a reference
// without a target state nothing. Every fact read records on the
// view.
//
// A stated text lifts by the shape src folds for the type, with an
// alias followed to the type it names:
//   - Text lifts to a string literal of the text.
//   - A boolean lifts to a truth value.
//   - A scalar lifts to a number at the shape's width.
//   - Every other form lifts to raw text in the language of the
//     declaration that states it, which only that language's
//     backend spells.
//
// The kernel reads a type's authored values here before it asks the
// language to derive. A language's composite walk reads the values
// of each element here before it derives the element. The type
// folds only where a half is stated.
//
// # Allocation contract
//
// Authored allocates nothing where no declaration states a value. A
// stated value allocates the fold of its type in a binding of its own:
// the binding's memo, the memo's first group and the shape, three
// allocations for a builtin type, and the children of a structural one.
func (v View) Authored(src SourceRules, subject symbol.Identity, ref *node.TypeRef) (Sample, Sample) {
	var target symbol.Identity
	if ref != nil {
		target = ref.Target
	}
	sample, sampleLang, sampled := v.stated(v.Kernel.Sample, subject, target)
	alternate, alternateLang, alternated := v.stated(v.Kernel.Alternate, subject, target)
	if !sampled && !alternated {
		return Sample{}, Sample{}
	}
	shape := NewBound(src, v, nil).valueShape(ref)
	var s, a Sample
	if sampled {
		s = Of(authoredValue(sampleLang, shape, sample))
	}
	if alternated {
		a = Of(authoredValue(alternateLang, shape, alternate))
	}
	return s, a
}

// stated returns the text one authored key states on the declaration
// that has a type, else on the declaration the type names, with the
// language of the declaration that states it.
func (v View) stated(k meta.Key[string], subject, target symbol.Identity) (string, symbol.Lang, bool) {
	if text, held := Fact(v, subject, k); held {
		return text, subject.Lang, true
	}
	if text, held := Fact(v, target, k); held {
		return text, target.Lang, true
	}
	return "", "", false
}

// Fact returns the value arbitration selects for a subject and a key,
// and records the read where the view has a read set. A view without
// facts, a zero key and a zero subject read nothing. It allocates what
// the read set allocates to record a new edge.
func Fact[T meta.FactValue](v View, id symbol.Identity, k meta.Key[T]) (T, bool) {
	var zero T
	if v.Facts == nil || k.IsZero() || id.IsZero() {
		return zero, false
	}
	if v.Reads == nil {
		return meta.Get(v.Facts, id, k)
	}
	return meta.Fact(v.Facts, v.Reads, id, k)
}

// authoredValue returns authored text as a value of a shape: a
// string literal for text, a truth value for a boolean, a number at
// the shape's width for a scalar, and raw text in lang for every
// other form.
func authoredValue(lang symbol.Lang, shape TypeShape, text string) emit.Value {
	switch shape.Form {
	case symbol.FormText:
		return emit.Literal(emit.LiteralString, text)
	case symbol.FormBool:
		return emit.Literal(emit.LiteralBool, text)
	case symbol.FormScalar:
		if shape.Class == ScalarFloat {
			return emit.Number(emit.LiteralFloat, text, shape.Bits)
		}
		return emit.Number(emit.LiteralInt, text, shape.Bits)
	default:
		return emit.Raw(lang, text)
	}
}
