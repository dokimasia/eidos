// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// TestScripted pins each decision that the rules of the scripted language
// return. The kernel tests its walks against these rules.
func TestScripted(t *testing.T) {
	t.Parallel()

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("promotes over embeds", func(t *testing.T) {
			t.Parallel()

			s := rulestest.Scripted()
			assert.Equal(t, s.Lang(), frontendtest.ScriptedLang, "for the scripted language")
			assert.Equal(t, s.Members(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesEmbeds}, Shadowing: rules.ShadowPromote,
			}, "the policy is Go's")
		})
	})

	t.Run("roles", func(t *testing.T) {
		t.Parallel()

		t.Run("return input and value roles without an error model", func(t *testing.T) {
			t.Parallel()

			s := rulestest.Scripted()
			assert.Equal(t, s.ParamRole(&node.Param{}, rules.View{}), rules.ParamInput, "a parameter")
			roles, model := s.ReturnRoles([]*node.Return{{}, {}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue}, "the returns")
			assert.Equal(t, model, rules.ErrorsNone, "and the model")
		})
	})

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies the three builtins and calls the rest opaque", func(t *testing.T) {
			t.Parallel()

			s := rulestest.Scripted()
			assert.Equal(t, s.Builtin(&node.TypeRef{Spelling: "int"}, rules.View{}).Form, symbol.FormScalar, "int")
			assert.Equal(t, s.Builtin(&node.TypeRef{Spelling: "string"}, rules.View{}).Form, symbol.FormText, "string")
			assert.Equal(t, s.Builtin(&node.TypeRef{Spelling: "bool"}, rules.View{}).Form, symbol.FormBool, "bool")
			other := s.Builtin(&node.TypeRef{Spelling: "Other"}, rules.View{})
			assert.Equal(t, other.Form, symbol.FormOpaque, "the rest")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("finds a bare name in the package of the subject", func(t *testing.T) {
			t.Parallel()

			v, _ := recordingView(t)
			s := rulestest.Scripted()
			scope := rules.Scope{Subject: symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/store"}}
			decl, err := s.Resolve(scope, "Row", directive.ResolveCallableInScope, v)
			assert.NoError(t, err, "the struct resolves")
			assert.Equal(t, decl.Kind(), symbol.KindStruct, "as itself")
			_, err = s.Resolve(scope, "Nope", directive.ResolveCallableInScope, v)
			assert.HasError(t, err, "a name that the package does not declare returns an error")
			assert.HasPrefix(t, err.Error(), "rulestest: ", "under the package prefix")
		})
	})

	t.Run("values", func(t *testing.T) {
		t.Parallel()

		t.Run("derive for the builtins and refuse the rest", func(t *testing.T) {
			t.Parallel()

			s := rulestest.Scripted()
			sample, alternate := s.DeriveSamples(&node.TypeRef{Spelling: "string"}, "name", rules.View{})
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralString, "test-name"),
				"a string sample contains the hint")
			assert.Equal(t, alternate.Value, emit.Literal(emit.LiteralString, "other-name"), "twice")
			sample, _ = s.DeriveSamples(&node.TypeRef{Spelling: "bool"}, "", rules.View{})
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralBool, "true"), "a bool")
			sample, _ = s.DeriveSamples(nil, "", rules.View{})
			assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, "nothing has no value")
			row := &node.TypeRef{Spelling: "Row", Target: symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: "svc/store", Name: "Row", Kind: symbol.KindStruct,
			}}
			sample, _ = s.DeriveSamples(row, "", rules.View{})
			assert.Equal(t, sample.Refusal, rules.RefusedUnresolved,
				"a named type that the view does not contain refuses as unresolved")
			v, reads := recordingView(t)
			sample, _ = s.DeriveSamples(row, "", v)
			assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral,
				"a named type that the view contains refuses with no literal")
			assert.Equal(t, slices.Collect(reads.Identities()), []symbol.Identity{row.Target},
				"after reading the declaration through the view")

			zero, held := s.ZeroValue(&node.TypeRef{Spelling: "bool"}, rules.View{})
			assert.True(t, held, "a bool has a zero")
			assert.Equal(t, zero.Text, "false", "spelt false")
			_, held = s.ZeroValue(&node.TypeRef{Spelling: "Row"}, rules.View{})
			assert.False(t, held, "a named type has none")
			_, held = s.ZeroValue(nil, rules.View{})
			assert.False(t, held, "nor does nothing")

			lit, held := s.LiteralFor(rules.Scope{}, &node.TypeRef{Spelling: "int"}, "12", rules.View{})
			assert.True(t, held, "an integer parses")
			assert.Equal(t, lit.Text, "12", "to its own text")
			_, held = s.LiteralFor(rules.Scope{}, &node.TypeRef{Spelling: "int"}, "x", rules.View{})
			assert.False(t, held, "text that is not an integer has no literal")
			_, held = s.LiteralFor(rules.Scope{}, &node.TypeRef{Spelling: "bool"}, "true", rules.View{})
			assert.False(t, held, "a bool takes no literal from text")
			_, held = s.LiteralFor(rules.Scope{}, nil, "x", rules.View{})
			assert.False(t, held, "nor does nothing")
		})
	})

	t.Run("generics", func(t *testing.T) {
		t.Parallel()

		t.Run("derive an int witness for every parameter", func(t *testing.T) {
			t.Parallel()

			s := rulestest.Scripted().(rules.GenericsRules)
			ref, held := s.Derive(&node.TypeParam{Name: "T"}, rules.View{})
			assert.True(t, held, "every parameter has a witness")
			assert.Equal(t, ref.Spelling, "int", "the witness is an int")
			assert.False(t, s.Reified(), "the language keeps no type argument at runtime")
		})
	})
}

// recordingView returns a view over the scripted tree with a fresh read
// set, and returns the set, so that a case can check the reads.
func recordingView(tb assert.TB) (rules.View, *store.ReadSet) {
	tb.Helper()

	_, f := setup(tb)
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the graph returns a reader")
	return rules.NewView(reader, f.Facts, reads, f.Keys), reads
}
