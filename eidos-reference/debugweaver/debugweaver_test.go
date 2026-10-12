// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package debugweaver_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/debugweaver"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/mockgen"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with the structs under the directive repo, the
// declarations that the hosts write for them, and the mock of the store.
const (
	catalogPackage = "example.com/shop/catalog"
	productRepo    = "ProductRepo"
	reviewRepo     = "ReviewRepo"
	productMock    = "ProductRepositoryMock"
	storeMock      = "StoreMock"
	productName    = "Product"
	reviewName     = "Review"
)

// debugweaver logs the entry of each method with a receiver that derives
// from a repository, and passes the plugin suite.
func TestDebugweaver(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the repositories of the package catalog", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := debugweaver.New()
				return p, catalog(tb, p).Fixture
			})
		})

		t.Run("logs the entry of each method of an implementation and of a mock of a repository", func(t *testing.T) {
			t.Parallel()

			woven := weave(t)
			logged := emit.RefExpr(symbol.Identity{
				Lang:    golang.Lang,
				Package: debugweaver.DefaultPackage,
				Name:    debugweaver.DefaultFunc,
				Kind:    symbol.KindFunction,
			})
			for _, host := range []string{productRepo, productMock, reviewRepo} {
				methods := woven[host]
				assert.NotEmpty(t, methods, host+" has methods")
				origin := productName
				if host == reviewRepo {
					origin = reviewName
				}
				for _, m := range methods {
					expect.Equal(t, m.Body.Prologue.Items(), []emit.Stmt{{
						Kind: emit.StmtExpr,
						Value: emit.Expr{Kind: emit.ExprCall, Fn: &logged, Args: []emit.Expr{
							emit.ValueExpr(emit.Literal(emit.LiteralString, debugweaver.DefaultFormat)),
							emit.ValueExpr(emit.Literal(emit.LiteralString, origin+"."+m.Name)),
						}},
					}}, "the prologue of "+host+"."+m.Name+" logs its entry")
				}
			}
		})

		t.Run("leaves the methods of a double of an interface that is no repository as they are", func(t *testing.T) {
			t.Parallel()

			for _, m := range weave(t)[storeMock] {
				expect.Empty(t, m.Body.Prologue.Items(), "the prologue of "+storeMock+"."+m.Name+" has no statement")
			}
		})
	})
}

// catalog returns a fixture over the package catalog of the tree, loaded
// under the context of tb and stamped by repogen, with the target golang,
// the plugin debugweaver p, and the units of repogen and mockgen seeded.
func catalog(tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(tb, referencetest.Spec{
		Dir:        referencetest.Tree,
		Packages:   []string{catalogPackage},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p, mockgen.New()},
		Annotators: []plugin.Annotator{repogen.New().(plugin.Annotator)},
		Hosts:      []plugin.Generator{repogen.New(), mockgen.New()},
	})
}

// weave runs debugweaver over the package catalog of the tree, and returns
// the methods of each struct that the hosts write, by the name of the
// struct.
func weave(t *testing.T) map[string][]*emit.Method {
	t.Helper()

	p := debugweaver.New()
	res := catalog(t, p).Generate(t, p)
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
