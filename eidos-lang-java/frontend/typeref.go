// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tokens an array type's dimensions spell, one pair per dimension,
// and the ones a type argument list spells.
const (
	dimensionOpen = "["
	dimensionPair = "[]"
	argsOpen      = "<"
	argsClose     = ">"
	argsSeparator = ","
)

// typeRef lowers one type. A reference spells its source's tokens
// without the whitespace between them, one space kept between two
// identifier tokens, so a reformatted signature spells one identity,
// and without its type annotations, which are no part of a signature.
// It sets the structural forms the syntax states and leaves every other
// type Named with its spelling:
//
//   - A name, a qualified name and a primitive type are Named. A
//     generic type is Named with its arguments in Args, and its bare
//     name in the spelling. A name a single-type import binds, and a
//     qualified name, record the package they name as their package.
//   - T[] is a List of T, one per dimension.
//   - ? extends T is a Wildcard of T with Variance Out, ? super T one
//     with Variance In, and ? a Wildcard without a child.
func (l *lowering) typeRef(n treesitter.Node) *node.TypeRef {
	switch n.Kind() {
	case l.v.annotatedType:
		parts := l.children(n)
		return l.typeRef(parts[len(parts)-1])
	case l.v.typeIdentifier, l.v.scopedTypeIdentifier:
		spelling := l.spelling(n)
		return &node.TypeRef{Spelling: spelling, Pos: n.Pos(), Package: l.scope.packageOf(spelling)}
	case l.v.genericType:
		name := l.children(n)[0]
		spelling := l.spelling(name)
		ref := &node.TypeRef{Spelling: spelling, Pos: n.Pos(), Package: l.scope.packageOf(spelling)}
		for _, arg := range l.children(l.firstOf(n, l.v.typeArguments)) {
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

// spelling returns a type name's spelling without its annotations: a
// simple name, or a qualified name's segments joined by dots, a generic
// segment spelled whole.
func (l *lowering) spelling(n treesitter.Node) string {
	if n.Kind() != l.v.scopedTypeIdentifier {
		return n.Compact()
	}
	var segments []string
	for _, part := range l.children(n) {
		if k := part.Kind(); k != l.v.annotation && k != l.v.markerAnnotation {
			segments = append(segments, l.spelling(part))
		}
	}
	return strings.Join(segments, nameSeparator)
}

// dimensioned lowers a type behind the dimensions a declaration states
// for it, each a List: an array type's own, and a declarator's or a
// method's, as int a[] and int f()[] state them.
func (l *lowering) dimensioned(typ, dims treesitter.Node) *node.TypeRef {
	ref := l.typeRef(typ)
	for child := range dims.AllChildren() {
		if !child.Named() && child.Text() == dimensionOpen {
			ref = listOf(ref)
		}
	}
	return ref
}

// wildcardOf lowers a wildcard: its bound as its one child, with
// Variance In for super and Out for extends, and no child for an
// unbounded one.
func (l *lowering) wildcardOf(n treesitter.Node) *node.TypeRef {
	ref := &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos(), Form: symbol.FormWildcard}
	for _, child := range l.children(n) {
		switch k := child.Kind(); {
		case k == l.v.superKeyword:
			ref.Variance = symbol.VarianceIn
		case k != l.v.annotation && k != l.v.markerAnnotation:
			ref.Elems = append(ref.Elems, l.typeRef(child))
			if ref.Variance == symbol.VarianceInvariant {
				ref.Variance = symbol.VarianceOut
			}
		}
	}
	return ref
}

// listOf returns a List of a type, spelled as the type with its
// arguments and one more dimension.
func listOf(elem *node.TypeRef) *node.TypeRef {
	return &node.TypeRef{
		Spelling: spelled(elem) + dimensionPair, Pos: elem.Pos, Form: symbol.FormList,
		Elems: []*node.TypeRef{elem},
	}
}

// spelled returns a reference's spelling with its type arguments, which
// a generic reference keeps apart from its bare name.
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
