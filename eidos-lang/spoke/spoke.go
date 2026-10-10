// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spoke

import (
	"fmt"
	"strconv"

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

// The noun phrases that [Describe] returns for the two streams, and the
// start of its phrase for a form outside the vocabulary.
const (
	syncStream  = "a synchronous stream"
	asyncStream = "an asynchronous stream"
	unknownForm = "the form "
)

// Describe returns the noun phrase that a refusal uses for the form of
// s. The phrase has its article, as in a tuple, an inline type or an
// asynchronous stream. For a form outside the vocabulary, it returns the
// form's number. It allocates nothing for a form in the vocabulary.
func Describe(s rules.TypeShape) string {
	if s.Form == symbol.FormStream {
		if s.Async {
			return asyncStream
		}
		return syncStream
	}
	if name, declared := forms[s.Form]; declared {
		return name
	}
	return unknownForm + strconv.Itoa(int(s.Form))
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
