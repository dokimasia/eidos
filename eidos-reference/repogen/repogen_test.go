// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package repogen_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with the structs under the directive repo, the
// structs, and the repository and the implementation of Product.
const (
	catalogPackage = "example.com/shop/catalog"
	productName    = "Product"
	reviewName     = "Review"
	repositoryName = "ProductRepository"
	repoName       = "ProductRepo"
	bodyTemplate   = "repo.tmpl"
	receiverName   = "p"
)

// otherRegistrant is the registrant that claims the namespace of repogen
// before repogen registers its keys.
const otherRegistrant = "other"

// signatureOf is the spelling of the parameters and the results of one
// method.
type signatureOf struct {
	name    string
	params  []string
	returns []string
}

// repogen stamps the structs under its directive, writes a repository and
// an implementation of each, and passes the plugin suite.
func TestRepogen(t *testing.T) {
	t.Parallel()

	wanted := []signatureOf{
		{name: "Get", params: []string{"context.Context", "string"}, returns: []string{"*Product", "error"}},
		{name: "List", params: []string{"context.Context"}, returns: []string{"[]*Product", "error"}},
		{name: "Put", params: []string{"context.Context", "*Product"}, returns: []string{"error"}},
		{name: "Delete", params: []string{"context.Context", "string"}, returns: []string{"error"}},
	}

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := repogen.New()
				return p, catalog(ctx, tb, p).Fixture
			})
		})

		t.Run("stamps the fact on each struct under the directive", func(t *testing.T) {
			t.Parallel()

			p := repogen.New()
			f := catalog(t.Context(), t, p)
			assert.NoError(t, f.Annotate(t, p).Err, "the annotator runs whole")
			key, registered := meta.Lookup[bool](f.Keys, repogen.KeyRepository)
			assert.True(t, registered, "repogen registers its key")
			for _, name := range []string{productName, reviewName} {
				stamped, _ := meta.Get(f.Facts, f.Find(t, symbol.KindStruct, "", name).Identity(), key)
				expect.True(t, stamped, name+" is a struct under the directive")
			}
		})

		t.Run("writes a repository with the methods Get, List, Put and Delete", func(t *testing.T) {
			t.Parallel()

			repo, _ := generated(t)
			assert.Equal(t, repo.Name, repositoryName, "the repository joins its word onto the name of the struct")
			assert.Equal(t, signatures(repo.Methods.Items()), wanted, "the repository has the four methods")
		})

		t.Run("writes an implementation whose methods have a receiver and the template body", func(t *testing.T) {
			t.Parallel()

			_, zero := generated(t)
			assert.Equal(t, zero.Name, repoName, "the implementation joins its word onto the name of the struct")
			assert.Equal(
				t,
				signatures(zero.Methods.Items()),
				wanted,
				"the implementation has the methods of the repository",
			)
			for _, m := range zero.Methods.Items() {
				expect.Equal(t, m.Receiver.Name, receiverName, "the receiver of "+m.Name+" takes the first letter")
				expect.Equal(t, m.Body.Ref.Name, bodyTemplate, "the template writes the body of "+m.Name)
			}
		})

		t.Run("returns from Keys the error of a namespace that another registrant claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(otherRegistrant).ClaimNamespace(repogen.KeyRepository.Namespace()),
				"another registrant claims the namespace of repogen")
			keys, provides := repogen.New().(plugin.KeyProvider)
			assert.True(t, provides, "repogen registers keys")
			assert.HasError(t, keys.Keys(r.For(string(repogen.ID))), "repogen cannot claim its namespace")
		})
	})
}

// catalog returns a fixture over the package catalog of the tree, loaded
// under ctx, with the target golang and the plugin repogen p.
func catalog(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:      referencetest.Tree,
		Packages: []string{catalogPackage},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// generated runs repogen over the package catalog of the tree, and returns
// the repository and the implementation of Product.
func generated(t *testing.T) (*emit.Interface, *emit.Struct) {
	t.Helper()

	p := repogen.New()
	f := catalog(t.Context(), t, p)
	assert.NoError(t, f.Annotate(t, p).Err, "the annotator runs whole")
	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	var repo *emit.Interface
	var zero *emit.Struct
	for u := range res.Emit.Units() {
		for _, d := range u.Decls {
			switch decl := d.(type) {
			case *emit.Interface:
				if decl.Name == repositoryName {
					repo = decl
				}
			case *emit.Struct:
				if decl.Name == repoName {
					zero = decl
				}
			}
		}
	}
	assert.NotNil(t, repo, "repogen writes the repository of Product")
	assert.NotNil(t, zero, "repogen writes the implementation of Product")
	return repo, zero
}

// signatures returns the spelling of the parameters and the results of
// each method, in order.
func signatures(ms []*emit.Method) []signatureOf {
	out := make([]signatureOf, 0, len(ms))
	for _, m := range ms {
		s := signatureOf{name: m.Name}
		for _, p := range m.Params {
			s.params = append(s.params, p.Type.Spelling)
		}
		for _, r := range m.Returns {
			s.returns = append(s.returns, r.Type.Spelling)
		}
		out = append(out, s)
	}
	return out
}
