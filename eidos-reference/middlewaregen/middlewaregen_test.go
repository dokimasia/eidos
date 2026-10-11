// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package middlewaregen_test

import (
	"context"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/middlewaregen"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
)

// The package of the tree with a struct that handlergen serves, the name of
// its middleware function, the template of the body, and the parameter of
// the function.
const (
	ordersPackage  = "example.com/shop/orders"
	middlewareName = "OrdersMiddleware"
	bodyTemplate   = "middleware.tmpl"
	handlerParam   = "h"
)

// middlewaregen writes one middleware function with a named slot for each
// struct that handlergen serves, and passes the plugin suite.
func TestMiddlewaregen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package orders of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := middlewaregen.New()
				return p, orders(ctx, tb, p).Fixture
			})
		})

		t.Run("writes a middleware function of the handler with the named slot chain", func(t *testing.T) {
			t.Parallel()

			p := middlewaregen.New()
			res := orders(t.Context(), t, p).Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			units := slices.Collect(res.Emit.Units())
			assert.Length(t, units, 1, "middlewaregen writes one unit")
			assert.Length(t, units[0].Decls, 1, "the unit declares the function alone")
			fn, is := units[0].Decls[0].(*emit.Function)
			assert.True(t, is, "the declaration is a function")
			expect.That(t, fn.Name).Equal(middlewareName, "the function joins the word onto the name of the struct")
			expect.That(t, fn.Body.Ref.Name).Equal(bodyTemplate, "the template writes the body")
			_, declared := fn.Body.Slot(middlewaregen.ChainSlot)
			expect.True(t, declared, "the body declares the slot of the chain")
			expect.That(t, fn.Params[0].Name).Equal(handlerParam, "the function takes the handler")
		})
	})
}

// orders returns a fixture over the package orders of the tree, loaded
// under ctx and stamped by the annotator of handlergen, with the target
// golang and the plugin middlewaregen p.
func orders(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:        referencetest.Tree,
		Packages:   []string{ordersPackage},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p},
		Annotators: []plugin.Annotator{handlergen.New().(plugin.Annotator)},
	})
}
