// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"errors"
	"fmt"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix opens every refusal of the package: the language's
// identity, as every satellite's refusals open.
const refusalPrefix = string(protobuf.Lang) + ": "

// protobufName is the name of the language in a refusal.
const protobufName = "protobuf"

// The syntax of the composite forms. A field writes its label before its
// type, an rpc writes the stream mark before its message, and a map field
// writes its key and its value in angle brackets.
const (
	optionalLabel = "optional "
	repeatedLabel = "repeated "
	streamMark    = "stream "
	mapOpen       = "map<"
	mapSep        = ", "
	mapClose      = ">"
)

// Type spells the canonical shape s in protobuf under the policy p. It is
// the spoke of the protobuf target in the cross-language hub, and it
// spells each form this way:
//
//   - a signed integer of 8 to 32 bits as the choice of
//     [protobuf.Int32Encoding], and one of 64 bits or of the platform's
//     width as the choice of [protobuf.Int64Encoding];
//   - an unsigned integer as the choice of [protobuf.Uint32Encoding] or of
//     [protobuf.Uint64Encoding] by the same widths;
//   - a float of 32 bits as float and one of 64 bits as double;
//   - Bool, Text and Bytes as bool, string and bytes;
//   - an optional, a list and an asynchronous stream as the optional, the
//     list and the stream form of their child, which protobuf writes with
//     the label optional, the label repeated and the mark stream;
//   - a map as the map form of its key and its value;
//   - the top type as google.protobuf.Value, and the well-known timestamp,
//     duration and empty value as google.protobuf.Timestamp, Duration and
//     Empty, each with the file that declares it as its package;
//   - every other reference and sum as a translated reference with the
//     referent's flat name and the shape's target.
//
// A form of a form keeps its structure, such as a list of a list. The
// backend's lowering writes the wrapper message that protobuf needs for
// such a field.
//
// Type refuses a scalar of another width, an array, a tuple, a union, an
// intersection, a function type, a synchronous stream, a wildcard, an
// inline type and an opaque type, because protobuf has no spelling of
// them. It refuses a borrow, because a message holds a copy of its data.
// It refuses the well-known error, a type parameter and a reference with
// type arguments, because protobuf declares neither a failure value nor a
// generic type. It refuses an optional of an optional, because a field has
// one presence, and a read-only list or map, because every protobuf
// runtime writes through a repeated field and a map. It refuses a form
// whose child it refuses, and the error contains the source spelling of
// the child.
//
// # Allocation contract
//
// Type allocates each reference that it returns, the list of children of
// a structural reference and the spelling of each structural reference. A
// refusal allocates its error.
func Type(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	t, err := protoType(s, p)
	if err != nil {
		return nil, fmt.Errorf(refusalPrefix+"%w", err)
	}
	return t, nil
}

// protoType spells one shape by the rules of [Type], and returns a refusal
// without the language's prefix.
func protoType(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	spell := func(c rules.TypeShape) (*emit.TypeRef, error) { return protoType(c, p) }
	switch s.Form {
	case symbol.FormScalar:
		return scalar(s, p)
	case symbol.FormBool:
		return &emit.TypeRef{Spelling: protobuf.TypeBool}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: protobuf.TypeString}, nil
	case symbol.FormBytes:
		return &emit.TypeRef{Spelling: protobuf.TypeBytes}, nil
	case symbol.FormDynamic:
		return &emit.TypeRef{Spelling: protobuf.TypeValue, Package: protobuf.StructFile}, nil
	case symbol.FormOptional, symbol.FormList:
		return container(s, spell)
	case symbol.FormMap:
		return mapOf(s, spell)
	case symbol.FormStream:
		if !s.Async {
			return nil, fmt.Errorf("%s has no protobuf spelling", spoke.Describe(s))
		}
		return container(s, spell)
	case symbol.FormReference, symbol.FormSum:
		return reference(s)
	default:
		return nil, fmt.Errorf("%s has no protobuf spelling", spoke.Describe(s))
	}
}

