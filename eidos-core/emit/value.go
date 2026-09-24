// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"strconv"

	"go.dokimi.dev/eidos/core/symbol"
)

// ValueKind selects a value's populated fields. The zero kind
// names no value.
type ValueKind uint8

const (
	// ValueLiteral is Literal and Text: a number, a string's
	// content, a truth value, the absent value, or raw text in the
	// source language.
	ValueLiteral ValueKind = iota + 1
	// ValueConversion applies Type to Inner: Weekday(42).
	ValueConversion
	// ValueComposite is Type with Fields: Point{X: 42}, positional
	// where a field has no name.
	ValueComposite
	// ValueCall applies Callee to Args: time.Unix(42, 0).
	ValueCall
	// ValueAddress takes the address of Inner: &Point{X: 42}.
	ValueAddress
)

// String returns the kind's spelling, and the number for a kind
// nothing declares. Findings name value kinds, so a consumer
// matching on the spelling matches on API.
func (k ValueKind) String() string {
	switch k {
	case ValueLiteral:
		return "literal"
	case ValueConversion:
		return "conversion"
	case ValueComposite:
		return "composite"
	case ValueCall:
		return "call"
	case ValueAddress:
		return "address"
	default:
		return strconv.Itoa(int(k))
	}
}

// LiteralKind names what a literal's text is, so a target spells
// it its own way: the string's quotes, the absent value's name.
// The zero kind names no literal.
type LiteralKind uint8

const (
	// LiteralInt is an integer in decimal text.
	LiteralInt LiteralKind = iota + 1
	// LiteralFloat is a floating-point number in decimal text.
	LiteralFloat
	// LiteralString is a string. Text is the content, unquoted.
	LiteralString
	// LiteralBool is a truth value. Text is "true" or "false".
	LiteralBool
	// LiteralNil is the language's absent value. Text is empty.
	LiteralNil
	// LiteralRaw is Text in the source language, which only that
	// language's backend spells and another language's refuses.
	LiteralRaw
)

// String returns the kind's spelling, and the number for a kind
// nothing declares.
func (k LiteralKind) String() string {
	switch k {
	case LiteralInt:
		return "int"
	case LiteralFloat:
		return "float"
	case LiteralString:
		return "string"
	case LiteralBool:
		return "bool"
	case LiteralNil:
		return "nil"
	case LiteralRaw:
		return "raw"
	default:
		return strconv.Itoa(int(k))
	}
}

// Value is one value a generated body writes: a sample, a zero, a
// literal read from text. Every reference in it is qualified by
// the backend for the file it is written into, which is what text
// alone could never do. Kind selects the populated fields, so a
// consumer switches on it and reads without an assertion. The
// zero Value names nothing.
//
// A copy of a Value shares what its pointers and slices refer to:
// Type, Inner and each composite field's Key are pointers, and
// Fields and Args are slices. Inner is a pointer because a struct
// cannot contain itself.
type Value struct {
	Kind    ValueKind       `json:"kind"`
	Literal LiteralKind     `json:"literal,omitzero"`
	Text    string          `json:"text,omitzero"`
	Bits    int             `json:"bits,omitzero"`   // LiteralInt and LiteralFloat: the number type's width, 0 for none
	Lang    symbol.Lang     `json:"lang,omitzero"`   // LiteralRaw: the language the text is written in
	Type    *TypeRef        `json:"type,omitzero"`   // Conversion and Composite: the type spelled
	Callee  symbol.Identity `json:"callee,omitzero"` // Call: the function spelled and imported
	Fields  []ValueField    `json:"fields,omitzero"` // Composite
	Args    []Value         `json:"args,omitzero"`   // Call
	Inner   *Value          `json:"inner,omitzero"`  // Conversion and Address
}

// ValueField is one entry of a composite, in one of three forms: a
// named field where Name is set, a keyed entry where Key is, and a
// positional element where neither is. The three cover a struct's
// fields, a map's entries and a list's elements, which is every
// composite the languages in scope spell.
type ValueField struct {
	Name  string `json:"name,omitzero"`
	Key   *Value `json:"key,omitzero"`
	Value Value  `json:"value"`
}

// IsZero reports whether the value names nothing.
func (v Value) IsZero() bool { return v.Kind == 0 }

// Literal returns a literal value of one kind. Raw text takes
// [Raw] instead, because only a backend of the text's own language
// can spell it.
func Literal(k LiteralKind, text string) Value {
	return Value{Kind: ValueLiteral, Literal: k, Text: text}
}

// Number returns a numeric literal written for a number type of a
// width in bits: 32 for a single-precision float, 64 for a long
// integer. A width of 0 states none, which is a platform-sized
// integer or a number whose type the derivation did not know. A
// target whose literal syntax depends on the width, such as Java's
// float and long suffixes, reads it, and every other target spells
// the text alone.
func Number(k LiteralKind, text string, bits int) Value {
	return Value{Kind: ValueLiteral, Literal: k, Text: text, Bits: bits}
}

// Raw returns a literal an author wrote as text in one language: a
// target spells it where the language is its own and refuses it
// otherwise, because nothing can translate it.
func Raw(lang symbol.Lang, text string) Value {
	return Value{Kind: ValueLiteral, Literal: LiteralRaw, Text: text, Lang: lang}
}

// NamedField returns a composite's named field.
func NamedField(name string, v Value) ValueField { return ValueField{Name: name, Value: v} }

// KeyedEntry returns a composite's keyed entry: a map's one pair.
func KeyedEntry(key, v Value) ValueField { return ValueField{Key: &key, Value: v} }

// Element returns a composite's positional element: a list's one
// item.
func Element(v Value) ValueField { return ValueField{Value: v} }

// Conversion returns a value converted to a type.
func Conversion(t *TypeRef, inner Value) Value {
	return Value{Kind: ValueConversion, Type: t, Inner: &inner}
}

// Composite returns a composite value of a type.
func Composite(t *TypeRef, fields ...ValueField) Value {
	return Value{Kind: ValueComposite, Type: t, Fields: fields}
}

// Call returns a call of a function, imported by identity.
func Call(callee symbol.Identity, args ...Value) Value {
	return Value{Kind: ValueCall, Callee: callee, Args: args}
}

// Address returns the address of a value.
func Address(inner Value) Value {
	return Value{Kind: ValueAddress, Inner: &inner}
}
