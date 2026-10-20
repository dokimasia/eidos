// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// lowerSum lowers the sum s into the shapes of GraphQL, which has no sum
// type of its own:
//
//   - A sum becomes a union with the name, the documentation, the comment
//     and the directives of the sum. Each variant becomes an object type
//     named after the sum and the variant, with the documentation, the
//     comment, the directives and the fields of the variant, and the union
//     joins them. Each object type implements the contracts of the sum
//     through its own copy of them, because the settle rewrites a reference
//     in place. The specification requires at least one field of an object
//     type, so a variant without a field is refused.
//   - Under the marker graphql:input, a sum becomes an input object with the
//     directive oneOf, as [lowerOneOf] writes it. An input object implements
//     no interface, so a sum with contracts is refused there.
//   - A sum with type parameters becomes a union with the parameters, which
//     the coverage rejects at the render.
//
// A union has no members, so a sum with methods is refused, and so is a sum
// without a variant. The object types of the variants take the fields of
// the variants, because the sum leaves the store.
//
// The union allocates its alias, its target, the spelling of its target and
// the lists of its declarations, its members and its object types, once
// each. Each variant allocates the name of its object type, and the copy of
// the contracts of a sum with contracts.
func lowerSum(s *emit.Sum) ([]symbol.Symbol, error) {
	set, err := checkAnnotations(symbol.KindSum, s.Name, s.Annotations)
	if err != nil {
		return nil, err
	}
	variants := s.Variants.Items()
	switch {
	case s.Methods.Len() > 0:
		return nil, fmt.Errorf(refusalPrefix+"a union has no members, and the sum %s states methods", s.Name)
	case len(variants) == 0:
		return nil, fmt.Errorf(refusalPrefix+"a union joins at least one type, "+
			"and the sum %s states no variant", s.Name)
	}
	if set&inputBit != 0 && len(s.TypeParams) == 0 {
		if len(s.Implements) > 0 {
			return nil, fmt.Errorf(refusalPrefix+"an input object implements no interface, "+
				"and the sum %s implements %s", s.Name, s.Implements[0].Spelling)
		}
		return lowerOneOf(s)
	}
	u := &emit.Alias{
		Origin: s.Origin, Doc: s.Doc, Comment: s.Comment, Name: s.Name, Visibility: s.Visibility,
		Defined: true, TypeParams: s.TypeParams, Annotations: s.Annotations,
	}
	if len(s.TypeParams) > 0 {
		return []symbol.Symbol{u}, nil
	}
	sum := caser.Pascal(s.Name)
	out := make([]symbol.Symbol, 1, 1+len(variants))
	out[0] = u
	objects := make([]emit.Struct, len(variants))
	members := make([]emit.TypeRef, len(variants))
	elems := make([]*emit.TypeRef, len(variants))
	size := (len(variants) - 1) * len(memberSep)
	var synthesized []symbol.Symbol
	for k, v := range variants {
		if _, err := checkAnnotations(symbol.KindSumVariant, v.Name, v.Annotations); err != nil {
			return nil, err
		}
		fields := v.Fields.Items()
		if len(fields) == 0 {
			return nil, fmt.Errorf(refusalPrefix+"an object type declares at least one field, "+
				"and the variant %s of %s declares none", v.Name, s.Name)
		}
		name := sum + caser.Pascal(v.Name)
		h := &host{name: name, origin: s.Origin}
		for _, f := range fields {
			if err := h.field(f); err != nil {
				return nil, err
			}
		}
		objects[k] = emit.Struct{
			Origin: s.Origin, Doc: v.Doc, Comment: v.Comment, Name: name, Visibility: s.Visibility,
			Implements: lowering.CopyTypeRefs(s.Implements), Annotations: v.Annotations, Fields: emit.SlotOf(fields...),
		}
		members[k] = emit.TypeRef{Spelling: name}
		elems[k] = &members[k]
		size += len(name)
		out = append(out, &objects[k])
		synthesized = append(synthesized, h.out...)
	}
	var spelling strings.Builder
	spelling.Grow(size)
	for k, m := range members {
		if k > 0 {
			spelling.WriteString(memberSep)
		}
		spelling.WriteString(m.Spelling)
	}
	u.Target = &emit.TypeRef{Spelling: spelling.String(), Form: symbol.FormUnion, Elems: elems}
	return append(out, synthesized...), nil
}

// lowerOneOf lowers the sum s under the marker graphql:input into an input
// object with the directive oneOf and one nullable field for each variant,
// named after the variant:
//
//   - A variant with one field takes the type of that field.
//   - A variant with two or more fields takes a synthesized input object
//     named after the sum, the variant and Input, with those fields.
//   - A variant without a field takes Boolean, whose presence selects the
//     variant.
//
// The field of a variant has the documentation, the comment and the
// directives of the variant, and of its one field where the variant states
// none. The input object allocates its annotations, the list of its
// declarations, the list of its fields and the fields once each.
func lowerOneOf(s *emit.Sum) ([]symbol.Symbol, error) {
	variants := s.Variants.Items()
	members := make([]emit.Field, len(variants))
	fields := make([]*emit.Field, len(variants))
	h := &host{name: s.Name, origin: s.Origin, input: true}
	var synthesized []symbol.Symbol
	for k, v := range variants {
		if _, err := checkAnnotations(symbol.KindSumVariant, v.Name, v.Annotations); err != nil {
			return nil, err
		}
		member := &members[k]
		*member = emit.Field{Doc: v.Doc, Comment: v.Comment, Name: v.Name, Annotations: v.Annotations}
		inner := v.Fields.Items()
		switch len(inner) {
		case 0:
			member.Type = &emit.TypeRef{Spelling: graphql.BooleanScalar}
		case 1:
			f := inner[0]
			member.Type = f.Type
			if len(member.Doc) == 0 {
				member.Doc = f.Doc
			}
			if member.Comment == "" {
				member.Comment = f.Comment
			}
			member.Annotations = append(slices.Clip(member.Annotations), f.Annotations...)
		default:
			name := caser.Pascal(s.Name) + caser.Pascal(v.Name) + inputSuffix
			object := &host{name: name, origin: s.Origin, input: true}
			for _, f := range inner {
				if err := object.field(f); err != nil {
					return nil, err
				}
			}
			member.Type = &emit.TypeRef{Spelling: name}
			synthesized = append(synthesized, &emit.Struct{
				Origin: s.Origin, Name: name, Visibility: s.Visibility,
				Annotations: symbol.Annotations{{Name: string(graphql.InputMarker)}},
				Fields:      emit.SlotOf(inner...),
			})
			synthesized = append(synthesized, object.out...)
		}
		if member.Type != nil && member.Type.Form != symbol.FormOptional {
			member.Type = &emit.TypeRef{
				Spelling: member.Type.Spelling, Form: symbol.FormOptional, Elems: []*emit.TypeRef{member.Type},
			}
		}
		if err := h.field(member); err != nil {
			return nil, err
		}
		fields[k] = member
	}
	in := &emit.Struct{
		Origin: s.Origin, Doc: s.Doc, Comment: s.Comment, Name: s.Name, Visibility: s.Visibility,
		Annotations: append(slices.Clip(s.Annotations), symbol.Annotation{Name: graphql.OneOfDirective}),
		Fields:      emit.SlotOf(fields...),
	}
	out := make([]symbol.Symbol, 0, 1+len(h.out)+len(synthesized))
	out = append(out, in)
	out = append(out, h.out...)
	return append(out, synthesized...), nil
}
