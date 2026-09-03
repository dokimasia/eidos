// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The contract's registry and its refusing default are what a
// composition composes rules through.
func TestContract(t *testing.T) {
	t.Parallel()

	t.Run("Registry", func(t *testing.T) {
		t.Parallel()

		t.Run("holds one value per language", func(t *testing.T) {
			t.Parallel()

			r := rules.NewRegistry()
			assert.NoError(t, r.Register(rulestest.Scripted()), "a language registers")
			held, registered := r.For(rulestest.Scripted().Lang())
			assert.True(t, registered, "and is found")
			assert.False(t, rules.IsAbsent(held), "as itself")
			assert.Equal(t, r.Languages(), []symbol.Lang{rulestest.Scripted().Lang()}, "listed")
		})

		t.Run("refuses a nil value, the zero language and a second value", func(t *testing.T) {
			t.Parallel()

			r := rules.NewRegistry()
			assert.HasError(t, r.Register(nil), "a nil value returns for no language")
			assert.HasError(t, r.Register(rules.Absent("")), "the zero language returns for none")
			assert.NoError(t, r.Register(rulestest.Scripted()), "the first registers")
			err := r.Register(rulestest.Scripted())
			assert.HasError(t, err, "the second is refused")
			assert.HasPrefix(t, err.Error(), "rules: ", "under the package prefix")
		})

		t.Run("returns the absent value for an unregistered language", func(t *testing.T) {
			t.Parallel()

			r := rules.NewRegistry()
			held, registered := r.For("mars")
			assert.False(t, registered, "nothing registered for it")
			assert.True(t, rules.IsAbsent(held), "so the refusing value stands in")
			assert.Equal(t, held.Lang(), symbol.Lang("mars"), "under the language asked for")
		})
	})

	t.Run("Absent", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses everything as a value", func(t *testing.T) {
			t.Parallel()

			a := rules.Absent("mars")
			assert.True(t, rules.IsAbsent(a), "is the absent value")
			assert.Equal(t, a.Lang(), symbol.Lang("mars"), "for its language")
			assert.Equal(t, a.Members(), rules.MemberPolicy{}, "contributes nothing")
			assert.Equal(t, a.ParamRole(&node.Param{}, rules.View{}), rules.ParamInput, "every parameter is input")
			roles, model := a.ReturnRoles([]*node.Return{{}, {}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue}, "every return a value")
			assert.Equal(t, model, rules.ErrorsNone, "under no error model")
			assert.Equal(t, a.Builtin(builtin("x"), rules.View{}).Form, symbol.FormOpaque, "every builtin opaque")
			_, err := a.Resolve(rules.Scope{}, "x", directive.ResolveCallableInScope, rules.View{})
			assert.HasError(t, err, "nothing resolves")
			sample, alternate := a.SamplesOf(builtin("x"), "x", rules.View{})
			assert.Equal(t, sample.Refusal, rules.RefusedNoRules, "a sample refuses for want of rules")
			assert.Equal(t, alternate.Refusal, rules.RefusedNoRules, "both halves")
			_, held := a.ZeroValue(builtin("x"), rules.View{})
			assert.False(t, held, "no zero")
			_, held = a.LiteralFor(nil, builtin("x"), "1", rules.View{})
			assert.False(t, held, "no literal")
			assert.Equal(t, a.TypeName("Builder", "User"), "UserBuilder", "and a plain join")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("spells every enum and numbers the rest", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.ParamContext.String(), "context", "a parameter role")
			assert.Equal(t, rules.ParamRole(9).String(), "9", "and an undeclared one")
			assert.Equal(t, rules.ReturnError.String(), "error", "a return role")
			assert.Equal(t, rules.ReturnRole(9).String(), "9", "and an undeclared one")
			assert.Equal(t, rules.ErrorsResultType.String(), "result-type", "an error model")
			assert.Equal(t, rules.ErrorModel(9).String(), "9", "and an undeclared one")
			assert.Equal(t, rules.ContributesImplements.String(), "implements", "a contribution")
			assert.Equal(t, rules.Contribution(9).String(), "9", "and an undeclared one")
			assert.Equal(t, rules.ShadowLinearise.String(), "linearise", "a shadowing rule")
			assert.Equal(t, rules.Shadowing(9).String(), "9", "and an undeclared one")
		})
	})
}
