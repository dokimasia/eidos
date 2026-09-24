// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"os"
	"testing/fstest"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The fixture's namespaces.
const (
	svcPkg = "svc.store"
	depPkg = "dep"
)

// setup is the suite's entry: protobuf's rules over the loaded
// schema tree.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()
	return protorules.New(),
		rulestest.Loaded(tb, protofrontend.New(), os.DirFS("testdata/schema"), protobuf.Keys)
}

// fixture is the loaded tree with a tracked view over it.
type fixture struct {
	*rulestest.Fixture
	view rules.View
}

// loaded returns the fixture with a tracked view.
func loaded(tb assert.TB) *fixture {
	tb.Helper()

	_, f := setup(tb)
	return viewed(tb, f)
}

// loadedFrom loads a tree of the case's own schemas, for a case that
// needs declarations the shared fixture does not state.
func loadedFrom(tb assert.TB, files map[string]string) *fixture {
	tb.Helper()

	tree := fstest.MapFS{}
	for path, body := range files {
		tree[path] = &fstest.MapFile{Data: []byte(body)}
	}
	return viewed(tb, rulestest.Loaded(tb, protofrontend.New(), tree, protobuf.Keys))
}

// viewed returns a loaded tree with a tracked view over it.
func viewed(tb assert.TB, f *rulestest.Fixture) *fixture {
	tb.Helper()

	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	return &fixture{
		Fixture: f,
		view:    rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys},
	}
}

// bound returns protobuf's rules bound over the fixture's view.
func (f *fixture) bound() rules.Bound { return rules.NewBound(protorules.New(), f.view, nil) }

// decl looks a declaration up by identity.
func (f *fixture) decl(tb assert.TB, want symbol.Identity) symbol.Symbol {
	tb.Helper()

	sym, held := f.Graph.Lookup(want)
	assert.True(tb, held, "the fixture declares "+want.String())
	return sym
}

// id returns a top-level identity in one namespace.
func id(pkg, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: pkg, Name: name, Kind: kind}
}

// ref returns a resolved reference to a fixture declaration.
func ref(pkg, name string, kind symbol.Kind) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Target: id(pkg, name, kind)}
}

// member returns the identity of a declaration nested in another.
func member(pkg, owner, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: pkg, Owner: owner, Name: name, Kind: kind}
}

// builtin returns an unresolved named reference: a scalar, or a
// message the workspace does not declare.
func builtin(spelling string) *node.TypeRef { return &node.TypeRef{Spelling: spelling} }

// composite returns a structural reference over children.
func composite(spelling string, form symbol.TypeForm, children ...*node.TypeRef) *node.TypeRef {
	return &node.TypeRef{Spelling: spelling, Form: form, Elems: children}
}
