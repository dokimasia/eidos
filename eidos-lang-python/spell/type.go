// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"errors"
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix is the start of every refusal of the package. It is the
// identity of the language, as in the refusals of every satellite.
const refusalPrefix = string(python.Lang) + ": "

// The builtin types of Python that the spoke writes, and the package path of
// their module builtins. Python reads a name of builtins without an import.
// A reference to a builtin records the module, so no declaration of the
// file binds the reference, and the backend writes builtins.int where a
// declaration hides int. None is a keyword, which no declaration hides, so
// a reference to None records no module.
const (
	builtinsModule = "builtins"
	intType        = "int"
	floatType      = "float"
	boolType       = "bool"
	strType        = "str"
	bytesType      = "bytes"
	noneType       = "None"
	objectType     = "object"
	exceptionType  = "Exception"
	listType       = "list"
	dictType       = "dict"
	tupleType      = "tuple"
)

// The types of the standard library that the spoke writes, and the package
// paths of their modules. The package path of a module is its dotted name
// with slashes.
const (
	anyType           = "Any"
	neverType         = "Never"
	sequenceType      = "Sequence"
	mappingType       = "Mapping"
	callableType      = "Callable"
	iteratorType      = "Iterator"
	asyncIteratorType = "AsyncIterator"
	datetimeType      = "datetime"
	timedeltaType     = "timedelta"
	typingModule      = "typing"
	abcModule         = "collections/abc"
	datetimeModule    = "datetime"
)

// The syntax of Python's type expressions. A generic class writes its
// arguments in brackets, a union separates its members with a bar, and
// tuple[()] is a tuple without members.
const (
	argsOpen   = "["
	argsClose  = "]"
	listSep    = ", "
	unionSep   = " | "
	emptyTuple = tupleType + argsOpen + "()" + argsClose
)

// spelledStack is the number of spellings of members that [spelled] keeps
// on the stack. A form with more members allocates their list.
const spelledStack = 4

// floatBits is the width of a float of Python, a double of IEEE 754, which
// holds every value of a narrower float.
const floatBits = 64

// Type spells the canonical shape s in Python under the policy p. It is the
// spoke of the Python target in the cross-language hub, and it spells each
// form this way:
//
//   - every integer as int, whose values have no bound, and a float of 64
//     bits or fewer as float;
//   - Bool, Text and Bytes as bool, str and bytes, and the top type as the
//     choice of [python.DynamicPolicy]: object, or Any of typing;
//   - an optional as the union of its type and None, as in str | None;
//   - a list as list[T], and an immutable list as Sequence[T];
//   - an array of a stated length as a tuple of that many members, and an
//     array of 8-bit unsigned integers as bytes;
//   - a map as dict[K, V], and an immutable map as Mapping[K, V];
//   - a tuple as tuple[A, B], and a tuple without members as tuple[()];
//   - a union as A | B, and a union without members as Never of typing;
//   - a function type as Callable[[A, B], R], whose result is None without
//     a result and a tuple for two results or more;
//   - a synchronous stream as Iterator[T], and an asynchronous stream as
//     AsyncIterator[T];
//   - a borrow as its type, and a wildcard as its upper bound or as object
//     without a bound;
//   - an intersection of interfaces as the intersection of the references
//     without a spelling. The lowering of the backend declares a Protocol
//     that extends each member, and refers to it in place of the
//     intersection;
//   - the well-known timestamp and duration as datetime and timedelta of
//     datetime, the well-known empty value as None, and the well-known
//     error as Exception;
//   - every other reference and sum as a translated reference with the
//     referent's flat name, the shape's target and the translated type
//     arguments, which Python writes in brackets.
//
// Sequence, Mapping, Callable, Iterator and AsyncIterator are the types of
// collections.abc. A named reference has the package path of its module in
// Package, and builtins for a builtin. None is a keyword and has no module.
// A tuple of Python is immutable, so the spoke writes a tuple and an array of
// either mutability as an immutable tuple.
//
// Type refuses an inline type, because its shape has the types of its
// members and not their names. It refuses an opaque type and an
// intersection of a member that is not an interface, because Python has no
// form of them. It refuses a wildcard with a lower bound, a float of more
// than 64 bits, an array of an unstated length and an intersection without
// members. It refuses a form whose child it refuses, and the error contains
// the source spelling of the child.
//
// # Allocation contract
//
// Type allocates each reference that it returns, the list of children of a
// structural reference, the list of arguments of a named one, and the
// spelling of each structural reference. The spelling of a function type
// allocates once more for its result, and once more for a result of two
// types or more. A refusal allocates its error.
func Type(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	t, err := pythonType(s, p)
	if err != nil {
		return nil, fmt.Errorf(refusalPrefix+"%w", err)
	}
	return t, nil
}

