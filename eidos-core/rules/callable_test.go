// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The names of the fixture callables.
const (
	fetchName = "Fetch"
	getName   = "Get"
)

// roles overrides the scripted language's classification the way a
// language with a context type and a last-return error model does.
type roles struct {
	rules.SourceRules
}

// ParamRole calls a Context parameter a context.
func (roles) ParamRole(p *node.Param, _ rules.View) rules.ParamRole {
	if p.Type != nil && p.Type.Spelling == "Context" {
		return rules.ParamContext
	}
	return rules.ParamInput
}

// ReturnRoles calls a trailing error the error return.
func (roles) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	out := make([]rules.ReturnRole, len(rs))
	if n := len(rs); n > 0 && rs[n-1].Type != nil && rs[n-1].Type.Spelling == "error" {
		out[n-1] = rules.ReturnError
		return out, rules.ErrorsLastReturn
	}
	return out, rules.ErrorsNone
}

// short returns fewer roles than returns, which the mapping pads.
type short struct {
	rules.SourceRules
}

// ReturnRoles returns one role however many returns there are.
func (short) ReturnRoles([]*node.Return, rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return []rules.ReturnRole{rules.ReturnValue}, rules.ErrorsNone
}

// The mapping is the kernel's and the roles are the language's, so
// the shape of the view and the split of the work are pinned here.
func TestCallable(t *testing.T) {
	t.Parallel()

	t.Run("CallableOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the roles the language classifies", func(t *testing.T) {
			t.Parallel()

			c, is := rules.NewBound(roles{scripted()}, viewOnly(t), nil).CallableOf(fetch())
			assert.True(t, is, "a function maps")
			assert.Equal(t, c.Params[0].Role, rules.ParamContext, "the language's parameter role")
			assert.Equal(t, c.Returns[1].Role, rules.ReturnError, "the language's return role")
			assert.Equal(t, c.Errors, rules.ErrorsLastReturn, "the language's error model")
		})

		t.Run("returns the signature a function declares", func(t *testing.T) {
			t.Parallel()

			c, is := rules.NewBound(roles{scripted()}, viewOnly(t), nil).CallableOf(fetch())
			assert.True(t, is, "a function maps")
			assert.Nil(t, c.Receiver, "a function has no receiver")
			assert.True(t, c.Async, "the model's asynchrony")
			assert.True(t, c.Params[1].Variadic, "the model's variadic flag")
			assert.Equal(t, c.Returns[1].Name, "err", "the model's return name")
		})

		t.Run("returns the name of a function", func(t *testing.T) {
			t.Parallel()

			c, _ := rules.NewBound(roles{scripted()}, viewOnly(t), nil).CallableOf(fetch())
			assert.Equal(t, c.Name, fetchName, "the declared name")
		})

		t.Run("returns the name of a method", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			c, _ := b.CallableOf(coretest.Method(svcPath, rowName, getName))
			assert.Equal(t, c.Name, getName, "the declared name")
		})

		t.Run("returns the receiver of an instance method", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			m := coretest.Method(svcPath, rowName, getName)
			m.Receiver = &node.Param{Name: "r", Type: named(svcPath, rowName, symbol.KindStruct)}
			c, is := b.CallableOf(m)
			assert.True(t, is, "a method maps")
			assert.Equal(t, c.Receiver.Name, "r", "with its receiver")
		})

		t.Run("returns no receiver for a type-level method", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			m := coretest.Method(svcPath, rowName, getName)
			m.Receiver = &node.Param{Name: "r", Type: named(svcPath, rowName, symbol.KindStruct)}
			m.Level = symbol.LevelType
			c, is := b.CallableOf(m)
			assert.True(t, is, "a method maps")
			assert.Nil(t, c.Receiver, "a type-level method has no receiver")
		})

		t.Run("pads the roles a language leaves short with ReturnValue", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Two")
			fn.Returns = []*node.Return{{Type: builtin(intSpelling)}, {Type: builtin(strSpelling)}}
			c, _ := rules.NewBound(short{scripted()}, viewOnly(t), nil).CallableOf(fn)
			assert.Equal(t, c.Returns[1].Role, rules.ReturnValue, "the second return has no role of the language's")
		})

		t.Run("skips a nil parameter", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Two")
			fn.Params = []*node.Param{nil, {Name: "a", Type: builtin(intSpelling)}}
			c, _ := rules.NewBound(scripted(), viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Params, 1, "the nil entry has no view")
			assert.Equal(t, c.Params[0].Name, "a", "the declared parameter remains")
		})

		t.Run("skips a nil return", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Two")
			fn.Returns = []*node.Return{{Type: builtin(intSpelling)}, nil, {Type: builtin(strSpelling)}}
			c, _ := rules.NewBound(scripted(), viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Returns, 2, "the nil entry has no view")
			assert.Equal(t, c.Returns[1].Ref.Spelling, strSpelling, "the declared returns remain in order")
		})

		t.Run("returns nil lists for a bare signature", func(t *testing.T) {
			t.Parallel()

			bare := coretest.Function(svcPath, "Bare")
			bare.Params, bare.Returns = nil, nil
			c, is := rules.NewBound(scripted(), viewOnly(t), nil).CallableOf(bare)
			assert.True(t, is, "a function is callable")
			expect.Nil(t, c.Params, "no parameters, no list")
			expect.Nil(t, c.Returns, "no returns, no list")
		})

		tests := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "reports false for a struct", give: coretest.Struct(svcPath, rowName)},
			{name: "reports false for a nil symbol", give: nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
				_, is := b.CallableOf(tt.give)
				assert.False(t, is, "only a function or a method is callable")
			})
		}
	})
}

// fetch returns an asynchronous function of a context and a variadic
// int, returning a string and a named error.
func fetch() *node.Function {
	fn := coretest.Function(svcPath, fetchName)
	fn.Params = []*node.Param{
		{Name: "ctx", Type: builtin("Context")},
		{Name: "rest", Type: builtin(intSpelling), Variadic: symbol.VariadicPositional},
	}
	fn.Returns = []*node.Return{{Type: builtin(strSpelling)}, {Name: "err", Type: builtin("error")}}
	fn.Async = true
	return fn
}
