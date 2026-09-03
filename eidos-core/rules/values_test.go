// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// Authored values beat derived ones by contract, and every refusal
// carries its reason, which is what a check generator stands on.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("SamplesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the carrier's authored values before deriving", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.ID(svcPath, rowName, symbol.KindField)
			subject.Owner, subject.Name = rowName, "name"
			assert.NoError(t, meta.Stamp(facts, b.View().Kernel.Sample, `"us-east"`, meta.Claim{Subject: subject}),
				"the author states one value")
			sample, alternate := b.SamplesOf(subject, builtin(strSpelling), "name")
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralRaw, `"us-east"`),
				"the authored half arrives as raw text")
			assert.Equal(t, alternate.Value, emit.Literal(emit.LiteralString, "other-name"),
				"and the other half derives independently")
		})

		t.Run("reads the type's own authored values next", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			typ := coretest.ID(svcPath, rowName, symbol.KindStruct)
			assert.NoError(t, meta.Stamp(facts, b.View().Kernel.Sample, "Row{}", meta.Claim{Subject: typ}),
				"the type states a value")
			assert.NoError(t, meta.Stamp(facts, b.View().Kernel.Alternate, "Row{N: 1}", meta.Claim{Subject: typ}),
				"and its alternate")
			sample, alternate := b.SamplesOf(symbol.Identity{}, named(svcPath, rowName, symbol.KindStruct), "row")
			assert.True(t, sample.OK() && alternate.OK(), "both halves come from the type")
			assert.Equal(t, sample.Value.Text, "Row{}", "the first")
			assert.Equal(t, alternate.Value.Text, "Row{N: 1}", "and the second")
		})

		t.Run("derives where nothing is authored and refuses with a reason", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			sample, alternate := b.SamplesOf(symbol.Identity{}, builtin(intSpelling), "n")
			assert.Equal(t, sample.Value.Text, "42", "the language derives the first")
			assert.Equal(t, alternate.Value.Text, "7", "and the second")
			refused, _ := b.SamplesOf(symbol.Identity{}, named(svcPath, rowName, symbol.KindStruct), "row")
			assert.False(t, refused.OK(), "a type the language cannot value refuses")
			assert.Equal(t, refused.Refusal, rules.RefusedUnresolved, "with its reason")
			assert.Equal(t, refused.Refusal.String(), "unresolved", "which spells")
			assert.Equal(t, rules.Refusal(9).String(), "9", "and an undeclared one numbers")
		})

		t.Run("refuses on the zero view", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(scripted(), rules.View{}, nil)
			sample, alternate := b.SamplesOf(symbol.Identity{}, builtin(intSpelling), "n")
			assert.Equal(t, sample.Refusal, rules.RefusedNoView, "no view, no read")
			assert.Equal(t, alternate.Refusal, rules.RefusedNoView, "on both halves")
			assert.False(t, rules.Refused(rules.RefusedDepth).OK(), "a refused sample is not OK")
			assert.True(t, rules.Of(emit.Literal(emit.LiteralInt, "1")).OK(), "and a valued one is")
		})
	})

	t.Run("Witnesses", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the authored witness first and derives the rest", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			params := []*node.TypeParam{
				{ID: coretest.ID(svcPath, "T", symbol.KindTypeParam), Name: "T"},
				{ID: coretest.ID(svcPath, "U", symbol.KindTypeParam), Name: "U"},
			}
			want := coretest.ID(svcPath, rowName, symbol.KindStruct)
			assert.NoError(t, meta.Stamp(facts, b.View().Kernel.Witness, want, meta.Claim{Subject: params[0].ID}),
				"the author names one witness")
			got := b.Witnesses(params)
			assert.Length(t, got, 2, "the list is whole")
			assert.Equal(t, got[0].Target, want, "the authored one carries its target")
			assert.Equal(t, got[0].Spelling, rowName, "and its bare name")
			assert.Equal(t, got[1].Spelling, intSpelling, "the derived one is the language's")
		})

		t.Run("returns nothing where any parameter has no witness", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, hierarchy())
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(nongeneric{scripted()}, v, nil)
			params := []*node.TypeParam{{ID: coretest.ID(svcPath, "T", symbol.KindTypeParam), Name: "T"}}
			assert.Length(t, b.Witnesses(params), 0, "a language without the capability derives none")
			assert.Length(t, b.Witnesses(nil), 0, "and no parameters take none")
			assert.Length(t, b.Witnesses([]*node.TypeParam{nil}), 0, "nor a nil entry")
			zero := rules.NewBound(scripted(), rules.View{}, nil)
			assert.Length(t, zero.Witnesses(params), 0, "nor the zero view")
		})
	})

	t.Run("EmitRef", func(t *testing.T) {
		t.Parallel()

		t.Run("restates a reference with its structure and arguments", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{
				Spelling: "map[string]List[int]", Form: symbol.FormMap,
				Elems: []*node.TypeRef{
					builtin(strSpelling),
					{
						Spelling: "List", Target: coretest.ID(svcPath, "List", symbol.KindStruct),
						Args: []*node.TypeRef{builtin(intSpelling)},
					},
				},
			}
			got := rules.EmitRef(ref)
			assert.Equal(t, got.Form, symbol.FormMap, "the form carries")
			assert.Length(t, got.Elems, 2, "with the children")
			assert.Equal(t, got.Elems[1].Target, ref.Elems[1].Target, "targets included")
			assert.Length(t, got.Elems[1].Args, 1, "and the arguments")
			assert.Equal(t, got.Elems[1].Args[0].Spelling, intSpelling, "spelled as read")
			assert.True(t, rules.EmitRef(nil) == nil, "nil stays nil")
		})
	})
}
