// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The mapping is the kernel's and the roles are the language's, so
// the shape of the view and the split of the work are pinned here.
func TestCallable(t *testing.T) {
	t.Parallel()

	t.Run("CallableOf", func(t *testing.T) {
		t.Parallel()

		t.Run("maps a function and asks the language for roles", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Fetch")
			fn.Params = []*node.Param{
				{Name: "ctx", Type: builtin("Context")},
				{Name: "rest", Type: builtin(intSpelling), Variadic: symbol.VariadicPositional},
			}
			fn.Returns = []*node.Return{{Type: builtin(strSpelling)}, {Name: "err", Type: builtin("error")}}
			fn.Async = true
			b := rules.NewBound(roles{scripted()}, viewOnly(t), nil)
			c, is := b.CallableOf(fn)
			assert.True(t, is, "a function maps")
			assert.True(t, c.Receiver == nil, "with no receiver")
			assert.True(t, c.Async, "carrying its asynchrony")
			assert.Equal(t, c.Params[0].Role, rules.ParamContext, "the language's parameter role")
			assert.True(t, c.Params[1].Variadic, "the model's variadic flag")
			assert.Equal(t, c.Returns[1].Role, rules.ReturnError, "the language's return role")
			assert.Equal(t, c.Returns[1].Name, "err", "with the model's name")
			assert.Equal(t, c.Errors, rules.ErrorsLastReturn, "and its error model")
		})

		t.Run("keeps a receiver at instance level and drops it at type level", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			m := coretest.Method(svcPath, rowName, "Get")
			m.Receiver = &node.Param{Name: "r", Type: named(svcPath, rowName, symbol.KindStruct)}
			c, is := b.CallableOf(m)
			assert.True(t, is, "a method maps")
			assert.Equal(t, c.Receiver.Name, "r", "with its receiver")
			m.Level = symbol.LevelType
			c, _ = b.CallableOf(m)
			assert.True(t, c.Receiver == nil, "and none at type level")
		})

		t.Run("pads roles a language leaves short and skips nil entries", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Two")
			fn.Params = []*node.Param{nil, {Name: "a", Type: builtin(intSpelling)}}
			fn.Returns = []*node.Return{{Type: builtin(intSpelling)}, nil, {Type: builtin(strSpelling)}}
			b := rules.NewBound(short{scripted()}, viewOnly(t), nil)
			c, _ := b.CallableOf(fn)
			assert.Length(t, c.Params, 1, "a nil parameter is skipped")
			assert.Length(t, c.Returns, 2, "and a nil return")
			assert.Equal(t, c.Returns[1].Role, rules.ReturnValue, "a role the language left short pads as a value")
		})

		t.Run("refuses every other kind", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			_, is := b.CallableOf(coretest.Struct(svcPath, rowName))
			assert.False(t, is, "a struct is no callable")
			_, is = b.CallableOf(nil)
			assert.False(t, is, "nor is nothing")
		})

		t.Run("carries no lists for a bare signature", func(t *testing.T) {
			t.Parallel()

			bare := coretest.Function(svcPath, "Bare")
			bare.Params, bare.Returns = nil, nil
			b := rules.NewBound(scripted(), viewOnly(t), nil)
			c, is := b.CallableOf(bare)
			assert.True(t, is, "a function is callable")
			assert.True(t, c.Params == nil, "no parameters, no list")
			assert.True(t, c.Returns == nil, "no returns, no list")
			assert.True(t, c.Receiver == nil, "and no receiver")
		})
	})
}

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

// ReturnRoles calls a trailing error the error carrier.
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