// pythonType spells one shape by the rules of [Type], and returns a refusal
// without the language's prefix.
func pythonType(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	spell := func(c rules.TypeShape) (*emit.TypeRef, error) { return pythonType(c, p) }
	switch s.Form {
	case symbol.FormScalar:
		return scalar(s)
	case symbol.FormBool:
		return &emit.TypeRef{Spelling: boolType, Package: builtinsModule}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: strType, Package: builtinsModule}, nil
	case symbol.FormBytes:
		return &emit.TypeRef{Spelling: bytesType, Package: builtinsModule}, nil
	case symbol.FormDynamic:
		if p.Choice(python.DynamicPolicy) == python.Any {
			return &emit.TypeRef{Spelling: anyType, Package: typingModule}, nil
		}
		return &emit.TypeRef{Spelling: objectType, Package: builtinsModule}, nil
	case symbol.FormBorrow:
		if err := spoke.Arity(s, 1); err != nil {
			return nil, err
		}
		return spoke.Child(s.Elems[0], spell)
	case symbol.FormOptional, symbol.FormList, symbol.FormMap, symbol.FormStream:
		return generic(s, spell)
	case symbol.FormArray:
		return array(s, spell)
	case symbol.FormTuple, symbol.FormUnion:
		return members(s, spell)
	case symbol.FormIntersection:
		return intersection(s, spell)
	case symbol.FormFunc:
		return function(s, spell)
	case symbol.FormWildcard:
		return wildcard(s, spell)
	case symbol.FormReference, symbol.FormSum:
		return reference(s, spell)
	default:
		return nil, fmt.Errorf("%s has no Python spelling", spoke.Describe(s))
	}
}

// scalar spells a scalar: every integer as int, because an int of Python has
// no bound, and a float of 64 bits or fewer and of the platform's width as
// float, which has 64 bits and holds each of their values. A wider float
// has no Python spelling.
func scalar(s rules.TypeShape) (*emit.TypeRef, error) {
	switch {
	case s.Class == rules.ScalarInt || s.Class == rules.ScalarUint:
		return &emit.TypeRef{Spelling: intType, Package: builtinsModule}, nil
	case s.Class == rules.ScalarFloat && s.Bits <= floatBits:
		return &emit.TypeRef{Spelling: floatType, Package: builtinsModule}, nil
	default:
		return nil, fmt.Errorf("no Python %s type has %d bits", s.Class, s.Bits)
	}
}

// generic spells a form of one child, or of a key and a value: an optional
// as the union of its type and None, a list as list[T] or Sequence[T], a map
// as dict[K, V] or Mapping[K, V], and a stream as Iterator[T] or
// AsyncIterator[T].
func generic(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	n := 1
	if s.Form == symbol.FormMap {
		n = 2
	}
	if err := spoke.Arity(s, n); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	immutable := s.Mutability == symbol.MutabilityImmutable
	switch {
	case s.Form == symbol.FormOptional:
		return &emit.TypeRef{
			Spelling: spelled("", elems, "", unionSep+noneType), Form: symbol.FormOptional, Elems: elems,
		}, nil
	case s.Form == symbol.FormList && immutable:
		return &emit.TypeRef{Spelling: sequenceType, Package: abcModule, Args: elems}, nil
	case s.Form == symbol.FormList:
		return &emit.TypeRef{Spelling: listType, Package: builtinsModule, Args: elems}, nil
	case s.Form == symbol.FormMap && immutable:
		return &emit.TypeRef{Spelling: mappingType, Package: abcModule, Args: elems}, nil
	case s.Form == symbol.FormMap:
		return &emit.TypeRef{Spelling: dictType, Package: builtinsModule, Args: elems}, nil
	case s.Async:
		return &emit.TypeRef{Spelling: asyncIteratorType, Package: abcModule, Args: elems}, nil
	default:
		return &emit.TypeRef{Spelling: iteratorType, Package: abcModule, Args: elems}, nil
	}
}

// array spells an array of a stated length as a tuple of that many members,
// each spelled on its own, and an array of 8-bit unsigned integers as bytes.
// It refuses an array of an unstated length.
func array(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if s.Length == 0 {
		return nil, errors.New("an array of an unstated length has no Python spelling")
	}
	if err := spoke.Arity(s, 1); err != nil {
		return nil, err
	}
	elem := s.Elems[0]
	if elem.Form == symbol.FormScalar && elem.Class == rules.ScalarUint && elem.Bits == 8 {
		return &emit.TypeRef{Spelling: bytesType, Package: builtinsModule}, nil
	}
	elems := make([]*emit.TypeRef, s.Length)
	for i := range elems {
		t, err := spoke.Child(elem, spell)
		if err != nil {
			return nil, err
		}
		elems[i] = t
	}
	return tupleOf(elems), nil
}

