// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package callee_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/middlewaregen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The workspace tree from the directory of the package, and its package
// with the struct Orders.
const (
	tree          = "../../testdata/tree"
	ordersPackage = "example.com/shop/orders"
	ordersName    = "Orders"
)

// The host whose units the cases seed, its family, the plugin that runs a
// rule, and the declarations that the host writes for Orders.
const (
	hostID         plugin.ID = "host"
	probeID        plugin.ID = "probe"
	hostWord                 = "host"
	middlewareName           = "OrdersMiddleware"
	validatorName            = "ValidateOrders"
	handlerName              = "OrdersHandler"
	contractName             = "OrdersCloser"
	closeName                = "Close"
	handlerParam             = "h"
	writerParam              = "w"
	requestParam             = "r"
	receiverParam            = "o"
)

// The function that the options of the cases set, and the format of the
// weaver.
const (
	logPackage = "log"
	logFunc    = "Printf"
	logFormat  = "debug: %s entered"
)

// reporter records the codes of the findings that a check reports.
type reporter struct {
	codes []diag.Code
}

// Errorf records the code c.
func (r *reporter) Errorf(c diag.Code, _ string, _ ...any) {
	r.codes = append(r.codes, c)
}

// Each rule appends one call of the function of its options into a body
// that the host wrote, and reports Missing where the options set no
// function.
func TestCallee(t *testing.T) {
	t.Parallel()

	logged := emit.RefExpr(
		symbol.Identity{Lang: golang.Lang, Package: logPackage, Name: logFunc, Kind: symbol.KindFunction},
	)

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			givePackage string
			giveFunc    string
			want        bool
			wantCodes   []diag.Code
		}{
			{name: "returns true for a package and a function", givePackage: logPackage, giveFunc: logFunc, want: true},
			{name: "returns false for an empty package", giveFunc: logFunc, wantCodes: []diag.Code{callee.Missing}},
			{
				name:        "returns false for an empty function",
				givePackage: logPackage,
				wantCodes:   []diag.Code{callee.Missing},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := &reporter{}
				expect.Equal(t, callee.Check(r, tt.givePackage, tt.giveFunc), tt.want,
					"Check reports whether the options name a function")
				expect.Equal(t, r.codes, tt.wantCodes, "Check reports Missing for empty options alone")
			})
		}
	})

	t.Run("Chain", func(t *testing.T) {
		t.Parallel()

		t.Run("appends the wrap of the handler into the slot of the chain", func(t *testing.T) {
			t.Parallel()

			out := run(t, symbol.KindFunction, callee.Chain(&callee.Options{Package: logPackage, Func: logFunc}))
			slot, _ := out.middleware.Body.Slot(middlewaregen.ChainSlot)
			assert.Equal(t, slot.Items(), []emit.Stmt{
				{
					Kind:  emit.StmtAssign,
					Names: []string{handlerParam},
					Value: emit.Expr{
						Kind: emit.ExprCall,
						Fn:   &logged,
						Args: []emit.Expr{{Kind: emit.ExprName, Name: handlerParam}},
					},
				},
			}, "the chain assigns the wrapped handler to h")
		})

		t.Run("leaves a function without the slot of the chain as it is", func(t *testing.T) {
			t.Parallel()

			out := run(t, symbol.KindFunction, callee.Chain(&callee.Options{Package: logPackage, Func: logFunc}))
			assert.Equal(t, out.validator.Body, emit.Body{}, "the validator has no contribution")
		})

		t.Run("reports Missing at the subject for empty options", func(t *testing.T) {
			t.Parallel()

			out := run(t, symbol.KindFunction, callee.Chain(&callee.Options{}))
			assertMissing(t, out.sink, 1)
			slot, _ := out.middleware.Body.Slot(middlewaregen.ChainSlot)
			assert.Empty(t, slot.Items(), "the chain has no statement")
		})
	})

	t.Run("Epilogue", func(t *testing.T) {
		t.Parallel()

		t.Run("appends the call of the request into the epilogue of ServeHTTP", func(t *testing.T) {
			t.Parallel()

			out := run(t, symbol.KindMethod, callee.Epilogue(&callee.Options{Package: logPackage, Func: logFunc}))
			assert.Equal(t, out.serveHTTP.Body.Epilogue.Items(), []emit.Stmt{{
				Kind: emit.StmtExpr,
				Value: emit.Expr{Kind: emit.ExprCall, Fn: &logged, Args: []emit.Expr{
					{Kind: emit.ExprName, Name: writerParam},
					{Kind: emit.ExprName, Name: requestParam},
				}},
			}}, "the epilogue calls the function with the parameters of ServeHTTP")
		})

		t.Run("leaves a method other than ServeHTTP as it is", func(t *testing.T) {
			t.Parallel()

			out := run(t, symbol.KindMethod, callee.Epilogue(&callee.Options{Package: logPackage, Func: logFunc}))
			assert.Equal(t, out.close.Body, emit.Body{}, "the method Close has no contribution")
		})

		t.Run("reports Missing at the subject for empty options", func(t *testing.T) {
			t.Parallel()

			out := run(t, symbol.KindMethod, callee.Epilogue(&callee.Options{}))
			assertMissing(t, out.sink, 1)
			assert.Empty(t, out.serveHTTP.Body.Epilogue.Items(), "the epilogue has no statement")
		})
	})

	t.Run("Weave", func(t *testing.T) {
		t.Parallel()

		t.Run("appends the log of the entry into the prologue of each method with a receiver", func(t *testing.T) {
			t.Parallel()

			out := run(
				t,
				symbol.KindMethod,
				callee.Weave(&callee.Log{Package: logPackage, Func: logFunc, Format: logFormat}),
			)
			for _, m := range []*emit.Method{out.serveHTTP, out.close} {
				expect.Equal(t, m.Body.Prologue.Items(), []emit.Stmt{{
					Kind: emit.StmtExpr,
					Value: emit.Expr{Kind: emit.ExprCall, Fn: &logged, Args: []emit.Expr{
						emit.ValueExpr(emit.Literal(emit.LiteralString, logFormat)),
						emit.ValueExpr(emit.Literal(emit.LiteralString, ordersName+"."+m.Name)),
					}},
				}}, "the prologue of "+m.Name+" logs the qualified name of the method")
			}
		})

		t.Run("leaves a method without a receiver as it is", func(t *testing.T) {
			t.Parallel()

			out := run(
				t,
				symbol.KindMethod,
				callee.Weave(&callee.Log{Package: logPackage, Func: logFunc, Format: logFormat}),
			)
			assert.Equal(t, out.signature.Body, emit.Body{}, "the method of the interface has no contribution")
		})

		t.Run("reports Missing at the subject for empty options", func(t *testing.T) {
			t.Parallel()

			out := run(t, symbol.KindMethod, callee.Weave(&callee.Log{Format: logFormat}))
			assertMissing(t, out.sink, 2)
		})

		t.Run("passes the plugin suite in a plugin of its own", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := sdk.NewPlugin(probeID).
					Handle(sdk.OnEmit(symbol.KindMethod,
						callee.Weave(&callee.Log{Package: logPackage, Func: logFunc, Format: logFormat}))).
					Build()
				return p, referencetest.New(ctx, tb, referencetest.Spec{
					Dir:      tree,
					Packages: []string{ordersPackage},
					Target:   golang.Target,
					Plugins:  []plugin.Plugin{p},
					Hosts:    []plugin.Generator{host()},
				}).Fixture
			})
		})
	})
}

