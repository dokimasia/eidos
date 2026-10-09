// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/detectors"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the deleters and of the other writers of the cases, and of
// the type parameter of a generic writer. The removal verbs vary in case,
// because the detector compares them without regard to case.
const (
	deleteName    = "Delete"
	removeName    = "remove"
	delName       = "DEL"
	evictName     = "Evict"
	purgeName     = "purge"
	setName       = "Set"
	recordName    = "Record"
	mutateName    = "Mutate"
	storeName     = "Store"
	typeParamName = "T"
)

// typeParamRef refers to the type parameter T of the generic writer Store,
// and typeParamIn is a parameter of the type T.
var (
	typeParamRef = &node.TypeRef{Spelling: typeParamName, Target: symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Owner:   storeName,
		Name:    typeParamName,
		Kind:    symbol.KindTypeParam,
	}}
	typeParamIn = rules.ParamView{Ref: typeParamRef}
)

// writers are the detectors of the writers, each with a callable that it
// reports.
var writers = []struct {
	name   string
	detect func(rules.Callable, rules.Bound) bool
	give   rules.Callable
}{
	{"Writer", detectors.Writer, callable(saveName, []rules.ParamView{valueIn}, failure)},
	{"Deleter", detectors.Deleter, callable(deleteName, []rules.ParamView{key}, failure)},
	{"AnsweringWriter", detectors.AnsweringWriter, callable(storeName, []rules.ParamView{valueIn}, value, failure)},
	{"CompositeWriter", detectors.CompositeWriter, callable(setName, []rules.ParamView{key, valueIn}, failure)},
	{"MultiArgWriter", detectors.MultiArgWriter, callable(recordName, []rules.ParamView{key, key, key}, failure)},
	{"Mutator", detectors.Mutator, callable(mutateName, []rules.ParamView{valueIn})},
}

// A writer takes the values that it persists and returns no value beside
// its error, and each writer detector states one variant.
func TestWriter(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Writer, []detection{
			{
				name: "reports a callable that takes one input and returns only its error",
				give: callable(saveName, []rules.ParamView{valueIn}, failure),
				want: true,
			},
			{
				name: "refuses a callable without an error model",
				give: callable(saveName, []rules.ParamView{valueIn}),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(saveName, []rules.ParamView{key, valueIn}, failure),
			},
			{
				name: "refuses a callable that returns a value",
				give: callable(saveName, []rules.ParamView{valueIn}, value, failure),
			},
		})
	})

	t.Run("Deleter", func(t *testing.T) {
		t.Parallel()

		var tests []detection
		for _, verb := range []string{deleteName, removeName, delName, evictName, purgeName} {
			tests = append(tests, detection{
				name: "reports a writer named " + verb,
				give: callable(verb, []rules.ParamView{key}, failure),
				want: true,
			})
		}
		tests = append(tests,
			detection{
				name: "refuses a writer with another name",
				give: callable(saveName, []rules.ParamView{key}, failure),
			},
			detection{
				name: "refuses a callable that is no writer",
				give: callable(deleteName, []rules.ParamView{key}),
			},
		)
		detect(t, detectors.Deleter, tests)
	})

	t.Run("AnsweringWriter", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.AnsweringWriter, []detection{
			{
				name: "reports a callable that returns a value of the type of its input",
				give: callable(storeName, []rules.ParamView{valueIn}, value, failure),
				want: true,
			},
			{
				name: "counts an optional value as the type that it wraps",
				give: callable(storeName, []rules.ParamView{valueIn}, optional, failure),
				want: true,
			},
			{
				name: "counts an optional input as the type that it wraps",
				give: callable(storeName, []rules.ParamView{{Ref: optionalRef}}, value, failure),
				want: true,
			},
			{
				name: "reports a generic callable that returns a value of its type parameter",
				give: callable(storeName, []rules.ParamView{typeParamIn}, rules.ReturnView{Ref: typeParamRef}, failure),
				want: true,
			},
			{
				name: "refuses a value of another type",
				give: callable(storeName, []rules.ParamView{valueIn}, rules.ReturnView{Ref: sourceRef}, failure),
			},
			{
				name: "refuses an input of a builtin type",
				give: callable(storeName, []rules.ParamView{key}, rules.ReturnView{Ref: stringRef}, failure),
			},
			{
				name: "refuses an input without a type",
				give: callable(storeName, []rules.ParamView{{}}, value, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(storeName, []rules.ParamView{valueIn}, value),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(storeName, []rules.ParamView{valueIn, valueIn}, value, failure),
			},
			{
				name: "refuses a callable of two values",
				give: callable(storeName, []rules.ParamView{valueIn}, value, value, failure),
			},
		})
	})

	t.Run("CompositeWriter", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.CompositeWriter, []detection{
			{
				name: "reports a callable that takes a key and a value and returns only its error",
				give: callable(setName, []rules.ParamView{key, valueIn}, failure),
				want: true,
			},
			{
				name: "refuses a callable of one input",
				give: callable(setName, []rules.ParamView{valueIn}, failure),
			},
			{
				name: "refuses a callable of three inputs",
				give: callable(setName, []rules.ParamView{key, key, valueIn}, failure),
			},
			{
				name: "refuses a callable that returns a value",
				give: callable(setName, []rules.ParamView{key, valueIn}, value, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(setName, []rules.ParamView{key, valueIn}),
			},
		})
	})

	t.Run("MultiArgWriter", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.MultiArgWriter, []detection{
			{
				name: "reports a callable that takes three inputs and returns only its error",
				give: callable(recordName, []rules.ParamView{key, key, key}, failure),
				want: true,
			},
			{
				name: "reports a callable that takes four inputs",
				give: callable(recordName, []rules.ParamView{key, key, key, key}, failure),
				want: true,
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(recordName, []rules.ParamView{key, key}, failure),
			},
			{
				name: "refuses a callable that returns a value",
				give: callable(recordName, []rules.ParamView{key, key, key}, value, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(recordName, []rules.ParamView{key, key, key}),
			},
		})
	})

	t.Run("Mutator", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Mutator, []detection{
			{
				name: "reports a callable that takes one input and returns nothing",
				give: callable(mutateName, []rules.ParamView{valueIn}),
				want: true,
			},
			{
				name: "refuses a callable with an error model",
				give: callable(mutateName, []rules.ParamView{valueIn}, failure),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(mutateName, []rules.ParamView{valueIn, valueIn}),
			},
			{
				name: "refuses a callable that returns a value",
				give: callable(mutateName, []rules.ParamView{valueIn}, value),
			},
		})
	})
}

// The writer detectors allocate nothing once the binding has folded the
// references of the callable.
func TestWriterAllocs(t *testing.T) {
	for _, tt := range writers {
		t.Run(tt.name, func(t *testing.T) {
			b := bound(t)
			var got bool
			expect.MaxAllocs(t, func() { got = tt.detect(tt.give, b) }, 0,
				"the detector allocates nothing over folded references")
			assert.True(t, got, "the detector reports the callable")
		})
	}
}

// BenchmarkWriter measures each writer detector over a callable that it
// reports, with the references folded before the loop.
func BenchmarkWriter(b *testing.B) {
	for _, tt := range writers {
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
