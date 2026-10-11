// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package mockgen_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/reference/internal/double"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/mockgen"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the tree with interfaces to mock, the interface under
// the directive and the struct of the repositories of repogen, the mocks,
// and the template of the family.
const (
	catalogPackage = "example.com/shop/catalog"
	webPackage     = "web/store"
	storeName      = "Store"
	storeMock      = "StoreMock"
	productName    = "Product"
	productMock    = "ProductRepositoryMock"
	reviewMock     = "ReviewRepositoryMock"
	fileTemplate   = "mock.tmpl"
)

// The names of the methods of the mocks of the Go Store, of the TypeScript
// Store and of the repository of Product, joined by spaces.
const (
	goMethods      = "Get Put"
	webMethods     = "get put"
	productMethods = "Get List Put Delete"
)

// The tree of an interface with a property, its package, and the
// interface.
const (
	propertyTree  = "testdata/property"
	shapedPackage = "web/shaped"
	shapedName    = "Shaped"
)

// mockgen records a mock of each interface under its directive and of each
// emitted interface into its claimed family, and passes the plugin suite.
func TestMockgen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := mockgen.New()
				return p, catalog(ctx, tb, p).Fixture
			})
		})

		t.Run("passes the plugin suite over the TypeScript store of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := mockgen.New()
				return p, web(ctx, tb, p).Fixture
			})
		})

		t.Run("records the mock of an interface under the directive with a method for each method", func(t *testing.T) {
			t.Parallel()

			p := mockgen.New()
			mocks := recorded(t, catalog(t.Context(), t, p), p)
			mock := mocks[storeMock]
			assert.NotNil(t, mock, "mockgen records the mock of Store")
			expect.That(t, names(mock.Methods.Items())).Equal(goMethods, "the mock has the methods of Store")
			expect.That(t, mock.Implements[0].Spelling).Equal(storeName, "the mock implements Store")
		})

		t.Run("records the mock of an emitted repository with the origin of the repository", func(t *testing.T) {
			t.Parallel()

			p := mockgen.New()
			f := catalog(t.Context(), t, p)
			mock := recorded(t, f, p)[productMock]
			assert.NotNil(t, mock, "mockgen records the mock of the repository of Product")
			expect.That(t, mock.Origin).Equal(f.Find(t, symbol.KindStruct, "", productName).Identity(),
				"the mock derives from Product")
			expect.That(t, names(mock.Methods.Items())).
				Equal(productMethods, "the mock has the methods of the repository")
		})

		t.Run("records no mock of the repository of a struct under skip for mockgen", func(t *testing.T) {
			t.Parallel()

			p := mockgen.New()
			assert.NotContains(t, recorded(t, catalog(t.Context(), t, p), p), reviewMock,
				"the skip on Review excludes its repository")
		})

		t.Run("records the TypeScript mock of the TypeScript store", func(t *testing.T) {
			t.Parallel()

			p := mockgen.New()
			mock := recorded(t, web(t.Context(), t, p), p)[storeMock]
			assert.NotNil(t, mock, "mockgen records the mock of the TypeScript Store")
			expect.That(t, names(mock.Methods.Items())).Equal(webMethods, "the mock has the methods of Store")
		})

		t.Run("reports Property at an interface with a property and records no mock of it", func(t *testing.T) {
			t.Parallel()

			p := mockgen.New()
			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:      propertyTree,
				Packages: []string{shapedPackage},
				Target:   typescript.Target,
				Plugins:  []plugin.Plugin{p},
			})
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			found := slices.Collect(res.Sink.All())
			assert.Length(t, found, 1, "mockgen reports the one interface")
			expect.That(t, found[0].Code).Equal(double.Property, "the finding is Property")
			expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindInterface, "", shapedName).Position().Line,
				"the finding is at the interface")
			assert.Empty(t, recorded(t, f, p), "mockgen records no mock of Shaped")
		})
	})
}

// catalog returns a fixture over the package catalog of the tree, loaded
// under ctx and stamped by repogen, with the target golang, the plugin
// mockgen p, and the units of repogen seeded.
func catalog(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:        referencetest.Tree,
		Packages:   []string{catalogPackage},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p},
		Annotators: []plugin.Annotator{repogen.New().(plugin.Annotator)},
		Hosts:      []plugin.Generator{repogen.New()},
	})
}

// web returns a fixture over the TypeScript store of the tree, loaded under
// ctx, with the target typescript and the plugin mockgen p.
func web(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:      referencetest.Tree,
		Packages: []string{webPackage},
		Target:   typescript.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// recorded runs mockgen p over the fixture f, and returns the mocks that it
// records by name. It checks that each unit of mockgen claims its files
// through the file template, under the test tag.
func recorded(t *testing.T, f *referencetest.Fixture, p plugin.Plugin) map[string]*emit.Struct {
	t.Helper()

	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	mocks := map[string]*emit.Struct{}
	for u := range res.Emit.Units() {
		if u.Plugin != mockgen.ID {
			continue
		}
		expect.That(t, u.Template).Equal(fileTemplate, "the family claims its files through the template")
		expect.That(t, u.Tag).Equal(mockgen.TestTag, "the family routes into the test files")
		for _, d := range u.Decls {
			mock := d.(*emit.Struct)
			mocks[mock.Name] = mock
		}
	}
	return mocks
}

// names returns the names of the methods, joined by spaces.
func names(ms []*emit.Method) string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return strings.Join(out, " ")
}
