// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/detectors"
	"go.dokimi.dev/eidos/sdk/rules"
)

// The names of the closers and of the other lifecycle callables of the
// cases. The release verbs vary in case, because the detector compares
// them without regard to case.
const (
	closeName      = "Close"
	shutdownName   = "shutdown"
	stopName       = "STOP"
	disconnectName = "Disconnect"
	terminateName  = "terminate"
	startName      = "Start"
	resetName      = "Reset"
	errName        = "Err"
	readyName      = "Ready"
)

// lifecycles are the detectors of the lifecycle callables, each with a
// callable that it reports.
var lifecycles = []struct {
	name   string
	detect func(rules.Callable, rules.Bound) bool
	give   rules.Callable
}{
	{"Lifecycle", detectors.Lifecycle, callable(startName, []rules.ParamView{ctx}, failure)},
	{"VoidLifecycle", detectors.VoidLifecycle, callable(resetName, nil)},
	{"PoisonAccessor", detectors.PoisonAccessor, callable(errName, nil, failure)},
	{"Closer", detectors.Closer, callable(closeName, nil, failure)},
	{"Predicate", detectors.Predicate, callable(readyName, nil, truth)},
}

// A lifecycle callable takes nothing but a context, and each lifecycle
// detector states one variant.
func TestLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Lifecycle", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Lifecycle, []detection{
			{
				name: "reports a callable that takes only a context and returns only its error",
				give: callable(startName, []rules.ParamView{ctx}, failure),
				want: true,
			},
			{
				name: "refuses a callable without a context",
				give: callable(startName, nil, failure),
			},
			{
				name: "refuses a callable with an input",
				give: callable(startName, []rules.ParamView{ctx, key}, failure),
			},
			{
				name: "refuses a callable that returns a value",
				give: callable(startName, []rules.ParamView{ctx}, value, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(startName, []rules.ParamView{ctx}),
			},
		})
	})

	t.Run("VoidLifecycle", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.VoidLifecycle, []detection{
			{
				name: "reports a callable that takes nothing and returns nothing",
				give: callable(resetName, nil),
				want: true,
			},
			{
				name: "refuses a callable with a parameter",
				give: callable(resetName, []rules.ParamView{ctx}),
			},
			{
				name: "refuses a callable with a return",
				give: callable(resetName, nil, value),
			},
			{
				name: "refuses a callable with an error model",
				give: rules.Callable{Name: resetName, Errors: rules.ErrorsThrown},
			},
		})
	})

	t.Run("PoisonAccessor", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.PoisonAccessor, []detection{
			{
				name: "reports a callable that takes nothing and returns only its error",
				give: callable(errName, nil, failure),
				want: true,
			},
			{
				name: "refuses a callable with a context",
				give: callable(errName, []rules.ParamView{ctx}, failure),
			},
			{
				name: "refuses a callable that returns a value",
				give: callable(errName, nil, value, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(errName, nil),
			},
		})
	})

	t.Run("Closer", func(t *testing.T) {
		t.Parallel()

		var tests []detection
		for _, verb := range []string{closeName, shutdownName, stopName, disconnectName, terminateName} {
			tests = append(tests, detection{
				name: "reports a poison accessor named " + verb,
				give: callable(verb, nil, failure),
				want: true,
			})
		}
		tests = append(tests,
			detection{
				name: "refuses a poison accessor with another name",
				give: callable(errName, nil, failure),
			},
			detection{
				name: "refuses a callable that is no poison accessor",
				give: callable(closeName, nil),
			},
		)
		detect(t, detectors.Closer, tests)
	})

	t.Run("Predicate", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Predicate, []detection{
			{
				name: "reports a callable that takes nothing and returns a truth value",
				give: callable(readyName, nil, truth),
				want: true,
			},
			{
				name: "refuses a callable with a parameter",
				give: callable(readyName, []rules.ParamView{key}, truth),
			},
			{
				name: "refuses a callable of two returns",
				give: callable(readyName, nil, truth, failure),
			},
			{
				name: "refuses a return that is no truth value",
				give: callable(readyName, nil, number),
			},
		})
	})
}

// The lifecycle detectors allocate nothing once the binding has folded
// the references of the callable.
func TestLifecycleAllocs(t *testing.T) {
	for _, tt := range lifecycles {
		t.Run(tt.name, func(t *testing.T) {
			b := bound(t)
			var got bool
			expect.MaxAllocs(t, func() { got = tt.detect(tt.give, b) }, 0,
				"the detector allocates nothing over folded references")
			assert.True(t, got, "the detector reports the callable")
		})
	}
}

// BenchmarkLifecycle measures each lifecycle detector over a callable that
// it reports, with the references folded before the loop.
func BenchmarkLifecycle(b *testing.B) {
	for _, tt := range lifecycles {
		b.Run(tt.name, func(b *testing.B) {
			bnd := bound(b)
			var got bool
			c := bench.Start(b).MaxAllocs(0).Warmup(1)
			defer c.End()
			for c.Loop() {
				got = tt.detect(tt.give, bnd)
			}
			assert.True(b, got, "the detector reports the callable")
		})
	}
}
