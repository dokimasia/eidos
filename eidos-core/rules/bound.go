// Copyright Dokimasia B.V. 2026
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
// [TypeShape] out of line, because a shape is 184 bytes and a Go map
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
// forLang takes the resolver of the view from [View.WithRules], and gives
// every other language [Absent] where the view has none. Nil rules bind
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
		forLang = view.forLang
	}
	if forLang == nil {
		forLang = Absent
	}
	return Bound{source: source, forLang: forLang, view: view, memo: map[*node.TypeRef]TypeShape{}}
}

// Source returns the rules of the bound language. A plugin reads the
// optional capabilities of the rules through the methods of Bound, such
// as [Bound.Tag], and asserts none itself. It allocates nothing.
func (b Bound) Source() SourceRules { return b.source }

// View returns the view the binding reads through. It allocates
// nothing.
func (b Bound) View() View { return b.view }

// Lang returns the bound language. It allocates nothing.
func (b Bound) Lang() symbol.Lang { return b.source.Lang() }

// CallableOf projects a function or a method, and reports false for any
// other symbol, nil included. The language classifies each parameter and
// each return, and names the error model. Under [ErrorsResultType], a
// language with [ResultRules] splits a return of a result type into its
// value and its failure.
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

// FieldTypeOf folds the type of a field with the field's presence: the
// shape that TypeOf returns for f.Type, inside an optional where f has
// presence. f has presence where f.Optional is set, as on a TypeScript
// property with a question mark, or where the bound language implements
// [PresenceRules] and reports it. FieldTypeOf returns the shape unchanged
// where it is an optional or [symbol.FormDynamic], because both contain
// the absent value. A nil field folds to Opaque.
//
// # Allocation contract
//
// FieldTypeOf allocates what TypeOf allocates and what the language's
// Presence allocates, and the list of the one child where it adds the
// optional.
func (b Bound) FieldTypeOf(f *node.Field) TypeShape {
	if f == nil {
		return Opaque(nil)
	}
	s := b.typeOf(f.Type)
	if s.Form == symbol.FormOptional || s.Form == symbol.FormDynamic {
		return s
	}
	present := f.Optional
	if !present {
		if p, has := b.source.(PresenceRules); has {
			present = p.Presence(f, b.view)
		}
	}
	if !present {
		return s
	}
	return TypeShape{Form: symbol.FormOptional, Spelling: s.Spelling, Elems: []TypeShape{s}}
}

// MembersOf walks a type's effective member set under the language's
// [MemberPolicy]. It reports false for a symbol without member lists of
// its own. An alias has none.
//
// A contributor whose declaration is an alias contributes these members at
// the depth of the alias:
//   - the methods that the package of the alias declares outside every type
//     with the name of the alias as their owner, as Go declares the methods
//     of a defined type
//   - for a transparent alias, what its target contributes
//   - for a defined type, the fields and the embeds of a struct under it,
//     or every member of an interface under it
//
// A defined type has none of the methods that the type under it declares.
// A builtin or an array under it contributes nothing more.
//
// # Allocation contract
//
// MembersOf allocates the members it returns, the gaps it returns, and
// the path of each contributor it descends into. A contributor with type
// arguments allocates the members it restates, and so does a contributor
// whose declaration is a generic alias. The walk's working storage, the
// linearizations of [ShadowLinearise] included, comes from a pool. A walk
// of a struct that embeds one type allocates two: the members and the one
// path. A defined type costs the same, because its methods are the
// declarations of its package.
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
// DeriveSamples of the language allocates to derive a missing half.
func (b Bound) SamplesOf(subject symbol.Identity, ref *node.TypeRef, hint string) (Sample, Sample) {
	return b.samplesOf(subject, ref, hint)
}

// Tag returns the value of the tag of f under key. It reports false
// where f has no tag under key, for a nil f, and where the language does
// not implement [TagRules]. It allocates what the Tag of the language
// allocates.
func (b Bound) Tag(f *node.Field, key string) (string, bool) {
	tags, declared := b.source.(TagRules)
	if !declared || f == nil {
		return "", false
	}
	return tags.Tag(f, key)
}

// SentinelName returns the name of the error value that a generator
// derives from base. It reports false where the language does not
// implement [ErrorValueRules]. It allocates what the SentinelName of the
// language allocates.
func (b Bound) SentinelName(base string) (string, bool) {
	names, declared := b.source.(ErrorValueRules)
	if !declared {
		return "", false
	}
	return names.SentinelName(base), true
}

// SentinelBase returns the base that the sentinel name ident composes. It
// reports false where ident does not follow the convention of the
// language for the name of an error value, and where the language does
// not implement [ErrorValueRules]. It allocates what the SentinelBase of
// the language allocates.
func (b Bound) SentinelBase(ident string) (string, bool) {
	names, declared := b.source.(ErrorValueRules)
	if !declared {
		return "", false
	}
	return names.SentinelBase(ident)
}

// Importable returns the reason that the code of the package pkg cannot
// refer to the referent of ref, and nil where it can, through the rules of
// the bound language. A generator calls it before it records a
// declaration with ref into a file of pkg, such as a method of another
// package whose signature refers to an internal package. For a reference
// whose target is a function, the result states whether the code can call
// the function. Importable returns nil for a nil reference and where the
// language does not implement [ImportRules]. It allocates what the
// Importable of the language allocates.
func (b Bound) Importable(pkg symbol.Identity, ref *node.TypeRef) error {
	imports, declared := b.source.(ImportRules)
	if !declared || ref == nil {
		return nil
	}
	return imports.Importable(pkg, ref)
}

// ErrorContract returns the contract of decl as an error type, through
// the rules of the language of decl. It reports false where decl is not a
// declaration, where it is not an error type, and where those rules do not
// implement [ErrorTypeRules]. It allocates what the ErrorContract of the
// language allocates.
func (b Bound) ErrorContract(decl symbol.Symbol) (ErrorContract, bool) {
	d, declares := decl.(node.Declaration)
	if !declares {
		return ErrorContract{}, false
	}
	errs, declared := b.rulesOf(d.Identity().Lang).(ErrorTypeRules)
	if !declared {
		return ErrorContract{}, false
	}
	return errs.ErrorContract(decl, b.view)
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

// LiteralFor reads text in the language as a value of a type in the scope
// s, which [View.ScopeOf] returns for a declaration. It reports false
// where the text is no such value, for a nil reference, and on the zero
// view. It allocates what the LiteralFor of the language allocates.
func (b Bound) LiteralFor(s Scope, ref *node.TypeRef, text string) (emit.Value, bool) {
	if b.view.IsZero() || ref == nil {
		return emit.Value{}, false
	}
	return b.source.LiteralFor(s, ref, text, b.view)
}

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

// rulesOf returns the rules of a language: the bound rules for the bound
// language, and the rules that the resolver of the binding returns for
// another one. It allocates nothing beyond what the resolver allocates.
func (b Bound) rulesOf(lang symbol.Lang) SourceRules {
	if lang == b.source.Lang() {
		return b.source
	}
	return b.forLang(lang)
}
