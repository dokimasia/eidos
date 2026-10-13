// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"errors"
	"fmt"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The predeclared types that the Go spoke writes, and the spelling of a
// byte slice.
const (
	boolType   = "bool"
	stringType = "string"
	byteType   = "byte"
	anyType    = "any"
	bytesType  = "[]" + byteType
)

// The brackets of a Go argument list, which a spelled child writes its
// arguments in.
const (
	argsOpener = "["
	argsCloser = "]"
)

// goName is the name of the language in a refusal.
const goName = "Go"

// spelledStack is the number of spellings of children that [restated]
// keeps on the stack. A function type of more children allocates their
// list.
const spelledStack = 4

// The package of Go's time types, and the spellings of the two types
// that the well-known timestamp and duration take.
const (
	timePackage  = "time"
	timeType     = "time.Time"
	durationType = "time.Duration"
)

// The spelling of the well-known empty value, and the iterator over pairs
// of an element and an error that an asynchronous stream takes, with its
// package.
const (
	emptyStruct = "struct{}"
	iterPackage = "iter"
	iterType    = "iter.Seq2"
	errorType   = "error"
)

// Type spells the canonical shape s in Go. It is the spoke of the Go
// target in the cross-language hub. Go does not declare a lowering
// policy, so Type does not read p. It spells each form this way:
//
//   - a scalar as the predeclared type of its class and width, and int
//     and uint for the platform's width;
//   - Bool, Text and Bytes as bool, string and []byte, and the top type as
//     any;
//   - an optional as a pointer, a list as a slice, an array as an array
//     of its length, a map as a map, a function as a function type, a
//     synchronous stream as a channel, and an asynchronous stream as
//     iter.Seq2[T, error], with the import of iter;
//   - an optional of a list, bytes, a map, a function type, a stream, the
//     top type, the error value or a sum as the type itself, because the
//     nil value of its Go spelling is the absent value;
//   - a wildcard as its upper bound, and as any where it has no bound;
//   - the well-known timestamp and duration as time.Time and
//     time.Duration, with the import of time, the well-known empty value
//     as struct{}, and the well-known error as error;
//   - every other reference and sum as a translated reference with the
//     referent's flat name, the shape's target and the translated type
//     arguments, which Go writes in brackets.
//
// A consumer ranges over the iterator of an asynchronous stream, and the
// stream ends with the first pair whose error is not nil.
//
// Type refuses a tuple, a union, an intersection, a borrow, a wildcard
// with a lower bound, an inline type and an opaque type, because Go has
// no spelling of them. A Go pointer aliases and a Go value copies, so
// neither is a borrow. Type refuses a scalar of another width and an
// array of an unstated length. It refuses an immutable list, array and
// map, because a holder can write through a Go slice, array and map. It
// refuses a map key of a list, a map, a function type and bytes, because
// Go does not compare their values. It refuses a form whose child it
// refuses, and the error contains the source spelling of the child.
//
// # Allocation contract
//
// Type allocates each reference that it returns, the list of children
// of a structural reference, the list of arguments of a translated one,
// and the spelling of each structural reference. A refusal allocates its
// error.
func Type(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	t, err := goType(s)
	if err != nil {
		return nil, fmt.Errorf(refusalPrefix+"%w", err)
	}
	return t, nil
}

// goType spells one shape by the rules of [Type], and returns a refusal
// without the language's prefix.
func goType(s rules.TypeShape) (*emit.TypeRef, error) {
	switch s.Form {
	case symbol.FormScalar:
		return scalar(s)
	case symbol.FormBool:
		return &emit.TypeRef{Spelling: boolType}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: stringType}, nil
	case symbol.FormDynamic:
		return &emit.TypeRef{Spelling: anyType}, nil
	case symbol.FormBytes:
		return &emit.TypeRef{
			Spelling: bytesType, Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: byteType}},
		}, nil
	case symbol.FormOptional, symbol.FormList, symbol.FormArray, symbol.FormStream:
		return container(s)
	case symbol.FormMap:
		return mapOf(s)
	case symbol.FormFunc:
		return function(s)
	case symbol.FormWildcard:
		return wildcard(s)
	case symbol.FormReference, symbol.FormSum:
		return reference(s)
	default:
		return nil, fmt.Errorf("%s has no Go spelling", spoke.Describe(s))
	}
}

// scalar spells a scalar as the predeclared type of its class and width.
func scalar(s rules.TypeShape) (*emit.TypeRef, error) {
	name, declared := golang.NumericName(golang.Numeric{Class: s.Class, Width: s.Bits})
	switch {
	case declared:
		return &emit.TypeRef{Spelling: name}, nil
	case s.Bits == 0:
		return nil, fmt.Errorf("no Go %s type has the platform's width", s.Class)
	default:
		return nil, fmt.Errorf("no Go %s type has %d bits", s.Class, s.Bits)
	}
}

