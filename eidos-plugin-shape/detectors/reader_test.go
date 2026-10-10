// Copyright Dokimasia B.V. 2026
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

// readers are the detectors of the readers, each with a callable that it
// reports.
var readers = []struct {
	name   string
	detect func(rules.Callable, rules.Bound) bool
	give   rules.Callable
}{
	{"Reader", detectors.Reader, callable(getName, []rules.ParamView{key}, value, failure)},
	{"ReaderNoError", detectors.ReaderNoError, callable(getName, []rules.ParamView{key}, value)},
	{"ReaderWithBool", detectors.ReaderWithBool, callable(getName, []rules.ParamView{key}, value, okFlag)},
	{"PointerReader", detectors.PointerReader, callable(getName, []rules.ParamView{key}, optional)},
	{"BatchReader", detectors.BatchReader, callable(getName, []rules.ParamView{keys}, list, failure)},
	{"MultiReader", detectors.MultiReader, callable(getName, []rules.ParamView{key}, value, number, failure)},
	{"Lookup", detectors.Lookup, callable(getName, []rules.ParamView{key}, value, number, truth)},
	{"StreamReader", detectors.StreamReader, callable(getName, nil, stream)},
}

// A reader takes a key and returns the value of the key, and each reader
// detector states one way to return it.
func TestReader(t *testing.T) {
	t.Parallel()

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Reader, []detection{
			{
				name: "reports a callable that takes a key and returns a value beside its error",
				give: callable(getName, []rules.ParamView{key}, value, failure),
				want: true,
			},
			{
				name: "reports a key of a declared type",
				give: callable(getName, []rules.ParamView{valueIn}, value, failure),
				want: true,
			},
			{
				name: "refuses a callable without an error model",
				give: callable(getName, []rules.ParamView{key}, value),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, key}, value, failure),
			},
			{
				name: "refuses a callable of two values",
				give: callable(getName, []rules.ParamView{key}, value, value, failure),
			},
			{
				name: "refuses a list key",
				give: callable(getName, []rules.ParamView{listIn}, value, failure),
			},
			{
				name: "refuses a bytes key",
				give: callable(getName, []rules.ParamView{{Ref: bytesRef}}, value, failure),
			},
			{
				name: "refuses a map key",
				give: callable(getName, []rules.ParamView{{Ref: mapRef}}, value, failure),
			},
			{
				name: "refuses a function key",
				give: callable(getName, []rules.ParamView{{Ref: funcRef}}, value, failure),
			},
			{
				name: "refuses a key of an inline interface",
				give: callable(getName, []rules.ParamView{{Ref: inlineInterfaceRef}}, value, failure),
			},
			{
				name: "reports a key of an inline record",
				give: callable(getName, []rules.ParamView{{Ref: inlineRecordRef}}, value, failure),
				want: true,
			},
			{
				name: "reports a key of an inline body without a member",
				give: callable(getName, []rules.ParamView{{Ref: emptyInlineRef}}, value, failure),
				want: true,
			},
		})
	})

	t.Run("ReaderNoError", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.ReaderNoError, []detection{
			{
				name: "reports a callable that takes a key and returns a value",
				give: callable(getName, []rules.ParamView{key}, value),
				want: true,
			},
			{
				name: "refuses a callable with an error model",
				give: callable(getName, []rules.ParamView{key}, value, failure),
			},
			{
				name: "refuses a variadic key",
				give: callable(getName, []rules.ParamView{keys}, value),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, key}, value),
			},
			{
				name: "refuses a callable of two returns",
				give: callable(getName, []rules.ParamView{key}, value, okFlag),
			},
			{
				name: "refuses a stream return",
				give: callable(getName, []rules.ParamView{key}, stream),
			},
		})
	})

	t.Run("ReaderWithBool", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.ReaderWithBool, []detection{
			{
				name: "reports a callable that returns a value and an ok flag",
				give: callable(getName, []rules.ParamView{key}, value, okFlag),
				want: true,
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, key}, value, okFlag),
			},
			{
				name: "refuses a callable of three returns",
				give: callable(getName, []rules.ParamView{key}, value, number, truth),
			},
			{
				name: "refuses a second return that is no truth value",
				give: callable(getName, []rules.ParamView{key}, value, failure),
			},
		})
	})

	t.Run("PointerReader", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.PointerReader, []detection{
			{
				name: "reports a callable that returns an optional value",
				give: callable(getName, []rules.ParamView{key}, optional),
				want: true,
			},
			{
				name: "refuses a callable with an error model",
				give: callable(getName, []rules.ParamView{key}, optional, failure),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, key}, optional),
			},
			{
				name: "refuses a callable of two returns",
				give: callable(getName, []rules.ParamView{key}, optional, okFlag),
			},
			{
				name: "refuses a return that is not optional",
				give: callable(getName, []rules.ParamView{key}, value),
			},
		})
	})

	t.Run("BatchReader", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.BatchReader, []detection{
			{
				name: "reports a callable that takes variadic keys and returns a list beside its error",
				give: callable(getName, []rules.ParamView{keys}, list, failure),
				want: true,
			},
			{
				name: "reports a callable that returns bytes",
				give: callable(getName, []rules.ParamView{keys}, rules.ReturnView{Ref: bytesRef}, failure),
				want: true,
			},
			{
				name: "refuses a callable without an error model",
				give: callable(getName, []rules.ParamView{keys}, list),
			},
			{
				name: "refuses keys that are not variadic",
				give: callable(getName, []rules.ParamView{key}, list, failure),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, keys}, list, failure),
			},
			{
				name: "refuses a callable of two values",
				give: callable(getName, []rules.ParamView{keys}, list, number, failure),
			},
			{
				name: "refuses a value that is no list",
				give: callable(getName, []rules.ParamView{keys}, value, failure),
			},
		})
	})

	t.Run("MultiReader", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.MultiReader, []detection{
			{
				name: "reports a callable that returns two values beside its error",
				give: callable(getName, []rules.ParamView{key}, value, number, failure),
				want: true,
			},
			{
				name: "refuses a callable of one value",
				give: callable(getName, []rules.ParamView{key}, value, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(getName, []rules.ParamView{key}, value, number),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, key}, value, number, failure),
			},
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Lookup, []detection{
			{
				name: "reports a callable that returns two values and a truth value",
				give: callable(getName, []rules.ParamView{key}, value, number, truth),
				want: true,
			},
			{
				name: "refuses a third return that is no truth value",
				give: callable(getName, []rules.ParamView{key}, value, number, failure),
			},
			{
				name: "refuses a callable of two returns",
				give: callable(getName, []rules.ParamView{key}, value, okFlag),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, key}, value, number, truth),
			},
		})
	})

	t.Run("StreamReader", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.StreamReader, []detection{
			{
				name: "reports a callable that returns a stream",
				give: callable(getName, nil, stream),
				want: true,
			},
			{
				name: "reports a callable that takes one input and returns a stream",
				give: callable(getName, []rules.ParamView{key}, stream),
				want: true,
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(getName, []rules.ParamView{key, key}, stream),
			},
			{
				name: "refuses a stream beside an error",
				give: callable(getName, nil, stream, failure),
			},
			{
				name: "refuses a return that is no stream",
				give: callable(getName, nil, value),
			},
		})
	})
}

// The reader detectors allocate nothing once the binding has folded the
// references of the callable.
func TestReaderAllocs(t *testing.T) {
	for _, tt := range readers {
		t.Run(tt.name, func(t *testing.T) {
			b := bound(t)
			var got bool
			expect.MaxAllocs(t, func() { got = tt.detect(tt.give, b) }, 0,
				"the detector allocates nothing over folded references")
			assert.True(t, got, "the detector reports the callable")
		})
	}
}

// BenchmarkReader measures each reader detector over a callable that it
// reports, with the references folded before the loop.
func BenchmarkReader(b *testing.B) {
	for _, tt := range readers {
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
