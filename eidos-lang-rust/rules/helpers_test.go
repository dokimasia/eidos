// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"os"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/rust"
	rustfrontend "go.dokimi.dev/eidos/lang/rust/frontend"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the fixture crate: the crate root and its modules.
const (
	shopPkg   = "shop"
	modelsPkg = "shop/models"
	errorsPkg = "shop/errors"
	storePkg  = "shop/store"
	utilPkg   = "shop/util"
)

// The packages of the standard library that the specs build references to.
const (
	collectionsPkg = "std/collections"
	timePkg        = "std/time"
)

// The declarations of the fixture crate that several specs read.
var (
	orderID    = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Order", Kind: symbol.KindStruct}
	itemID     = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Item", Kind: symbol.KindStruct}
	idID       = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Id", Kind: symbol.KindStruct}
	statusID   = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Status", Kind: symbol.KindEnum}
	colorID    = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Color", Kind: symbol.KindEnum}
	shapeID    = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Shape", Kind: symbol.KindSum}
	pageID     = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Page", Kind: symbol.KindStruct}
	settingsID = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Settings", Kind: symbol.KindStruct}
	guardedID  = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Guarded", Kind: symbol.KindStruct}
	treeID     = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Tree", Kind: symbol.KindSum}
	nothingID  = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Nothing", Kind: symbol.KindStruct}
	storeID    = symbol.Identity{Lang: rust.Lang, Package: storePkg, Name: "Store", Kind: symbol.KindInterface}
	memoryID   = symbol.Identity{Lang: rust.Lang, Package: storePkg, Name: "Memory", Kind: symbol.KindStruct}
	resultID   = symbol.Identity{Lang: rust.Lang, Package: errorsPkg, Name: "Result", Kind: symbol.KindAlias}
	goneID     = symbol.Identity{Lang: rust.Lang, Package: "nowhere", Name: "Gone", Kind: symbol.KindStruct}
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

// fixture is the loaded crate with a tracked view over it, and the read set
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

// method returns the method of the name that the type id of the fixture
// declares.
func (f *fixture) method(tb assert.TB, id symbol.Identity, name string) *node.Method {
	tb.Helper()

	var methods []*node.Method
	switch d := f.decl(tb, id).(type) {
	case *node.Struct:
		methods = d.Methods
	case *node.Interface:
		methods = d.Methods
	}
	at := slices.IndexFunc(methods, func(m *node.Method) bool { return m.Name == name })
	assert.NotEqual(tb, at, -1, id.Name+" declares the method "+name)
	return methods[at]
}

// field returns the field of the name that the struct id of the fixture
// declares.
func (f *fixture) field(tb assert.TB, id symbol.Identity, name string) *node.Field {
	tb.Helper()

	s, is := f.decl(tb, id).(*node.Struct)
	assert.True(tb, is, id.Name+" is a struct")
	at := slices.IndexFunc(s.Fields, func(fl *node.Field) bool { return fl.Name == name })
	assert.NotEqual(tb, at, -1, id.Name+" declares the field "+name)
	return s.Fields[at]
}

// setup is the entry of the suite: the Rust rules over the fixture crate.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()

	return rustrules.New(), rulestest.Loaded(tb, rustfrontend.New(nil), os.DirFS("testdata/tree"))
}

// loaded returns the fixture crate with a tracked view over it.
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
