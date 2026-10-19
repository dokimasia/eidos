// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Field is a data member of a type: a Go struct field, a Java or
// TypeScript property, a Rust struct field, a Sum variant's payload
// entry.
//
// Level distinguishes an instance field from a static or companion
// one. Mutability carries the Kotlin val against var distinction,
// TypeScript readonly and Java final, which a generator emitting a
// data class has to reproduce. A field declared without a name, as
// a positional Sum variant payload is, leaves Name empty and
// carries only its type. Computed marks a name that is an expression
// of the language, such as a TypeScript property declared as [key],
// and the name is the source text of the expression.
//
// On the node side, Tag is the tag text in the spelling of the source
// language, with its enclosing delimiters stripped and unevaluated, the
// way [Constant.Value] is an expression. Only languages with per-field
// tags fill it. On the emit side, Tags is a slot of the entries of the
// tag, which the target joins into its tag syntax. A plugin appends an
// entry to the field of another plugin through the slot view of its
// emitter, so the header of the file names the plugin.
//
// Receives names the type a field attaches to when the field is
// declared outside that type's own declaration: a protobuf extension, a
// Kotlin extension property. Host remains the declaration that contains
// the field, or the file of a field at file level, so the identity of
// the field is in the scope that declares it. A field declared inside
// its type leaves Receives nil.
//
//eidos:subject
type Field struct {
	ID          symbol.Identity    `eidos:"node"`
	Origin      symbol.Identity    `eidos:"emit"`
	Pos         position.Pos       `eidos:"node"`
	Doc         []string           `eidos:"both"`
	Comment     string             `eidos:"both,fact=Comment"` // trailing line comment; "" when none
	Name        string             `eidos:"both,name"`         // "" when positional
	Visibility  symbol.Visibility  `eidos:"both,fact=Visibility"`
	Level       symbol.Level       `eidos:"both,fact=Level"`
	Mutability  symbol.Mutability  `eidos:"both,fact=Mutability"`
	Hard        bool               `eidos:"both,fact=HardPrivate"` // runtime-private: TypeScript's # names
	Optional    bool               `eidos:"both,fact=Optional"`    // present-or-absent: TypeScript's ?
	Computed    bool               `eidos:"both,fact=Computed"`    // the name is an expression: TypeScript's [expr]
	Type        *TypeRef           `eidos:"both,walk"`
	Receives    *TypeRef           `eidos:"both,walk,fact=Receives"` // set when declared outside the type it attaches to
	Value       string             `eidos:"both,fact=Value"`         // initializer's source spelling, unevaluated; "" when none
	Tag         string             `eidos:"node"`                    // tag text without delimiters, "" when none
	Tags        []*TagEntry        `eidos:"emit,slot=tags,fact=Tag"` // the entries of the tag, in order
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"`
	Host        symbol.Identity    `eidos:"node"`
}

// Variable is a binding declared outside any type: a Go
// package-level var, a TypeScript exported let, a Python
// module-level assignment, a Kotlin top-level val.
//
// Mutability separates a Kotlin val from a var. A binding fixed at
// compile time is a [Constant] instead, which is what a Kotlin
// const val and a Go const are.
//
// Type is nil when the source states none and the language infers
// it. That is a declared limit rather than a failure: the
// declaration still projects, and the metadata of the language
// contains the inferred spelling for a reader that needs it.
//
// Value is source text in the language of the declaration. A backend
// does not read the text, so a generator whose value names a package
// imports the package through the use builtin of its template.
//
//eidos:subject
type Variable struct {
	ID          symbol.Identity    `eidos:"node"`
	Origin      symbol.Identity    `eidos:"emit"`
	Pos         position.Pos       `eidos:"node"`
	Doc         []string           `eidos:"both"`
	Comment     string             `eidos:"both,fact=Comment"` // trailing line comment; "" when none
	Name        string             `eidos:"both,name"`
	Visibility  symbol.Visibility  `eidos:"both,fact=Visibility"`
	Mutability  symbol.Mutability  `eidos:"both,fact=Mutability"`
	Type        *TypeRef           `eidos:"both,walk"`       // nil when the source states none
	Value       string             `eidos:"both,fact=Value"` // initializer's source spelling, unevaluated; "" when none
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"`
}

// Constant is a binding fixed at compile time: a Go const, a Java
// static final, a TypeScript const, a Kotlin const val, a Rust
// const.
//
// Type is nil for an untyped constant, which Go has and most
// languages do not. Value is the source spelling of the value
// expression, verbatim and unevaluated, as [EnumVariant.Value] is.
//
// A backend does not read the text of Value, so a generator whose
// value names a package imports the package through the use builtin
// of its template.
//
//eidos:subject
type Constant struct {
	ID          symbol.Identity    `eidos:"node"`
	Origin      symbol.Identity    `eidos:"emit"`
	Pos         position.Pos       `eidos:"node"`
	Doc         []string           `eidos:"both"`
	Comment     string             `eidos:"both,fact=Comment"` // trailing line comment; "" when none
	Name        string             `eidos:"both,name"`
	Visibility  symbol.Visibility  `eidos:"both,fact=Visibility"`
	Type        *TypeRef           `eidos:"both,walk"` // nil when untyped
	Value       string             `eidos:"both"`      // source spelling, unevaluated
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"`
}
