// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
)

// The optional capabilities. A language declares one by satisfying
// the interface on its [SourceRules] value, and a consumer finds it
// by asserting on [Bound.Source]. A consumer that asserts and is
// refused reports once and generates nothing for that language,
// which beats a projection built on a default nobody chose.

// EnumRules projects an enumeration.
type EnumRules interface {
	EnumOf(e *node.Enum, v View) EnumInfo
}

// EnumForm says where a variant's textual form comes from.
type EnumForm uint8

const (
	// EnumIdentifier derives the text from the variant's name: the
	// only form where the declared value carries no text.
	EnumIdentifier EnumForm = iota
	// EnumValue takes the text from the declared value, which for a
	// textual enumeration is the textual form.
	EnumValue
)

// String returns the form's spelling.
func (f EnumForm) String() string {
	switch f {
	case EnumIdentifier:
		return "identifier"
	case EnumValue:
		return "value"
	default:
		return strconv.Itoa(int(f))
	}
}

// EnumInfo is what a language returns about one enumeration.
type EnumInfo struct {
	Form       EnumForm      // where a variant's text comes from
	Variants   []VariantText // in declaration order
	Zero       string        // the variant whose value is the type's zero; "" when none
	Duplicate  string        // the first text two variants share; "" when none
	OutOfRange emit.Value    // a value outside the declared set; zero when none derives
	Foreign    []string      // packages declaring variants outside the type's own, sorted
}

// VariantText is one variant's identifier and its textual form as
// a string literal, quoted by the language that spells it.
type VariantText struct {
	Name string
	Text emit.Value
}

// ErrorValueRules names error values: paired inverses, so the two
// can never drift.
type ErrorValueRules interface {
	SentinelName(base string) string
	IsSentinelName(ident string) bool
}

// TagRules reads a language's per-field tags.
type TagRules interface {
	Tag(f *node.Field, key string) (string, bool)
}

// GenericsRules reasons about type parameters.
type GenericsRules interface {
	// Derive returns a witness for one parameter the author left
	// unstated, and reports false where the bound's type set is
	// not knowable without loading the declaring package.
	Derive(p *node.TypeParam, v View) (*node.TypeRef, bool)
	// Substitute rewrites a reference with each parameter replaced
	// by its argument, copying what it rewrites, and returns the
	// reference unchanged where it names no parameter.
	Substitute(ref *node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) *node.TypeRef
	// Reified reports whether the language keeps type arguments at
	// runtime; a Java backend lowers differently under erasure.
	Reified() bool
}

// PropertyRules computes the properties view: getters paired with
// setters. A projection, never a change to the model.
type PropertyRules interface {
	Properties(s *node.Struct, v View) []Property
}

// Property is one computed property.
type Property struct {
	Name   string
	Type   *node.TypeRef
	Getter *node.Method
	Setter *node.Method // nil for a read-only property
}

// ConstructRules returns the constructors of a type.
type ConstructRules interface {
	Constructors(s *node.Struct, v View) []Callable
}

// ThrowsRules returns the failure types a callable declares.
type ThrowsRules interface {
	Throws(c Callable) []*node.TypeRef
}

// OwnershipRules says how a parameter is passed.
type OwnershipRules interface {
	Ownership(p ParamView) Ownership
}

// Ownership is how a parameter is passed.
type Ownership uint8

const (
	// OwnByValue moves or copies the value.
	OwnByValue Ownership = iota
	// OwnBorrow lends the value read-only.
	OwnBorrow
	// OwnBorrowMut lends the value for mutation.
	OwnBorrowMut
)

// String returns the ownership's spelling.
func (o Ownership) String() string {
	switch o {
	case OwnByValue:
		return "by-value"
	case OwnBorrow:
		return "borrow"
	case OwnBorrowMut:
		return "borrow-mut"
	default:
		return strconv.Itoa(int(o))
	}
}

// PromotionRules returns the members a constructor in another
// package can set, promotion included, in declaration order.
type PromotionRules interface {
	Settable(s *node.Struct, v View) []Member
}

// EqualityRules reports whether a type works where the language
// demands equality, and which member references break it.
type EqualityRules interface {
	Comparable(ref *node.TypeRef, v View) (ok bool, problems []*node.TypeRef)
}
