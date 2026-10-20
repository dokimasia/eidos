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

// The names of the fixture callables, and the spellings of the result
// type, of its unit value and of its failure in the language of results.
const (
	fetchName      = "Fetch"
	getName        = "Get"
	resultSpelling = "Result"
	unitSpelling   = "()"
	errorSpelling  = "error"
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

// results reports its failures in a result type, the way Rust returns
// Result<T, E>.
type results struct {
	rules.SourceRules
}

// ReturnRoles gives every return the role of a value, and states the
// result-type model where a return is spelled Result.
func (results) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	model := rules.ErrorsNone
	for _, r := range rs {
		if r.Type.Spelling == resultSpelling {
			model = rules.ErrorsResultType
		}
	}
	return make([]rules.ReturnRole, len(rs)), model
}

// Result splits Result<T, E> into T and E, and a result of the unit value
// into its failure alone.
func (results) Result(ref *node.TypeRef, _ rules.View) (value, failure *node.TypeRef, ok bool) {
	switch {
	case ref.Spelling != resultSpelling:
		return nil, nil, false
	case ref.Args[0].Spelling == unitSpelling:
		return nil, ref.Args[1], true
	default:
		return ref.Args[0], ref.Args[1], true
	}
}

// contexts gives every callable the context of a call, the way protobuf
// gives one to every rpc.
type contexts struct {
	rules.SourceRules
}

// contextView is the view of the context that contexts gives.
var contextView = rules.ParamView{
	Name: "ctx", Ref: &node.TypeRef{Spelling: "context", Target: rules.WellKnownContext}, Role: rules.ParamContext,
}

