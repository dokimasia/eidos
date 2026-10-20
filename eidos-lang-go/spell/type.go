// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"errors"
	"fmt"
	"strings"

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
// that the well-known timestamp and duration take. The well-known context
// takes the type of the package context.
const (
	timePackage    = "time"
	timeType       = "time.Time"
	durationType   = "time.Duration"
	contextPackage = "context"
	contextType    = "context.Context"
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

// protobufLang is the language of the declarations that a binding of gRPC
// names as protoc-gen-go names them.
const protobufLang symbol.Lang = "protobuf"

// The dot that opens a fully-qualified protobuf name, the separator of the
// names of a nested declaration, and the mark of a name that starts with an
// underscore.
const (
	protobufDot = "."
	nameSep     = '_'
	leadingMark = 'X'
)

// knownBase is the import path below which protoc-gen-go refers to the
// well-known types of protobuf.
const knownBase = "google.golang.org/protobuf/types/known/"

// knownType is the Go type of one well-known type of protobuf: its named
// reference with its package, and the spelling of a pointer to it where
// protoc-gen-go refers to it through one, as to every message.
type knownType struct {
	named   emit.TypeRef
	pointer string
}

// knownTypes maps the full name of each well-known type of protobuf to its
// Go type under a binding of gRPC. The spoke only reads the table.
var knownTypes = map[string]knownType{
	"google.protobuf.Timestamp":   known("timestamppb", "Timestamp", true),
	"google.protobuf.Duration":    known("durationpb", "Duration", true),
	"google.protobuf.Empty":       known("emptypb", "Empty", true),
	"google.protobuf.Any":         known("anypb", "Any", true),
	"google.protobuf.Struct":      known("structpb", "Struct", true),
	"google.protobuf.Value":       known("structpb", "Value", true),
	"google.protobuf.ListValue":   known("structpb", "ListValue", true),
	"google.protobuf.NullValue":   known("structpb", "NullValue", false),
	"google.protobuf.FieldMask":   known("fieldmaskpb", "FieldMask", true),
	"google.protobuf.DoubleValue": known("wrapperspb", "DoubleValue", true),
	"google.protobuf.FloatValue":  known("wrapperspb", "FloatValue", true),
	"google.protobuf.Int64Value":  known("wrapperspb", "Int64Value", true),
	"google.protobuf.UInt64Value": known("wrapperspb", "UInt64Value", true),
	"google.protobuf.Int32Value":  known("wrapperspb", "Int32Value", true),
	"google.protobuf.UInt32Value": known("wrapperspb", "UInt32Value", true),
	"google.protobuf.BoolValue":   known("wrapperspb", "BoolValue", true),
	"google.protobuf.StringValue": known("wrapperspb", "StringValue", true),
	"google.protobuf.BytesValue":  known("wrapperspb", "BytesValue", true),
}

// known returns the entry of the Go type name of the package pkg below
// [knownBase], with the spelling of a pointer to it where pointer is set.
func known(pkg, name string, pointer bool) knownType {
	k := knownType{named: emit.TypeRef{Spelling: pkg + protobufDot + name, Package: knownBase + pkg}}
	if pointer {
		k.pointer = "*" + k.named.Spelling
	}
	return k
}

// Type spells the canonical shape s in Go. It is the spoke of the Go
// target in the cross-language hub. Under the choices grpc-go and
// connect-go of [golang.GRPC] in p, a reference to a protobuf declaration
// spells as protoc-gen-go names it: a message as a pointer, as
// *GetRequest, a nested declaration with underscores, as Session_State, and
// a well-known type of protobuf as its type below
// google.golang.org/protobuf/types/known, as *timestamppb.Timestamp. It
// spells each other form this way:
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
//     top type, the error value, an interface or a sum as the type itself,
//     because the nil value of its Go spelling is the absent value;
//   - a wildcard as its upper bound, and as any where it has no bound;
//   - the well-known timestamp and duration as time.Time and
//     time.Duration, with the import of time, the well-known empty value
//     as struct{}, the well-known error as error, and the well-known
//     context as context.Context;
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
func Type(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	// A zero policy declares no key, and its binding is the default none.
	choice, _ := p.Lookup(golang.GRPC)
	t, err := speller{bound: choice == golang.GRPCGo || choice == golang.GRPCConnect}.goType(s)
	if err != nil {
		return nil, fmt.Errorf(refusalPrefix+"%w", err)
	}
	return t, nil
}

// speller spells shapes by the rules of [Type]. bound reports a binding of
// gRPC, under which a reference to a protobuf declaration spells as
// protoc-gen-go names the declaration.
type speller struct {
	bound bool
}

// goType spells one shape by the rules of [Type], and returns a refusal
// without the language's prefix.
func (g speller) goType(s rules.TypeShape) (*emit.TypeRef, error) {
	if t, known := knownTypes[strings.TrimPrefix(s.Spelling, protobufDot)]; g.bound && known {
		return t.ref(), nil
	}
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
		return g.container(s)
	case symbol.FormMap:
		return g.mapOf(s)
	case symbol.FormFunc:
		return g.function(s)
	case symbol.FormWildcard:
		return g.wildcard(s)
	case symbol.FormReference, symbol.FormSum:
		return g.reference(s)
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
func (g speller) container(s rules.TypeShape) (*emit.TypeRef, error) {
	if s.Form == symbol.FormArray && s.Length == 0 {
		return nil, errors.New("an array of an unstated length has no Go spelling")
	}
	if err := spoke.ReadOnly(s, goName); err != nil {
		return nil, err
	}
	if err := spoke.Arity(s, 1); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, g.goType)
	if err != nil {
		return nil, err
	}
	switch {
	case s.Form == symbol.FormOptional && (nilable(s.Elems[0]) || g.bound && message(s.Elems[0])):
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
func (g speller) mapOf(s rules.TypeShape) (*emit.TypeRef, error) {
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
	elems, err := spoke.Children(s.Elems, g.goType)
	if err != nil {
		return nil, err
	}
	return restated(&emit.TypeRef{Form: symbol.FormMap, Elems: elems})
}

// function spells a function type: its parameters, then nothing, one
// result or a parenthesised list of results.
func (g speller) function(s rules.TypeShape) (*emit.TypeRef, error) {
	if err := spoke.Split(s); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, g.goType)
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
func (g speller) wildcard(s rules.TypeShape) (*emit.TypeRef, error) {
	switch {
	case len(s.Elems) == 0:
		return &emit.TypeRef{Spelling: anyType}, nil
	case s.Variance == symbol.VarianceIn:
		return nil, errors.New("a wildcard with a lower bound has no Go spelling")
	}
	return spoke.Child(s.Elems[0], g.goType)
}

// reference spells a reference or a sum: a well-known type as Go's time
// type, as the empty struct, as error or as context.Context, and every
// other referent as a translated reference under its flat name, with its
// translated arguments. Under a binding of gRPC a protobuf declaration
// takes the name that protoc-gen-go gives it, and a message is a pointer.
// A oneof refuses, because protoc-gen-go declares its interface
// unexported.
func (g speller) reference(s rules.TypeShape) (*emit.TypeRef, error) {
	if g.bound && s.Ref.Lang == protobufLang {
		named := &emit.TypeRef{Spelling: protocName(s.Ref), Target: s.Ref}
		switch s.Ref.Kind {
		case symbol.KindStruct:
			return restated(&emit.TypeRef{Form: symbol.FormOptional, Elems: []*emit.TypeRef{named}})
		case symbol.KindSum:
			return nil, fmt.Errorf("protoc-gen-go declares the interface of the oneof %s unexported", s.Ref.Name)
		default:
			return named, nil
		}
	}
	switch s.Ref {
	case rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: timeType, Package: timePackage}, nil
	case rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: durationType, Package: timePackage}, nil
	case rules.WellKnownEmpty:
		return &emit.TypeRef{Spelling: emptyStruct, Form: symbol.FormInline}, nil
	case rules.WellKnownError:
		return &emit.TypeRef{Spelling: errorType}, nil
	case rules.WellKnownContext:
		return &emit.TypeRef{Spelling: contextType, Package: contextPackage}, nil
	}
	args, err := spoke.Children(s.Args, g.goType)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: s.Ref.FlatName(), Target: s.Ref, Args: args}, nil
}

