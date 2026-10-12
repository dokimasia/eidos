// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"slices"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Composite returns the reference of a structural form with the spelling
// and the position of its source. elems are its children in the order of
// form, such as the key and then the value of a map. A nil child becomes an
// empty reference at pos, so the form keeps its arity and no consumer reads
// a nil child. The reference keeps elems as its children, so the caller
// does not change the slice afterwards.
//
// # Allocation contract
//
// Composite allocates the reference and one empty reference for each nil
// child.
func Composite(spelling string, pos position.Pos, form symbol.TypeForm, elems ...*node.TypeRef) *node.TypeRef {
	fill(elems, pos)
	return &node.TypeRef{Spelling: spelling, Pos: pos, Form: form, Elems: elems}
}

// FuncType returns the reference of a function type with the spelling and
// the position of its source. Its children are params and then results,
// and its Split is the index of the first result. A nil child becomes an
// empty reference at pos, as in [Composite]. A function type without
// results keeps params as its children, so the caller does not change
// params afterwards.
//
// # Allocation contract
//
// FuncType allocates the reference, the joined children of a function
// type with results, and one empty reference for each nil child.
func FuncType(spelling string, pos position.Pos, params, results []*node.TypeRef) *node.TypeRef {
	elems := params
	if len(results) > 0 {
		elems = slices.Concat(params, results)
	}
	fill(elems, pos)
	return &node.TypeRef{Spelling: spelling, Pos: pos, Form: symbol.FormFunc, Elems: elems, Split: len(params)}
}

// fill replaces each nil child of elems with an empty reference at pos.
func fill(elems []*node.TypeRef, pos position.Pos) {
	for i, e := range elems {
		if e == nil {
			elems[i] = &node.TypeRef{Pos: pos}
		}
	}
}
