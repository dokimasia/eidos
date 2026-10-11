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

// The tree of a constraint interface, its package, and the interface.
const (
	constraintTree    = "testdata/constraint"
	constraintPackage = "example.com/constraint"
	numberName        = "Number"
)

// The stub of the generic interface Cache of the tree, its call records,
// its type parameter, and the witness of the parameter in Go.
const (
	cacheStub    = "CacheStub"
	cacheGetCall = "CacheGetCall"
	cachePutCall = "CachePutCall"
	cacheParam   = "T"
	intWitness   = "int"
)

// The tree of generic interfaces, its Go package and its TypeScript module,
// and the stubs and call records of its interfaces.
const (
	genericTree    = "testdata/generic"
	genericPackage = "example.com/generic"
	bagPackage     = "web/bag"
	mapperPackage  = "web/mapper"
	mapperName     = "Mapper"
	shelfStub      = "ShelfStub"
	shelfFillCall  = "ShelfFillCall"
	shelfWrapCall  = "ShelfWrapCall"
	shelfTagCall   = "ShelfTagCall"
	labelsStub     = "LabelsStub"
	bagStub        = "BagStub"
	bagAddCall     = "BagAddCall"
	bagTakeCall    = "BagTakeCall"
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

		t.Run("reports Constraint at a constraint interface and records no stub of it", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:      constraintTree,
				Packages: []string{constraintPackage},
				Target:   golang.Target,
				Plugins:  []plugin.Plugin{p},
			})
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			found := slices.Collect(res.Sink.All())
			assert.Length(t, found, 1, "stubgen reports the one interface")
			expect.That(t, found[0].Code).Equal(double.Constraint, "the finding is Constraint")
			expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindInterface, "", numberName).Position().Line,
				"the finding is at the interface")
			stubs, tests := recorded(t, f, p)
			expect.Empty(t, stubs, "stubgen records no stub of Number")
			expect.Empty(t, tests, "stubgen records no test of Number")
		})

		t.Run("reports GenericMethod at an interface with a generic method and records no stub of it",
			func(t *testing.T) {
				t.Parallel()

				p := stubgen.New()
				f := generic(t.Context(), t, p, typescript.Target, mapperPackage)
				res := f.Generate(t, p)
				assert.NoError(t, res.Err, "the generator runs whole")
				found := slices.Collect(res.Sink.All())
				assert.Length(t, found, 1, "stubgen reports the one interface")
				expect.That(t, found[0].Code).Equal(stubgen.GenericMethod, "the finding is GenericMethod")
				expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindInterface, "", mapperName).Position().Line,
					"the finding is at the interface")
				stubs, tests := recorded(t, f, p)
				expect.Empty(t, stubs, "stubgen records no stub of Mapper")
				expect.Empty(t, tests, "stubgen records no test of Mapper")
			})

		t.Run("records the stub of a generic interface with the type parameters of the interface", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			stubs, _ := recorded(t, catalog(t.Context(), t, p), p)
			stub := stubs[cacheStub]
			assert.NotNil(t, stub, "stubgen records the stub of Cache")
			expect.That(t, typeParamNames(stub.TypeParams)).Equal(cacheParam, "the stub declares the T of Cache")
			expect.Equal(t, stub.Implements[0].Args, []*emit.TypeRef{{Spelling: cacheParam}},
				"the stub implements Cache at its own type parameter")
		})

		t.Run("records the test of a generic stub at the witnesses of its type parameters", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			_, tests := recorded(t, catalog(t.Context(), t, p), p)
			test := tests[cacheStub]
			assert.NotNil(t, test, "stubgen records the test of the stub of Cache")
			expect.That(t, typeParamNames(test.TypeParams)).Equal(cacheParam,
				"the test declares the T of Cache for its generic checks")
			expect.Equal(t, test.Implements[0].Args, []*emit.TypeRef{{Spelling: intWitness}},
				"the test checks the stub at int, the witness of T")
		})

		t.Run("records no test of a generic stub whose type parameter has no witness", func(t *testing.T) {
			t.Parallel()

			p := stubgen.New()
			stubs, tests := recorded(t, generic(t.Context(), t, p, golang.Target, genericPackage), p)
			expect.NotNil(t, stubs[labelsStub], "stubgen records the stub of Labels")
			expect.NotContains(t, tests, labelsStub, "int does not satisfy fmt.Stringer, so Labels has no witness")
		})

		cp, gp, wp := stubgen.New(), stubgen.New(), stubgen.New()
		catalogStubs, _ := recorded(t, catalog(t.Context(), t, cp), cp)
		goStubs, _ := recorded(t, generic(t.Context(), t, gp, golang.Target, genericPackage), gp)
		webStubs, _ := recorded(t, generic(t.Context(), t, wp, typescript.Target, bagPackage), wp)
		tests := []struct {
			name string
			stub *emit.Struct
			give string
			want string
		}{
			{
				name: "records the type parameter on the call record of a parameter of the type parameter",
				stub: catalogStubs[cacheStub], give: cachePutCall, want: cacheParam,
			},
			{
				name: "records the type parameter on the call record of a parameter whose type is a list of it",
				stub: goStubs[shelfStub], give: shelfFillCall, want: cacheParam,
			},
			{
				name: "records the type parameter on the call record of a parameter whose type is instantiated at it",
				stub: goStubs[shelfStub], give: shelfWrapCall, want: cacheParam,
			},
			{
				name: "records no type parameter on the call record of a parameter whose type name starts with it",
				stub: goStubs[shelfStub], give: shelfTagCall,
			},
			{
				name: "records no type parameter on the call record of a parameter of another type",
				stub: catalogStubs[cacheStub], give: cacheGetCall,
			},
			{
				name: "records no type parameter on the call record of a parameter without a type",
				stub: webStubs[bagStub], give: bagAddCall,
			},
			{
				name: "records no type parameter on the call record of a method without parameters",
				stub: webStubs[bagStub], give: bagTakeCall,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NotNil(t, tt.stub, "stubgen records the stub")
				assert.Equal(t, typeParamNames(callRecord(t, tt.stub, tt.give).TypeParams), tt.want,
					"the call record declares the type parameters that the types of its fields mention")
			})
		}
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

// generic returns a fixture over the package pkg of the tree of generic
// interfaces, loaded under ctx, with the plan's target and the plugin
// stubgen p.
func generic(
	ctx context.Context, tb assert.TB, p plugin.Plugin, target plugin.Target, pkg string,
) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:      genericTree,
		Packages: []string{pkg},
		Target:   target,
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

// typeParamNames returns the names of the type parameters, joined by
// spaces.
func typeParamNames(tps []*emit.TypeParam) string {
	out := make([]string, 0, len(tps))
	for _, tp := range tps {
		out = append(out, tp.Name)
	}
	return strings.Join(out, " ")
}

// callRecord returns the call record named name of the stub. It stops the
// test where the stub has no such record, or where the record is not a
// struct.
func callRecord(t *testing.T, stub *emit.Struct, name string) *emit.Struct {
	t.Helper()

	calls := stub.Types.Items()
	at := slices.IndexFunc(calls, func(d symbol.Symbol) bool { return emit.DeclaredName(d) == name })
	assert.NotEqual(t, at, -1, "the stub "+stub.Name+" has the call record "+name)
	call, isStruct := calls[at].(*emit.Struct)
	assert.True(t, isStruct, "the call record "+name+" is a struct")
	return call
}
