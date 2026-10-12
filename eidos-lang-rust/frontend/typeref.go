// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// integerSuffix contains the letters that open the type suffix of an
// integer literal, as in 16usize and 3u8: the first i or u, which is not a
// hexadecimal digit.
const integerSuffix = "iu"

// typeRef lowers one type. The spelling of a reference is the sequence of
// its source tokens without the whitespace between them, with one space
// kept between two identifier tokens, so a reformatted signature has one
// identity. typeRef sets the structural forms that the syntax states and
// leaves every other type Named with its spelling:
//
//   - A name, a path and a primitive type are Named. A generic type is
//     Named with its bare name or path as its spelling and its arguments
//     in Args, where a lifetime, a const argument and an associated type
//     binding are each Named with their spelling. A path records its
//     module as its package, and so does a name that a use declaration
//     binds.
//   - &T is an immutable Borrow of T, and &mut T a mutable one.
//   - [T; N] is an Array of T, with N as its Length where N is an integer
//     literal, and [T] is a List of T.
//   - A tuple type is a Tuple of its members.
//   - A function pointer type is a Func of the types of its parameters and
//     then its return type. A function trait such as Fn(A) -> B is a
//     bound, and is Named.
//   - dyn Trait and impl Trait are an Intersection of their bounds, in
//     order: the one trait, and each operand of + in dyn A + B. A lifetime
//     and a use bound are left out, as from a bound list, and the spelling
//     of the reference keeps them with its dyn or impl.
//   - The unit type, a raw pointer and the never type are Named.
//
// A child that the source does not state is an empty reference at the
// position of its composite. typeRef returns nil for the zero Node.
func (l *lowering) typeRef(n treesitter.Node) *node.TypeRef {
	if n.IsZero() {
		return nil
	}
	switch n.Kind() {
	case l.v.typeIdentifier, l.v.scopedTypeIdentifier:
		spelling := n.Compact()
		return &node.TypeRef{Spelling: spelling, Pos: n.Pos(), Package: l.scope.packageOf(spelling)}
	case l.v.genericType:
		spelling := n.Child(l.v.fieldType).Compact()
		return &node.TypeRef{
			Spelling: spelling, Pos: n.Pos(), Package: l.scope.packageOf(spelling),
			Args: l.typeArgs(n.Child(l.v.fieldTypeArguments)),
		}
	case l.v.referenceType:
		borrow := l.structural(n, symbol.FormBorrow, n.Child(l.v.fieldType))
		borrow.Mutability = symbol.MutabilityImmutable
		if !n.FirstOf(l.v.mutableSpecifier).IsZero() {
			borrow.Mutability = symbol.MutabilityMutable
		}
		return borrow
	case l.v.arrayType:
		return l.array(n)
	case l.v.tupleType:
		var members []*node.TypeRef
		for member := range n.Content() {
			members = append(members, l.typeRef(member))
		}
		return plugin.Composite(n.Compact(), n.Pos(), symbol.FormTuple, members...)
	case l.v.functionType:
		if n.Child(l.v.fieldTrait).IsZero() {
			return l.funcRef(n)
		}
	case l.v.dynamicType, l.v.abstractType, l.v.boundedType:
		var bounds []treesitter.Node
		l.traitBounds(n, &bounds)
		return l.structural(n, symbol.FormIntersection, bounds...)
	}
	return &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos()}
}

// traitBounds appends the bounds of a trait object, an impl Trait type or
// a bounded type to out, in order: the trait of dyn and impl, and each
// operand of +, with the bounds of a nested operand in its place. A
// lifetime and a use bound constrain no type and append nothing, as in a
// bound list.
func (l *lowering) traitBounds(n treesitter.Node, out *[]treesitter.Node) {
	switch n.Kind() {
	case l.v.dynamicType, l.v.abstractType:
		l.traitBounds(n.Child(l.v.fieldTrait), out)
	case l.v.boundedType:
		for operand := range n.Content() {
			l.traitBounds(operand, out)
		}
	case l.v.lifetime, l.v.useBounds:
	default:
		*out = append(*out, n)
	}
}

// structural lowers a composite whose children are the given types, in
// order. A composite without children has nil children.
func (l *lowering) structural(n treesitter.Node, form symbol.TypeForm, children ...treesitter.Node) *node.TypeRef {
	if len(children) == 0 {
		return plugin.Composite(n.Compact(), n.Pos(), form)
	}
	elems := make([]*node.TypeRef, 0, len(children))
	for _, child := range children {
		elems = append(elems, l.typeRef(child))
	}
	return plugin.Composite(n.Compact(), n.Pos(), form, elems...)
}

// array lowers an array type: an Array of its element, whose Length is the
// integer literal of the type without its type suffix, and 0 for any other
// length expression, which the spelling keeps. A slice has no length, and
// is a List of its element.
func (l *lowering) array(n treesitter.Node) *node.TypeRef {
	length := n.Child(l.v.fieldLength)
	if length.IsZero() {
		return l.structural(n, symbol.FormList, n.Child(l.v.fieldElement))
	}
	ref := l.structural(n, symbol.FormArray, n.Child(l.v.fieldElement))
	digits := length.Text()
	if i := strings.IndexAny(digits, integerSuffix); i >= 0 {
		digits = digits[:i]
	}
	if v, err := strconv.ParseInt(digits, 0, strconv.IntSize); err == nil {
		ref.Length = int(v)
	}
	return ref
}

// funcRef lowers a function pointer type: the types of its parameters and
// then its return type as children, with the split where the return
// begins. A parameter states its type alone or behind a name. A variadic
// parameter states no type, so it has no child.
func (l *lowering) funcRef(n treesitter.Node) *node.TypeRef {
	var params []*node.TypeRef
	for p := range n.Child(l.v.fieldParameters).NamedChildren() {
		switch k := p.Kind(); {
		case k == l.v.parameter:
			params = append(params, l.typeRef(p.Child(l.v.fieldType)))
		case k != l.v.attributeItem && k != l.v.variadicParameter && !l.comment(p):
			params = append(params, l.typeRef(p))
		}
	}
	var results []*node.TypeRef
	if ret := n.Child(l.v.fieldReturnType); !ret.IsZero() {
		results = []*node.TypeRef{l.typeRef(ret)}
	}
	return plugin.FuncType(n.Compact(), n.Pos(), params, results)
}

// typeArgs lowers a type argument list, every argument in order.
func (l *lowering) typeArgs(args treesitter.Node) []*node.TypeRef {
	var out []*node.TypeRef
	for arg := range args.Content() {
		out = append(out, l.typeRef(arg))
	}
	return out
}
