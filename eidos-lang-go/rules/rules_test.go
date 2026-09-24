// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("speaks Go and walks embeds under promotion", func(t *testing.T) {
		t.Parallel()

		r := gorules.New()
		assert.Equal(t, r.Lang(), golang.Lang, "the language")
		policy := r.Members()
		assert.Equal(
			t,
			policy.Contributes,
			[]rules.Contribution{rules.ContributesEmbeds},
			"embeds contribute",
		)
		assert.Equal(t, policy.Shadowing, rules.ShadowPromote, "and promote")
		assert.Equal(t, policy.Depth, 0, "to the kernel's default depth")
		assert.True(t, policy.EmbedsAreFields, "and an embedded field is a member")
	})

	t.Run("records an embedded field beside the members it promotes", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		derived := f.decl(
			t,
			symbol.Identity{Lang: golang.Lang, Package: fxPath, Name: "Derived", Kind: symbol.KindStruct},
		)
		set, is := f.bound().MembersOf(derived)
		assert.True(t, is, "a struct walks")
		var names []string
		for _, m := range set.Members {
			switch d := m.Symbol.(type) {
			case *node.Field:
				names = append(names, d.Name)
			case *node.Embed:
				names = append(names, d.ID.Name)
			}
		}
		assert.Equal(t, names, []string{"Name", "Base", "Kind"},
			"Derived's field, its embedded field Base, and the Kind that Base promotes")
	})

	t.Run("classifies a context parameter and every other as input", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		load, is := f.decl(t, symbol.Identity{
			Lang: golang.Lang, Package: fxPath, Name: "Load", Kind: symbol.KindFunction,
			Disc: "context.Context,int",
		}).(*node.Function)
		assert.True(t, is, "Load is a function")
		c, held := f.bound().CallableOf(load)
		assert.True(t, held, "and callable")
		assert.Equal(t, c.Params[0].Role, rules.ParamContext, "the context is the context")
		assert.Equal(t, c.Params[1].Role, rules.ParamInput, "the id is input")
		assert.Equal(t, c.Returns[1].Role, rules.ReturnError, "the last error return is the error")
		assert.Equal(t, c.Errors, rules.ErrorsLastReturn, "under the last-return model")
		assert.False(t, c.Async, "Go is always synchronous")
	})

	t.Run("classifies an ok flag and an iterator", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		find, _ := f.decl(t, symbol.Identity{
			Lang: golang.Lang, Package: fxPath, Name: "Find", Kind: symbol.KindFunction, Disc: "int",
		}).(*node.Function)
		c, _ := f.bound().CallableOf(find)
		assert.Equal(
			t,
			c.Returns[1].Role,
			rules.ReturnOkBool,
			"the second of two returns, a bool, is ok",
		)
		assert.Equal(t, c.Errors, rules.ErrorsNone, "no error return, no model")
		all, _ := f.decl(t, id(fxPath, "All", symbol.KindFunction)).(*node.Function)
		c, _ = f.bound().CallableOf(all)
		assert.Equal(t, c.Returns[0].Role, rules.ReturnStream, "an iter.Seq return is a stream")
		roles, _ := gorules.New().ReturnRoles([]*node.Return{nil, {Type: nil}}, rules.View{})
		assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue},
			"a return without a type is a value")
	})

	t.Run("joins a type name keeping the base's export", func(t *testing.T) {
		t.Parallel()

		r := gorules.New()
		assert.Equal(t, r.TypeName("check", "Row"), "CheckRow", "an exported base stays exported")
		assert.Equal(
			t,
			r.TypeName("check", "row"),
			"checkRow",
			"an unexported base stays unexported",
		)
		assert.Equal(
			t,
			r.TypeName("mock", "HTTPClient"),
			"MockHTTPClient",
			"and an initialism keeps its shape",
		)
	})
}
