// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
)

// EnumRules projects an enumeration.
type EnumRules interface {
	// EnumOf returns what the language states about one enumeration.
	EnumOf(e *node.Enum, v View) EnumInfo
}

// EnumForm names where a variant's textual form comes from.
type EnumForm uint8

const (
	// EnumIdentifier derives the text from the variant's name: the
	// only form where the declared value has no text.
	EnumIdentifier EnumForm = iota
	// EnumValue takes the text from the declared value, which for a
	// textual enumeration is the textual form.
	EnumValue
)

// String returns the form's spelling, and the decimal number of an
// undeclared form. It allocates nothing for a declared form.
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
	Name string     // the variant's identifier
	Text emit.Value // the variant's textual form, a string literal
}

// ErrorValueRules names error values. Its two methods are inverses, so
// a name and its recognition cannot disagree.
type ErrorValueRules interface {
	// SentinelName returns the name of the error value a generator
	// derives from a base name.
	SentinelName(base string) string
	// IsSentinelName reports whether an identifier is a name
	// SentinelName returns.
	IsSentinelName(ident string) bool
}

// TagRules reads a language's per-field tags.
type TagRules interface {
	// Tag returns a field's tag value under a key, and reports false
	// where the field has no tag under it.
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
	// runtime. A Java backend lowers differently under erasure.
	Reified() bool
}

// PropertyRules computes the properties view: getters paired with
// setters. Computing the view leaves the model unchanged.
type PropertyRules interface {
	// Properties returns the computed properties of a struct.
	Properties(s *node.Struct, v View) []Property
}

// Property is one computed property.
type Property struct {
	Name   string        // the property's name
	Type   *node.TypeRef // the type the getter returns
	Getter *node.Method  // the method that reads the property
	Setter *node.Method  // nil for a read-only property
}

// ConstructRules returns the constructors of a type.
type ConstructRules interface {
	// Constructors returns the callables that construct a struct.
	Constructors(s *node.Struct, v View) []Callable
}

// ThrowsRules returns the failure types a callable declares.
type ThrowsRules interface {
	// Throws returns the failure types a callable declares.
	Throws(c Callable) []*node.TypeRef
}

// OwnershipRules returns how a parameter is passed.
type OwnershipRules interface {
	// Ownership returns how a parameter is passed.
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

// String returns the ownership's spelling, and the decimal number of an
// undeclared ownership. It allocates nothing for a declared ownership.
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
// package can set, promotion included, in declaration order, with a
// gap for every contributor the member walk could not read. A
// generator building a constructor over a set with gaps reports it
// incomplete and does not write a partial builder.
type PromotionRules interface {
	// Settable returns the members of a struct that a constructor in
	// another package can set.
	Settable(s *node.Struct, v View) MemberSet
}

// PresenceRules reports presence that a field's type does not contain. A
// field has presence when a reader can tell a field that is not set from
// a field that is set to the zero value of its type. protobuf implements
// it, because a proto3 field of a message type has presence through the
// message, which only the linked graph shows.
type PresenceRules interface {
	// Presence reports whether f has presence that its type does not
	// contain. It reads the declaration that the type of f resolves to
	// through v, so the caller's read set records the dependency.
	Presence(f *node.Field, v View) bool
}

// EqualityRules reports whether a type works where the language
// demands equality, and which member references break it.
type EqualityRules interface {
	// Comparable reports whether a type supports the language's
	// equality, and returns the member references that break it.
	Comparable(ref *node.TypeRef, v View) (ok bool, problems []*node.TypeRef)
}
