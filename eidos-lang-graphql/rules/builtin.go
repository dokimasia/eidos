// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The widths of the built-in numbers of GraphQL. The specification bounds
// an Int to a signed integer of 32 bits and a Float to a float of double
// precision.
const (
	intBits   = 32
	floatBits = 64
)

// Builtin returns the shape of a named reference that the resolution step
// left without a target. The built-in scalars of GraphQL project as these
// shapes:
//
//   - Int is a signed integer of 32 bits.
//   - Float is a float of 64 bits.
//   - String and ID are text, because an ID serializes as a string.
//   - Boolean is a bool.
//
// Every other spelling returns [symbol.FormOpaque] with the spelling: a type
// that the view does not declare, or the schema coordinate of a field of a
// document that the validator does not type. A nil reference returns Opaque
// too. The view is unread, and Builtin allocates nothing.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil {
		return rules.Opaque(nil)
	}
	switch ref.Spelling {
	case graphql.IntScalar:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, intBits)
	case graphql.FloatScalar:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, floatBits)
	case graphql.StringScalar, graphql.IDScalar:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case graphql.BooleanScalar:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	default:
		return rules.Opaque(ref)
	}
}

// ShapeOf returns the shape of the values of a custom scalar whose
// specifiedBy URL the table of [graphql.Scalar] contains: the shape of its
// entry, such as a signed integer of 64 bits for Long. It reports false for
// a custom scalar outside the table, for a union and for every other
// declaration, so a reference to it folds to a reference. A reference to a
// scalar has no type arguments, so the shape does not depend on the
// reference. The reference and the view are unread, and ShapeOf allocates
// nothing.
func (Rules) ShapeOf(_ *node.TypeRef, decl symbol.Symbol, _ rules.View) (rules.TypeShape, bool) {
	scalar, is := decl.(*node.Alias)
	if !is || scalar.Target != nil {
		return rules.TypeShape{}, false
	}
	return graphql.ScalarShape(scalar.Annotations)
}
