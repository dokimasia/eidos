// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// View is what a projection reads through. It has the tracked declaration
// reader of the invocation, the arbitrated facts of the run, the read set
// into which both record, and the keys of the kernel for the authored
// values that the walks read first. The workspace builds one for each
// invocation with [NewView].
//
// A view reads a fact through [Fact] alone, which records the read. A
// rules implementation reads a key of another registrant through a handle
// from [meta.Named], which [Fact] resolves by its name. The fact store of
// the view is not exported, so no read bypasses the read set.
//
// The zero View reads nothing. A read of an untracked graph would record
// no edge, so a projection that receives the zero View refuses.
//
// # Concurrency
//
// A View is not safe for concurrent use, because the read set into which it
// records is not.
//
// # Allocation contract
//
// A read allocates only to record a new edge in the read set.
// [View.Authored] states what it allocates to lift a stated value.
type View struct {
	decls  *store.Reader
	facts  *meta.Facts
	reads  meta.Recorder
	kernel meta.KernelKeys
	// forLang returns the rules of another language for a binding that a
	// rules implementation makes through the view, and is nil where the
	// view resolves no rules.
	forLang func(symbol.Lang) SourceRules
}

// NewView returns a view over decls and facts, whose fact reads record into
// reads. With a nil recorder the view reads without tracking, as only the
// walks of the kernel do. kernel contains the keys of the kernel for the
// authored values. A nil decls returns the zero view, and a nil facts
// reads no fact. NewView allocates nothing.
func NewView(decls *store.Reader, facts *meta.Facts, reads meta.Recorder, kernel meta.KernelKeys) View {
	return View{decls: decls, facts: facts, reads: reads, kernel: kernel}
}

// WithRules returns a copy of v whose bindings resolve the rules of
// another language through forLang. [NewBound] reads it where its own
// forLang is nil, so a binding that a rules implementation makes through
// the view resolves the rules that the match resolves. WithRules does
// not allocate.
func (v View) WithRules(forLang func(symbol.Lang) SourceRules) View {
	v.forLang = forLang
	return v
}

// IsZero reports whether the view reads nothing. It allocates nothing.
func (v View) IsZero() bool { return v.decls == nil }

// Reader returns the tracked declaration reader of the view, and nil for
// the zero view. It allocates nothing.
func (v View) Reader() *store.Reader { return v.decls }

// Kernel returns the keys of the kernel under which the view reads the
// authored values. It allocates nothing.
func (v View) Kernel() meta.KernelKeys { return v.kernel }

// Lookup returns one declaration by identity, and records the read. The
// zero view and the zero identity return nothing. It allocates what the
// read set allocates to record a new edge.
func (v View) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	if v.decls == nil || id.IsZero() {
		return nil, false
	}
	return v.decls.Lookup(id)
}

// PackageOf returns the package that a declaration is in, and records the
// read. The zero view and the zero identity return nothing. It allocates
// what the read set allocates to record a new edge.
func (v View) PackageOf(id symbol.Identity) (*node.Package, bool) {
	if v.decls == nil || id.IsZero() {
		return nil, false
	}
	return v.decls.PackageOf(id)
}

// ScopeOf returns the scope of a declaration, which has the identity of
// the declaration and the file that contains its position. It records the
// reads of the declaration and of its package, because a change of either
// can change the file. The scope of a declaration that the view does not
// contain has no file, and so has the scope of a declaration whose
// position is in no file of its package. ScopeOf allocates what the read
// set allocates to record a new edge.
func (v View) ScopeOf(id symbol.Identity) Scope {
	s := Scope{Subject: id}
	decl, declared := v.Lookup(id)
	pkg, placed := v.PackageOf(id)
	if !declared || !placed {
		return s
	}
	at := decl.Position().File
	for _, f := range pkg.Files {
		if f != nil && f.Pos.File == at {
			s.File = f
			return s
		}
	}
	return s
}

