// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"strconv"
	"strings"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// typeRefusal opens every refusal of the spoke: the language's identity,
// as every satellite's refusals open.
const typeRefusal = string(rust.Lang) + ": "

// The types and the syntax that the Rust spoke writes. An array writes
// its length after arrayLength, and a tuple of one member closes with
// singleClose.
const (
	boolType       = "bool"
	stringType     = "String"
	byteType       = "u8"
	unitType       = "()"
	vecType        = "Vec"
	optionType     = "Option"
	hashMapType    = "HashMap"
	boxType        = "Box"
	systemTimeType = "SystemTime"
	durationType   = "Duration"
	arrayOpen      = "["
	arrayLength    = "; "
	arrayClose     = "]"
	fnOpen         = "dyn Fn("
	fnClose        = ")"
	resultArrow    = " -> "
	tupleOpen      = "("
	tupleClose     = ")"
	singleClose    = ",)"
	argsOpener     = "<"
	argsCloser     = ">"
	listSep        = ", "
)

// The modules of the library types that the Rust spoke writes, in the
// slash-separated form that the backend writes with Rust's separator.
// Vec, Option and Box are in the prelude, so they do not need a use
// declaration.
const (
	collectionsModule = "std/collections"
	timeModule        = "std/time"
)

// Rust's integer types by width in bits, 0 for the platform's width, and
// its floating-point types.
var (
	rustInts   = map[int]string{0: "isize", 8: "i8", 16: "i16", 32: "i32", 64: "i64", 128: "i128"}
	rustUints  = map[int]string{0: "usize", 8: "u8", 16: "u16", 32: "u32", 64: "u64", 128: "u128"}
	rustFloats = map[int]string{32: "f32", 64: "f64"}
)

// Type spells the canonical shape s in Rust. It is the spoke of the Rust
// target in the cross-language hub. Rust does not declare a lowering
// policy, so Type does not read p. It spells each form this way:
//
//   - a scalar as the primitive of its class and width, and isize and
//     usize for the platform's width;
//   - Bool, Text and Bytes as bool, String and Vec<u8>;
//   - an optional as Option<T>, a list as Vec<T>, an array as [T; n], a
//     map as std::collections::HashMap<K, V>, and a tuple as a tuple;
//   - a function type as Box<dyn Fn(A) -> R>, without the arrow where it
//     has no result, and with a tuple of two or more results;
//   - a wildcard as its upper bound;
//   - the well-known timestamp and duration as std::time::SystemTime and
//     std::time::Duration;
//   - every other reference and sum as a translated reference with the
//     referent's declared name, the shape's target and the translated
//     type arguments, which Rust writes in angle brackets.
//
// Type refuses a union, an intersection, a stream, an inline type and an
// opaque type, because Rust has no spelling of them. It refuses a borrow,
// because a Rust borrow needs a lifetime, and the shape of a type of
// another language has none. It refuses a wildcard with a lower bound or
// without a bound, a scalar of another width, and an array of an unstated
// length. It refuses a form whose child it refuses, and the error
// contains the source spelling of the child.
//
// # Allocation contract
//
// Type allocates each reference that it returns, the list of children
// of a structural reference, the list of arguments of a named one, and
// the spelling of each structural reference. A refusal allocates its
// error.
func Type(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	t, err := rustType(s)
	if err != nil {
		return nil, fmt.Errorf(typeRefusal+"%w", err)
	}
	return t, nil
}

// rustType spells one shape by the rules of [Type], and returns a
// refusal without the language's prefix.
func rustType(s rules.TypeShape) (*emit.TypeRef, error) {
	switch s.Form {
	case symbol.FormScalar:
		return scalar(s)
	case symbol.FormBool:
		return &emit.TypeRef{Spelling: boolType}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: stringType}, nil
	case symbol.FormBytes:
		return &emit.TypeRef{Spelling: vecType, Args: []*emit.TypeRef{{Spelling: byteType}}}, nil
	case symbol.FormOptional, symbol.FormList, symbol.FormArray:
		return container(s)
	case symbol.FormMap:
		if len(s.Elems) != 2 {
			return nil, fmt.Errorf("a map takes two children, a key and a value, and the shape has %d", len(s.Elems))
		}
		args, err := spoke.Children(s.Elems, rustType)
		if err != nil {
			return nil, err
		}
		return &emit.TypeRef{Spelling: hashMapType, Package: collectionsModule, Args: args}, nil
	case symbol.FormTuple:
		elems, err := spoke.Children(s.Elems, rustType)
		if err != nil {
			return nil, err
		}
		return &emit.TypeRef{Spelling: tupleOf(elems), Form: symbol.FormTuple, Elems: elems}, nil
	case symbol.FormFunc:
		return function(s)
	case symbol.FormBorrow:
		return nil, fmt.Errorf("a borrow of another language has no lifetime, and a Rust borrow needs one")
	case symbol.FormWildcard:
		if len(s.Elems) == 0 || s.Variance == symbol.VarianceIn {
			return nil, fmt.Errorf("a wildcard without an upper bound has no Rust spelling")
		}
		return spoke.Child(s.Elems[0], rustType)
	case symbol.FormReference, symbol.FormSum:
		return reference(s)
	default:
		return nil, fmt.Errorf("%s has no Rust spelling", spoke.Describe(s))
	}
}