// written are the declarations that the host wrote for Orders, after a
// rule ran over them, and the findings of the rule.
type written struct {
	middleware, validator *emit.Function
	serveHTTP, close      *emit.Method
	signature             *emit.Method
	sink                  *diag.Sink
}

// run seeds the units of the host over the package orders of the tree, and
// runs a plugin with the rule on the emitted values of kind k. It returns
// the declarations of the host after the rule ran.
func run(t *testing.T, k symbol.Kind, rule func(*sdk.EmitMatch, *sdk.Emitter) error) written {
	t.Helper()

	p := sdk.NewPlugin(probeID).Handle(sdk.OnEmit(k, rule)).Build()
	f := referencetest.New(t.Context(), t, referencetest.Spec{
		Dir:      tree,
		Packages: []string{ordersPackage},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
		Hosts:    []plugin.Generator{host()},
	})
	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the rule runs whole")
	var out written
	out.sink = res.Sink
	for u := range res.Emit.Units() {
		for _, d := range u.Decls {
			switch decl := d.(type) {
			case *emit.Function:
				if decl.Name == middlewareName {
					out.middleware = decl
				} else {
					out.validator = decl
				}
			case *emit.Struct:
				out.serveHTTP, out.close = decl.Methods.Items()[0], decl.Methods.Items()[1]
			case *emit.Interface:
				out.signature = decl.Methods.Items()[0]
			}
		}
	}
	return out
}

// host returns the generator whose units the cases seed. For the struct
// Orders, it writes a middleware function with the slot of the chain, a
// validator without the slot, a handler with the methods ServeHTTP and
// Close on a receiver, and an interface with the method Close.
func host() plugin.Generator {
	return sdk.NewPlugin(hostID).
		Output(plugin.Output{Per: plugin.PerSource, Word: hostWord}).
		Handle(sdk.OnStruct(func(m *sdk.StructMatch, e *sdk.Emitter) error {
			id := m.Struct.ID
			middleware := &emit.Function{Origin: id, Name: middlewareName, Params: []*emit.Param{{Name: handlerParam}}}
			middleware.Body.Declare(middlewaregen.ChainSlot)
			validator := &emit.Function{Origin: id, Name: validatorName, Params: []*emit.Param{{Name: requestParam}}}
			handler := &emit.Struct{Origin: id, Name: handlerName}
			handler.Methods.Append(
				&emit.Method{
					Origin: id, Name: handlergen.ServeHTTP, Receiver: &emit.Param{Name: receiverParam},
					Params: []*emit.Param{{Name: writerParam}, {Name: requestParam}},
				},
				&emit.Method{Origin: id, Name: closeName, Receiver: &emit.Param{Name: receiverParam}},
			)
			contract := &emit.Interface{Origin: id, Name: contractName}
			contract.Methods.Append(&emit.Method{Origin: id, Name: closeName})
			e.File().Append(middleware, validator, handler, contract)
			return nil
		})).
		Build().(plugin.Generator)
}

// assertMissing checks that the sink has n findings, each Missing at Error
// severity at the struct Orders.
func assertMissing(tb assert.TB, sink *diag.Sink, n int) {
	tb.Helper()

	count := 0
	for d := range sink.All() {
		expect.Equal(tb, d.Code, callee.Missing, "the finding is Missing")
		expect.Equal(tb, d.Severity, diag.SeverityError, "the finding is an Error")
		count++
	}
	assert.Equal(tb, count, n, "the rule reports each subject that it calls no function for")
}
