// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Enum is a closed set of named values carrying no payload: a Java
// enum class, a TypeScript enum, a proto enum, a Go constant group.
//
// The split from [Sum] goes by payload rather than by syntax. A
// variant set where no variant carries fields is an Enum; one where
// any variant does is a Sum.
//
// Fields and Methods exist because a Java enum is a class: it
// declares instance state and behaviour alongside its variants.
// Most languages leave both empty.
//
//eidos:subject
type Enum struct {
	ID          symbol.Identity   `eidos:"node"`
	Origin      symbol.Identity   `eidos:"emit"`
	Pos         position.Pos      `eidos:"node"`
	Doc         []string          `eidos:"both"`
	Name        string            `eidos:"both,name"`
	Visibility  symbol.Visibility `eidos:"both,fact=Visibility"`
	Const       bool              `eidos:"both,fact=ConstEnum"` // inlined at use: TypeScript's const enum
	Variants    []*EnumVariant    `eidos:"both,walk,slot=variants"`
	Fields      []*Field          `eidos:"both,walk,slot=fields,fact=Fields"`   // Java enums carry instance state
	Methods     []*Method         `eidos:"both,walk,slot=methods,fact=Methods"` // and behaviour
	Annotations Annotations       `eidos:"emit,fact=Annotations"`
}

// EnumVariant is one member of an [Enum].
//
// Value holds the source spelling of the value expression verbatim,
// including an unevaluated form such as a Go iota arithmetic
// expression or a Java variant's constructor arguments. Evaluating
// it is the language's job, not the model's, and a variant whose
// language assigns values implicitly leaves it empty.
type EnumVariant struct {
	ID          symbol.Identity `eidos:"node"`
	Origin      symbol.Identity `eidos:"emit"`
	Pos         position.Pos    `eidos:"node"`
	Doc         []string        `eidos:"both"`
	Name        string          `eidos:"both,name"`
	Value       string          `eidos:"both,fact=Value"` // source spelling, unevaluated
	Annotations Annotations     `eidos:"emit,fact=Annotations"`
	Host        symbol.Identity `eidos:"node"`
}

// Sum is a closed set of named variants carrying payloads: a Rust
// data enum, a Kotlin or Java sealed class hierarchy, a proto
// oneof, a Swift associated-value enum.
//
// Sum is a declaration kind. The untagged union that TypeScript and
// Python spell with a bar is a type shape instead, and the two stay
// separate because conflating them is how a target language ends up
// guessing which one it is spelling.
//
//eidos:subject
type Sum struct {
	ID          symbol.Identity   `eidos:"node"`
	Origin      symbol.Identity   `eidos:"emit"`
	Pos         position.Pos      `eidos:"node"`
	Doc         []string          `eidos:"both"`
	Name        string            `eidos:"both,name"`
	Visibility  symbol.Visibility `eidos:"both,fact=Visibility"`
	TypeParams  []*TypeParam      `eidos:"both,walk,fact=TypeParams"` // Rust data enums are generic
	Variants    []*SumVariant     `eidos:"both,walk,slot=variants"`
	Methods     []*Method         `eidos:"both,walk,slot=methods,fact=Methods"`
	Annotations Annotations       `eidos:"emit,fact=Annotations"`
}

// SumVariant is one variant of a [Sum]: a name and a field list.
//
// A variant whose payload is positional, as a Rust tuple variant
// is, fills Fields with unnamed entries in declaration order.
type SumVariant struct {
	ID          symbol.Identity `eidos:"node"`
	Origin      symbol.Identity `eidos:"emit"`
	Pos         position.Pos    `eidos:"node"`
	Doc         []string        `eidos:"both"`
	Name        string          `eidos:"both,name"`
	Fields      []*Field        `eidos:"both,walk,slot=fields"` // the payload; unnamed when positional
	Annotations Annotations     `eidos:"emit,fact=Annotations"`
	Host        symbol.Identity `eidos:"node"`
}
