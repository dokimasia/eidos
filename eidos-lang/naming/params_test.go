// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/emit"
)

// The names of the parameters of the cases: a declared name, the
// positional names of the first two positions, and the numbered name of
// the first position.
const (
	declaredParam = "ctx"
	firstParam    = "a0"
	secondParam   = "a1"
	numberedParam = "a00"
)

// paramsAllocs is the ceiling of a call of Params that names one
// parameter: the set of the names of the method and the text of the name.
const paramsAllocs = 2

// TestParams pins the name that Params gives each parameter without a
// name, and the names that it takes first.
func TestParams(t *testing.T) {
	t.Parallel()

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		generic := method(nil, nil, "")
		generic.TypeParams = []*emit.TypeParam{{Name: firstParam}}
		tests := []struct {
			name string
			give *emit.Method
			want []string
		}{
			{
				name: "names each parameter without a name by its position",
				give: method(nil, nil, "", ""),
				want: []string{firstParam, secondParam},
			},
			{
				name: "keeps the name of a named parameter",
				give: method(nil, nil, declaredParam, ""),
				want: []string{declaredParam, secondParam},
			},
			{
				name: "numbers a positional name that another parameter has",
				give: method(nil, nil, "", firstParam),
				want: []string{numberedParam, firstParam},
			},
			{
				name: "numbers a positional name that a result has",
				give: method(nil, []string{firstParam}, ""),
				want: []string{numberedParam},
			},
			{
				name: "numbers a positional name that a type parameter has",
				give: generic,
				want: []string{numberedParam},
			},
			{
				name: "numbers a positional name that the receiver has",
				give: method(&emit.Param{Name: firstParam}, nil, ""),
				want: []string{numberedParam},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got []string
				for _, p := range naming.Params(tt.give).Params.Items() {
					got = append(got, p.Name)
				}
				assert.Equal(t, got, tt.want, "every parameter of the method has a name of its own")
			})
		}

		t.Run("returns the method that it names", func(t *testing.T) {
			t.Parallel()

			m := method(nil, nil, "")
			assert.Equal(t, naming.Params(m), m, "Params names the parameters in place", assert.ByIdentity())
		})

		t.Run("returns nil for a nil method", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, naming.Params(nil), "a nil method has no parameters")
		})
	})
}

// TestParamsAllocs checks the allocation ceilings of Params in a run
// without benchmarks: nothing for a method whose parameters have names,
// and the set and the text of the name for a parameter without one.
func TestParamsAllocs(t *testing.T) {
	named, unnamed := method(nil, nil, declaredParam), method(nil, nil, "")
	param := unnamed.Params.Items()[0]
	var got *emit.Method
	assert.MaxAllocs(t, func() { got = naming.Params(named) }, 0,
		"Params allocates nothing for a method whose parameters have names")
	assert.Equal(t, got.Params.Items()[0].Name, declaredParam, "Params keeps the declared name")
	assert.MaxAllocs(t, func() {
		param.Name = ""
		got = naming.Params(unnamed)
	}, paramsAllocs, "Params allocates the set of the names and the new name")
	assert.Equal(t, got.Params.Items()[0].Name, firstParam, "Params names the parameter by its position")
}

// BenchmarkParams measures Params over a method whose one parameter has
// no name, which a generator of doubles names for each mirrored method.
func BenchmarkParams(b *testing.B) {
	b.Run("Params", func(b *testing.B) {
		m := method(nil, nil, "")
		param := m.Params.Items()[0]
		c := bench.Start(b).MaxAllocs(paramsAllocs)
		defer c.End()
		for c.Loop() {
			param.Name = ""
			m = naming.Params(m)
		}
		assert.Equal(b, param.Name, firstParam, "Params names the parameter by its position")
	})
}

// method returns a method with the receiver, the named results and one
// parameter for each name of params, where an empty name is a parameter
// without a name.
func method(receiver *emit.Param, results []string, params ...string) *emit.Method {
	m := &emit.Method{Name: "Put", Receiver: receiver}
	for _, name := range params {
		m.Params.Append(&emit.Param{Name: name})
	}
	for _, name := range results {
		m.Returns.Append(&emit.Return{Name: name})
	}
	return m
}
