// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"errors"
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The marks of the type expressions of GraphQL. A list writes its element
// in brackets, and a type that is not optional ends in the mark of a
// non-null type.
const (
	listOpen    = "["
	listClose   = "]"
	nonNullMark = "!"
)

// Type spells the canonical shape s in GraphQL under the policy p. It is the
// spoke of the GraphQL target in the cross-language hub, and it spells each
// form this way:
//
//   - a signed integer of 8, 16 or 32 bits and an unsigned integer of 8 or
//     16 bits as Int, and an unsigned integer of 32 bits as the choice of
//     [graphql.Uint32Type];
//   - an integer of 64 bits or of the platform's width as the choice of
//     [graphql.Int64Type], or of [graphql.Uint64Type] for an unsigned one;
//   - a float of 32 or 64 bits as Float, and Bool and Text as Boolean and
//     String;
//   - Bytes, Dynamic, the well-known timestamp and the well-known duration
//     as the choices of [graphql.BytesType], [graphql.DynamicType],
//     [graphql.TimestampType] and [graphql.DurationType];
//   - a map as the choice of [graphql.MapType]. The choice
//     [graphql.Entries] spells the map form of its key and its value, and
//     the lowering of the backend writes it as a list of a synthesized type
//     of entries;
//   - an optional as the nullable form of its child, and a list as the
//     non-null list of its child;
//   - an asynchronous stream as the stream form of its child, as the whole
//     type of a member. The lowering of the backend admits it on a field of
//     the root of subscriptions alone;
//   - every other reference and sum as a translated reference with the flat
//     name of the referent and the target of the shape.
//
// A named reference has the bare name as its spelling, and a structural
// reference has its type expression, as [Expression] states.
//
// Type refuses a scalar of another width, an array, a tuple, a union, an
// intersection, a function type, a synchronous stream, a borrow, a wildcard,
// an inline type and an opaque type, because GraphQL has no form for them
// and its ecosystem has no common mapping. It refuses the well-known error,
// the well-known empty value and a reference with type arguments, because
// GraphQL does not declare a failure value, an empty value or a generic
// type. It refuses an optional of an optional, because a GraphQL type has
// one nullability, and an asynchronous stream inside another form. It
// refuses a form whose child it refuses, and the error contains the source
// spelling of the child.
//
// # Allocation contract
//
// Type allocates each reference that it returns, the list of children of a
// structural reference and the spelling of a list and of a stream. The
// flat name of a nested referent allocates once. A refusal allocates its
// error.
func Type(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	t, err := graphqlType(s, p, true)
	if err != nil {
		return nil, fmt.Errorf(refusalPrefix+"%w", err)
	}
	return t, nil
}

// Expression returns the GraphQL type expression of t as the type of a
// field, an argument or a variable. The expression of a named reference is
// its spelling with the mark !, and the expression of a structural
// reference is its spelling. t is not nil. Expression allocates the
// expression of a named reference, and does not allocate for a structural
// one.
func Expression(t *emit.TypeRef) string {
	if t.Form == symbol.FormNamed {
		return t.Spelling + nonNullMark
	}
	return t.Spelling
}

// graphqlType spells one shape by the rules of [Type], and returns a refusal
// without the prefix of the language. top reports whether s is the whole
// type of a member, the one position of an asynchronous stream.
func graphqlType(s rules.TypeShape, p plugin.Policy, top bool) (*emit.TypeRef, error) {
	spell := func(c rules.TypeShape) (*emit.TypeRef, error) { return graphqlType(c, p, false) }
	switch s.Form {
	case symbol.FormScalar:
		return scalar(s, p)
	case symbol.FormBool:
		return &emit.TypeRef{Spelling: graphql.BooleanScalar}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: graphql.StringScalar}, nil
	case symbol.FormBytes:
		return &emit.TypeRef{Spelling: string(p.Choice(graphql.BytesType))}, nil
	case symbol.FormDynamic:
		return &emit.TypeRef{Spelling: string(p.Choice(graphql.DynamicType))}, nil
	case symbol.FormOptional:
		return optional(s, spell)
	case symbol.FormList:
		return list(s, spell)
	case symbol.FormMap:
		return mapOf(s, p, spell)
	case symbol.FormStream:
		return stream(s, top, spell)
	case symbol.FormReference, symbol.FormSum:
		return reference(s, p)
	default:
		return nil, fmt.Errorf("%s has no GraphQL spelling", spoke.Describe(s))
	}
}

