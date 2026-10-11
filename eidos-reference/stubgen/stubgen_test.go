// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stubgen_test

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
	"go.dokimi.dev/eidos/reference/stubgen"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the tree with the interface Store under the directive,
// the interface, its stub, and the templates of the two families.
const (
	catalogPackage = "example.com/shop/catalog"
	webPackage     = "web/store"
	storeName      = "Store"
	storeStub      = "StoreStub"
	stubTemplate   = "stub.tmpl"
	testTemplate   = "stub.test.tmpl"
)

// The names of the methods of the Go and the TypeScript stubs, joined by
// spaces, the names of the call records of the Go stub, and the names of
// the parameters of its methods.
const (
	goMethods    = "Get Put"
	webMethods   = "get put"
	getCall      = "StoreGetCall"
	putCall      = "StorePutCall"
	ctxParam     = "ctx"
	idParam      = "id"
	productParam = "p"
)

// The tree of an interface with a property, its package, and the
// interface.
const (
	propertyTree  = "testdata/property"
	shapedPackage = "web/shaped"
	shapedName    = "Shaped"
)

// stubgen records the stub of each interface under its directive into the
// family of the stubs, and the test of the stub into the family of the
// tests. It passes the plugin suite.
func TestStubgen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := stubgen.New()
				return p, catalog(ctx, tb, p).Fixture
			})
		})

		t.Run("passes the plugin suite over the TypeScript store of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := stubgen.New()
				return p, web(ctx, tb, p).Fixture
			})
		})

		t.Run("records the stub of an interface under the directive with a method for each method", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			stubs, _ := recorded(t, catalog(t.Context(), t, p), p)
			stub := stubs[storeStub]
			assert.NotNil(t, stub, "stubgen records the stub of Store")
			expect.That(t, methodNames(stub.Methods.Items())).Equal(goMethods, "the stub has the methods of Store")
			expect.That(t, stub.Implements[0].Spelling).Equal(storeName, "the stub implements Store")
		})

		t.Run("records a call record for each method of the stub", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			stubs, _ := recorded(t, catalog(t.Context(), t, p), p)
			stub := stubs[storeStub]
			assert.NotNil(t, stub, "stubgen records the stub of Store")
			calls := stub.Types.Items()
			assert.Length(t, calls, 2, "the stub has a call record for each method")
			get, isStruct := calls[0].(*emit.Struct)
			assert.True(t, isStruct, "the call record of Get is a struct")
			put, isStruct := calls[1].(*emit.Struct)
			assert.True(t, isStruct, "the call record of Put is a struct")
			expect.That(t, get.Name).Equal(getCall, "the first call record is the record of Get")
			expect.Equal(t, get.Fields.Items(), []*emit.Field{{Name: ctxParam}, {Name: idParam}},
				"the call record of Get has a field for each parameter of Get")
			expect.That(t, put.Name).Equal(putCall, "the second call record is the record of Put")
			expect.Equal(t, put.Fields.Items(), []*emit.Field{{Name: ctxParam}, {Name: productParam}},
				"the call record of Put has a field for each parameter of Put")
		})

		t.Run("records the test of the stub with the methods of the stub without a receiver", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			_, tests := recorded(t, catalog(t.Context(), t, p), p)
			test := tests[storeStub]
			assert.NotNil(t, test, "stubgen records the test of the stub of Store")
			methods := test.Methods.Items()
			expect.That(t, methodNames(methods)).Equal(goMethods, "the test has the methods of the stub")
			for _, m := range methods {
				expect.Nil(t, m.Receiver, "the method "+m.Name+" of the test has no receiver")
			}
		})

		t.Run("records the TypeScript stub of the TypeScript store", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			stubs, tests := recorded(t, web(t.Context(), t, p), p)
			assert.NotNil(t, stubs[storeStub], "stubgen records the stub of the TypeScript Store")
			expect.That(t, methodNames(stubs[storeStub].Methods.Items())).Equal(webMethods,
				"the stub has the methods of Store")
			expect.That(t, tests).Length(1, "stubgen records the test of the stub")
		})

		t.Run("reports Property at an interface with a property and records no stub of it", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:      propertyTree,
				Packages: []string{shapedPackage},
				Target:   typescript.Target,
				Plugins:  []plugin.Plugin{p},
			})
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			found := slices.Collect(res.Sink.All())
			assert.Length(t, found, 1, "stubgen reports the one interface")
			expect.That(t, found[0].Code).Equal(double.Property, "the finding is Property")
			expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindInterface, "", shapedName).Position().Line,
				"the finding is at the interface")
			stubs, tests := recorded(t, f, p)
			expect.Empty(t, stubs, "stubgen records no stub of Shaped")
			expect.Empty(t, tests, "stubgen records no test of Shaped")
		})
	})
}

// catalog returns a fixture over the package catalog of the tree, loaded
// under ctx, with the target golang and the plugin stubgen p.
func catalog(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:      referencetest.Tree,
		Packages: []string{catalogPackage},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// web returns a fixture over the TypeScript store of the tree, loaded under
// ctx, with the target typescript and the plugin stubgen p.
func web(ctx context.Context, tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:      referencetest.Tree,
		Packages: []string{webPackage},
		Target:   typescript.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// recorded runs stubgen p over the fixture f. It returns the stubs and the
// tests that stubgen records, each by name. It checks that each unit claims
// its files through the template of its family.
func recorded(t *testing.T, f *referencetest.Fixture, p plugin.Plugin) (stubs, tests map[string]*emit.Struct) {
	t.Helper()

	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	stubs, tests = map[string]*emit.Struct{}, map[string]*emit.Struct{}
	for u := range res.Emit.Units() {
		into, want := stubs, stubTemplate
		if u.Tag == stubgen.TestTag {
			into, want = tests, testTemplate
		}
		expect.That(t, u.Template).Equal(want, "the family claims its files through its template")
		for _, d := range u.Decls {
			record := d.(*emit.Struct)
			into[record.Name] = record
		}
	}
	return stubs, tests
}

// methodNames returns the names of the methods, joined by spaces.
func methodNames(ms []*emit.Method) string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return strings.Join(out, " ")
}
