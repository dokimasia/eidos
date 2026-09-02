// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol

import "go.dokimi.dev/eidos/core/position"

// Symbol is the least any declaration returns.
//
// Every kind on both model sides satisfies it. Synthesized emit
// values carry the zero [position.Pos], and kinds that carry no
// documentation return nil from Docs.
type Symbol interface {
	Kind() Kind
	Position() position.Pos
	Docs() []string
}

// Membered is any kind that carries a member list: Struct,
// Interface, Enum, Sum, SumVariant and the callables' hosts
// satisfy it, each returning nil for a list its shape does not
// hold — a Sum's members are its variants, so its FieldList is
// nil.
//
// The slices hold the side's concrete kinds, adapted to []Symbol so
// neutral code needs no side import. Each call allocates the
// adapter slice; code on a hot path walks the concrete structs
// instead.
type Membered interface {
	Symbol
	FieldList() []Symbol
	MethodList() []Symbol
	EmbedList() []Symbol
}

// Typed is any kind whose meaning includes a type reference: Field,
// Param, Return, Variable, Constant and Alias satisfy it.
//
// The result's Kind is [KindTypeRef]. It is nil when the source
// declares no type, as for an inferred variable or an untyped
// constant.
type Typed interface {
	Symbol
	TypeRef() Symbol
}
