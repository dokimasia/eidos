// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package validategen_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/validategen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
)

// The package of the tree with a struct that handlergen serves, the
// validator of the struct, its parameter, and the variable that the
// prologue binds.
const (
	ordersPackage = "example.com/shop/orders"
	validatorName = "ValidateOrders"
	requestParam  = "r"
	errVar        = "err"
)

// closerID is the name of the generator that writes a method other than
// ServeHTTP for a struct.
const closerID plugin.ID = "closer"

// The family of the closer, its method, and the receiver of the method.
const (
	closerWord   = "closer"
	closeName    = "Close"
	receiverName = "c"
)

// validategen writes a validator of each struct that handlergen serves,
// guards ServeHTTP with it, and passes the plugin suite.
func TestValidategen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the handler of the package orders", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := validategen.New()
				return p, orders(ctx, tb, p).Fixture
			})
		})

		t.Run("writes a validator of the struct that returns nil", func(t *testing.T) {
			t.Parallel()

			var validator *emit.Function
			for u := range generated(t).Units() {
				if u.Plugin != validategen.ID {
					continue
				}
				expect.That(t, u.Word).Equal(validategen.Word, "the validator is in the family of the validators")
				validator = u.Decls[0].(*emit.Function)
			}
			assert.NotNil(t, validator, "validategen writes the validator")
			expect.That(t, validator.Name).Equal(validatorName, "the validator joins its prefix onto the struct")
			expect.That(t, validator.Params[0].Name).Equal(requestParam, "the validator takes the request")
			assert.Equal(t, validator.Body.Stmts, []emit.Stmt{{
				Kind: emit.StmtReturn, Value: emit.ValueExpr(emit.Literal(emit.LiteralNil, "")),
			}}, "the validator returns nil")
		})

		t.Run("guards ServeHTTP with the validator in its prologue", func(t *testing.T) {
			t.Parallel()

			var prologue []emit.Stmt
			for u := range generated(t).Units() {
				if h, is := u.Decls[0].(*emit.Struct); is {
					prologue = h.Methods.Items()[0].Body.Prologue.Items()
				}
			}
			validate := emit.Expr{Kind: emit.ExprName, Name: validatorName}
			assert.Equal(t, prologue, []emit.Stmt{
				{
					Kind: emit.StmtAssign, Declare: true, Names: []string{errVar},
					Value: emit.Expr{Kind: emit.ExprCall, Fn: &validate, Args: []emit.Expr{
						{Kind: emit.ExprName, Name: requestParam},
					}},
				},
				{Kind: emit.StmtGuard, Name: errVar, Then: []emit.Stmt{{Kind: emit.StmtReturn}}},
			}, "ServeHTTP returns where the validator reports an error")
		})

		t.Run("leaves a method of the struct other than ServeHTTP as it is", func(t *testing.T) {
			t.Parallel()

			p := validategen.New()
			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:        referencetest.Tree,
				Packages:   []string{ordersPackage},
				Target:     golang.Target,
				Plugins:    []plugin.Plugin{p},
				Annotators: []plugin.Annotator{handlergen.New().(plugin.Annotator)},
				Hosts:      []plugin.Generator{closer()},
			})
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			var closed *emit.Method
			for u := range res.Emit.Units() {
				if s, is := u.Decls[0].(*emit.Struct); is {
					closed = s.Methods.Items()[0]
				}
			}
			assert.NotNil(t, closed, "the host writes the method Close")
			assert.Equal(t, closed.Body, emit.Body{}, "the method Close has no contribution")
		})
	})
}

// closer returns a generator that writes, for each struct, a struct with
// the method Close on a receiver, whose origin is the struct.
func closer() plugin.Generator {
	return sdk.NewPlugin(closerID).
		Output(plugin.Output{Per: plugin.PerSource, Word: closerWord}).
		Handle(sdk.OnStruct(func(m *sdk.StructMatch, e *sdk.Emitter) error {
			s := &emit.Struct{Origin: m.Struct.ID, Name: closerWord + m.Struct.Name}
			s.Methods.Append(&emit.Method{Origin: m.Struct.ID, Name: closeName, Receiver: &emit.Param{Name: receiverName}})
			e.File().Append(s)
			return nil
		})).
		Build().(plugin.Generator)
}

// generated runs validategen over the handler of the package orders, and
// returns the emit store of the run.
func generated(t *testing.T) *plugin.Emit {
	t.Helper()

	p := validategen.New()
	res := orders(t.Context(), t, p).Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	return res.Emit
}

// orders returns a fixture over the package orders of the tree, loaded
// under ctx and stamped by handlergen, with the target golang, the plugin
// validategen p, and the units of handlergen seeded.
func orders(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:        referencetest.Tree,
		Packages:   []string{ordersPackage},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p},
		Annotators: []plugin.Annotator{handlergen.New().(plugin.Annotator)},
		Hosts:      []plugin.Generator{handlergen.New()},
	})
}
