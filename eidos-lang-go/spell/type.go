// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The predeclared types and the syntax that the Go spoke writes. A
// function type closes its parameters with funcClose, and it writes one
// result after resultSep, or two or more between resultsOpen and
// resultsClose.
const (
	boolType     = "bool"
	stringType   = "string"
	byteType     = "byte"
	anyType      = "any"
	pointerMark  = "*"
	sliceMark    = "[]"
	arrayOpen    = "["
	arrayClose   = "]"
	mapOpen      = "map["
	mapClose     = "]"
	chanMark     = "chan "
	funcOpen     = "func("
	funcClose    = ")"
	resultSep    = " "
	resultsOpen  = " ("
	resultsClose = ")"
)

// The brackets of a Go argument list, which a spelled child writes its
// arguments in, and the separator of a list.
const (
	argsOpener = "["
	argsCloser = "]"
	listSep    = ", "
)

// The package of Go's time types, and the spellings of the two types
// that the well-known timestamp and duration take.
const (
	timePackage  = "time"
	timeType     = "time.Time"
	durationType = "time.Duration"
)

// ints and uints are Go's integer types by width in bits, 0 for the
// platform's width, and floats are Go's floating-point types.
var (
	ints   = map[int]string{0: "int", 8: "int8", 16: "int16", 32: "int32", 64: "int64"}
	uints  = map[int]string{0: "uint", 8: "uint8", 16: "uint16", 32: "uint32", 64: "uint64"}
	floats = map[int]string{32: "float32", 64: "float64"}
)

// Type spells the canonical shape s in Go. It is the spoke of the Go
// target in the cross-language hub. Go does not declare a lowering
// policy, so Type does not read p. It spells each form this way:
//
//   - a scalar as the predeclared type of its class and width, and int
//     and uint for the platform's width;
//   - Bool, Text and Bytes as bool, string and []byte;
//   - an optional as a pointer, a list as a slice, an array as an array
//     of its length, a map as a map, a function as a function type, and
//     a synchronous stream as a channel;
//   - a wildcard as its upper bound, and as any where it has no bound;
//   - the well-known timestamp and duration as time.Time and
//     time.Duration, with the import of time;
//   - every other reference and sum as a translated reference with the
//     referent's declared name, the shape's target and the translated
//     type arguments, which Go writes in brackets.
//
// Type refuses a tuple, a union, an intersection, an asynchronous
// stream, a borrow, a wildcard with a lower bound, an inline type and an
// opaque type, because Go has no spelling of them. A Go pointer aliases
// and a Go value copies, so neither is a borrow. Type refuses a scalar
// of another width and an array of an unstated length. It refuses a map
// key of a list, a map, a function type and bytes, because Go does not
// compare their values. It refuses a form whose child it refuses, and
// the error contains the source spelling of the child.
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
	case symbol.FormBytes:
		return &emit.TypeRef{
			Spelling: sliceMark + byteType, Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: byteType}},
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
	widths := floats
	switch s.Class {
	case rules.ScalarInt:
		widths = ints
	case rules.ScalarUint:
		widths = uints
	}
	name, held := widths[s.Bits]
	switch {
	case held:
		return &emit.TypeRef{Spelling: name}, nil
	case s.Bits == 0:
		return nil, fmt.Errorf("no Go %s type has the platform's width", s.Class)
	default:
		return nil, fmt.Errorf("no Go %s type has %d bits", s.Class, s.Bits)
	}
}

