// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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
// carries only its type.
type Field struct {
	ID         symbol.Identity   `eidos:"node"`
	Origin     symbol.Identity   `eidos:"emit"`
	Pos        position.Pos      `eidos:"node"`
	Doc        []string          `eidos:"both"`
	Name       string            `eidos:"both"` // "" when positional
	Visibility symbol.Visibility `eidos:"both"`
	Level      symbol.Level      `eidos:"both"`
	Mutability symbol.Mutability `eidos:"both"`
	Type       *TypeRef          `eidos:"both,walk"`
	Host       symbol.Identity   `eidos:"node"`
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
// declaration still projects, and the language's own metadata
// carries the inferred spelling for anyone who needs it.
type Variable struct {
	ID         symbol.Identity   `eidos:"node"`
	Origin     symbol.Identity   `eidos:"emit"`
	Pos        position.Pos      `eidos:"node"`
	Doc        []string          `eidos:"both"`
	Name       string            `eidos:"both"`
	Visibility symbol.Visibility `eidos:"both"`
	Mutability symbol.Mutability `eidos:"both"`
	Type       *TypeRef          `eidos:"both,walk"` // nil when the source states none
}

// Constant is a binding fixed at compile time: a Go const, a Java
// static final, a TypeScript const, a Kotlin const val, a Rust
// const.
//
// Type is nil for an untyped constant, which Go has and most
// languages do not. Value holds the source spelling of the value
// expression verbatim, unevaluated, the same contract
// [EnumVariant.Value] carries.
type Constant struct {
	ID         symbol.Identity   `eidos:"node"`
	Origin     symbol.Identity   `eidos:"emit"`
	Pos        position.Pos      `eidos:"node"`
	Doc        []string          `eidos:"both"`
	Name       string            `eidos:"both"`
	Visibility symbol.Visibility `eidos:"both"`
	Type       *TypeRef          `eidos:"both,walk"` // nil when untyped
	Value      string            `eidos:"both"`      // source spelling, unevaluated
}
