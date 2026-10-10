// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/spoke"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix opens every refusal of the spoke: the language's
// identity, as every satellite's refusals open.
const refusalPrefix = string(typescript.Lang) + ": "

// The builtin and global types that the TypeScript spoke writes.
const (
	numberType        = "number"
	booleanType       = "boolean"
	stringType        = "string"
	unknownType       = "unknown"
	voidType          = "void"
	recordType        = "Record"
	partialType       = "Partial"
	mapType           = "Map"
	asyncIterableType = "AsyncIterable"
)

// The syntax that the TypeScript spoke writes. A parameter of an arrow
// type writes its type after paramTypeSep, and a group is an operand in
// parentheses.
const (
	listMark        = "[]"
	tupleOpen       = "["
	tupleClose      = "]"
	unionSep        = " | "
	intersectionSep = " & "
	paramsOpen      = "("
	paramsClose     = ")"
	paramName       = "p"
	paramTypeSep    = ": "
	arrow           = " => "
	groupOpen       = "("
	groupClose      = ")"
	argsOpener      = "<"
	argsCloser      = ">"
	listSep         = ", "
)

// Type spells the canonical shape s in TypeScript under the policy p. It
// is the spoke of the TypeScript target in the cross-language hub, and it
// spells each form this way:
//
//   - an integer of 8 to 32 bits and every float as number, and an
//     integer of 64 bits or of the platform's width as the choice of
//     [typescript.Int64];
//   - Bool and Text as boolean and string, and Bytes as the choice of
//     [typescript.Bytes];
//   - an optional as a union of its type and the choice of
//     [typescript.Absent];
//   - a list as T[], an array of a stated length as a tuple of that many
//     elements, a tuple as a tuple, a union as a union and an
//     intersection as an intersection;
//   - a map as Record<K, V> where the spoke spells the key as string or
//     as number, as Partial<Record<K, V>> for an enum key, and as
//     Map<K, V> otherwise;
//   - a function type as an arrow type whose parameters are p0, p1 and
//     so on, which returns void without a result and a tuple of two or
//     more results;
//   - an asynchronous stream as AsyncIterable<T>, a borrow as its type,
//     and a wildcard as its upper bound or as unknown without a bound;
//   - the well-known timestamp as the choice of [typescript.Timestamp];
//   - every other reference and sum as a translated reference with the
//     referent's declared name, the shape's target and the translated
//     type arguments, which TypeScript writes in angle brackets.
//
// Inside a list, an optional, a union and an intersection, the spoke
// writes a union, an intersection and a function type in parentheses. A
// global type that the spoke writes does not need an import.
//
// Type refuses a synchronous stream, the well-known duration, a wildcard
// with a lower bound, an inline type and an opaque type, because
// TypeScript has no spelling of them. It refuses an integer of another
// width and an array of an unstated length. It refuses a form whose
// child it refuses, and the error contains the source spelling of the
// child.
//
// # Allocation contract
//
// Type allocates each reference that it returns, the list of children
// of a structural reference, the list of arguments of a named one, and
// the spelling of each structural reference. A function type also
// allocates the spelling of each parameter. A refusal allocates its
// error.
func Type(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	t, err := tsType(s, p)
	if err != nil {
		return nil, fmt.Errorf(refusalPrefix+"%w", err)
	}
	return t, nil
}

