// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package handlergen_test

import (
	"context"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with a struct under the directive handler, the
// struct, and its handler.
const (
	ordersPackage = "example.com/shop/orders"
	ordersName    = "Orders"
	handlerName   = "OrdersHandler"
)

// The tree of the structs under the directive handler without a method
// Serve that a handler can call, its package, and its five structs.
const (
	noServeTree    = "testdata/noserve"
	noServePackage = "example.com/noserve"
	ledgerName     = "Ledger"
	reportName     = "Report"
	tallyName      = "Tally"
	copyName       = "Copy"
	cookieName     = "Cookie"
)

// The statement that ServeHTTP calls Serve with, the receiver's name that
// it reads, and the parameters of ServeHTTP.
const (
	serveCall   = "o.Orders.Serve"
	receiver    = "o"
	writerName  = "w"
	requestName = "r"
)

// otherRegistrant is the registrant that claims the namespace of
// handlergen before handlergen registers its keys.
const otherRegistrant = "other"

// handlergen stamps the structs that it serves, writes a handler for each,
// and passes the plugin suite.
func TestHandlergen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package orders of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := handlergen.New()
				return p, orders(ctx, tb, p).Fixture
			})
		})

		t.Run("stamps the fact on a struct under the directive with the method Serve", func(t *testing.T) {
			t.Parallel()

			p := handlergen.New()
			f := orders(t.Context(), t, p)
			res := f.Annotate(t, p)
			assert.NoError(t, res.Err, "the annotator runs whole")
			key, registered := meta.Lookup[bool](f.Keys, handlergen.KeyHandler)
			assert.True(t, registered, "handlergen registers its key")
			stamped, _ := meta.Get(f.Facts, f.Find(t, symbol.KindStruct, "", ordersName).Identity(), key)
			assert.True(t, stamped, "Orders is a struct that handlergen serves")
		})

		t.Run("writes a handler with a field that points at the struct", func(t *testing.T) {
			t.Parallel()

			h := generated(t)
			assert.Equal(t, h.Name, handlerName, "the handler joins the word onto the name of the struct")
			fields := h.Fields.Items()
			assert.Length(t, fields, 1, "the handler has one field")
			expect.That(t, fields[0].Name).Equal(ordersName, "the field has the name of the struct")
			expect.That(t, fields[0].Type.Form).Equal(symbol.FormOptional, "the field is a pointer")
			expect.That(t, fields[0].Type.Elems[0].Spelling).Equal(ordersName, "the field points at the struct")
		})

		t.Run("writes ServeHTTP on a pointer receiver with the writer and the request", func(t *testing.T) {
			t.Parallel()

			methods := generated(t).Methods.Items()
			assert.Length(t, methods, 1, "the handler has one method")
			m := methods[0]
			expect.That(t, m.Name).Equal(handlergen.ServeHTTP, "the method serves HTTP")
			expect.That(t, m.Receiver.Name).Equal(receiver, "the receiver takes the first letter of the handler")
			expect.That(t, m.Receiver.Type.Form).Equal(symbol.FormOptional, "the receiver is a pointer")
			names := make([]string, 0, len(m.Params))
			for _, p := range m.Params {
				names = append(names, p.Name)
			}
			assert.Equal(t, names, []string{writerName, requestName}, "ServeHTTP takes the writer and the request")
		})

		t.Run("writes ServeHTTP that calls Serve of the field with the writer and the request", func(t *testing.T) {
			t.Parallel()

			stmts := generated(t).Methods.Items()[0].Body.Stmts
			assert.Length(t, stmts, 1, "the content is one statement")
			call := stmts[0].Value
			expect.That(t, call.Kind).Equal(emit.ExprCall, "the statement is a call")
			expect.That(t, call.Fn.Name).Equal(serveCall, "the call reaches Serve through the field")
			expect.That(t, call.Args).Equal([]emit.Expr{
				{Kind: emit.ExprName, Name: writerName},
				{Kind: emit.ExprName, Name: requestName},
			}, "the call passes the writer and the request")
		})

		t.Run("returns from Keys the error of a namespace that another registrant claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(otherRegistrant).ClaimNamespace(handlergen.KeyHandler.Namespace()),
				"another registrant claims the namespace of handlergen")
			keys, provides := handlergen.New().(plugin.KeyProvider)
			assert.True(t, provides, "handlergen registers keys")
			assert.HasError(t, keys.Keys(r.For(string(handlergen.ID))), "handlergen cannot claim its namespace")
		})

		t.Run("reports MissingServe at each struct under the directive without the method Serve", func(t *testing.T) {
			t.Parallel()

			p := handlergen.New()
			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:      noServeTree,
				Packages: []string{noServePackage},
				Target:   golang.Target,
				Plugins:  []plugin.Plugin{p},
			})
			res := f.Annotate(t, p)
			assert.NoError(t, res.Err, "the annotator runs whole")
			var lines []int
			for d := range res.Sink.All() {
				expect.Equal(t, d.Code, handlergen.MissingServe, "the finding is MissingServe")
				expect.Equal(t, d.Severity, diag.SeverityError, "the finding is an Error")
				lines = append(lines, d.Pos.Line)
			}
			assert.Equal(t, lines, []int{
				f.Find(t, symbol.KindStruct, "", cookieName).Position().Line,
				f.Find(t, symbol.KindStruct, "", copyName).Position().Line,
				f.Find(t, symbol.KindStruct, "", ledgerName).Position().Line,
				f.Find(t, symbol.KindStruct, "", reportName).Position().Line,
				f.Find(t, symbol.KindStruct, "", tallyName).Position().Line,
			}, "each struct without a method Serve of a writer and a request has the finding, "+
				"in the order of the names")
			assert.Empty(t, slices.Collect(f.Generate(t, p).Emit.Units()), "handlergen writes no handler for them")
		})
	})
}

// orders returns a fixture over the package orders of the tree, loaded
// under ctx, with the target golang and the keys and the directive of the
// plugin handlergen p.
func orders(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:      referencetest.Tree,
		Packages: []string{ordersPackage},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// generated runs handlergen over the package orders of the tree, and
// returns the one declaration that it writes, which is the handler of
// Orders.
func generated(t *testing.T) *emit.Struct {
	t.Helper()

	p := handlergen.New()
	f := orders(t.Context(), t, p)
	assert.NoError(t, f.Annotate(t, p).Err, "the annotator runs whole")
	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	units := slices.Collect(res.Emit.Units())
	assert.Length(t, units, 1, "handlergen writes one unit")
	expect.That(t, units[0].Word).Equal(handlergen.Word, "the unit is in the family of the handlers")
	assert.Length(t, units[0].Decls, 1, "the unit declares the handler alone")
	h, is := units[0].Decls[0].(*emit.Struct)
	assert.True(t, is, "the handler is a struct")
	return h
}