// scalar spells a scalar as the primitive of its class and width.
func scalar(s rules.TypeShape) (*emit.TypeRef, error) {
	widths := rustFloats
	switch s.Class {
	case rules.ScalarInt:
		widths = rustInts
	case rules.ScalarUint:
		widths = rustUints
	}
	name, held := widths[s.Bits]
	switch {
	case held:
		return &emit.TypeRef{Spelling: name}, nil
	case s.Bits == 0:
		return nil, fmt.Errorf("no Rust %s type has the platform's width", s.Class)
	default:
		return nil, fmt.Errorf("no Rust %s type has %d bits", s.Class, s.Bits)
	}
}

// container spells a form of one child: an optional as Option<T>, a list
// as Vec<T>, and an array as [T; n]. It refuses an array of an unstated
// length.
func container(s rules.TypeShape) (*emit.TypeRef, error) {
	switch {
	case s.Form == symbol.FormArray && s.Length == 0:
		return nil, fmt.Errorf("an array of an unstated length has no Rust spelling")
	case len(s.Elems) != 1:
		return nil, fmt.Errorf("%s takes one child, and the shape has %d", spoke.Describe(s), len(s.Elems))
	}
	elems, err := spoke.Children(s.Elems, rustType)
	if err != nil {
		return nil, err
	}
	switch s.Form {
	case symbol.FormOptional:
		return &emit.TypeRef{Spelling: optionType, Args: elems}, nil
	case symbol.FormList:
		return &emit.TypeRef{Spelling: vecType, Args: elems}, nil
	default:
		inner := spellref.Spell(elems[0], argsOpener, argsCloser, unitType)
		return &emit.TypeRef{
			Spelling: arrayOpen + inner + arrayLength + strconv.Itoa(s.Length) + arrayClose, Form: symbol.FormArray,
			Length: s.Length, Elems: elems,
		}, nil
	}
}

// function spells a function type as a boxed trait object of Fn with
// its parameters. One result follows the arrow, two or more results
// follow it as a tuple, and a function type without a result has no
// arrow.
func function(s rules.TypeShape) (*emit.TypeRef, error) {
	if s.Split < 0 || s.Split > len(s.Elems) {
		return nil, fmt.Errorf("a function type splits its %d children at %d", len(s.Elems), s.Split)
	}
	elems, err := spoke.Children(s.Elems, rustType)
	if err != nil {
		return nil, err
	}
	params := make([]string, s.Split)
	for i, e := range elems[:s.Split] {
		params[i] = spellref.Spell(e, argsOpener, argsCloser, unitType)
	}
	var text string
	switch results := elems[s.Split:]; len(results) {
	case 0:
		text = fnOpen + strings.Join(params, listSep) + fnClose
	case 1:
		result := spellref.Spell(results[0], argsOpener, argsCloser, unitType)
		text = fnOpen + strings.Join(params, listSep) + fnClose + resultArrow + result
	default:
		text = fnOpen + strings.Join(params, listSep) + fnClose + resultArrow + tupleOf(results)
	}
	fn := &emit.TypeRef{Spelling: text, Form: symbol.FormFunc, Split: s.Split, Elems: elems}
	return &emit.TypeRef{Spelling: boxType, Args: []*emit.TypeRef{fn}}, nil
}

// reference spells a reference or a sum: a well-known type as the type of
// std::time, and every other referent as a translated reference with its
// translated arguments.
func reference(s rules.TypeShape) (*emit.TypeRef, error) {
	switch s.Ref {
	case rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: systemTimeType, Package: timeModule}, nil
	case rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: durationType, Package: timeModule}, nil
	}
	args, err := spoke.Children(s.Args, rustType)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: s.Ref.Name, Target: s.Ref, Args: args}, nil
}

// tupleOf returns the spelling of a tuple of refs: (A, B), (A,) for one
// member, and () for none.
func tupleOf(refs []*emit.TypeRef) string {
	parts := make([]string, len(refs))
	for i, r := range refs {
		parts[i] = spellref.Spell(r, argsOpener, argsCloser, unitType)
	}
	if len(parts) == 1 {
		return tupleOpen + parts[0] + singleClose
	}
	return tupleOpen + strings.Join(parts, listSep) + tupleClose
}