// tsType spells one shape by the rules of [Type], and returns a refusal
// without the language's prefix.
func tsType(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	spell := func(c rules.TypeShape) (*emit.TypeRef, error) { return tsType(c, p) }
	switch s.Form {
	case symbol.FormScalar:
		return scalar(s, p)
	case symbol.FormBool:
		return &emit.TypeRef{Spelling: booleanType}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: stringType}, nil
	case symbol.FormBytes:
		return &emit.TypeRef{Spelling: string(p.Choice(typescript.Bytes))}, nil
	case symbol.FormOptional, symbol.FormList, symbol.FormBorrow:
		return container(s, p, spell)
	case symbol.FormArray:
		return tuple(s, spell)
	case symbol.FormTuple, symbol.FormUnion, symbol.FormIntersection:
		return members(s, spell)
	case symbol.FormMap:
		return mapOf(s, spell)
	case symbol.FormFunc:
		return function(s, spell)
	case symbol.FormStream:
		switch {
		case !s.Async:
			return nil, fmt.Errorf("%s has no TypeScript spelling", spoke.Describe(s))
		case len(s.Elems) != 1:
			return nil, fmt.Errorf("%s takes one child, and the shape has %d", spoke.Describe(s), len(s.Elems))
		}
		elems, err := spoke.Children(s.Elems, spell)
		if err != nil {
			return nil, err
		}
		return &emit.TypeRef{Spelling: asyncIterableType, Args: elems}, nil
	case symbol.FormWildcard:
		return wildcard(s, spell)
	case symbol.FormReference, symbol.FormSum:
		return reference(s, p, spell)
	default:
		return nil, fmt.Errorf("%s has no TypeScript spelling", spoke.Describe(s))
	}
}

// scalar spells a scalar: an integer of 8 to 32 bits and every float as
// number, and an integer of 64 bits or of the platform's width as the
// choice of [typescript.Int64].
func scalar(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	switch {
	case s.Class == rules.ScalarFloat:
		return &emit.TypeRef{Spelling: numberType}, nil
	case s.Bits == 8 || s.Bits == 16 || s.Bits == 32:
		return &emit.TypeRef{Spelling: numberType}, nil
	case s.Bits == 64 || s.Bits == 0:
		return &emit.TypeRef{Spelling: string(p.Choice(typescript.Int64))}, nil
	default:
		return nil, fmt.Errorf("no TypeScript %s type has %d bits", s.Class, s.Bits)
	}
}

// container spells a form of one child: an optional as a union of its
// type and the choice of [typescript.Absent], a list as T[], and a borrow
// as its type.
func container(
	s rules.TypeShape, p plugin.Policy, spell func(rules.TypeShape) (*emit.TypeRef, error),
) (*emit.TypeRef, error) {
	if len(s.Elems) != 1 {
		return nil, fmt.Errorf("%s takes one child, and the shape has %d", spoke.Describe(s), len(s.Elems))
	}
	child, err := spoke.Child(s.Elems[0], spell)
	if err != nil {
		return nil, err
	}
	switch s.Form {
	case symbol.FormBorrow:
		return child, nil
	case symbol.FormList:
		return &emit.TypeRef{
			Spelling: grouped(child) + listMark, Form: symbol.FormList, Elems: []*emit.TypeRef{child},
		}, nil
	}
	absent := &emit.TypeRef{Spelling: string(p.Choice(typescript.Absent))}
	return &emit.TypeRef{
		Spelling: grouped(child) + unionSep + absent.Spelling, Form: symbol.FormUnion,
		Elems: []*emit.TypeRef{child, absent},
	}, nil
}

