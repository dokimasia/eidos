// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// releaseVerbs are the names of a closer, compared without regard to
// case.
var releaseVerbs = []string{"close", "shutdown", "stop", "disconnect", "terminate"}

// Lifecycle reports a callable that takes only a context and returns no
// value beside its error.
func Lifecycle(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.context && s.fails && s.inputs == 0 && s.values == 0
}

// VoidLifecycle reports a callable that takes nothing, returns nothing,
// and has no error model.
func VoidLifecycle(c rules.Callable, _ rules.Bound) bool {
	return len(c.Params) == 0 && len(c.Returns) == 0 && c.Errors == rules.ErrorsNone
}

// PoisonAccessor reports a callable that takes nothing and returns no
// value beside its error.
func PoisonAccessor(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.fails && len(c.Params) == 0 && s.values == 0
}

// Closer reports a poison accessor whose name is one of the release verbs
// close, shutdown, stop, disconnect and terminate, compared without
// regard to case.
func Closer(c rules.Callable, b rules.Bound) bool {
	return PoisonAccessor(c, b) &&
		slices.ContainsFunc(releaseVerbs, func(v string) bool { return strings.EqualFold(c.Name, v) })
}

// Predicate reports a callable that takes nothing and returns one truth
// value.
func Predicate(c rules.Callable, b rules.Bound) bool {
	return len(c.Params) == 0 && len(c.Returns) == 1 && b.TypeOf(c.Returns[0].Ref).Form == symbol.FormBool
}