// scalar spells a number. An integer of 8 or 16 bits and a signed integer
// of 32 bits are Int. A wider integer is the choice of the policy of its
// sign and width, and a float of 32 or 64 bits is Float.
func scalar(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	var spelling string
	switch {
	case s.Class == rules.ScalarFloat && (s.Bits == 32 || s.Bits == 64):
		spelling = graphql.FloatScalar
	case s.Class == rules.ScalarFloat:
		return nil, fmt.Errorf("no GraphQL %s type has %d bits", s.Class, s.Bits)
	case s.Bits == 8 || s.Bits == 16 || (s.Bits == 32 && s.Class == rules.ScalarInt):
		spelling = graphql.IntScalar
	case s.Bits == 32:
		spelling = string(p.Choice(graphql.Uint32Type))
	case s.Bits == 64 || s.Bits == 0:
		key := graphql.Int64Type
		if s.Class == rules.ScalarUint {
			key = graphql.Uint64Type
		}
		spelling = string(p.Choice(key))
	default:
		return nil, fmt.Errorf("no GraphQL %s type has %d bits", s.Class, s.Bits)
	}
	return &emit.TypeRef{Spelling: spelling}, nil
}

// optional spells an optional as the nullable form of its child. The
// spelling is the type expression of the child without its mark !. It
// refuses an optional of an optional.
func optional(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if err := spoke.Arity(s, 1); err != nil {
		return nil, err
	}
	if s.Elems[0].Form == symbol.FormOptional {
		return nil, errors.New("an optional of an optional has no GraphQL spelling")
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{
		Spelling: strings.TrimSuffix(elems[0].Spelling, nonNullMark), Form: symbol.FormOptional, Elems: elems,
	}, nil
}

// list spells a list as the non-null list of its child, with the type
// expression of the child in brackets.
func list(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if err := spoke.Arity(s, 1); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	// One concatenation writes the expression of the child and the
	// brackets, so the spelling allocates once.
	var spelling string
	if elems[0].Form == symbol.FormNamed {
		spelling = listOpen + elems[0].Spelling + nonNullMark + listClose + nonNullMark
	} else {
		spelling = listOpen + elems[0].Spelling + listClose + nonNullMark
	}
	return &emit.TypeRef{Spelling: spelling, Form: symbol.FormList, Elems: elems}, nil
}

// mapOf spells a map as the choice of its policy: the map form of its key
// and its value under the choice of entries, and the scalar of the choice
// otherwise.
func mapOf(
	s rules.TypeShape, p plugin.Policy, spell func(rules.TypeShape) (*emit.TypeRef, error),
) (*emit.TypeRef, error) {
	if err := spoke.Arity(s, 2); err != nil {
		return nil, err
	}
	if choice := p.Choice(graphql.MapType); choice != graphql.Entries {
		return &emit.TypeRef{Spelling: string(choice)}, nil
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Form: symbol.FormMap, Elems: elems}, nil
}

// stream spells an asynchronous stream as the whole type of a member: the
// stream form of its child, with the type expression of the child as its
// spelling. It refuses a synchronous stream and a stream inside another
// form.
func stream(s rules.TypeShape, top bool, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if !s.Async {
		return nil, fmt.Errorf("%s has no GraphQL spelling", spoke.Describe(s))
	}
	if !top {
		return nil, errors.New("an asynchronous stream inside another form has no GraphQL spelling")
	}
	if err := spoke.Arity(s, 1); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: Expression(elems[0]), Form: symbol.FormStream, Async: true, Elems: elems}, nil
}

// reference spells a reference or a sum: the well-known timestamp and
// duration as the choices of their policies, and every other referent as a
// translated reference under its flat name. It refuses the well-known error
// and empty value, and a reference with type arguments.
func reference(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	switch {
	case s.Ref == rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: string(p.Choice(graphql.TimestampType))}, nil
	case s.Ref == rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: string(p.Choice(graphql.DurationType))}, nil
	case s.Ref == rules.WellKnownError:
		return nil, errors.New("the well-known error has no GraphQL spelling")
	case s.Ref == rules.WellKnownEmpty:
		return nil, errors.New("the well-known empty value has no GraphQL spelling")
	case len(s.Args) > 0:
		return nil, errors.New("a reference with type arguments has no GraphQL spelling")
	}
	return &emit.TypeRef{Spelling: s.Ref.FlatName(), Target: s.Ref}, nil
}
