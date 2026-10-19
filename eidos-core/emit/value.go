// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"strconv"

	"go.dokimi.dev/eidos/core/symbol"
)

// ValueKind selects the fields of a [Value] that are set. The zero kind
// is no value.
type ValueKind uint8

const (
	// ValueLiteral is a literal with Literal and Text: a number, the
	// content of a string, a truth value, the absent value, or raw text
	// in the source language.
	ValueLiteral ValueKind = iota + 1
	// ValueConversion converts Inner to Type: Weekday(42).
	ValueConversion
	// ValueComposite is a composite of Type with Fields: Point{X: 42}. A
	// field without a name or a key is positional.
	ValueComposite
	// ValueCall calls Callee with Args: time.Unix(42, 0).
	ValueCall
	// ValueAddress takes the address of Inner: &Point{X: 42}.
	ValueAddress
	// ValueName is the name of the declaration Callee, such as a
	// constant or a variable: time.Second. A target spells the name with
	// the import of its package, as it spells the callee of a call.
	ValueName
	// ValueConstruction constructs Type from Args: new Set([42]). A target
	// whose language constructs a value through its type, such as the new
	// expression of TypeScript and Java, spells it with the import of the
	// type. A target without such a form refuses it.
	ValueConstruction
)

// String returns the spelling of the kind, and the decimal number of a
// kind that the package does not declare. A finding contains the spelling
// of a value kind, so a consumer can match on it. String allocates
// nothing for a declared kind.
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
	case ValueName:
		return "name"
	case ValueConstruction:
		return "construction"
	default:
		return strconv.Itoa(int(k))
	}
}

// LiteralKind is the kind of the text of a literal. A target spells each
// kind in its own way, such as the quotes of a string or the name of the
// absent value. The zero kind is no literal.
type LiteralKind uint8

const (
	// LiteralInt is an integer in decimal text.
	LiteralInt LiteralKind = iota + 1
	// LiteralFloat is a floating-point number in decimal text.
	LiteralFloat
	// LiteralString is a string. Text is the content without quotes.
	LiteralString
	// LiteralBool is a truth value. Text is "true" or "false".
	LiteralBool
	// LiteralNil is the absent value of the language. Text is empty.
	LiteralNil
	// LiteralRaw is Text in the source language. Only the backend of
	// that language spells it, and the backend of another language
	// refuses it.
	LiteralRaw
)

// String returns the spelling of the kind, and the decimal number of a
// kind that the package does not declare. String allocates nothing for a
// declared kind.
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

// Value is one value that a generated body writes, such as a sample, a
// zero or a literal read from text. The backend qualifies every reference
// in it for the file that it writes, which text alone cannot do. Kind
// selects the fields that are set, so a consumer switches on Kind and
// reads the fields without an assertion. The zero Value is no value.
//
// Type, Inner and the Key of each composite field are pointers, and
// Fields and Args are slices. A copy of a Value shares what they point
// to. Inner is a pointer because a struct cannot contain itself.
type Value struct {
	Kind    ValueKind       `json:"kind"`
	Literal LiteralKind     `json:"literal,omitzero"`
	Text    string          `json:"text,omitzero"`
	Bits    int             `json:"bits,omitzero"`   // LiteralInt and LiteralFloat: the width in bits of the number type, 0 for none
	Lang    symbol.Lang     `json:"lang,omitzero"`   // LiteralRaw: the language of the text
	Type    *TypeRef        `json:"type,omitzero"`   // Conversion, Composite and Construction: the type of the result
	Callee  symbol.Identity `json:"callee,omitzero"` // Call and Name: the declaration that the target spells and imports
	Fields  []ValueField    `json:"fields,omitzero"` // Composite
	Args    []Value         `json:"args,omitzero"`   // Call and Construction
	Inner   *Value          `json:"inner,omitzero"`  // Conversion and Address
}

// ValueField is one entry of a composite in one of three forms. Name is
// set for a named field, Key for a keyed entry, and neither for a
// positional element. The three forms cover the fields of a struct, the
// entries of a map and the elements of a list. Every composite of the
// languages in scope has one of them.
type ValueField struct {
	Name  string `json:"name,omitzero"`
	Key   *Value `json:"key,omitzero"`
	Value Value  `json:"value"`
}

// IsZero reports whether the value is the zero Value. It allocates
// nothing.
func (v Value) IsZero() bool { return v.Kind == 0 }

// Literal returns a literal of one kind. Use [Raw] for raw text, because
// only the backend of its language can spell it. Literal allocates
// nothing.
func Literal(k LiteralKind, text string) Value {
	return Value{Kind: ValueLiteral, Literal: k, Text: text}
}

// Number returns a numeric literal for a number type of a width in bits,
// such as 32 for a single-precision float or 64 for a long integer. A
// width of 0 is no width, which is a platform-sized integer or a number
// of an unknown type. A target whose literal syntax depends on the width,
// such as the float and long suffixes of Java, reads it. Every other
// target spells the text alone. Number allocates nothing.
func Number(k LiteralKind, text string, bits int) Value {
	return Value{Kind: ValueLiteral, Literal: k, Text: text, Bits: bits}
}

// Raw returns a literal that an author wrote as text in one language. A
// target spells it where the language is its own, and refuses it
// otherwise, because no target translates it. Raw allocates nothing.
func Raw(lang symbol.Lang, text string) Value {
	return Value{Kind: ValueLiteral, Literal: LiteralRaw, Text: text, Lang: lang}
}

// NamedField returns a named field of a composite. It allocates nothing.
func NamedField(name string, v Value) ValueField { return ValueField{Name: name, Value: v} }

// KeyedEntry returns a keyed entry of a composite, which is one pair of a
// map. It allocates the key, one allocation.
func KeyedEntry(key, v Value) ValueField { return ValueField{Key: &key, Value: v} }

// Element returns a positional element of a composite, which is one item
// of a list. It allocates nothing.
func Element(v Value) ValueField { return ValueField{Value: v} }

// Conversion returns a value converted to a type. It allocates the inner
// value, one allocation.
func Conversion(t *TypeRef, inner Value) Value {
	return Value{Kind: ValueConversion, Type: t, Inner: &inner}
}

// Composite returns a composite value of a type. It allocates the list of
// fields of a variadic call, one allocation.
func Composite(t *TypeRef, fields ...ValueField) Value {
	return Value{Kind: ValueComposite, Type: t, Fields: fields}
}

// Call returns a call of a function, which a backend imports by its
// identity. It allocates the list of arguments of a variadic call, one
// allocation.
func Call(callee symbol.Identity, args ...Value) Value {
	return Value{Kind: ValueCall, Callee: callee, Args: args}
}

// Address returns the address of a value. It allocates the inner value,
// one allocation.
func Address(inner Value) Value {
	return Value{Kind: ValueAddress, Inner: &inner}
}

// Name returns the name of the declaration id as a value, such as a
// constant or a variable. A backend imports the package of the
// declaration, as it does for the callee of a call. Name allocates
// nothing.
func Name(id symbol.Identity) Value {
	return Value{Kind: ValueName, Callee: id}
}

// Construction returns a construction of the type t from the arguments
// args, such as new Map([["a", "b"]]) in TypeScript. A construction
// without arguments is the empty value of a collection type, such as new
// Set(). A backend imports the type, as it does for the type of a
// composite. Construction allocates the list of arguments of a variadic
// call, one allocation, and nothing without arguments.
func Construction(t *TypeRef, args ...Value) Value {
	return Value{Kind: ValueConstruction, Type: t, Args: args}
}
