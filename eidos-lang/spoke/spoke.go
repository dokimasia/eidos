// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spoke

import (
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// forms are the noun phrases that [Describe] returns, by form.
var forms = map[symbol.TypeForm]string{
	symbol.FormNamed:        "a named type",
	symbol.FormOptional:     "an optional",
	symbol.FormList:         "a list",
	symbol.FormArray:        "an array",
	symbol.FormMap:          "a map",
	symbol.FormFunc:         "a function type",
	symbol.FormTuple:        "a tuple",
	symbol.FormUnion:        "a union",
	symbol.FormIntersection: "an intersection",
	symbol.FormBorrow:       "a borrow",
	symbol.FormWildcard:     "a wildcard",
	symbol.FormInline:       "an inline type",
	symbol.FormScalar:       "a scalar",
	symbol.FormBool:         "a boolean",
	symbol.FormText:         "text",
	symbol.FormBytes:        "bytes",
	symbol.FormReference:    "a reference",
	symbol.FormSum:          "a sum",
	symbol.FormOpaque:       "a type that the rules of its language do not classify",
	symbol.FormDynamic:      "the top type",
}

// The noun phrases that [Describe] returns for the two streams and for a
// type parameter, and the start of its phrase for a form outside the
// vocabulary.
const (
	syncStream  = "a synchronous stream"
	asyncStream = "an asynchronous stream"
	typeParam   = "a type parameter"
	unknownForm = "the form "
)

// The phrases that [Arity] writes for the number of children that a form
// takes: words for one and two, and digits before children for every
// other number.
const (
	oneChild    = "one child"
	twoChildren = "two children"
	children    = " children"
)

// Describe returns the noun phrase that a refusal uses for the form of
// s. The phrase has its article, as in a tuple, an inline type or an
// asynchronous stream. The opaque shape of a type parameter is a type
// parameter. For a form outside the vocabulary, it returns the form's
// number. It allocates nothing for a form in the vocabulary.
func Describe(s rules.TypeShape) string {
	switch {
	case s.Form == symbol.FormStream && s.Async:
		return asyncStream
	case s.Form == symbol.FormStream:
		return syncStream
	case s.Form == symbol.FormOpaque && s.Ref.Kind == symbol.KindTypeParam:
		return typeParam
	}
	if name, declared := forms[s.Form]; declared {
		return name
	}
	return unknownForm + strconv.Itoa(int(s.Form))
}

// Param returns the reference that a spoke writes for the shape s of a
// type parameter: the spelling of the parameter without a target, because
// the declaration that a generator writes declares the parameter under its
// name. It reports false for every other shape, the opaque shape of a
// reference that names no type parameter included. A spoke of a language
// with type parameters calls it before it reads the form of a shape.
//
// # Allocation contract
//
// Param allocates the reference of a type parameter, one allocation, and
// nothing for every other shape.
func Param(s rules.TypeShape) (*emit.TypeRef, bool) {
	if s.Form != symbol.FormOpaque || s.Ref.Kind != symbol.KindTypeParam {
		return nil, false
	}
	return &emit.TypeRef{Spelling: s.Spelling}, true
}

// Arity returns an error where s does not have n children, such as "a
// list takes one child, and the shape has 2". The error starts with the
// phrase that [Describe] returns for the form of s. It returns nil where s
// has n children.
//
// # Allocation contract
//
// Arity allocates nothing for a shape of n children. An error allocates
// its message.
func Arity(s rules.TypeShape, n int) error {
	if len(s.Elems) == n {
		return nil
	}
	var want string
	switch n {
	case 1:
		want = oneChild
	case 2:
		want = twoChildren
	default:
		want = strconv.Itoa(n) + children
	}
	return fmt.Errorf("%s takes %s, and the shape has %d", Describe(s), want, len(s.Elems))
}

// Split returns an error where the split of the function type s is not
// between 0 and the number of its children. The error contains the number
// of children and the split. It returns nil for a split inside the
// children, the last position included.
//
// # Allocation contract
//
// Split allocates nothing for a split inside the children. An error
// allocates its message.
func Split(s rules.TypeShape) error {
	if s.Split >= 0 && s.Split <= len(s.Elems) {
		return nil
	}
	return fmt.Errorf("a function type splits its %d children at %d", len(s.Elems), s.Split)
}

// ReadOnly returns an error where the shape s is immutable, such as "a
// read-only list has no Go spelling" for the language lang. A spoke whose
// language writes the form of s as writable refuses such a shape, because
// a writable spelling drops the constraint of the shape. ReadOnly returns
// nil where s is not immutable.
//
// # Allocation contract
//
// ReadOnly allocates nothing for a shape that is not immutable. An error
// allocates its message.
func ReadOnly(s rules.TypeShape, lang string) error {
	if s.Mutability != symbol.MutabilityImmutable {
		return nil
	}
	_, noun, _ := strings.Cut(Describe(s), " ")
	return fmt.Errorf("a read-only %s has no %s spelling", noun, lang)
}

// Children spells each shape of a list through spell, in order, and
// returns nil for an empty list. At the first refusal of spell, it stops
// and returns the error that [Child] returns.
//
// # Allocation contract
//
// Children allocates the list of references, and what spell allocates
// for each shape. A refusal allocates its error.
func Children(shapes []rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) ([]*emit.TypeRef, error) {
	if len(shapes) == 0 {
		return nil, nil
	}
	out := make([]*emit.TypeRef, 0, len(shapes))
	for _, c := range shapes {
		t, err := Child(c, spell)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// Child spells one child of a shape through spell. The error of a child
// that spell refuses starts with the source spelling of the child. The
// refusal of a nested type contains the spelling of each child down to
// the child without a spelling in the target.
//
// # Allocation contract
//
// Child allocates what spell allocates. A refusal allocates its error.
func Child(c rules.TypeShape, spell func(rules.TypeShape) (*emit.TypeRef, error)) (*emit.TypeRef, error) {
	t, err := spell(c)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.Spelling, err)
	}
	return t, nil
}
