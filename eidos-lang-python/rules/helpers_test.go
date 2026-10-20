// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"os"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	pythonfrontend "go.dokimi.dev/eidos/lang/python/frontend"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the fixture tree: the package shop, its models and its API.
const (
	shopPkg   = "shop"
	modelsPkg = "shop/models"
	apiPkg    = "shop/api"
)

// The modules whose classes and forms the rules classify, as package paths.
const (
	builtinsModule   = "builtins"
	typingModule     = "typing"
	typingExtensions = "typing_extensions"
	abcModule        = "collections/abc"
	collectionsMod   = "collections"
	datetimeModule   = "datetime"
)

// literalSpelling is the spelling of the form Literal, which the specs of the
// shapes and of the values build references to.
const literalSpelling = "Literal"

// The declarations of the fixture tree that several specs read.
var (
	orderID   = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Order", Kind: symbol.KindStruct}
	itemID    = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Item", Kind: symbol.KindStruct}
	statusID  = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Status", Kind: symbol.KindEnum}
	catalogID = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Catalog", Kind: symbol.KindStruct}
	innerID   = symbol.Identity{
		Lang:    python.Lang,
		Package: modelsPkg,
		Owner:   "Outer",
		Name:    "Inner",
		Kind:    symbol.KindStruct,
	}
	fetchID = symbol.Identity{
		Lang:    python.Lang,
		Package: apiPkg,
		Name:    "fetch",
		Kind:    symbol.KindFunction,
		Disc:    "int",
	}
	goneID = symbol.Identity{Lang: python.Lang, Package: "nowhere", Name: "Gone", Kind: symbol.KindStruct}
)

// allocCall is one call that an allocation test and a benchmark share: the
// method that it calls, which is the name of its benchmark, the case that it
// measures where the method has more than one call, its ceiling, the call,
// and the check of the result of the call. A case whose ceiling only a
// benchmark checks sets bench, which measures the case in place of the call,
// and no list that an allocation test reads contains it.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
	bench    func(b *testing.B)
}

// fixture is the loaded tree with a tracked view over it, and the read set
// into which the view records.
type fixture struct {
	*rulestest.Fixture
	view  rules.View
	reads *store.ReadSet
}

// decl returns the declaration of the fixture with the identity id.
func (f *fixture) decl(tb assert.TB, id symbol.Identity) symbol.Symbol {
	tb.Helper()

	sym, held := f.Graph.Lookup(id)
	assert.True(tb, held, "the fixture declares "+id.String())
	return sym
}

// member returns the field of the name that the class id of the fixture
// declares.
func (f *fixture) member(tb assert.TB, id symbol.Identity, name string) *node.Field {
	tb.Helper()

	s, is := f.decl(tb, id).(*node.Struct)
	assert.True(tb, is, id.Name+" is a class")
	at := slices.IndexFunc(s.Fields, func(fl *node.Field) bool { return fl.Name == name })
	assert.NotEqual(tb, at, -1, id.Name+" declares the field "+name)
	return s.Fields[at]
}

// resolvedType resolves the spelling as a type in the scope, and returns the
// identity of the declaration that it resolves to.
func (f *fixture) resolvedType(tb assert.TB, scope rules.Scope, name string) symbol.Identity {
	tb.Helper()

	got, err := pythonrules.Rules{}.Resolve(scope, name, directive.ResolveTypeInScope, f.view)
	assert.NoError(tb, err, name+" resolves")
	decl, named := got.(node.Declaration)
	assert.True(tb, named, "the result is a declaration")
	return decl.Identity()
}

// file returns the file of the fixture at the path, in the package pkg.
func (f *fixture) file(tb assert.TB, pkg, path string) *node.File {
	tb.Helper()

	file, is := f.decl(tb, symbol.Identity{Lang: python.Lang, Package: pkg, Name: path, Kind: symbol.KindFile}).(*node.File)
	assert.True(tb, is, path+" is a file")
	return file
}

// setup is the entry of the suite: the Python rules over the fixture tree,
// which loads with the standard library of typeshed and without a virtual
// environment.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()

	stores, err := pythonfrontend.Stores(func(string) string { return "" })
	assert.NoError(tb, err, "the stores of the frontend open")
	return pythonrules.New(), rulestest.LoadedWith(tb, pythonfrontend.New(nil), os.DirFS("testdata/tree"), stores)
}

// loaded returns the fixture tree with a tracked view over it.
func loaded(tb assert.TB) *fixture {
	tb.Helper()

	_, f := setup(tb)
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph returns a reader")
	return &fixture{Fixture: f, view: rules.NewView(reader, f.Facts, reads, f.Keys), reads: reads}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and the
// result of each call.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its ceiling:
// one sub-benchmark for each method, in the order of the first call of each
// method, and inside it one for each case of a method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if _, seen := byMethod[c.name]; !seen {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling, and
// checks the result of the last call. The contract warms up with one call,
// so the count leaves out what the first call initialises. A case that sets
// bench runs it instead.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	if tt.bench != nil {
		tt.bench(b)
		return
	}
	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// paramNames returns the names of the parameters of a callable, in order.
func paramNames(c rules.Callable) []string {
	out := make([]string, 0, len(c.Params))
	for _, p := range c.Params {
		out = append(out, p.Name)
	}
	return out
}

// paramOptional returns whether a call can leave out each parameter of a
// callable, in order.
func paramOptional(c rules.Callable) []bool {
	out := make([]bool, 0, len(c.Params))
	for _, p := range c.Params {
		out = append(out, p.Optional)
	}
	return out
}

// paramPassing returns the way a call passes each argument of a callable, in
// order.
func paramPassing(c rules.Callable) []symbol.Passing {
	out := make([]symbol.Passing, 0, len(c.Params))
	for _, p := range c.Params {
		out = append(out, p.Passing)
	}
	return out
}
