// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The names of the type parameters of the fixtures of Mentioned, and the
// spellings of the types that refer to them.
const (
	mentionKey       = "K"
	mentionValue     = "V"
	mentionExtra     = "X"
	mentionList      = "[]V"
	mentionGeneric   = "Map"
	mentionQualified = "pkg.V"
)

// mentionedAllocs is the ceiling of a call of Mentioned that returns a
// parameter: the list that it returns.
const mentionedAllocs = 1

// TestMentioned pins the type parameters that references mention, the
// parameters that the bounds of the result add, and the order of the
// result.
func TestMentioned(t *testing.T) {
	t.Parallel()

	t.Run("Mentioned", func(t *testing.T) {
		t.Parallel()

		key, value, extra := &emit.TypeParam{Name: mentionKey}, &emit.TypeParam{Name: mentionValue},
			&emit.TypeParam{Name: mentionExtra}
		boundKey := &emit.TypeParam{Name: mentionKey, Bounds: []*emit.TypeRef{{Spelling: mentionValue}}}
		defaultKey := &emit.TypeParam{Name: mentionKey, Default: &emit.TypeRef{Spelling: mentionExtra}}
		typedKey := &emit.TypeParam{Name: mentionKey, Const: true, Type: &emit.TypeRef{Spelling: mentionExtra}}
		chainValue := &emit.TypeParam{Name: mentionValue, Bounds: []*emit.TypeRef{{Spelling: mentionKey}}}
		chainExtra := &emit.TypeParam{Name: mentionExtra, Bounds: []*emit.TypeRef{{Spelling: mentionValue}}}
		tests := []struct {
			name string
			give []*emit.TypeRef
			tps  []*emit.TypeParam
			want []*emit.TypeParam
		}{
			{
				name: "returns the parameter that a named reference spells",
				give: []*emit.TypeRef{{Spelling: mentionValue}},
				tps:  []*emit.TypeParam{key, value},
				want: []*emit.TypeParam{value},
			},
			{
				name: "returns the parameter that a type argument mentions",
				give: []*emit.TypeRef{{Spelling: mentionGeneric, Args: []*emit.TypeRef{{Spelling: mentionKey}}}},
				tps:  []*emit.TypeParam{key, value},
				want: []*emit.TypeParam{key},
			},
			{
				name: "returns the parameter that a child of a structural reference mentions",
				give: []*emit.TypeRef{{
					Spelling: mentionList, Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: mentionValue}},
				}},
				tps:  []*emit.TypeParam{key, value},
				want: []*emit.TypeParam{value},
			},
			{
				name: "returns the parameters in the order of tps",
				give: []*emit.TypeRef{{Spelling: mentionValue}, {Spelling: mentionKey}},
				tps:  []*emit.TypeParam{key, value},
				want: []*emit.TypeParam{key, value},
			},
			{
				name: "returns the parameter that the bound of a returned parameter mentions",
				give: []*emit.TypeRef{{Spelling: mentionKey}},
				tps:  []*emit.TypeParam{boundKey, value},
				want: []*emit.TypeParam{boundKey, value},
			},
			{
				name: "returns the parameter that the default of a returned parameter mentions",
				give: []*emit.TypeRef{{Spelling: mentionKey}},
				tps:  []*emit.TypeParam{defaultKey, extra},
				want: []*emit.TypeParam{defaultKey, extra},
			},
			{
				name: "returns the parameter that the value type of a returned parameter mentions",
				give: []*emit.TypeRef{{Spelling: mentionKey}},
				tps:  []*emit.TypeParam{typedKey, extra},
				want: []*emit.TypeParam{typedKey, extra},
			},
			{
				name: "returns the parameter that a chain of bounds mentions",
				give: []*emit.TypeRef{{Spelling: mentionExtra}},
				tps:  []*emit.TypeParam{key, chainValue, chainExtra},
				want: []*emit.TypeParam{key, chainValue, chainExtra},
			},
			{
				name: "returns nil where only an unmentioned parameter has a bound that mentions another",
				give: []*emit.TypeRef{{Spelling: mentionGeneric}},
				tps:  []*emit.TypeParam{key, chainValue},
				want: nil,
			},
			{
				name: "returns nil for a qualified name that ends in the name of a parameter",
				give: []*emit.TypeRef{{Spelling: mentionQualified}},
				tps:  []*emit.TypeParam{value},
				want: nil,
			},
			{
				name: "returns nil for a nil reference",
				give: []*emit.TypeRef{nil},
				tps:  []*emit.TypeParam{value},
				want: nil,
			},
			{
				name: "returns nil without a reference",
				give: nil,
				tps:  []*emit.TypeParam{value},
				want: nil,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, rules.Mentioned(tt.tps, tt.give...), tt.want,
					"Mentioned returns the type parameters that the references use")
			})
		}

		t.Run("returns the type parameters of tps themselves", func(t *testing.T) {
			t.Parallel()

			got := rules.Mentioned([]*emit.TypeParam{key, value}, &emit.TypeRef{Spelling: mentionValue})
			assert.Length(t, got, 1, "one parameter is mentioned")
			assert.Equal(t, got[0], value, "the result contains the parameter of tps", assert.ByIdentity())
		})
	})
}

// TestMentionedAllocs checks the allocation ceiling of Mentioned in a run
// without benchmarks. The list of the result is the one allocation.
func TestMentionedAllocs(t *testing.T) {
	tps, refs := mentionedFixture()
	var got []*emit.TypeParam
	assert.MaxAllocs(t, func() { got = rules.Mentioned(tps, refs...) }, mentionedAllocs,
		"Mentioned allocates the list that it returns")
	assert.Length(t, got, 2, "Mentioned returns the mentioned parameter and the parameter of its bound")
}

// BenchmarkMentioned measures Mentioned over a reference that mentions a
// parameter whose bound mentions another.
func BenchmarkMentioned(b *testing.B) {
	b.Run("Mentioned", func(b *testing.B) {
		tps, refs := mentionedFixture()
		c := bench.Start(b).MaxAllocs(mentionedAllocs)
		defer c.End()
		var got []*emit.TypeParam
		for c.Loop() {
			got = rules.Mentioned(tps, refs...)
		}
		assert.Length(b, got, 2, "Mentioned returns the mentioned parameter and the parameter of its bound")
	})
}

// mentionedFixture returns three type parameters and one reference to a
// list of the second, whose bound mentions the first.
func mentionedFixture() ([]*emit.TypeParam, []*emit.TypeRef) {
	tps := []*emit.TypeParam{
		{Name: mentionKey},
		{Name: mentionValue, Bounds: []*emit.TypeRef{{Spelling: mentionKey}}},
		{Name: mentionExtra},
	}
	refs := []*emit.TypeRef{{
		Spelling: mentionList, Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: mentionValue}},
	}}
	return tps, refs
}
