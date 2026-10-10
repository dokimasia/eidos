// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// restMark opens a rest member of a tuple type.
const restMark = "..."

// typeRef lowers one type. A reference spells its source's tokens
// without the whitespace between them, one space kept between two
// identifier tokens, so a reformatted signature spells one identity.
// It sets the structural forms the syntax states and leaves every other
// type Named with its spelling:
//
//   - A name, a qualified name and a predefined type are Named. A
//     generic instantiation is Named with its arguments in Args, and
//     its bare name in the spelling. A name an import binds records the
//     import's module specifier as its package.
//   - T[] is a List, and a tuple a Tuple of its members. A readonly
//     array or tuple is the List or Tuple it reads, spelled with its
//     readonly.
//   - A union is Optional of its one other member where every other
//     member is undefined or null, and a Union of its members otherwise.
//     An intersection is an Intersection of its members. A nested union
//     or intersection flattens into one of its own kind.
//   - A function type is a Func of its parameters' types and then its
//     return type, and a constructor type a Func of its parameters' types
//     and then the type it constructs, spelled with its new.
//   - An object type with one index signature and no other member is a
//     Map of the key and the value. A mapped type is Named, and every
//     other object type Inline, with its members as the reference's
//     fields and methods.
//   - Conditional, indexed access, keyof, typeof, literal and template
//     literal types are Named.
//
// Parentheses and the colon of a type annotation unwrap. It returns nil
// for the zero Node.
func (l *lowering) typeRef(n treesitter.Node) *node.TypeRef {
	if n.IsZero() {
		return nil
	}
	switch n.Kind() {
	case l.v.typeAnnotation, l.v.typePredicateAnnotation, l.v.assertsAnnotation, l.v.parenthesizedType:
		return l.typeRef(l.firstType(n))
	case l.v.genericType:
		name := n.Child(l.v.fieldName).Compact()
		return &node.TypeRef{
			Spelling: name, Pos: n.Pos(), Package: l.importOf(name),
			Args: l.typeArgs(n.Child(l.v.fieldTypeArguments)),
		}
	case l.v.typeIdentifier, l.v.nestedTypeIdentifier:
		spelling := n.Compact()
		return &node.TypeRef{Spelling: spelling, Pos: n.Pos(), Package: l.importOf(spelling)}
	case l.v.arrayType:
		return l.structural(n, symbol.FormList, l.firstType(n))
	case l.v.optionalType:
		return l.structural(n, symbol.FormOptional, l.firstType(n))
	case l.v.tupleType:
		return l.tuple(n)
	case l.v.unionType:
		return l.union(n)
	case l.v.intersectionType:
		var members []treesitter.Node
		l.flatten(n, &members)
		return l.structural(n, symbol.FormIntersection, members...)
	case l.v.functionType, l.v.constructorType:
		return l.funcRef(n)
	case l.v.readonlyType:
		return l.readonly(n)
	case l.v.objectType:
		return l.object(n)
	default:
		return &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos()}
	}
}

// structural lowers a composite whose children are the given types, in
// order.
func (l *lowering) structural(n treesitter.Node, form symbol.TypeForm, children ...treesitter.Node) *node.TypeRef {
	ref := &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos(), Form: form}
	for _, child := range children {
		ref.Elems = append(ref.Elems, l.elem(child))
	}
	return ref
}

// elem lowers a composite's child, and a reference that spells nothing
// where the source states no type, so a composite keeps its arity.
func (l *lowering) elem(n treesitter.Node) *node.TypeRef {
	if ref := l.typeRef(n); ref != nil {
		return ref
	}
	return &node.TypeRef{Pos: n.Pos()}
}

// tuple lowers a tuple type's members in order: a labeled member's
// type, an optional member as Optional, and a rest member Named,
// spelled with its ... and the type it spreads, so the spelling tells
// a rest member from an array member, labeled or not.
func (l *lowering) tuple(n treesitter.Node) *node.TypeRef {
	ref := &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos(), Form: symbol.FormTuple}
	for member := range n.NamedChildren() {
		switch member.Kind() {
		case l.v.comment:
		case l.v.requiredParameter:
			typ := member.Child(l.v.fieldType)
			if member.Child(l.v.fieldName).Kind() == l.v.restPattern {
				ref.Elems = append(ref.Elems, &node.TypeRef{
					Spelling: restMark + l.firstType(typ).Compact(), Pos: member.Pos(),
				})
				continue
			}
			ref.Elems = append(ref.Elems, l.elem(typ))
		case l.v.optionalParameter:
			ref.Elems = append(ref.Elems, l.structural(member, symbol.FormOptional, member.Child(l.v.fieldType)))
		default:
			ref.Elems = append(ref.Elems, l.elem(member))
		}
	}
	return ref
}