// container spells a form of one child: an optional as a pointer, a list
// as a slice, an array as an array of its length, and a synchronous
// stream as a channel. It refuses an array of an unstated length and an
// asynchronous stream.
func container(s rules.TypeShape) (*emit.TypeRef, error) {
	switch {
	case s.Form == symbol.FormArray && s.Length == 0:
		return nil, fmt.Errorf("an array of an unstated length has no Go spelling")
	case s.Form == symbol.FormStream && s.Async:
		return nil, fmt.Errorf("%s has no Go spelling", spoke.Describe(s))
	case len(s.Elems) != 1:
		return nil, fmt.Errorf("%s takes one child, and the shape has %d", spoke.Describe(s), len(s.Elems))
	}
	elems, err := spoke.Children(s.Elems, goType)
	if err != nil {
		return nil, err
	}
	inner := spellref.Spell(elems[0], argsOpener, argsCloser, anyType)
	t := &emit.TypeRef{Form: s.Form, Elems: elems}
	switch s.Form {
	case symbol.FormOptional:
		t.Spelling = pointerMark + inner
	case symbol.FormList:
		t.Spelling = sliceMark + inner
	case symbol.FormArray:
		t.Length = s.Length
		t.Spelling = arrayOpen + strconv.Itoa(s.Length) + arrayClose + inner
	default:
		t.Spelling = chanMark + inner
	}
	return t, nil
}

// mapOf spells a map. It refuses a key whose values Go does not compare.
func mapOf(s rules.TypeShape) (*emit.TypeRef, error) {
	if len(s.Elems) != 2 {
		return nil, fmt.Errorf("a map takes two children, a key and a value, and the shape has %d", len(s.Elems))
	}
	switch k := s.Elems[0]; k.Form {
	case symbol.FormList, symbol.FormMap, symbol.FormFunc, symbol.FormBytes:
		return nil, fmt.Errorf("%s: Go does not compare the values of %s, so the type cannot be a map key",
			k.Spelling, spoke.Describe(k))
	}
	elems, err := spoke.Children(s.Elems, goType)
	if err != nil {
		return nil, err
	}
	key := spellref.Spell(elems[0], argsOpener, argsCloser, anyType)
	value := spellref.Spell(elems[1], argsOpener, argsCloser, anyType)
	return &emit.TypeRef{Spelling: mapOpen + key + mapClose + value, Form: symbol.FormMap, Elems: elems}, nil
}

// function spells a function type: its parameters, then nothing, one
// result or a parenthesised list of results.
func function(s rules.TypeShape) (*emit.TypeRef, error) {
	if s.Split < 0 || s.Split > len(s.Elems) {
		return nil, fmt.Errorf("a function type splits its %d children at %d", len(s.Elems), s.Split)
	}
	elems, err := spoke.Children(s.Elems, goType)
	if err != nil {
		return nil, err
	}
	spelled := make([]string, len(elems))
	for i, e := range elems {
		spelled[i] = spellref.Spell(e, argsOpener, argsCloser, anyType)
	}
	params := strings.Join(spelled[:s.Split], listSep)
	var text string
	switch results := spelled[s.Split:]; len(results) {
	case 0:
		text = funcOpen + params + funcClose
	case 1:
		text = funcOpen + params + funcClose + resultSep + results[0]
	default:
		text = funcOpen + params + funcClose + resultsOpen + strings.Join(results, listSep) + resultsClose
	}
	return &emit.TypeRef{Spelling: text, Form: symbol.FormFunc, Split: s.Split, Elems: elems}, nil
}

// wildcard spells a wildcard as any where it has no bound, and as its
// upper bound otherwise. It refuses a lower bound.
func wildcard(s rules.TypeShape) (*emit.TypeRef, error) {
	switch {
	case len(s.Elems) == 0:
		return &emit.TypeRef{Spelling: anyType}, nil
	case s.Variance == symbol.VarianceIn:
		return nil, fmt.Errorf("a wildcard with a lower bound has no Go spelling")
	}
	return spoke.Child(s.Elems[0], goType)
}

// reference spells a reference or a sum: a well-known type as Go's time
// type, and every other referent as a translated reference with its
// translated arguments.
func reference(s rules.TypeShape) (*emit.TypeRef, error) {
	switch s.Ref {
	case rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: timeType, Package: timePackage}, nil
	case rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: durationType, Package: timePackage}, nil
	}
	args, err := spoke.Children(s.Args, goType)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: s.Ref.Name, Target: s.Ref, Args: args}, nil
}
