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
	"go.dokimi.dev/eidos/sdk"
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

// The tree of a constraint interface, its package, and the interface.
const (
	constraintTree    = "testdata/constraint"
	constraintPackage = "example.com/constraint"
	numberName        = "Number"
)

// The mock of the generic interface Cache of the tree, and the type
// parameter of Cache.
const (
	cacheMock  = "CacheMock"
	cacheParam = "T"
)

// hostID is the plugin that emits the generic interface Shelf beside Store.
const hostID plugin.ID = "shelfhost"

// The word of the host, the interface Shelf, its mock, its type parameter,
// and its method Put with the parameter of Put.
const (
	hostWord   = "shelf"
	shelfName  = "Shelf"
	shelfMock  = "ShelfMock"
	shelfParam = "V"
	putName    = "Put"
	valueParam = "v"
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

		t.Run("reports Constraint at a constraint interface and records no mock of it", func(t *testing.T) {
			t.Parallel()

			p := mockgen.New()
			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:      constraintTree,
				Packages: []string{constraintPackage},
				Target:   golang.Target,
				Plugins:  []plugin.Plugin{p},
			})
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			found := slices.Collect(res.Sink.All())
			assert.Length(t, found, 1, "mockgen reports the one interface")
			expect.That(t, found[0].Code).Equal(double.Constraint, "the finding is Constraint")
			expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindInterface, "", numberName).Position().Line,
				"the finding is at the interface")
			assert.Empty(t, recorded(t, f, p), "mockgen records no mock of Number")
		})

		t.Run("records the mock of a generic interface with the type parameters of the interface", func(t *testing.T) {
			t.Parallel()

			p := mockgen.New()
			mock := recorded(t, catalog(t.Context(), t, p), p)[cacheMock]
			assert.NotNil(t, mock, "mockgen records the mock of Cache")
			expect.That(t, typeParamNames(mock.TypeParams)).Equal(cacheParam, "the mock declares the T of Cache")
			expect.Equal(t, mock.Implements[0].Args, []*emit.TypeRef{{Spelling: cacheParam}},
				"the mock implements Cache at its own type parameter")
		})

		t.Run("records the mock of an emitted generic interface with the type parameters of the interface",
			func(t *testing.T) {
				t.Parallel()

				p := mockgen.New()
				f := referencetest.New(t.Context(), t, referencetest.Spec{
					Dir:      referencetest.Tree,
					Packages: []string{catalogPackage},
					Target:   golang.Target,
					Plugins:  []plugin.Plugin{p},
					Hosts:    []plugin.Generator{shelfHost()},
				})
				mock := recorded(t, f, p)[shelfMock]
				assert.NotNil(t, mock, "mockgen records the mock of the emitted Shelf")
				expect.That(t, typeParamNames(mock.TypeParams)).Equal(shelfParam, "the mock declares the V of Shelf")
				expect.Equal(t, mock.Implements[0].Args, []*emit.TypeRef{{Spelling: shelfParam}},
					"the mock implements Shelf at its own type parameter")
			})
	})
}

// shelfHost returns a generator that emits the generic interface Shelf for
// the interface Store of the source, as repogen emits a repository for a
// struct. Shelf has the method Put, which takes a value of its type
// parameter.
func shelfHost() plugin.Generator {
	g, _ := sdk.NewPlugin(hostID).
		Output(plugin.Output{Per: plugin.PerSource, Word: hostWord}).
		Handle(sdk.OnInterface(func(m *sdk.InterfaceMatch, e *sdk.Emitter) error {
			if m.Interface.Name != storeName {
				return nil
			}
			shelf := &emit.Interface{
				Origin:     m.Interface.ID,
				Name:       shelfName,
				TypeParams: []*emit.TypeParam{{Name: shelfParam}},
			}
			shelf.Methods.Append(&emit.Method{
				Name:   putName,
				Params: []*emit.Param{{Name: valueParam, Type: &emit.TypeRef{Spelling: shelfParam}}},
			})
			e.File().Append(shelf)
			return nil
		})).
		Build().(plugin.Generator)
	return g
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

// typeParamNames returns the names of the type parameters, joined by
// spaces.
func typeParamNames(tps []*emit.TypeParam) string {
	out := make([]string, 0, len(tps))
	for _, tp := range tps {
		out = append(out, tp.Name)
	}
	return strings.Join(out, " ")
}