// union lowers a union type: Optional of its one member that is not
// undefined or null where every other member is, and a Union of every
// member otherwise. A union nested in a union flattens into it.
func (l *lowering) union(n treesitter.Node) *node.TypeRef {
	var members []treesitter.Node
	l.flatten(n, &members)
	var kept []treesitter.Node
	for _, m := range members {
		if !l.nullish(m) {
			kept = append(kept, m)
		}
	}
	if len(kept) == 1 && len(members) > 1 {
		return l.structural(n, symbol.FormOptional, kept[0])
	}
	return l.structural(n, symbol.FormUnion, members...)
}

// flatten appends a union's or an intersection's members to out, the
// members of a nested one of the same kind in its place.
func (l *lowering) flatten(n treesitter.Node, out *[]treesitter.Node) {
	for m := range n.NamedChildren() {
		switch m.Kind() {
		case l.v.comment:
		case n.Kind():
			l.flatten(m, out)
		default:
			*out = append(*out, m)
		}
	}
}

// nullish reports whether a union member is the undefined or the null
// literal type.
func (l *lowering) nullish(m treesitter.Node) bool {
	if m.Kind() != l.v.literalType {
		return false
	}
	inner := l.firstType(m)
	return inner.Kind() == l.v.undefined || inner.Kind() == l.v.null
}

// funcRef lowers a function type or a constructor type: its parameters'
// types and then its result as children, the split where the result
// begins. A function type states its result as its return type, and a
// constructor type as the type it constructs.
func (l *lowering) funcRef(n treesitter.Node) *node.TypeRef {
	ref := &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos(), Form: symbol.FormFunc}
	for p := range n.Child(l.v.fieldParameters).NamedChildren() {
		if p.Kind() == l.v.requiredParameter || p.Kind() == l.v.optionalParameter {
			ref.Elems = append(ref.Elems, l.elem(p.Child(l.v.fieldType)))
		}
	}
	ref.Split = len(ref.Elems)
	result := n.Child(l.v.fieldReturnType)
	if n.Kind() == l.v.constructorType {
		result = n.Child(l.v.fieldType)
	}
	if !result.IsZero() {
		ref.Elems = append(ref.Elems, l.elem(result))
	}
	return ref
}

// readonly lowers a readonly array or tuple as the List or Tuple it
// reads, spelled with its readonly. The grammar admits readonly before
// any type and TypeScript before an array or a tuple alone, so a named
// type after it keeps its own spelling, which the resolution step reads.
func (l *lowering) readonly(n treesitter.Node) *node.TypeRef {
	ref := l.elem(l.firstType(n))
	if ref.Form != symbol.FormNamed {
		ref.Spelling, ref.Pos = n.Compact(), n.Pos()
	}
	return ref
}

// object lowers an object type: a Map of the key and the value for a
// lone index signature, Named for a mapped type, and Inline otherwise,
// its members lowered as an interface's are and recorded on the
// reference. A member of an inline body has no identity, so a carrier
// on one refuses and no mark stamps it.
func (l *lowering) object(n treesitter.Node) *node.TypeRef {
	var members []treesitter.Node
	for m := range n.NamedChildren() {
		if m.Kind() == l.v.comment {
			continue
		}
		if m.Kind() == l.v.indexSignature && !l.firstOf(m, l.v.mappedTypeClause).IsZero() {
			return &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos()}
		}
		members = append(members, m)
	}
	if len(members) == 1 && members[0].Kind() == l.v.indexSignature {
		sig := members[0]
		return l.structural(n, symbol.FormMap, sig.Child(l.v.fieldIndexType), sig.Child(l.v.fieldType))
	}
	ref := &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos(), Form: symbol.FormInline}
	l.inline++
	l.objectMembers(n, &ref.Fields, &ref.Methods)
	l.inline--
	return ref
}

// typeArgs lowers a type argument list, in order.
func (l *lowering) typeArgs(args treesitter.Node) []*node.TypeRef {
	var out []*node.TypeRef
	for arg := range args.NamedChildren() {
		if arg.Kind() != l.v.comment {
			out = append(out, l.typeRef(arg))
		}
	}
	return out
}

// firstType returns a node's first named child that is not a comment:
// the type a wrapper wraps.
func (l *lowering) firstType(n treesitter.Node) treesitter.Node {
	for child := range n.NamedChildren() {
		if child.Kind() != l.v.comment {
			return child
		}
	}
	return treesitter.Node{}
}

// importOf returns the module specifier of the import that binds a
// spelling's first name, and nothing for a name no import binds.
func (l *lowering) importOf(spelling string) string {
	head, _, _ := strings.Cut(spelling, namespaceSeparator)
	return l.module.imports[head].specifier
}
