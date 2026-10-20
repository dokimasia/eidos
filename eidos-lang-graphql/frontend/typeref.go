// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"github.com/dokimasia/gqlparser/v2/ast"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The marks of a GraphQL type expression. A list writes its element in
// brackets, and a type that is not optional ends in the mark of a non-null
// type.
const (
	typeListOpen  = "["
	typeListClose = "]"
	nonNull       = "!"
)

// typeRef lowers the GraphQL type t into the node model. A non-null type
// loads as the type itself, and a nullable type as the optional form of it.
// A list loads as the list form, so [String!] is an optional list of text.
// place returns the position of each node of t, and leaf returns the named
// reference of the named type at the bottom of t.
//
// A named reference has the bare name as its spelling. A structural
// reference has the type expression as its spelling: a list with the
// brackets and the marks of its element and its own mark !, and an
// optional the expression of its child without the mark of the child.
func typeRef(
	t *ast.Type, place func(*ast.Position) position.Pos, leaf func(name string, at position.Pos) *node.TypeRef,
) *node.TypeRef {
	at := place(t.Position)
	var inner *node.TypeRef
	if t.Elem == nil {
		inner = leaf(t.NamedType, at)
	} else {
		elem := typeRef(t.Elem, place, leaf)
		inner = plugin.Composite(typeListOpen+expression(elem)+typeListClose+nonNull, at, symbol.FormList, elem)
	}
	if t.NonNull {
		return inner
	}
	return plugin.Composite(nullable(inner), at, symbol.FormOptional, inner)
}

// named returns the named reference of the type name at at, spelled with the
// bare name. The resolution step finds its target.
func named(name string, at position.Pos) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Pos: at}
}

// expression returns the type expression of the reference t as the type of a
// member: the bare name of a named reference with the mark !, and the
// spelling of a structural reference.
func expression(t *node.TypeRef) string {
	if t.Form == symbol.FormNamed {
		return t.Spelling + nonNull
	}
	return t.Spelling
}

// nullable returns the expression of the nullable form of t: the bare name
// of a named reference, and the spelling of a list without its mark !.
func nullable(t *node.TypeRef) string {
	if t.Form == symbol.FormNamed {
		return t.Spelling
	}
	return t.Spelling[:len(t.Spelling)-len(nonNull)]
}