// Context returns the view of the context for every callable.
func (contexts) Context(symbol.Symbol, rules.View) (rules.ParamView, bool) { return contextView, true }

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

		{
			tests := []struct {
				name string
				give *node.Param
				want bool
			}{
				{
					name: "sets Optional for a parameter with a default",
					give: &node.Param{Name: "n", Type: builtin(intSpelling), Default: "1"},
					want: true,
				},
				{
					name: "sets Optional for a parameter with the optional mark",
					give: &node.Param{Name: "n", Type: builtin(intSpelling), Optional: true},
					want: true,
				},
				{
					name: "sets Optional for a parameter that collects the rest",
					give: &node.Param{Name: "n", Type: builtin(intSpelling), Variadic: symbol.VariadicPositional},
					want: true,
				},
				{
					name: "leaves Optional false for a parameter that needs an argument",
					give: &node.Param{Name: "n", Type: builtin(intSpelling)},
					want: false,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					fn := coretest.Function(svcPath, "One")
					fn.Params = []*node.Param{tt.give}
					c, _ := rules.NewBound(scripted(), viewOnly(t), nil).CallableOf(fn)
					assert.Length(t, c.Params, 1, "the parameter has a view")
					assert.Equal(t, c.Params[0].Optional, tt.want,
						"the view states whether a call can leave the parameter out")
				})
			}
		}

		t.Run("returns the way a call passes each parameter", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Passed")
			fn.Params = []*node.Param{
				{Name: "a", Type: builtin(intSpelling), Passing: symbol.PassingPositional},
				{Name: "b", Type: builtin(intSpelling), Passing: symbol.PassingKeyword},
			}
			c, _ := rules.NewBound(scripted(), viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Params, 2, "each parameter has a view")
			expect.Equal(t, c.Params[0].Passing, symbol.PassingPositional, "the first passes by position alone")
			expect.Equal(t, c.Params[1].Passing, symbol.PassingKeyword, "the second passes by keyword alone")
		})

		t.Run("splits a return of a result type into its value and its failure", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Load")
			fn.Returns = []*node.Return{{Name: "out", Type: result(builtin(intSpelling))}}
			c, _ := rules.NewBound(results{scripted()}, viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Returns, 2, "the result has the view of its value and of its failure")
			expect.Equal(t, c.Returns[0].Ref.Spelling, intSpelling, "the value comes first")
			expect.Equal(t, c.Returns[0].Role, rules.ReturnValue, "the value has the role of the return")
			expect.Equal(t, c.Returns[1].Ref.Spelling, errorSpelling, "the failure comes second")
			expect.Equal(t, c.Returns[1].Role, rules.ReturnError, "the failure has the role of an error")
			expect.Equal(t, c.Returns[1].Name, "out", "the failure has the name of the return")
		})

		t.Run("returns the failure alone for a result without a value", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Store")
			fn.Returns = []*node.Return{{Type: result(builtin(unitSpelling))}}
			c, _ := rules.NewBound(results{scripted()}, viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Returns, 1, "a result of the unit value has no view of a value")
			expect.Equal(t, c.Returns[0].Role, rules.ReturnError, "the one view is the failure")
		})

		t.Run("keeps a return of another type under the result-type model", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Pair")
			fn.Returns = []*node.Return{{Type: result(builtin(intSpelling))}, {Type: builtin(strSpelling)}}
			c, _ := rules.NewBound(results{scripted()}, viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Returns, 3, "the result splits and the other return keeps one view")
			expect.Equal(t, c.Returns[2].Ref.Spelling, strSpelling, "the other return keeps its reference")
			expect.Equal(t, c.Returns[2].Role, rules.ReturnValue, "the other return keeps its role")
		})

		t.Run("returns one view of each return under another error model", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Plain")
			fn.Returns = []*node.Return{{Type: builtin(strSpelling)}}
			c, _ := rules.NewBound(results{scripted()}, viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Returns, 1, "a callable without a result type has one view of each return")
			expect.Equal(t, c.Errors, rules.ErrorsNone, "the language states no failure channel")
		})

		t.Run("keeps a result type of a language without the capability", func(t *testing.T) {
			t.Parallel()

			fn := coretest.Function(svcPath, "Load")
			fn.Returns = []*node.Return{{Type: result(builtin(intSpelling))}}
			c, _ := rules.NewBound(scripted(), viewOnly(t), nil).CallableOf(fn)
			assert.Length(t, c.Returns, 1, "the result has one view")
			expect.Equal(t, c.Returns[0].Ref.Spelling, resultSpelling, "the view has the reference of the result")
		})

		t.Run("puts the context of the language before the parameters", func(t *testing.T) {
			t.Parallel()

			c, _ := rules.NewBound(contexts{scripted()}, viewOnly(t), nil).CallableOf(fetch())
			assert.Length(t, c.Params, len(fetch().Params)+1, "the context and every parameter")
			expect.Equal(t, c.Params[0], contextView, "the context comes first")
			expect.Equal(t, c.Params[1].Name, fetch().Params[0].Name, "the first parameter follows")
		})

		t.Run("returns the context alone for a signature without parameters", func(t *testing.T) {
			t.Parallel()

			bare := coretest.Function(svcPath, "Bare")
			bare.Params = nil
			c, _ := rules.NewBound(contexts{scripted()}, viewOnly(t), nil).CallableOf(bare)
			assert.Equal(t, c.Params, []rules.ParamView{contextView}, "the one view is the context")
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

	t.Run("IsImplementation", func(t *testing.T) {
		t.Parallel()

		signature := &node.Method{Name: fetchName, Overload: true}
		implementation := &node.Method{Name: fetchName}
		static := &node.Method{Name: fetchName, Level: symbol.LevelType}
		other := &node.Method{Name: getName}
		tests := []struct {
			name        string
			give        *node.Method
			giveMethods []*node.Method
			want        bool
		}{
			{
				name: "reports true for a method without the mark of an overloaded name", give: implementation,
				giveMethods: []*node.Method{signature, implementation}, want: true,
			},
			{
				name: "reports false for an overload signature", give: signature,
				giveMethods: []*node.Method{signature, implementation},
			},
			{
				name: "reports false for a method of a name without an overload", give: other,
				giveMethods: []*node.Method{signature, other},
			},
			{
				name: "reports false for a method of another level than the overloads", give: static,
				giveMethods: []*node.Method{signature, static},
			},
			{name: "reports false for a nil method", giveMethods: []*node.Method{signature}},
			{
				name: "reports false for a method beside a nil method", give: implementation,
				giveMethods: []*node.Method{nil, implementation},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, rules.IsImplementation(tt.give, tt.giveMethods), tt.want,
					"whether the method is the implementation of its overloads")
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
	fn.Returns = []*node.Return{{Type: builtin(strSpelling)}, {Name: "err", Type: builtin(errorSpelling)}}
	fn.Async = true
	return fn
}

// result returns the reference Result<value, error> of the language of
// results.
func result(value *node.TypeRef) *node.TypeRef {
	ref := builtin(resultSpelling)
	ref.Args = []*node.TypeRef{value, builtin(errorSpelling)}
	return ref
}
