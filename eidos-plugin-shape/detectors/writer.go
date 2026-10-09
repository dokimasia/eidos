// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/rules"
)

// removalVerbs are the names of a deleter, compared without regard to
// case.
var removalVerbs = []string{"delete", "remove", "del", "evict", "purge"}

// Writer reports a callable that takes one input and returns no value
// beside its error.
func Writer(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.fails && s.inputs == 1 && s.values == 0
}

// Deleter reports a writer whose name is one of the removal verbs delete,
// remove, del, evict and purge, compared without regard to case.
func Deleter(c rules.Callable, b rules.Bound) bool {
	return Writer(c, b) &&
		slices.ContainsFunc(removalVerbs, func(v string) bool { return strings.EqualFold(c.Name, v) })
}

// AnsweringWriter reports a callable that takes one input and returns one
// value of the declared type of the input beside its error. An optional
// of the type counts as the type, and a type parameter is a declared type.
// A builtin type is no declared type, because a builtin such as a string
// is the type of a key and of a value alike.
func AnsweringWriter(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	if !s.fails || s.inputs != 1 || s.values != 1 {
		return false
	}
	in := declared(s.input.Ref)
	return !in.IsZero() && in == declared(s.value.Ref)
}

// CompositeWriter reports a callable that takes two inputs, a key and a
// value, and returns no value beside its error.
func CompositeWriter(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.fails && s.inputs == 2 && s.values == 0
}

// MultiArgWriter reports a callable that takes three or more inputs and
// returns no value beside its error.
func MultiArgWriter(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.fails && s.inputs >= 3 && s.values == 0
}

// Mutator reports a callable that takes one input, returns nothing, and
// has no error model.
func Mutator(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return !s.fails && s.inputs == 1 && len(c.Returns) == 0
}
