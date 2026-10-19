// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	graphqlfrontend "go.dokimi.dev/eidos/lang/graphql/frontend"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the fixture tree: the package of the schema of the shop
// and the package of its documents.
const (
	shopPkg = "shop"
	docsPkg = "shop/documents"
)

// The declarations of the fixture tree that several specs read: the object
// type Book, the enum Status with three values, and the custom scalars.
// Long, Base64String and DateTime state a URL of the table, and Upload
// states none.
var (
	bookID   = symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Book", Kind: symbol.KindStruct}
	statusID = symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Status", Kind: symbol.KindEnum}
	longID   = symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Long", Kind: symbol.KindAlias}
	base64ID = symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Base64String", Kind: symbol.KindAlias}
	dateID   = symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "DateTime", Kind: symbol.KindAlias}
	uploadID = symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Upload", Kind: symbol.KindAlias}
)

// allocCall is one call that an allocation test and a benchmark share: the
// method that it calls, which is the name of its benchmark, the case that it
// measures where the method has more than one call, its ceiling, the call,
// and the check of the result of the call. A case whose ceiling only a
// benchmark checks sets bench, which measures the case in place of the
// call, and no list that an allocation test reads contains it.
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

// setup is the entry of the suite: the GraphQL rules over the loaded
// fixture tree.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()

	return graphqlrules.New(), rulestest.Loaded(tb, graphqlfrontend.New(), os.DirFS("testdata/schema"))
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
