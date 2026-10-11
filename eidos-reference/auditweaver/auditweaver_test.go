// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package auditweaver_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/auditweaver"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/mockgen"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with Product under the directive audit, the
// declarations that the hosts write, and the struct whose methods derive
// from them.
const (
	catalogPackage = "example.com/shop/catalog"
	productRepo    = "ProductRepo"
	productMock    = "ProductRepositoryMock"
	reviewRepo     = "ReviewRepo"
	storeMock      = "StoreMock"
	productName    = "Product"
)

// auditweaver logs the entry of each method with a receiver that derives
// from a declaration under its directive, and passes the plugin suite.
func TestAuditweaver(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the repositories of the package catalog", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := auditweaver.New()
				return p, catalog(ctx, tb, p).Fixture
			})
		})

		t.Run("logs the entry of each method that derives from a struct under the directive", func(t *testing.T) {
			t.Parallel()

			woven := weave(t)
			logged := emit.RefExpr(symbol.Identity{
				Lang:    golang.Lang,
				Package: auditweaver.DefaultPackage,
				Name:    auditweaver.DefaultFunc,
				Kind:    symbol.KindFunction,
			})
			for _, host := range []string{productRepo, productMock} {
				methods := woven[host]
				assert.NotEmpty(t, methods, host+" has methods")
				for _, m := range methods {
					expect.Equal(t, m.Body.Prologue.Items(), []emit.Stmt{{
						Kind: emit.StmtExpr,
						Value: emit.Expr{Kind: emit.ExprCall, Fn: &logged, Args: []emit.Expr{
							emit.ValueExpr(emit.Literal(emit.LiteralString, auditweaver.DefaultFormat)),
							emit.ValueExpr(emit.Literal(emit.LiteralString, productName+"."+m.Name)),
						}},
					}}, "the prologue of "+host+"."+m.Name+" logs its entry")
				}
			}
		})

		t.Run(
			"leaves the methods that derive from a declaration without the directive as they are",
			func(t *testing.T) {
				t.Parallel()

				woven := weave(t)
				for _, host := range []string{reviewRepo, storeMock} {
					for _, m := range woven[host] {
						expect.Empty(t, m.Body.Prologue.Items(), "the prologue of "+host+"."+m.Name+" has no statement")
					}
				}
			},
		)
	})
}

// catalog returns a fixture over the package catalog of the tree, loaded
// under ctx and stamped by repogen, with the target golang, the plugin
// auditweaver p, and the units of repogen and mockgen seeded.
func catalog(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:        referencetest.Tree,
		Packages:   []string{catalogPackage},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p, mockgen.New()},
		Annotators: []plugin.Annotator{repogen.New().(plugin.Annotator)},
		Hosts:      []plugin.Generator{repogen.New(), mockgen.New()},
	})
}

// weave runs auditweaver over the package catalog of the tree, and returns
// the methods of each struct that the hosts write, by the name of the
// struct.
func weave(t *testing.T) map[string][]*emit.Method {
	t.Helper()

	p := auditweaver.New()
	res := catalog(t.Context(), t, p).Generate(t, p)
	assert.NoError(t, res.Err, "the weaver runs whole")
	out := map[string][]*emit.Method{}
	for u := range res.Emit.Units() {
		for _, d := range u.Decls {
			if s, is := d.(*emit.Struct); is {
				out[s.Name] = s.Methods.Items()
			}
		}
	}
	return out
}
