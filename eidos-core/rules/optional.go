// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
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
	Duplicate  string        // the identifier of the first variant whose value a later one repeats; "" when none
	OutOfRange emit.Value    // a value outside the declared set; zero when none derives
	Foreign    []string      // packages declaring variants outside the type's own, sorted
}

// VariantText is one variant's identifier and its textual form as
// a string literal, quoted by the language that spells it.
type VariantText struct {
	Name string     // the variant's identifier
	Text emit.Value // the variant's textual form, a string literal
}

// ErrorValueRules names error values. SentinelBase recognises the names
// that SentinelName returns, so a name and its recognition cannot
// disagree.
type ErrorValueRules interface {
	// SentinelName returns the name of the error value a generator
	// derives from a base name.
	SentinelName(base string) string
	// SentinelBase returns the base that SentinelName composes ident
	// from, and reports false where ident is not a name that
	// SentinelName returns.
	SentinelBase(ident string) (string, bool)
}

// ErrorTypeRules is the optional capability of a language whose types can
// be errors, such as a Go type with the method Error and a TypeScript
// class that extends Error. [Bound.ErrorContract] calls it.
type ErrorTypeRules interface {
	// ErrorContract returns the contract of decl as an error type, and
	// reports false where decl is not one. It reads the members of decl
	// through v.
	ErrorContract(decl symbol.Symbol, v View) (ErrorContract, bool)
}

// ErrorContract is what a language states about an error type. The zero
// value is the contract of an error type that inherits its message, and
// that declares no cause and no hooks.
type ErrorContract struct {
	// Message is the member that returns the message of the error: the
	// method Error of a Go type, and the field message where a TypeScript
	// class declares one. It is nil where the type inherits the member
	// from the error of its language.
	Message symbol.Symbol
	// Cause is the field that stores the error that caused this one, and
	// nil where the type has none.
	Cause *node.Field
	// Unwrap, Is and As are the hooks that the error functions of the
	// language call, and nil where the type does not declare one.
	Unwrap, Is, As *node.Method
	// Pointer reports that a pointer to the type is the error and a value
	// of the type is not, as a Go method Error with a pointer receiver
	// states.
	Pointer bool
}

// ResultRules is the optional capability of a language whose callable
// reports a failure in the type of its return, as a Rust function returns
// Result<T, E>. [Bound.CallableOf] asks it for each return of a callable
// whose error model is [ErrorsResultType], and splits a return of a result
// type into the view of its value and the view of its failure.
type ResultRules interface {
	// Result returns the reference of the value and the reference of the
	// failure of the result type ref, and reports false for a reference of
	// another type. value is nil for a result without a value, such as
	// Result<(), E>. Result reads through v, so each read records.
	Result(ref *node.TypeRef, v View) (value, failure *node.TypeRef, ok bool)
}

// TextRules is the optional capability of a language whose text types take
// a string literal through a conversion, as a String of Rust takes
// String::from("a"). [View.Authored] asks it for the value of a text that an
// author stated for a type that folds to text.
type TextRules interface {
	// Text returns the value of the type ref whose content is text, and the
	// string literal of text where the type takes the literal itself. Text
	// reads through v, so each read records.
	Text(ref *node.TypeRef, text string, v View) emit.Value
}

// ContextRules is the optional capability of a language whose callables
// take a context that their signatures do not state, as every rpc of
// protobuf takes the cancellation and the deadline of its call.
// [Bound.CallableOf] asks it for each callable and puts the view of the
// context before the parameters.
type ContextRules interface {
	// Context returns the view of the context of the callable sym, with the
	// role [ParamContext], and reports false for a callable without one.
	Context(sym symbol.Symbol, v View) (ParamView, bool)
}

// ImportRules is the optional capability of a language that restricts the
// declarations that the code of a package can refer to. Go code imports no
// internal package of another tree and no external test package, and refers
// to no unexported name of another package. [Bound.Importable] calls it.
type ImportRules interface {
	// Importable returns the reason that the code of the package pkg cannot
	// refer to the referent of ref, and nil where it can. It checks each
	// reference inside ref too, such as the element of a list.
	Importable(pkg symbol.Identity, ref *node.TypeRef) error
}

// ValueImportRules is the optional capability of a language that restricts
// the declarations to which the values in the code of a package can refer,
// as [ImportRules] restricts the declarations of a type reference.
// [Bound.ImportableValue] calls it.
type ValueImportRules interface {
	// ImportableValue returns the reason that the code of the package pkg
	// cannot refer to a declaration of v, and nil where it can. It checks
	// the callee and the type of v, and the fields, the keys, the arguments
	// and the inner value that v contains.
	ImportableValue(pkg symbol.Identity, v emit.Value) error
}

// TagRules reads a language's per-field tags.
type TagRules interface {
	// Tag returns a field's tag value under a key, and reports false
	// where the field has no tag under it.
	Tag(f *node.Field, key string) (string, bool)
}

// WellKnownRules is the optional capability of a language that declares
// a type of its standard library as a well-known type, such as Go's
// time.Time. The fold calls the rules of the language of a target before
// it classifies the target.
type WellKnownRules interface {
	// WellKnown returns the well-known identity of the declaration id,
	// and reports false for a declaration that the registry does not
	// bless. It does not allocate.
	WellKnown(id symbol.Identity) (symbol.Identity, bool)
}

// ShapeRules is the optional capability of a language that declares a type
// whose values have the shape of another form, such as a GraphQL custom
// scalar whose specification is a signed integer of 64 bits and the class
// list of Python, whose values are lists. The fold asks the rules of the
// language of a target after [WellKnownRules] and before it classifies the
// target.
type ShapeRules interface {
	// ShapeOf returns the shape of the values of the reference ref to
	// decl, and reports false for a declaration whose references fold to a
	// reference to it. The shape can depend on the type arguments of ref,
	// as the shape of Literal["a"] of Python does. The fold gives the shape
	// the spelling of the reference. Where the shape is a structural form
	// without children and ref has type arguments, the fold takes the
	// folded arguments as the children of the form, and the first argument
	// alone as the child of a stream, as it does for the shape of
	// [SourceRules.Builtin]. ShapeOf reads through v, so each read records.
	ShapeOf(ref *node.TypeRef, decl symbol.Symbol, v View) (TypeShape, bool)
}

// SatisfactionRules is the optional capability of a language whose
// satisfaction of an interface reads more than the member sets: a method
// that belongs to a pointer to its type alone, such as a Go method with a
// pointer receiver, and an interface that constrains a type set, such as
// a Go constraint. [Bound.Satisfies] calls it.
type SatisfactionRules interface {
	// ValueReceiver reports whether a value of the type of m has m. It
	// allocates nothing beyond what the view records.
	ValueReceiver(m *node.Method, v View) bool
	// TypeSet reports whether i constrains a type set beside its
	// methods, which a comparison of members cannot prove. It allocates
	// nothing beyond what the view records.
	TypeSet(i *node.Interface, v View) bool
}

// GenericsRules reasons about type parameters. The kernel substitutes
// arguments for parameters itself, through [Substitute].
type GenericsRules interface {
	// Derive returns a witness for one parameter that the author left
	// unstated. It reports false where the type set of the bound is not
	// knowable without loading the declaring package.
	Derive(p *node.TypeParam, v View) (*node.TypeRef, bool)
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
