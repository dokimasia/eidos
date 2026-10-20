// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names that the declarations of the cases evaluate: the module of a
// base, a base and a mixin of the file, a contract, and the decorators of a
// class and of a callable.
const (
	modelsModule = "models"
	baseName     = "Base"
	mixinName    = "Mixin"
	sizedName    = "Sized"
	sealedName   = "sealed"
	tracedName   = "traced"
)

// TestEvaluated pins the names that a Python module evaluates when it runs
// a declaration.
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
				name: "yields the bases of a class before its decorators",
				give: decoratedClass(),
				want: []string{modelsModule, sizedName, mixinName, sealedName, tracedName},
			},
			{
				name: "yields the bases of a nested class",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Struct{
					Name: "Inner", Extends: []*emit.TypeRef{{Spelling: baseName}},
				})},
				want: []string{baseName},
			},
			{
				name: "yields the bases of a Protocol before its decorators",
				give: &emit.Interface{
					Name: "Store", Extends: []*emit.TypeRef{{Spelling: baseName}},
					Annotations: symbol.Annotations{{Name: sealedName}},
				},
				want: []string{baseName, sealedName},
			},
			{
				name: "yields the bases of the bases marker of an enum",
				give: &emit.Enum{
					Name: phaseName, Underlying: &emit.TypeRef{Spelling: baseName},
					Annotations: symbol.Annotations{
						{Name: string(python.BasesMarker), Args: []string{mixinName, "enum.IntFlag"}},
					},
				},
				want: []string{mixinName, "enum"},
			},
			{
				name: "yields the underlying type of an enum without the bases marker",
				give: &emit.Enum{Name: phaseName, Underlying: &emit.TypeRef{Spelling: mixinName}},
				want: []string{mixinName},
			},
			{
				name: "yields the decorators of a method of an enum",
				give: &emit.Enum{Name: phaseName, Methods: emit.SlotOf(&emit.Method{
					Name: "label", Annotations: symbol.Annotations{{Name: tracedName}},
				})},
				want: []string{tracedName},
			},
			{
				name: "yields the first name of each decorator of a function",
				give: &emit.Function{Name: "load", Annotations: symbol.Annotations{
					{Name: tracedName}, {Name: "functools.cache"},
				}},
				want: []string{tracedName, "functools"},
			},
			{
				name: "yields nothing for the decorator of a method that names its property",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "size", Annotations: symbol.Annotations{{Name: "size.setter"}},
				})},
			},
			{
				name: "yields nothing for a marker",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{keywordsMarker}},
			},
			{name: "yields nothing for a constant", give: &emit.Constant{Name: "LIMIT", Value: baseName + "()"}},
			{name: "yields nothing for a nil function", give: (*emit.Function)(nil)},
			{name: "yields nothing for a nil class", give: (*emit.Struct)(nil)},
			{name: "yields nothing for a nil Protocol", give: (*emit.Interface)(nil)},
			{name: "yields nothing for a nil enum", give: (*emit.Enum)(nil)},
			{
				name: "yields nothing for a nil base",
				give: &emit.Struct{Name: rowName, Extends: []*emit.TypeRef{nil}},
			},
			{
				name: "yields nothing for an embed without a type",
				give: &emit.Struct{Name: rowName, Embeds: emit.SlotOf(&emit.Embed{})},
			},
			{
				name: "yields nothing for a nil method",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf[*emit.Method](nil)},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got []string
				backend.Evaluated(tt.give, func(name string) { got = append(got, name) })
				assert.Equal(t, got, tt.want, "the names in the order of the declaration")
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
	assert.Equal(t, names, 5, "Evaluated yields the five names of the class")
}

// BenchmarkEvaluated measures the names that the render reads from every
// declaration of a module.
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

// decoratedClass returns a class that extends a class of another module,
// implements a contract and embeds a mixin, with a decorator, a marker and a
// decorated method.
func decoratedClass() *emit.Struct {
	return &emit.Struct{
		Name:        rowName,
		Extends:     []*emit.TypeRef{{Spelling: modelsModule + "." + baseName}},
		Implements:  []*emit.TypeRef{{Spelling: sizedName}},
		Embeds:      emit.SlotOf(&emit.Embed{Ref: &emit.TypeRef{Spelling: mixinName}}),
		Annotations: symbol.Annotations{{Name: sealedName}, keywordsMarker},
		Methods:     emit.SlotOf(&emit.Method{Name: "load", Annotations: symbol.Annotations{{Name: tracedName}}}),
	}
}
