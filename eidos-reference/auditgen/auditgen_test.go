// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package auditgen_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/auditgen"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with a struct that handlergen serves, the
// function that auditgen calls, and the parameters of ServeHTTP.
const (
	ordersPackage = "example.com/shop/orders"
	auditPackage  = "example.com/shop/httpmw/audit"
	auditFunc     = "Record"
	writerParam   = "w"
	requestParam  = "r"
)

// auditgen calls the function that its options set at the end of
// ServeHTTP, and passes the plugin suite.
func TestAuditgen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the handler of the package orders", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := configured()
				return p, orders(ctx, tb, p).Fixture
			})
		})

		t.Run("appends the record of the request into the epilogue of ServeHTTP", func(t *testing.T) {
			t.Parallel()

			p := configured()
			res := orders(t.Context(), t, p).Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			var epilogue []emit.Stmt
			for u := range res.Emit.Units() {
				for _, d := range u.Decls {
					if h, is := d.(*emit.Struct); is {
						epilogue = h.Methods.Items()[0].Body.Epilogue.Items()
					}
				}
			}
			ref := emit.RefExpr(symbol.Identity{
				Lang: golang.Lang, Package: auditPackage, Name: auditFunc, Kind: symbol.KindFunction,
			})
			assert.Equal(t, epilogue, []emit.Stmt{{
				Kind: emit.StmtExpr,
				Value: emit.Expr{Kind: emit.ExprCall, Fn: &ref, Args: []emit.Expr{
					{Kind: emit.ExprName, Name: writerParam},
					{Kind: emit.ExprName, Name: requestParam},
				}},
			}}, "the epilogue calls the function with the writer and the request")
		})
	})
}

// configured returns auditgen with the options of the tree's config.
func configured() plugin.Generator {
	p := auditgen.New()
	o := p.(plugin.OptionsProvider).Options().(*callee.Options)
	o.Package, o.Func = auditPackage, auditFunc
	return p
}

// orders returns a fixture over the package orders of the tree, loaded
// under ctx and stamped by handlergen, with the target golang, the plugin
// auditgen p, and the units of handlergen seeded.
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
