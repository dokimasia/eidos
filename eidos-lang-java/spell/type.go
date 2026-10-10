// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// typeRefusal opens every refusal of the spoke: the language's identity,
// as every satellite's refusals open.
const typeRefusal = string(java.Lang) + ": "

// The types and the syntax that the Java spoke writes.
const (
	booleanType   = "boolean"
	boxedBoolean  = "Boolean"
	stringType    = "String"
	byteType      = "byte"
	objectType    = "Object"
	listType      = "List"
	mapType       = "Map"
	runnableType  = "Runnable"
	instantType   = "Instant"
	durationType  = "Duration"
	arrayMark     = "[]"
	upperWildcard = "? extends "
	lowerWildcard = "? super "
	anyWildcard   = "?"
	argsOpener    = "<"
	argsCloser    = ">"
)

// The packages of the library types that the Java spoke writes, in the
// slash-separated form that the backend writes with Java's separator.
const (
	utilPackage     = "java/util"
	functionPackage = "java/util/function"
	timePackage     = "java/time"
)

// The primitive types of Java by width in bits, 0 for the platform's
// width, the boxed class of each primitive, and the functional
// interfaces of java.util.function by their numbers of parameters and
// results. Java represents an unsigned integer by the signed type of its
// width.
var (
	javaInts   = map[int]string{0: "long", 8: "byte", 16: "short", 32: "int", 64: "long"}
	javaFloats = map[int]string{32: "float", 64: "double"}
	javaBoxes  = map[string]string{
		"byte":   "Byte",
		"short":  "Short",
		"int":    "Integer",
		"long":   "Long",
		"float":  "Float",
		"double": "Double",
	}
	javaFuncs = map[[2]int]string{
		{0, 1}: "Supplier",
		{1, 0}: "Consumer",
		{1, 1}: "Function",
		{2, 0}: "BiConsumer",
		{2, 1}: "BiFunction",
	}
)

// Type spells the canonical shape s in Java. It is the spoke of the Java
// target in the cross-language hub. Java does not declare a lowering
// policy, so Type does not read p. It spells each form this way:
//
//   - a scalar as the primitive of its width, long for the platform's
//     width, and an unsigned integer as the signed type of its width;
//   - Bool, Text and Bytes as boolean, String and byte[], and the top type
//     as Object;
//   - an optional as the boxed class of its type, because a Java
//     reference can be null;
//   - a list as java.util.List, a map as java.util.Map, and an array of
//     any length as T[], because a Java array type has no length;
//   - a function type as Runnable or as the interface of
//     java.util.function for the numbers of its parameters and results:
//     Supplier, Consumer, Function, BiConsumer or BiFunction;
//   - a borrow as its type, and a wildcard as ? extends B, ? super B or ?;
//   - the well-known timestamp and duration as java.time.Instant and
//     java.time.Duration;
//   - every other reference and sum as a translated reference with the
//     referent's flat name, the shape's target and the translated type
//     arguments, which Java writes in angle brackets.
//
// In a type argument and in a wildcard's bound, the spoke writes the
// boxed class of a primitive, so a list of a 32-bit integer is
// List<Integer>.
//
// Type refuses a tuple, a union, an intersection, a stream, an inline
// type and an opaque type, because Java has no spelling of them. It
// refuses the well-known empty value, because Java does not have a type
// of a value without data outside a declaration. It refuses a function
// type of more than two parameters or more than one result, and a scalar
// of another width. It refuses a form whose child it refuses, and the
// error contains the source spelling of the child.
//
// # Allocation contract
//
// Type allocates each reference that it returns, the list of children
// of a structural reference, the list of arguments of a named one, and
// the spelling of each structural reference. A refusal allocates its
// error.
func Type(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	t, err := javaType(s, false)
	if err != nil {
		return nil, fmt.Errorf(typeRefusal+"%w", err)
	}
	return t, nil
}

// javaType spells one shape by the rules of [Type], and a primitive as
// its boxed class where boxed is set. It returns a refusal without the
// language's prefix.
func javaType(s rules.TypeShape, boxed bool) (*emit.TypeRef, error) {
	argument := func(c rules.TypeShape) (*emit.TypeRef, error) { return javaType(c, true) }
	switch s.Form {
	case symbol.FormScalar:
		return scalar(s, boxed)
	case symbol.FormBool:
		if boxed {
			return &emit.TypeRef{Spelling: boxedBoolean}, nil
		}
		return &emit.TypeRef{Spelling: booleanType}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: stringType}, nil
	case symbol.FormDynamic:
		return &emit.TypeRef{Spelling: objectType}, nil
	case symbol.FormBytes:
		return &emit.TypeRef{
			Spelling: byteType + arrayMark, Form: symbol.FormArray, Elems: []*emit.TypeRef{{Spelling: byteType}},
		}, nil
	case symbol.FormOptional, symbol.FormList, symbol.FormArray, symbol.FormBorrow:
		return container(s, boxed)
	case symbol.FormMap:
		if len(s.Elems) != 2 {
			return nil, fmt.Errorf("a map takes two children, a key and a value, and the shape has %d", len(s.Elems))
		}
		args, err := spoke.Children(s.Elems, argument)
		if err != nil {
			return nil, err
		}
		return &emit.TypeRef{Spelling: mapType, Package: utilPackage, Args: args}, nil
	case symbol.FormFunc:
		return function(s, argument)
	case symbol.FormWildcard:
		return wildcard(s, argument)
	case symbol.FormReference, symbol.FormSum:
		return reference(s, argument)
	default:
		return nil, fmt.Errorf("%s has no Java spelling", spoke.Describe(s))
	}
}

