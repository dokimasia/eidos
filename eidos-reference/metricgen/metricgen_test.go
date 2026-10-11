// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package metricgen_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/metricgen"
	"go.dokimi.dev/eidos/reference/middlewaregen"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with a struct that handlergen serves, the
// function that metricgen wraps the handler in, and the middleware function
// of the struct.
const (
	ordersPackage  = "example.com/shop/orders"
	metricsPackage = "example.com/shop/httpmw/metrics"
	metricsFunc    = "RecordLatency"
	middlewareName = "OrdersMiddleware"
	handlerParam   = "h"
)

// metricgen wraps the handler of each middleware function in the function
// that its options set, and passes the plugin suite.
func TestMetricgen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the middleware of the package orders", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := configured()
				return p, orders(ctx, tb, p).Fixture
			})
		})

		t.Run("appends the wrap of the handler into the chain of the middleware", func(t *testing.T) {
			t.Parallel()

			p := configured()
			res := orders(t.Context(), t, p).Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			var chain []emit.Stmt
			for u := range res.Emit.Units() {
				for _, d := range u.Decls {
					if fn, is := d.(*emit.Function); is && fn.Name == middlewareName {
						slot, _ := fn.Body.Slot(middlewaregen.ChainSlot)
						chain = slot.Items()
					}
				}
			}
			ref := emit.RefExpr(symbol.Identity{
				Lang: golang.Lang, Package: metricsPackage, Name: metricsFunc, Kind: symbol.KindFunction,
			})
			assert.Equal(t, chain, []emit.Stmt{
				{
					Kind:  emit.StmtAssign,
					Names: []string{handlerParam},
					Value: emit.Expr{
						Kind: emit.ExprCall,
						Fn:   &ref,
						Args: []emit.Expr{{Kind: emit.ExprName, Name: handlerParam}},
					},
				},
			}, "the chain assigns the wrapped handler to h")
		})
	})
}

// configured returns metricgen with the options of the tree's config.
func configured() plugin.Generator {
	p := metricgen.New()
	o := p.(plugin.OptionsProvider).Options().(*callee.Options)
	o.Package, o.Func = metricsPackage, metricsFunc
	return p
}

// orders returns a fixture over the package orders of the tree and the
// package of the function of the options, loaded under ctx and stamped by
// handlergen, with the target golang, the plugin metricgen p, and the units
// of handlergen and middlewaregen seeded.
func orders(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:        referencetest.Tree,
		Packages:   []string{ordersPackage, metricsPackage},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p},
		Annotators: []plugin.Annotator{handlergen.New().(plugin.Annotator)},
		Hosts:      []plugin.Generator{handlergen.New(), middlewaregen.New()},
	})
}
