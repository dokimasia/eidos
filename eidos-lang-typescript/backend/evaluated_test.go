// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names that the decorated class of the cases evaluates: the module of
// its superclass, and the decorators of the class, of its field and of its
// method.
const (
	baseModule      = "ns"
	sealedDecorator = "sealed"
	watchDecorator  = "watch"
	logModule       = "log"
)

// TestEvaluated pins the names that a TypeScript module evaluates when it
// runs a declaration.
func TestEvaluated(t *testing.T) {
	t.Parallel()

	t.Run("Evaluated", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Symbol
			want []string
		}{
			{
				name: "yields the superclass and the decorators of a class",
				give: decoratedClass(),
				want: []string{baseModule, sealedDecorator, watchDecorator, logModule},
			},
			{
				name: "yields nothing for an interface",
				give: &emit.Interface{Name: rowName, Extends: []*emit.TypeRef{ref(rowName)}},
			},
			{name: "yields nothing for a nil class", give: (*emit.Struct)(nil)},
			{
				name: "yields nothing for a nil superclass",
				give: &emit.Struct{Name: rowName, Extends: []*emit.TypeRef{nil}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got []string
				backend.Evaluated(tt.give, func(name string) { got = append(got, name) })
				assert.Equal(t, got, tt.want, "the names in the order of the class")
			})
		}
	})
}

// TestEvaluatedAllocs checks in the ordinary run, which runs no benchmark,
// that Evaluated allocates nothing.
func TestEvaluatedAllocs(t *testing.T) {
	class := decoratedClass()
	names := 0
	yield := func(string) { names++ }
	assert.MaxAllocs(t, func() { backend.Evaluated(class, yield) }, 0, "Evaluated allocates nothing")
	names = 0
	backend.Evaluated(class, yield)
	assert.Equal(t, names, 4, "Evaluated yields the four names of the class")
}

// BenchmarkEvaluated measures the names that the render reads from every
// declaration of a file.
func BenchmarkEvaluated(b *testing.B) {
	class := decoratedClass()
	names := 0
	yield := func(string) { names++ }
	c := bench.Start(b).MaxAllocs(0)
	defer c.End()
	for c.Loop() {
		backend.Evaluated(class, yield)
	}
	assert.NotEqual(b, names, 0, "Evaluated yields the names of the class")
}

// decoratedClass returns a class that extends a class of a namespace, with
// a decorator, a decorated field and a decorated method, beside a nil field
// and a nil method.
func decoratedClass() *emit.Struct {
	s := &emit.Struct{
		Name:        rowName,
		Extends:     []*emit.TypeRef{ref(baseModule + ".Base")},
		Annotations: symbol.Annotations{{Name: sealedDecorator}},
	}
	s.Fields.Append(&emit.Field{Name: "size", Annotations: symbol.Annotations{{Name: watchDecorator}}}, nil)
	s.Methods.Append(&emit.Method{Name: "fill", Annotations: symbol.Annotations{{Name: logModule + ".calls"}}}, nil)
	return s
}
