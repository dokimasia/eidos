// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Bound is the kernel's walks and one language's decisions over
// one invocation's view: what a handler reaches through its match.
// It memoises [Bound.TypeOf] per reference for the invocation and
// nothing else, and dies with the invocation.
//
// A Bound is not safe for concurrent use, because the view it
// records into is not.
type Bound struct {
	source  SourceRules
	forLang func(symbol.Lang) SourceRules
	view    View
	memo    map[*node.TypeRef]TypeShape
}

// NewBound binds a language's rules to a view. forLang returns
// another language's rules for a contributor or a target declared
// in it; nil gives every other language [Absent].
func NewBound(source SourceRules, view View, forLang func(symbol.Lang) SourceRules) Bound {
	if source == nil {
		source = Absent("")
	}
	if forLang == nil {
		forLang = Absent
	}
	return Bound{source: source, forLang: forLang, view: view, memo: map[*node.TypeRef]TypeShape{}}
}

// Source returns the language's own rules, for the optional
// capability assertions.
func (b Bound) Source() SourceRules { return b.source }

// View returns the view the bound reads through.
func (b Bound) View() View { return b.view }

// Lang names the bound language.
func (b Bound) Lang() symbol.Lang { return b.source.Lang() }

// CallableOf projects a function or a method, and reports false
// for any other kind.
func (b Bound) CallableOf(sym symbol.Symbol) (Callable, bool) { return b.callableOf(sym) }

// TypeOf folds a reference into its shape. It is total: a nil
// reference and one the language cannot classify fold to Opaque.
func (b Bound) TypeOf(ref *node.TypeRef) TypeShape { return b.typeOf(ref) }

// MembersOf walks a type's effective member set, and reports false
// for a symbol that is not a type.
func (b Bound) MembersOf(sym symbol.Symbol) (MemberSet, bool) { return b.membersOf(sym) }

// SamplesOf returns two distinct values of a type: the authored
// ones on the declaration carrying the type and on the type's own
// declaration first, then the language's derivation. subject is
// the carrying declaration, zero for none.
func (b Bound) SamplesOf(subject symbol.Identity, ref *node.TypeRef, hint string) (Sample, Sample) {
	return b.samplesOf(subject, ref, hint)
}

// ZeroValue returns the type's zero value, and reports false where
// the language has none.
func (b Bound) ZeroValue(ref *node.TypeRef) (emit.Value, bool) {
	if b.view.IsZero() || ref == nil {
		return emit.Value{}, false
	}
	return b.source.ZeroValue(ref, b.view)
}

// LiteralFor turns text that went through the language's quoting
// into a value of a type, and reports false where it cannot be one.
func (b Bound) LiteralFor(f *node.File, ref *node.TypeRef, text string) (emit.Value, bool) {
	if b.view.IsZero() || ref == nil {
		return emit.Value{}, false
	}
	return b.source.LiteralFor(f, ref, text, b.view)
}

// TypeName joins a generator's word onto a base name the way the
// language spells a derived type name.
func (b Bound) TypeName(word, base string) string { return b.source.TypeName(word, base) }

// Witnesses returns one reference per type parameter, the authored
// witnesses first and the derived ones where the language can, or
// nil where any parameter has neither.
func (b Bound) Witnesses(params []*node.TypeParam) []*node.TypeRef { return b.witnesses(params) }