// Authored returns the two values that an author stated for a value of a
// type. Each half reads the declaration that has the type first, and then
// the declaration of the type. The two halves resolve independently. A
// half that neither declaration states is the zero [Sample], which the
// caller derives. A zero subject and a reference without a target state
// nothing. Every fact read records on the view.
//
// A stated text lifts by the shape that src folds for the type, with an
// alias followed to its target:
//   - Text lifts to a string literal of the text.
//   - A boolean lifts to a truth value.
//   - A scalar lifts to a number at the width of the shape.
//   - Every other form lifts through the LiteralFor of src, in the scope
//     of the declaration that states the text, where that declaration has
//     the language of src. A text that LiteralFor does not read lifts to
//     raw text in the language of the declaration, and so does a text of a
//     declaration in another language. Only the backend of that language
//     spells raw text.
//
// Nil rules read no text. The kernel reads the authored values of a type
// here before it asks the language to derive. The composite walk of a
// language reads the values of each element here before it derives the
// element. The type folds only where a half is stated.
//
// # Allocation contract
//
// Authored allocates nothing where no declaration states a value. A stated
// value allocates the fold of its type in a binding of its own, which is
// the memo of the binding, the first group of the memo and the shape,
// three allocations for a builtin type, and the children of a structural
// one. A text of another form also allocates what LiteralFor allocates,
// and what the read set allocates to record the reads of [View.ScopeOf].
func (v View) Authored(src SourceRules, subject symbol.Identity, ref *node.TypeRef) (Sample, Sample) {
	if src == nil {
		src = Absent("")
	}
	var target symbol.Identity
	if ref != nil {
		target = ref.Target
	}
	sample, sampleBy, sampled := v.stated(v.kernel.Sample, subject, target)
	alternate, alternateBy, alternated := v.stated(v.kernel.Alternate, subject, target)
	if !sampled && !alternated {
		return Sample{}, Sample{}
	}
	shape := NewBound(src, v, nil).valueShape(ref)
	var s, a Sample
	if sampled {
		s = Of(v.authoredValue(src, sampleBy, ref, shape, sample))
	}
	if alternated {
		a = Of(v.authoredValue(src, alternateBy, ref, shape, alternate))
	}
	return s, a
}

// stated returns the text that one authored key states on the declaration
// that has a type, or else on the declaration of the type, with the
// identity of the declaration that states it.
func (v View) stated(k meta.Key[string], subject, target symbol.Identity) (string, symbol.Identity, bool) {
	if text, held := Fact(v, subject, k); held {
		return text, subject, true
	}
	if text, held := Fact(v, target, k); held {
		return text, target, true
	}
	return "", symbol.Identity{}, false
}

// authoredValue returns authored text as a value of a shape. It returns a
// string literal for text, a truth value for a boolean, and a number at
// the width of the shape for a scalar. For every other form, it returns
// the value that the LiteralFor of src reads in the scope of by, the
// declaration that states the text, where by has the language of src. It
// returns raw text in the language of by otherwise.
func (v View) authoredValue(
	src SourceRules, by symbol.Identity, ref *node.TypeRef, shape TypeShape, text string,
) emit.Value {
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
		// Every other form lifts through the rules below.
	}
	if by.Lang == src.Lang() {
		if value, read := src.LiteralFor(v.ScopeOf(by), ref, text, v); read {
			return value
		}
	}
	return emit.Raw(by.Lang, text)
}

// Fact returns the value that the arbitration selects for a subject and a
// key, and records the read where the view has a read set. A named handle
// from [meta.Named] resolves by its name in the registry of the facts, and
// a name that the registry does not contain under T reads nothing. A view
// without facts, a zero key and a zero subject read nothing. It allocates
// what the read set allocates to record a new edge.
func Fact[T meta.FactValue](v View, id symbol.Identity, k meta.Key[T]) (T, bool) {
	var zero T
	if v.facts == nil || k.IsZero() || id.IsZero() {
		return zero, false
	}
	if v.reads == nil {
		return meta.Get(v.facts, id, k)
	}
	return meta.Fact(v.facts, v.reads, id, k)
}

// Unaliased follows a chain of transparent aliases that the view
// declares, and returns the reference at its end. A defined type is a
// type of its own, so the walk stops at one, and at an alias without a
// target. It follows at most eight aliases, so an alias cycle in a broken
// source ends. A nil reference returns nil, and every lookup records its
// read. Unaliased allocates what the read set allocates to record a new
// edge.
func Unaliased(ref *node.TypeRef, v View) *node.TypeRef {
	for range aliasDepth {
		if ref == nil || ref.Target.Kind != symbol.KindAlias {
			return ref
		}
		sym, _ := v.Lookup(ref.Target)
		alias, isAlias := sym.(*node.Alias)
		if !isAlias || alias.Defined || alias.Target == nil {
			return ref
		}
		ref = alias.Target
	}
	return ref
}
