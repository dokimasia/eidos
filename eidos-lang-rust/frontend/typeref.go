// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// integerSuffix opens the type suffix of an integer literal, as in 16usize
// and 3u8: the first i or u, which no hexadecimal digit is.
const integerSuffix = "iu"

// typeRef lowers one type. A reference spells its source's tokens
// without the whitespace between them, one space kept between two
// identifier tokens, so a reformatted signature spells one identity.
// It sets the structural forms the syntax states and leaves every other
// type Named with its spelling:
//
//   - A name, a path and a primitive type are Named. A generic type is
//     Named with its bare name or path in the spelling and its
//     arguments in Args, where a lifetime, a const argument and an
//     associated type binding are each Named with their spelling. A
//     path records the module it names as its package, and so does a
//     name a use declaration binds.
//   - &T and &mut T are a Borrow of T.
//   - [T; N] is an Array of T, with N as its Length where N is an
//     integer literal, and [T] is a List of T.
//   - A tuple type is a Tuple of its members.
//   - A function pointer type is a Func of its parameters' types and
//     then its return type. A function trait such as Fn(A) -> B is a
//     bound, and is Named.
//   - dyn Trait and impl Trait are an Intersection of their bounds, in
//     order: the one trait, and each operand of + in dyn A + B. A
//     lifetime and a use bound are left out, as from a bound list, and
//     the reference's spelling keeps them and its dyn or impl.
//   - The unit type, a raw pointer and the never type are Named.
//
// It returns nil for the zero Node.
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
		return l.structural(n, symbol.FormBorrow, n.Child(l.v.fieldType))
	case l.v.arrayType:
		return l.array(n)
	case l.v.tupleType:
		return l.structural(n, symbol.FormTuple, l.children(n)...)
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

// traitBounds appends the bounds of a trait object, an impl Trait type
// or a bounded type to out, in order: the trait of dyn and impl, and
// each operand of +, a nested operand's bounds in its place. A lifetime
// and a use bound constrain no type and append nothing, as in a bound
// list.
func (l *lowering) traitBounds(n treesitter.Node, out *[]treesitter.Node) {
	switch n.Kind() {
	case l.v.dynamicType, l.v.abstractType:
		l.traitBounds(n.Child(l.v.fieldTrait), out)
	case l.v.boundedType:
		for _, operand := range l.children(n) {
			l.traitBounds(operand, out)
		}
	case l.v.lifetime, l.v.useBounds:
	default:
		*out = append(*out, n)
	}
}

// structural lowers a composite whose children are the given types, in
// order.
func (l *lowering) structural(n treesitter.Node, form symbol.TypeForm, children ...treesitter.Node) *node.TypeRef {
	ref := &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos(), Form: form}
	for _, child := range children {
		ref.Elems = append(ref.Elems, l.typeRef(child))
	}
	return ref
}

// array lowers an array type: an Array of its element, whose Length is
// the integer literal the type states, its type suffix cut, and 0 for
// any other length expression, which the spelling keeps, and a List of
// the element for a slice, which states no length.
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

// funcRef lowers a function pointer type: its parameters' types and then
// its return type as children, the split where the return begins. A
// parameter states its type alone or behind a name, and a variadic one
// states none, so it has no child.
func (l *lowering) funcRef(n treesitter.Node) *node.TypeRef {
	ref := &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos(), Form: symbol.FormFunc}
	for p := range n.Child(l.v.fieldParameters).NamedChildren() {
		switch k := p.Kind(); {
		case k == l.v.parameter:
			ref.Elems = append(ref.Elems, l.typeRef(p.Child(l.v.fieldType)))
		case k != l.v.attributeItem && k != l.v.variadicParameter && !l.comment(p):
			ref.Elems = append(ref.Elems, l.typeRef(p))
		}
	}
	ref.Split = len(ref.Elems)
	if ret := n.Child(l.v.fieldReturnType); !ret.IsZero() {
		ref.Elems = append(ref.Elems, l.typeRef(ret))
	}
	return ref
}

// typeArgs lowers a type argument list, every argument in order.
func (l *lowering) typeArgs(args treesitter.Node) []*node.TypeRef {
	children := l.children(args)
	if len(children) == 0 {
		return nil
	}
	out := make([]*node.TypeRef, 0, len(children))
	for _, arg := range children {
		out = append(out, l.typeRef(arg))
	}
	return out
}

// children returns a node's named children that are not comments: a
// tuple type's members, a type argument list's arguments and a use
// list's trees.
func (l *lowering) children(n treesitter.Node) []treesitter.Node {
	var out []treesitter.Node
	for child := range n.NamedChildren() {
		if !l.comment(child) {
			out = append(out, child)
		}
	}
	return out
}