// message reports whether s is a reference to a protobuf message, which a
// binding of gRPC spells as a pointer whose nil value is the absent value.
func message(s rules.TypeShape) bool {
	return s.Form == symbol.FormReference && s.Ref.Lang == protobufLang && s.Ref.Kind == symbol.KindStruct
}

// protocName returns the name that protoc-gen-go gives the protobuf
// declaration id: the names of its owners and its own name, each in camel
// case, joined by underscores, as Session_State. A word of lower-case
// letters after an underscore takes a capital in place of the underscore,
// and a name that starts with an underscore starts with X, as protoc-gen-go
// writes a Go identifier. It allocates the name.
func protocName(id symbol.Identity) string {
	full := id.Name
	if id.Owner != "" {
		full = id.Owner + symbol.OwnerSep + id.Name
	}
	b := make([]byte, 0, len(full))
	for i := 0; i < len(full); i++ {
		c := full[i]
		lowerNext := i+1 < len(full) && 'a' <= full[i+1] && full[i+1] <= 'z'
		switch {
		case c == symbol.OwnerSep[0] && lowerNext:
		case c == symbol.OwnerSep[0]:
			b = append(b, nameSep)
		case c == nameSep && (i == 0 || full[i-1] == symbol.OwnerSep[0]):
			b = append(b, leadingMark)
		case c == nameSep && lowerNext:
		case '0' <= c && c <= '9':
			b = append(b, c)
		default:
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
			b = append(b, c)
			for ; i+1 < len(full) && 'a' <= full[i+1] && full[i+1] <= 'z'; i++ {
				b = append(b, full[i+1])
			}
		}
	}
	return string(b)
}

// ref returns a new reference to the Go type of the well-known type k, with
// the import of its package: a pointer where protoc-gen-go refers to the type
// through one. It allocates the reference, and for a pointer its element and
// the list of the element.
func (k knownType) ref() *emit.TypeRef {
	t := k.named
	if k.pointer == "" {
		return &t
	}
	return &emit.TypeRef{Spelling: k.pointer, Form: symbol.FormOptional, Elems: []*emit.TypeRef{&t}}
}

// nilable reports whether the nil value of Go's spelling of s is the
// absent value: a slice, bytes, a map, a function, a channel, an iterator,
// any, error, an interface and the interface of a sum.
func nilable(s rules.TypeShape) bool {
	switch s.Form {
	case symbol.FormList, symbol.FormBytes, symbol.FormMap, symbol.FormFunc, symbol.FormStream,
		symbol.FormDynamic, symbol.FormSum:
		return true
	case symbol.FormReference:
		return s.Ref == rules.WellKnownError || s.Ref.Kind == symbol.KindInterface
	default:
		return false
	}
}