// tuple spells an array of a stated length as a tuple of that many
// elements. It refuses an array of an unstated length.
func tuple(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	switch {
	case s.Length == 0:
		return nil, fmt.Errorf("an array of an unstated length has no TypeScript spelling")
	case len(s.Elems) != 1:
		return nil, fmt.Errorf("%s takes one child, and the shape has %d", spoke.Describe(s), len(s.Elems))
	}
	repeated := make([]rules.TypeShape, s.Length)
	for i := range repeated {
		repeated[i] = s.Elems[0]
	}
	elems, err := spoke.Children(repeated, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{
		Spelling: tupleOpen + joined(elems, listSep, false) + tupleClose, Form: symbol.FormTuple, Elems: elems,
	}, nil
}

// members spells a tuple, a union or an intersection from its members.
func members(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	var text string
	switch s.Form {
	case symbol.FormTuple:
		text = tupleOpen + joined(elems, listSep, false) + tupleClose
	case symbol.FormUnion:
		text = joined(elems, unionSep, true)
	default:
		text = joined(elems, intersectionSep, true)
	}
	return &emit.TypeRef{Spelling: text, Form: s.Form, Elems: elems}, nil
}

// mapOf spells a map as a record where the spoke spells the key as
// string or as number. It spells a map of an enum key as a partial
// record, and every other map as Map.
func mapOf(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if len(s.Elems) != 2 {
		return nil, fmt.Errorf("a map takes two children, a key and a value, and the shape has %d", len(s.Elems))
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	key := s.Elems[0]
	switch {
	case (key.Form == symbol.FormReference || key.Form == symbol.FormSum) && key.Ref.Kind == symbol.KindEnum:
		return &emit.TypeRef{Spelling: partialType, Args: []*emit.TypeRef{{Spelling: recordType, Args: elems}}}, nil
	case elems[0].Spelling == stringType || elems[0].Spelling == numberType:
		return &emit.TypeRef{Spelling: recordType, Args: elems}, nil
	default:
		return &emit.TypeRef{Spelling: mapType, Args: elems}, nil
	}
}

// function spells a function type as an arrow type with the parameters
// p0, p1 and so on. The arrow type returns void without a result, the
// result for one result, and a tuple of the results for two or more.
func function(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if s.Split < 0 || s.Split > len(s.Elems) {
		return nil, fmt.Errorf("a function type splits its %d children at %d", len(s.Elems), s.Split)
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	params := make([]string, s.Split)
	for i, e := range elems[:s.Split] {
		params[i] = paramName + strconv.Itoa(i) + paramTypeSep + spellref.Spell(e, argsOpener, argsCloser, unknownType)
	}
	var result string
	switch results := elems[s.Split:]; len(results) {
	case 0:
		result = voidType
	case 1:
		result = spellref.Spell(results[0], argsOpener, argsCloser, unknownType)
	default:
		result = tupleOpen + joined(results, listSep, false) + tupleClose
	}
	return &emit.TypeRef{
		Spelling: paramsOpen + strings.Join(params, listSep) + paramsClose + arrow + result,
		Form:     symbol.FormFunc, Split: s.Split, Elems: elems,
	}, nil
}

// wildcard spells a wildcard as unknown where it has no bound, and as its
// upper bound otherwise. It refuses a lower bound.
func wildcard(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	switch {
	case len(s.Elems) == 0:
		return &emit.TypeRef{Spelling: unknownType}, nil
	case s.Variance == symbol.VarianceIn:
		return nil, fmt.Errorf("a wildcard with a lower bound has no TypeScript spelling")
	}
	return spoke.Child(s.Elems[0], spell)
}

// reference spells a reference or a sum: the well-known timestamp as the
// choice of [typescript.Timestamp], and every other referent as a
// translated reference with its translated arguments. It refuses the
// well-known duration.
func reference(
	s rules.TypeShape, p plugin.Policy, spell func(rules.TypeShape) (*emit.TypeRef, error),
) (*emit.TypeRef, error) {
	switch s.Ref {
	case rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: string(p.Choice(typescript.Timestamp))}, nil
	case rules.WellKnownDuration:
		return nil, fmt.Errorf("the well-known duration has no TypeScript spelling")
	}
	args, err := spoke.Children(s.Args, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: s.Ref.Name, Target: s.Ref, Args: args}, nil
}

// joined returns the spellings of refs joined by sep, each in
// parentheses where group is set and the ref is a union, an intersection
// or a function type.
func joined(refs []*emit.TypeRef, sep string, group bool) string {
	parts := make([]string, len(refs))
	for i, r := range refs {
		if group {
			parts[i] = grouped(r)
		} else {
			parts[i] = spellref.Spell(r, argsOpener, argsCloser, unknownType)
		}
	}
	return strings.Join(parts, sep)
}

// grouped returns the spelling of an operand of a list, an optional, a
// union or an intersection. It writes a union, an intersection and a
// function type in parentheses, because the operator around the operand
// binds tighter than they do.
func grouped(t *emit.TypeRef) string {
	spelled := spellref.Spell(t, argsOpener, argsCloser, unknownType)
	switch t.Form {
	case symbol.FormUnion, symbol.FormIntersection, symbol.FormFunc:
		return groupOpen + spelled + groupClose
	default:
		return spelled
	}
}
