// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
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

// The scripted language's rules are the kernel's own proving
// ground, so each decision it returns is pinned.
func TestScripted(t *testing.T) {
	t.Parallel()

	view := func(tb assert.TB) rules.View {
		tb.Helper()
		_, f := setup(tb)
		reads := store.NewReadSet()
		reader, err := f.Graph.Reader(reads, nil)
		assert.NoError(tb, err, "the graph hands out a reader")
		return rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys}
	}

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

		t.Run("call everything input and value under no error model", func(t *testing.T) {
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

		t.Run("finds a bare name in the subject's package", func(t *testing.T) {
			t.Parallel()

			v := view(t)
			s := rulestest.Scripted()
			scope := rules.Scope{Subject: symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/store"}}
			decl, err := s.Resolve(scope, "Row", directive.ResolveCallableInScope, v)
			assert.NoError(t, err, "the struct resolves")
			assert.Equal(t, decl.Kind(), symbol.KindStruct, "as itself")
			_, err = s.Resolve(scope, "Nope", directive.ResolveCallableInScope, v)
			assert.HasError(t, err, "and a stranger name errors")
			assert.HasPrefix(t, err.Error(), "rulestest: ", "under the package prefix")
		})
	})

	t.Run("values", func(t *testing.T) {
		t.Parallel()

		t.Run("derive for the builtins and refuse the rest", func(t *testing.T) {
			t.Parallel()

			s := rulestest.Scripted()
			sample, alternate := s.SamplesOf(&node.TypeRef{Spelling: "string"}, "name", rules.View{})
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralString, "test-name"), "a string carries the hint")
			assert.Equal(t, alternate.Value, emit.Literal(emit.LiteralString, "other-name"), "twice")
			sample, _ = s.SamplesOf(&node.TypeRef{Spelling: "bool"}, "", rules.View{})
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralBool, "true"), "a bool")
			sample, _ = s.SamplesOf(nil, "", rules.View{})
			assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, "nothing has no value")
			row := &node.TypeRef{Spelling: "Row", Target: symbol.Identity{Name: "Row"}}
			sample, _ = s.SamplesOf(row, "", rules.View{})
			assert.Equal(t, sample.Refusal, rules.RefusedUnresolved, "and a named type refuses as unresolved")

			zero, held := s.ZeroValue(&node.TypeRef{Spelling: "bool"}, rules.View{})
			assert.True(t, held && zero.Text == "false", "a bool's zero")
			_, held = s.ZeroValue(&node.TypeRef{Spelling: "Row"}, rules.View{})
			assert.False(t, held, "a named type has none")
			_, held = s.ZeroValue(nil, rules.View{})
			assert.False(t, held, "nor does nothing")

			lit, held := s.LiteralFor(nil, &node.TypeRef{Spelling: "int"}, "12", rules.View{})
			assert.True(t, held && lit.Text == "12", "an integer parses")
			_, held = s.LiteralFor(nil, &node.TypeRef{Spelling: "int"}, "x", rules.View{})
			assert.False(t, held, "and text that is no integer refuses")
			_, held = s.LiteralFor(nil, &node.TypeRef{Spelling: "bool"}, "true", rules.View{})
			assert.False(t, held, "a bool takes no literal from text")
			_, held = s.LiteralFor(nil, nil, "x", rules.View{})
			assert.False(t, held, "nor does nothing")
			assert.Equal(t, s.TypeName("Builder", "Row"), "RowBuilder", "the join concatenates")
		})
	})

	t.Run("generics", func(t *testing.T) {
		t.Parallel()

		t.Run("derive an int witness and substitute by position", func(t *testing.T) {
			t.Parallel()

			s := rulestest.Scripted().(rules.GenericsRules)
			ref, held := s.Derive(&node.TypeParam{Name: "T"}, rules.View{})
			assert.True(t, held && ref.Spelling == "int", "every parameter witnesses as int")
			assert.False(t, s.Reified(), "and nothing is kept at runtime")

			params := []*node.TypeParam{{Name: "T"}}
			args := []*node.TypeRef{{Spelling: "string"}}
			list := &node.TypeRef{Spelling: "[]T", Form: symbol.FormList, Elems: []*node.TypeRef{{Spelling: "T"}}}
			got := s.Substitute(list, params, args)
			assert.Equal(t, got.Elems[0].Spelling, "string", "the parameter inside a form rewrites")
			assert.True(t, got != list, "on a copy")
			assert.Equal(t, s.Substitute(&node.TypeRef{Spelling: "int"}, params, args).Spelling, "int",
				"a reference naming no parameter stands")
			assert.True(t, s.Substitute(nil, params, args) == nil, "and nothing stays nothing")
			same := &node.TypeRef{Spelling: "T"}
			assert.True(t, s.Substitute(same, params, nil) == same, "a list mismatch rewrites nothing")
		})
	})
}
