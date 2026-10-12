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

// restMark opens a rest member of a tuple type.
const restMark = "..."

// typeRef lowers one type. The spelling of a reference is the tokens of
// its source without the whitespace between them, with one space between
// two identifier tokens, so a reformatted signature has one identity.
// typeRef sets the structural forms that the syntax states, and leaves
// every other type Named with its spelling:
//
//   - A name, a qualified name and a predefined type are Named. A generic
//     instantiation is Named with its arguments in Args, and its bare
//     name in the spelling. A name that an import binds records the
//     module specifier of the import as its package.
//   - T[] is a mutable List, and a tuple a mutable Tuple of its members.
//     A readonly array or tuple is an immutable List or Tuple, spelled
//     with its readonly.
//   - A union of one member and undefined or null is Optional of that
//     member, and Absent lists its undefined and null members in source
//     order. Every other union is a Union of its members. An intersection
//     is an Intersection of its members. A nested union or intersection
//     flattens into one of its own kind.
//   - A function type is a Func of the types of its parameters and then
//     its return type, and a constructor type a Func of the types of its
//     parameters and then the type that it constructs, spelled with its
//     new.
//   - An object type with one index signature and no other member is a
//     Map of the key and the value. The Map is immutable where the
//     signature is readonly, and mutable otherwise. A mapped type is
//     Named, and every other object type Inline, with its members as the
//     fields and the methods of the reference.
//   - Conditional, indexed access, keyof, typeof, literal and template
//     literal types are Named.
//
// typeRef unwraps parentheses and the colon of a type annotation. It
// returns nil for the zero Node.
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
		list := l.structural(n, symbol.FormList, l.firstType(n))
		list.Mutability = symbol.MutabilityMutable
		return list
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

// structural lowers a composite with the types as its children, in
// order. A child that the source does not state is an empty reference, as
// [plugin.Composite] builds it.
func (l *lowering) structural(n treesitter.Node, form symbol.TypeForm, children ...treesitter.Node) *node.TypeRef {
	elems := make([]*node.TypeRef, 0, len(children))
	for _, child := range children {
		elems = append(elems, l.typeRef(child))
	}
	return plugin.Composite(n.Compact(), n.Pos(), form, elems...)
}

// tuple lowers a tuple type to a mutable Tuple of its members in order:
// the type of a labeled member, an optional member as Optional, and a rest
// member Named, spelled with its ... and the type that it spreads. The
// spelling tells a rest member from an array member, labeled or not.
func (l *lowering) tuple(n treesitter.Node) *node.TypeRef {
	var elems []*node.TypeRef
	for member := range n.Content() {
		switch member.Kind() {
		case l.v.requiredParameter:
			typ := member.Child(l.v.fieldType)
			if member.Child(l.v.fieldName).Kind() == l.v.restPattern {
				elems = append(elems, &node.TypeRef{
					Spelling: restMark + l.firstType(typ).Compact(), Pos: member.Pos(),
				})
				continue
			}
			elems = append(elems, l.typeRef(typ))
		case l.v.optionalParameter:
			elems = append(elems, l.structural(member, symbol.FormOptional, member.Child(l.v.fieldType)))
		default:
			elems = append(elems, l.typeRef(member))
		}
	}
	tuple := plugin.Composite(n.Compact(), n.Pos(), symbol.FormTuple, elems...)
	tuple.Mutability = symbol.MutabilityMutable
	return tuple
}

// union lowers a union type. A union of one member and undefined or null
// lowers to Optional of that member, and Absent lists the undefined and
// null members in source order. Every other union lowers to a Union of its
// members. A union nested in a union flattens into it.
func (l *lowering) union(n treesitter.Node) *node.TypeRef {
	var members, kept, absent []treesitter.Node
	l.flatten(n, &members)
	for _, m := range members {
		if l.nullish(m) {
			absent = append(absent, m)
			continue
		}
		kept = append(kept, m)
	}
	if len(kept) != 1 || len(absent) == 0 {
		return l.structural(n, symbol.FormUnion, members...)
	}
	optional := l.structural(n, symbol.FormOptional, kept[0])
	optional.Absent = make([]*node.TypeRef, 0, len(absent))
	for _, m := range absent {
		optional.Absent = append(optional.Absent, l.typeRef(m))
	}
	return optional
}

