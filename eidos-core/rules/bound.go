// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Bound is the kernel's walks and one language's decisions over one
// invocation's view. A handler obtains one from its match. It memoises
// [Bound.TypeOf] per reference and nothing else, for the life of the
// invocation.
//
// Use the Bound that [NewBound] returns. The zero Bound has no rules,
// and a method that calls the language's rules panics on it.
//
// # Concurrency
//
// A Bound is not safe for concurrent use, because the view it records
// into and the memo it folds into are not.
//
// # Allocation contract
//
// [NewBound] allocates the memo, one allocation. The memo stores each
// [TypeShape] out of line, because a shape is 200 bytes and a Go map
// stores a value above 128 bytes in an allocation of its own. Each
// method states what it allocates.
type Bound struct {
	source  SourceRules                   // the bound language's rules
	forLang func(symbol.Lang) SourceRules // another language's rules, for a contributor declared in it
	view    View                          // the invocation's view, which every read records into
	memo    map[*node.TypeRef]TypeShape   // the shapes folded so far, per reference
}

// NewBound binds a language's rules to a view. forLang returns another
// language's rules for a contributor or a target declared in it. A nil
// forLang gives every other language [Absent], and nil rules bind
// [Absent] for the zero language.
//
// # Allocation contract
//
// NewBound allocates the memo, one allocation.
func NewBound(source SourceRules, view View, forLang func(symbol.Lang) SourceRules) Bound {
	if source == nil {
		source = Absent("")
	}
	if forLang == nil {
		forLang = Absent
	}
	return Bound{source: source, forLang: forLang, view: view, memo: map[*node.TypeRef]TypeShape{}}
}

// Source returns the language's own rules, for the optional capability
// assertions. It allocates nothing.
func (b Bound) Source() SourceRules { return b.source }

// View returns the view the binding reads through. It allocates
// nothing.
func (b Bound) View() View { return b.view }

// Lang returns the bound language. It allocates nothing.
func (b Bound) Lang() symbol.Lang { return b.source.Lang() }

// CallableOf projects a function or a method, and reports false for any
// other symbol, nil included. The language classifies each parameter and
// each return, and names the error model.
//
// # Allocation contract
//
// CallableOf allocates the receiver's view of an instance method, the
// parameter list and the return list of a signature that has entries,
// and what the language's classification allocates. A symbol that is not
// callable allocates nothing.
func (b Bound) CallableOf(sym symbol.Symbol) (Callable, bool) { return b.callableOf(sym) }

// TypeOf folds a reference into its shape. It is total: a nil reference
// and one the language cannot classify fold to [symbol.FormOpaque].
//
// # Allocation contract
//
// TypeOf allocates nothing for a reference the binding folded before. A
// first fold allocates the shape's lists of children and arguments, the
// shape's entry in the memo, and the memo's storage as it grows. A first
// fold of a builtin into an empty memo allocates two: the memo's first
// group and the shape.
func (b Bound) TypeOf(ref *node.TypeRef) TypeShape { return b.typeOf(ref) }

// MembersOf walks a type's effective member set under the language's
// [MemberPolicy], and reports false for a symbol that is not a type.
//
// # Allocation contract
//
// MembersOf allocates the members it returns, the gaps it returns, and
// the path of each contributor it descends into. A contributor with type
// arguments allocates the members it restates. The walk's working
// storage comes from a pool. A walk of a struct that embeds one type
// allocates two: the members and the one path.
func (b Bound) MembersOf(sym symbol.Symbol) (MemberSet, bool) { return b.membersOf(sym) }

// SamplesOf returns two distinct values of a type. The values an
// author stated on the declaration that has the type, and on the
// type's own declaration, come first through [View.Authored]. The
// language derives each half no author stated, and [Complete] pairs
// the two. subject is the declaration that has the type, zero for
// none. The zero view refuses both halves with [RefusedNoView].
//
// # Allocation contract
//
// SamplesOf allocates nothing of its own. It allocates what
// [View.Authored] allocates to lift a stated value, and what the
// language's SamplesOf allocates to derive a missing half.
func (b Bound) SamplesOf(subject symbol.Identity, ref *node.TypeRef, hint string) (Sample, Sample) {
	return b.samplesOf(subject, ref, hint)
}

// ZeroValue returns the type's zero value, and reports false where the
// language has none, for a nil reference, and on the zero view. It
// allocates what the language's ZeroValue allocates.
func (b Bound) ZeroValue(ref *node.TypeRef) (emit.Value, bool) {
	if b.view.IsZero() || ref == nil {
		return emit.Value{}, false
	}
	return b.source.ZeroValue(ref, b.view)
}

// LiteralFor turns text that went through the language's quoting into
// a value of a type, and reports false where it cannot be one, for a nil
// reference, and on the zero view. It allocates what the language's
// LiteralFor allocates.
func (b Bound) LiteralFor(f *node.File, ref *node.TypeRef, text string) (emit.Value, bool) {
	if b.view.IsZero() || ref == nil {
		return emit.Value{}, false
	}
	return b.source.LiteralFor(f, ref, text, b.view)
}

// TypeName joins a generator's word onto a base name the way the
// language spells a derived type name. It allocates what the language's
// TypeName allocates: one string for a join by concatenation.
func (b Bound) TypeName(word, base string) string { return b.source.TypeName(word, base) }

// Witnesses returns one reference per type parameter, the authored
// witnesses first and the derived ones where the language can. It
// returns nil where any parameter has neither, for no parameters, and on
// the zero view.
//
// # Allocation contract
//
// Witnesses allocates the list, one reference per authored witness, and
// what the language's Derive allocates for each derived one.
func (b Bound) Witnesses(params []*node.TypeParam) []*node.TypeRef { return b.witnesses(params) }