// container spells a form of one child: an optional as a pointer, or as
// the child itself where Go's nil value of the child is the absent value,
// a list as a slice, an array as an array of its length, a synchronous
// stream as a channel, and an asynchronous stream as an iterator over
// pairs of an element and an error. It refuses an array of an unstated
// length, and an immutable list or array.
func container(s rules.TypeShape) (*emit.TypeRef, error) {
	if s.Form == symbol.FormArray && s.Length == 0 {
		return nil, errors.New("an array of an unstated length has no Go spelling")
	}
	if err := spoke.ReadOnly(s, goName); err != nil {
		return nil, err
	}
	if err := spoke.Arity(s, 1); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, goType)
	if err != nil {
		return nil, err
	}
	switch {
	case s.Form == symbol.FormOptional && nilable(s.Elems[0]):
		return elems[0], nil
	case s.Form == symbol.FormStream && s.Async:
		return &emit.TypeRef{
			Spelling: iterType, Package: iterPackage, Args: []*emit.TypeRef{elems[0], {Spelling: errorType}},
		}, nil
	}
	return restated(&emit.TypeRef{Form: s.Form, Length: s.Length, Elems: elems})
}

// mapOf spells a map. It refuses an immutable map and a key whose values
// Go does not compare.
func mapOf(s rules.TypeShape) (*emit.TypeRef, error) {
	if err := spoke.ReadOnly(s, goName); err != nil {
		return nil, err
	}
	if err := spoke.Arity(s, 2); err != nil {
		return nil, err
	}
	switch k := s.Elems[0]; k.Form {
	case symbol.FormList, symbol.FormMap, symbol.FormFunc, symbol.FormBytes:
		return nil, fmt.Errorf("%s: Go does not compare the values of %s, so the type cannot be a map key",
			k.Spelling, spoke.Describe(k))
	default:
		// Go compares the values of every other form.
	}
	elems, err := spoke.Children(s.Elems, goType)
	if err != nil {
		return nil, err
	}
	return restated(&emit.TypeRef{Form: symbol.FormMap, Elems: elems})
}

// function spells a function type: its parameters, then nothing, one
// result or a parenthesised list of results.
func function(s rules.TypeShape) (*emit.TypeRef, error) {
	if err := spoke.Split(s); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, goType)
	if err != nil {
		return nil, err
	}
	return restated(&emit.TypeRef{Form: symbol.FormFunc, Split: s.Split, Elems: elems})
}

// restated sets the spelling of the structural reference t from the
// spellings of its children through [golang.Restate], and returns t. A
// child writes its arguments in the brackets of Go. restated allocates
// what Restate allocates, and the list of spellings of a function type of
// more than four children.
func restated(t *emit.TypeRef) (*emit.TypeRef, error) {
	var stack [spelledStack]string
	spelled := stack[:0]
	for _, e := range t.Elems {
		spelled = append(spelled, spellref.Spell(e, argsOpener, argsCloser, anyType))
	}
	spelling, err := golang.Restate(t, spelled)
	if err != nil {
		return nil, err
	}
	t.Spelling = spelling
	return t, nil
}

// wildcard spells a wildcard as any where it has no bound, and as its
// upper bound otherwise. It refuses a lower bound.
func wildcard(s rules.TypeShape) (*emit.TypeRef, error) {
	switch {
	case len(s.Elems) == 0:
		return &emit.TypeRef{Spelling: anyType}, nil
	case s.Variance == symbol.VarianceIn:
		return nil, errors.New("a wildcard with a lower bound has no Go spelling")
	}
	return spoke.Child(s.Elems[0], goType)
}

// reference spells a reference or a sum: a well-known type as Go's time
// type, as the empty struct or as error, and every other referent as a
// translated reference under its flat name, with its translated
// arguments.
func reference(s rules.TypeShape) (*emit.TypeRef, error) {
	switch s.Ref {
	case rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: timeType, Package: timePackage}, nil
	case rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: durationType, Package: timePackage}, nil
	case rules.WellKnownEmpty:
		return &emit.TypeRef{Spelling: emptyStruct, Form: symbol.FormInline}, nil
	case rules.WellKnownError:
		return &emit.TypeRef{Spelling: errorType}, nil
	}
	args, err := spoke.Children(s.Args, goType)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: s.Ref.FlatName(), Target: s.Ref, Args: args}, nil
}

// nilable reports whether the nil value of Go's spelling of s is the
// absent value: a slice, bytes, a map, a function, a channel, an iterator,
// any, error and the interface of a sum.
func nilable(s rules.TypeShape) bool {
	switch s.Form {
	case symbol.FormList, symbol.FormBytes, symbol.FormMap, symbol.FormFunc, symbol.FormStream,
		symbol.FormDynamic, symbol.FormSum:
		return true
	case symbol.FormReference:
		return s.Ref == rules.WellKnownError
	default:
		return false
	}
}
