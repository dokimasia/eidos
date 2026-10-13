// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tokens of the dimensions of an array type, one pair per dimension,
// and the tokens of a type argument list.
const (
	dimensionOpen = "["
	dimensionPair = "[]"
	argsOpen      = "<"
	argsClose     = ">"
	argsSeparator = ","
)

// typeRef lowers one type. The spelling of a reference is the sequence of
// its source tokens without the whitespace between them, with one space
// kept between two identifier tokens, so a reformatted signature has one
// identity. The spelling leaves out the type annotations, which are no
// part of a signature. typeRef sets the structural forms that the syntax
// states and leaves every other type Named with its spelling:
//
//   - A name, a qualified name and a primitive type are Named. A generic
//     type is Named with its arguments in Args, and its spelling is its
//     bare name. A name that a single-type import binds, and a qualified
//     name, record the package that they name as their package.
//   - T[] is a List of T, one per dimension.
//   - ? extends T is a Wildcard of T with Variance Out, ? super T one
//     with Variance In, and ? a Wildcard without a child.
func (l *lowering) typeRef(n treesitter.Node) *node.TypeRef {
	switch n.Kind() {
	case l.v.annotatedType:
		var typ treesitter.Node
		for part := range n.Content() {
			typ = part
		}
		return l.typeRef(typ)
	case l.v.typeIdentifier, l.v.scopedTypeIdentifier:
		spelling := l.spelling(n)
		return &node.TypeRef{Spelling: spelling, Pos: n.Pos(), Package: l.scope.packageOf(spelling)}
	case l.v.genericType:
		spelling := l.spelling(n.FirstOf(l.v.typeIdentifier, l.v.scopedTypeIdentifier))
		ref := &node.TypeRef{Spelling: spelling, Pos: n.Pos(), Package: l.scope.packageOf(spelling)}
		for arg := range n.FirstOf(l.v.typeArguments).Content() {
			ref.Args = append(ref.Args, l.typeRef(arg))
		}
		return ref
	case l.v.arrayType:
		return l.dimensioned(n.Child(l.v.fieldElement), n.Child(l.v.fieldDimensions))
	case l.v.wildcard:
		return l.wildcardOf(n)
	}
	return &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos()}
}

// spelling returns the spelling of a type name without its annotations: a
// simple name, or the segments of a qualified name joined by dots, with a
// generic segment spelled whole.
func (l *lowering) spelling(n treesitter.Node) string {
	if n.Kind() != l.v.scopedTypeIdentifier {
		return n.Compact()
	}
	var segments []string
	for part := range n.Content() {
		if k := part.Kind(); k != l.v.annotation && k != l.v.markerAnnotation {
			segments = append(segments, l.spelling(part))
		}
	}
	return strings.Join(segments, nameSeparator)
}

// dimensioned lowers a type behind the dimensions that a declaration
// states for it, each a List. The dimensions are those of an array type,
// of a declarator, as in int a[], or of a method, as in int f()[].
func (l *lowering) dimensioned(typ, dims treesitter.Node) *node.TypeRef {
	ref := l.typeRef(typ)
	for child := range dims.AllChildren() {
		if !child.Named() && string(child.Bytes()) == dimensionOpen {
			ref = listOf(ref)
		}
	}
	return ref
}

// wildcardOf lowers a wildcard. Its bound is its one child, with Variance
// In for super and Out for extends, and an unbounded wildcard has no
// child. The spelling is the compact form that the class-file lowering
// writes, from the spelling of the bound, so it leaves out the type
// annotations.
func (l *lowering) wildcardOf(n treesitter.Node) *node.TypeRef {
	variance := symbol.VarianceInvariant
	var bound []*node.TypeRef
	for child := range n.Content() {
		switch k := child.Kind(); {
		case k == l.v.superKeyword:
			variance = symbol.VarianceIn
		case k != l.v.annotation && k != l.v.markerAnnotation:
			bound = append(bound, l.typeRef(child))
			if variance == symbol.VarianceInvariant {
				variance = symbol.VarianceOut
			}
		}
	}
	spelling := wildcardMark
	if len(bound) > 0 {
		spelling = extendsBound
		if variance == symbol.VarianceIn {
			spelling = superBound
		}
		spelling += spelled(bound[0])
	}
	ref := plugin.Composite(spelling, n.Pos(), symbol.FormWildcard, bound...)
	ref.Variance = variance
	return ref
}

// listOf returns a List of a type. Its spelling is the spelling of the
// type with its arguments and one more dimension.
func listOf(elem *node.TypeRef) *node.TypeRef {
	return plugin.Composite(spelled(elem)+dimensionPair, elem.Pos, symbol.FormList, elem)
}

// spelled returns the spelling of a reference with its type arguments,
// which a generic reference keeps apart from its bare name.
func spelled(ref *node.TypeRef) string {
	if len(ref.Args) == 0 {
		return ref.Spelling
	}
	args := make([]string, 0, len(ref.Args))
	for _, a := range ref.Args {
		args = append(args, spelled(a))
	}
	return ref.Spelling + argsOpen + strings.Join(args, argsSeparator) + argsClose
}
