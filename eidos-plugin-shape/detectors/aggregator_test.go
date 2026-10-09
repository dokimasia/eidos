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

// The names of the aggregators and of the other callables of the cases.
const (
	countName   = "Count"
	pairName    = "Pair"
	consumeName = "Consume"
	addName     = "Add"
)

// aggregators are the detectors of the aggregators and the computations,
// each with a callable that it reports.
var aggregators = []struct {
	name   string
	detect func(rules.Callable, rules.Bound) bool
	give   rules.Callable
}{
	{"Aggregator", detectors.Aggregator, callable(countName, []rules.ParamView{ctx}, number, failure)},
	{"MultiAggregator", detectors.MultiAggregator, callable(pairName, []rules.ParamView{ctx}, number, value, failure)},
	{
		"StreamConsumer",
		detectors.StreamConsumer,
		callable(consumeName, []rules.ParamView{ctx, {Ref: sourceRef}}, number, failure),
	},
	{"Computation", detectors.Computation, callable(addName, []rules.ParamView{key, key}, number)},
}

// An aggregator computes a value over a whole collection, and a
// computation computes a value from its inputs alone.
func TestAggregator(t *testing.T) {
	t.Parallel()

	t.Run("Aggregator", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Aggregator, []detection{
			{
				name: "reports a callable that takes no input and returns one value beside its error",
				give: callable(countName, []rules.ParamView{ctx}, number, failure),
				want: true,
			},
			{
				name: "reports a callable that takes nothing and returns one value",
				give: callable(countName, nil, number),
				want: true,
			},
			{
				name: "refuses a callable with an input",
				give: callable(countName, []rules.ParamView{key}, number),
			},
			{
				name: "refuses a callable of two values",
				give: callable(countName, nil, number, value),
			},
		})
	})

	t.Run("MultiAggregator", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.MultiAggregator, []detection{
			{
				name: "reports a callable that takes no input and returns two values beside its error",
				give: callable(pairName, []rules.ParamView{ctx}, number, value, failure),
				want: true,
			},
			{
				name: "refuses a callable of one value",
				give: callable(pairName, []rules.ParamView{ctx}, number, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(pairName, nil, number, value),
			},
			{
				name: "refuses a callable with an input",
				give: callable(pairName, []rules.ParamView{key}, number, value, failure),
			},
		})
	})

	t.Run("StreamConsumer", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.StreamConsumer, []detection{
			{
				name: "reports a callable that takes a context and an interface and returns one value",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: sourceRef}}, number, failure),
				want: true,
			},
			{
				name: "reports an input of an inline interface",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: inlineInterfaceRef}}, number, failure),
				want: true,
			},
			{
				name: "refuses an input of an inline record",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: inlineRecordRef}}, number, failure),
			},
			{
				name: "refuses an input of an inline body without a member",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: emptyInlineRef}}, number, failure),
			},
			{
				name: "refuses a callable without a context",
				give: callable(consumeName, []rules.ParamView{{Ref: sourceRef}}, number, failure),
			},
			{
				name: "refuses a callable without an error model",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: sourceRef}}, number),
			},
			{
				name: "refuses a callable of two inputs",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: sourceRef}, key}, number, failure),
			},
			{
				name: "refuses a callable that returns no value",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: sourceRef}}, failure),
			},
			{
				name: "refuses an input of a struct",
				give: callable(consumeName, []rules.ParamView{ctx, valueIn}, number, failure),
			},
			{
				name: "refuses an input of a type that the view does not contain",
				give: callable(consumeName, []rules.ParamView{ctx, {Ref: ghostRef}}, number, failure),
			},
			{
				name: "refuses an input of a list",
				give: callable(consumeName, []rules.ParamView{ctx, listIn}, number, failure),
			},
			{
				name: "refuses an input without a type",
				give: callable(consumeName, []rules.ParamView{ctx, {}}, number, failure),
			},
		})
	})

	t.Run("Computation", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Computation, []detection{
			{
				name: "reports a callable that returns one value without a context and an error model",
				give: callable(addName, []rules.ParamView{key, key}, number),
				want: true,
			},
			{
				name: "reports a callable that takes nothing",
				give: callable(addName, nil, number),
				want: true,
			},
			{
				name: "refuses a callable with a context",
				give: callable(addName, []rules.ParamView{ctx, key}, number),
			},
			{
				name: "refuses a callable with an error model",
				give: callable(addName, []rules.ParamView{key}, number, failure),
			},
			{
				name: "refuses a callable of two returns",
				give: callable(addName, []rules.ParamView{key}, number, value),
			},
		})
	})
}

// The aggregator detectors allocate nothing once the binding has folded
// the references of the callable and read the declarations of the view.
func TestAggregatorAllocs(t *testing.T) {
	for _, tt := range aggregators {
		t.Run(tt.name, func(t *testing.T) {
			b := bound(t)
			var got bool
			expect.MaxAllocs(t, func() { got = tt.detect(tt.give, b) }, 0,
				"the detector allocates nothing over folded references")
			assert.True(t, got, "the detector reports the callable")
		})
	}
}

// BenchmarkAggregator measures each aggregator detector over a callable
// that it reports, with the references folded before the loop.
func BenchmarkAggregator(b *testing.B) {
	for _, tt := range aggregators {
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