// scalar spells a scalar as the primitive of its width, or as the
// primitive's boxed class where boxed is set.
func scalar(s rules.TypeShape, boxed bool) (*emit.TypeRef, error) {
	widths := javaInts
	if s.Class == rules.ScalarFloat {
		widths = javaFloats
	}
	name, held := widths[s.Bits]
	switch {
	case !held && s.Bits == 0:
		return nil, fmt.Errorf("no Java %s type has the platform's width", s.Class)
	case !held:
		return nil, fmt.Errorf("no Java %s type has %d bits", s.Class, s.Bits)
	case boxed:
		return &emit.TypeRef{Spelling: javaBoxes[name]}, nil
	default:
		return &emit.TypeRef{Spelling: name}, nil
	}
}

// container spells a form of one child: an optional as the boxed class
// of its type, a list as List<T>, an array as T[], and a borrow as its
// type in the borrow's own position.
func container(s rules.TypeShape, boxed bool) (*emit.TypeRef, error) {
	if len(s.Elems) != 1 {
		return nil, fmt.Errorf("%s takes one child, and the shape has %d", spoke.Describe(s), len(s.Elems))
	}
	switch s.Form {
	case symbol.FormBorrow:
		return spoke.Child(s.Elems[0], func(c rules.TypeShape) (*emit.TypeRef, error) { return javaType(c, boxed) })
	case symbol.FormArray:
		elem, err := spoke.Child(
			s.Elems[0],
			func(c rules.TypeShape) (*emit.TypeRef, error) { return javaType(c, false) },
		)
		if err != nil {
			return nil, err
		}
		return &emit.TypeRef{
			Spelling: spellref.Spell(elem, argsOpener, argsCloser, objectType) + arrayMark,
			Form:     symbol.FormArray, Elems: []*emit.TypeRef{elem},
		}, nil
	}
	args, err := spoke.Children(s.Elems, func(c rules.TypeShape) (*emit.TypeRef, error) { return javaType(c, true) })
	if err != nil {
		return nil, err
	}
	if s.Form == symbol.FormOptional {
		return args[0], nil
	}
	return &emit.TypeRef{Spelling: listType, Package: utilPackage, Args: args}, nil
}

// function spells a function type as Runnable or as the functional
// interface that the numbers of its parameters and results pick, with
// its parameters and then its results as the interface's arguments.
func function(s rules.TypeShape, argument func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if s.Split < 0 || s.Split > len(s.Elems) {
		return nil, fmt.Errorf("a function type splits its %d children at %d", len(s.Elems), s.Split)
	}
	params, results := s.Split, len(s.Elems)-s.Split
	if params == 0 && results == 0 {
		return &emit.TypeRef{Spelling: runnableType}, nil
	}
	name, held := javaFuncs[[2]int{params, results}]
	if !held {
		return nil, fmt.Errorf("a function type of %d parameters and %d results has no Java functional interface",
			params, results)
	}
	args, err := spoke.Children(s.Elems, argument)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: name, Package: functionPackage, Args: args}, nil
}

// wildcard spells a wildcard as ? where it has no bound, as ? super B
// for a lower bound, and as ? extends B otherwise.
func wildcard(s rules.TypeShape, argument func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	if len(s.Elems) == 0 {
		return &emit.TypeRef{Spelling: anyWildcard, Form: symbol.FormWildcard}, nil
	}
	bound, err := spoke.Child(s.Elems[0], argument)
	if err != nil {
		return nil, err
	}
	opener, variance := upperWildcard, symbol.VarianceOut
	if s.Variance == symbol.VarianceIn {
		opener, variance = lowerWildcard, symbol.VarianceIn
	}
	return &emit.TypeRef{
		Spelling: opener + spellref.Spell(bound, argsOpener, argsCloser, objectType), Form: symbol.FormWildcard,
		Variance: variance, Elems: []*emit.TypeRef{bound},
	}, nil
}

// reference spells a reference or a sum: the well-known timestamp and
// duration as the classes of java.time, and every other referent as a
// translated reference under its flat name, with its translated
// arguments. It refuses the well-known empty value.
func reference(s rules.TypeShape, argument func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	switch s.Ref {
	case rules.WellKnownTimestamp:
		return &emit.TypeRef{Spelling: instantType, Package: timePackage}, nil
	case rules.WellKnownDuration:
		return &emit.TypeRef{Spelling: durationType, Package: timePackage}, nil
	case rules.WellKnownEmpty:
		return nil, fmt.Errorf("the well-known empty value has no Java spelling")
	}
	args, err := spoke.Children(s.Args, argument)
	if err != nil {
		return nil, err
	}
	return &emit.TypeRef{Spelling: s.Ref.FlatName(), Target: s.Ref, Args: args}, nil
}