// scalar spells a scalar: an integer as the choice of the encoding policy
// of its sign and width, and a float of 32 or 64 bits as float or double.
func scalar(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	var key plugin.PolicyKey
	switch {
	case s.Class == rules.ScalarFloat && s.Bits == 32:
		return &emit.TypeRef{Spelling: protobuf.TypeFloat}, nil
	case s.Class == rules.ScalarFloat && s.Bits == 64:
		return &emit.TypeRef{Spelling: protobuf.TypeDouble}, nil
	case s.Class == rules.ScalarFloat:
		return nil, fmt.Errorf("no protobuf %s type has %d bits", s.Class, s.Bits)
	case s.Bits == 8 || s.Bits == 16 || s.Bits == 32:
		key = protobuf.Int32Encoding
		if s.Class == rules.ScalarUint {
			key = protobuf.Uint32Encoding
		}
	case s.Bits == 64 || s.Bits == 0:
		key = protobuf.Int64Encoding
		if s.Class == rules.ScalarUint {
			key = protobuf.Uint64Encoding
		}
	default:
		return nil, fmt.Errorf("no protobuf %s type has %d bits", s.Class, s.Bits)
	}
	return &emit.TypeRef{Spelling: string(p.Choice(key))}, nil
}

// container spells a form of one child: an optional under the label
// optional, a list under the label repeated, and an asynchronous stream
// under the mark stream. It refuses an optional of an optional and a
// read-only list.
func container(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if err := spoke.Arity(s, 1); err != nil {
		return nil, err
	}
	if err := spoke.ReadOnly(s, protobufName); err != nil {
		return nil, err
	}
	if s.Form == symbol.FormOptional && s.Elems[0].Form == symbol.FormOptional {
		return nil, errors.New("an optional of an optional has no protobuf spelling")
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	prefix := optionalLabel
	switch s.Form {
	case symbol.FormList:
		prefix = repeatedLabel
	case symbol.FormStream:
		prefix = streamMark
	default:
		// An optional takes the label optional.
	}
	return &emit.TypeRef{
		Spelling: prefix + elems[0].Spelling, Form: s.Form, Async: s.Async, Elems: elems,
	}, nil
}

// mapOf spells a map as the map form of its key and its value. It refuses
// a read-only map.
func mapOf(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if err := spoke.Arity(s, 2); err != nil {
		return nil, err
	}
	if err := spoke.ReadOnly(s, protobufName); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{
		Spelling: mapOpen + elems[0].Spelling + mapSep + elems[1].Spelling + mapClose,
		Form:     symbol.FormMap, Elems: elems,
	}, nil
}

// reference spells a reference or a sum: a well-known timestamp, duration
// and empty value as the well-known type of protobuf with the file that
// declares it, and every other referent as a translated reference under
// its flat name. It refuses the well-known error and a reference with
// type arguments.
func reference(s rules.TypeShape) (*emit.TypeRef, error) {
	switch {
	case s.Ref == rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: protobuf.TypeTimestamp, Package: protobuf.TimestampFile}, nil
	case s.Ref == rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: protobuf.TypeDuration, Package: protobuf.DurationFile}, nil
	case s.Ref == rules.WellKnownEmpty:
		return &emit.TypeRef{Spelling: protobuf.TypeEmpty, Package: protobuf.EmptyFile}, nil
	case s.Ref == rules.WellKnownError:
		return nil, errors.New("the well-known error has no protobuf spelling")
	case s.Ref == rules.WellKnownContext:
		return nil, errors.New("the well-known context has no protobuf spelling, because a schema states no context")
	case len(s.Args) > 0:
		return nil, errors.New("a reference with type arguments has no protobuf spelling")
	}
	return &emit.TypeRef{Spelling: s.Ref.FlatName(), Target: s.Ref}, nil
}