// members spells a tuple as tuple[A, B] and a union as A | B. A union
// without members is the bottom type, which Python spells as Never.
func members(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	elems, err := spoke.Children(s.Elems, spell)
	switch {
	case err != nil:
		return nil, err
	case s.Form == symbol.FormTuple:
		return tupleOf(elems), nil
	case len(elems) == 0:
		return &emit.TypeRef{Spelling: neverType, Package: typingModule}, nil
	}
	return &emit.TypeRef{Spelling: spelled("", elems, unionSep, ""), Form: symbol.FormUnion, Elems: elems}, nil
}

// intersection spells an intersection of interfaces as the intersection of
// the references, without a spelling. The lowering of the backend declares
// the Protocol that extends each member. It refuses an intersection without
// members, and an intersection of a member that is not an interface,
// because a Protocol extends Protocols alone.
func intersection(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if len(s.Elems) == 0 {
		return nil, errors.New("an intersection without members has no Python spelling")
	}
	for _, m := range s.Elems {
		if m.Form != symbol.FormReference || m.Ref.Kind != symbol.KindInterface {
			return nil, fmt.Errorf(
				"%s: a Protocol extends interfaces alone, and the member is no interface",
				m.Spelling,
			)
		}
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Form: symbol.FormIntersection, Elems: elems}, nil
}

// function spells a function type as Callable[[A, B], R]. The result R is
// None without a result, the result for one result, and a tuple of the
// results for two results or more.
func function(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if err := spoke.Split(s); err != nil {
		return nil, err
	}
	elems, err := spoke.Children(s.Elems, spell)
	if err != nil {
		return nil, err
	}
	var result string
	switch results := elems[s.Split:]; len(results) {
	case 0:
		result = noneType
	case 1:
		result = spellref.Spell(results[0], argsOpen, argsClose, objectType)
	default:
		result = spelled(tupleType+argsOpen, results, listSep, argsClose)
	}
	return &emit.TypeRef{
		Spelling: spelled(callableType+argsOpen+argsOpen, elems[:s.Split], listSep, argsClose+listSep+result+argsClose),
		Form:     symbol.FormFunc, Split: s.Split, Elems: elems,
	}, nil
}

// wildcard spells a wildcard as object where it has no bound, and as its
// upper bound otherwise. It refuses a lower bound.
func wildcard(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	switch {
	case len(s.Elems) == 0:
		return &emit.TypeRef{Spelling: objectType, Package: builtinsModule}, nil
	case s.Variance == symbol.VarianceIn:
		return nil, errors.New("a wildcard with a lower bound has no Python spelling")
	}
	return spoke.Child(s.Elems[0], spell)
}

// reference spells a reference or a sum: the well-known timestamp and
// duration as datetime and timedelta of datetime, the well-known empty value
// as None, the well-known error as Exception, and every other referent as a
// translated reference under its flat name, with its translated arguments.
func reference(s rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	switch s.Ref {
	case rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: datetimeType, Package: datetimeModule}, nil
	case rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: timedeltaType, Package: datetimeModule}, nil
	case rules.WellKnownEmpty:
		return &emit.TypeRef{Spelling: noneType}, nil
	case rules.WellKnownError:
		return &emit.TypeRef{Spelling: exceptionType, Package: builtinsModule}, nil
	case rules.WellKnownContext:
		return nil, errors.New("the well-known context has no Python spelling, because Python has no standard " +
			"type of the context of a call")
	}
	args, err := spoke.Children(s.Args, spell)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: s.Ref.FlatName(), Target: s.Ref, Args: args}, nil
}

// tupleOf returns the immutable tuple of the members elems: tuple[A, B], and
// tuple[()] without members.
func tupleOf(elems []*emit.TypeRef) *emit.TypeRef {
	text := emptyTuple
	if len(elems) > 0 {
		text = spelled(tupleType+argsOpen, elems, listSep, argsClose)
	}
	return &emit.TypeRef{
		Spelling: text, Form: symbol.FormTuple, Mutability: symbol.MutabilityImmutable, Elems: elems,
	}
}

// spelled returns open, the spellings of refs separated by sep, and closer,
// written into one buffer. A reference with arguments writes them in
// brackets. spelled allocates the text, and what the spelling of each
// reference with arguments allocates. A form of more than [spelledStack]
// members also allocates the list of their spellings.
func spelled(open string, refs []*emit.TypeRef, sep, closer string) string {
	var stack [spelledStack]string
	parts := stack[:0]
	n := len(open) + len(closer) + len(sep)*max(len(refs)-1, 0)
	for _, r := range refs {
		part := spellref.Spell(r, argsOpen, argsClose, objectType)
		parts = append(parts, part)
		n += len(part)
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteString(open)
	for i, part := range parts {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(part)
	}
	b.WriteString(closer)
	return b.String()
}