// flatten appends the members of a union or an intersection to out, and
// the members of a nested one of the same kind in its place.
func (l *lowering) flatten(n treesitter.Node, out *[]treesitter.Node) {
	for m := range n.Content() {
		if m.Kind() == n.Kind() {
			l.flatten(m, out)
			continue
		}
		*out = append(*out, m)
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

// funcRef lowers a function type or a constructor type through
// [plugin.FuncType]: the types of its parameters and then its result. A
// function type states its result as its return type, and a constructor
// type as the type that it constructs.
func (l *lowering) funcRef(n treesitter.Node) *node.TypeRef {
	var params []*node.TypeRef
	for p := range n.Child(l.v.fieldParameters).NamedChildren() {
		if p.Kind() == l.v.requiredParameter || p.Kind() == l.v.optionalParameter {
			params = append(params, l.typeRef(p.Child(l.v.fieldType)))
		}
	}
	result := n.Child(l.v.fieldReturnType)
	if n.Kind() == l.v.constructorType {
		result = n.Child(l.v.fieldType)
	}
	var results []*node.TypeRef
	if !result.IsZero() {
		results = []*node.TypeRef{l.typeRef(result)}
	}
	return plugin.FuncType(n.Compact(), n.Pos(), params, results)
}

// readonly lowers a readonly array or tuple to an immutable List or Tuple,
// spelled with its readonly. The grammar admits readonly before any type,
// and TypeScript admits it before an array or a tuple alone. A named type
// after readonly keeps its own spelling, because the resolution step reads
// it. A readonly without a type is an empty reference at its position.
func (l *lowering) readonly(n treesitter.Node) *node.TypeRef {
	ref := l.typeRef(l.firstType(n))
	if ref == nil {
		return &node.TypeRef{Pos: n.Pos()}
	}
	if ref.Form != symbol.FormNamed {
		ref.Spelling, ref.Pos = n.Compact(), n.Pos()
	}
	if ref.Form == symbol.FormList || ref.Form == symbol.FormTuple {
		ref.Mutability = symbol.MutabilityImmutable
	}
	return ref
}

// object lowers an object type: a Map of the key and the value for a lone
// index signature, Named for a mapped type, and Inline otherwise, with its
// members lowered as the members of an interface and recorded on the
// reference. The Map is immutable where its index signature is readonly.
// A member of an inline body has no identity, so a carrier on one is
// refused and no mark stamps it.
func (l *lowering) object(n treesitter.Node) *node.TypeRef {
	var members []treesitter.Node
	for m := range n.Content() {
		if m.Kind() == l.v.indexSignature && !m.FirstOf(l.v.mappedTypeClause).IsZero() {
			return &node.TypeRef{Spelling: n.Compact(), Pos: n.Pos()}
		}
		members = append(members, m)
	}
	if len(members) == 1 && members[0].Kind() == l.v.indexSignature {
		sig := members[0]
		entries := l.structural(n, symbol.FormMap, sig.Child(l.v.fieldIndexType), sig.Child(l.v.fieldType))
		entries.Mutability = symbol.MutabilityMutable
		if sig.HasToken(keywordReadonly) {
			entries.Mutability = symbol.MutabilityImmutable
		}
		return entries
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
	for arg := range args.Content() {
		out = append(out, l.typeRef(arg))
	}
	return out
}

// firstType returns the first named child of a node that is not a
// comment, which is the type that a wrapper wraps, and the zero Node where
// there is none.
func (l *lowering) firstType(n treesitter.Node) treesitter.Node {
	for child := range n.NamedChildren() {
		if child.Kind() != l.v.comment {
			return child
		}
	}
	return treesitter.Node{}
}

// importOf returns the module specifier of the import that binds the first
// name of a spelling, and the empty string for a name that no import
// binds.
func (l *lowering) importOf(spelling string) string {
	head, _, _ := strings.Cut(spelling, namespaceSeparator)
	return l.module.imports[head].specifier
}
