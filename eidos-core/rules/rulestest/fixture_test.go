// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
)

// The scripted tree the suite runs over: a struct whose fields the
// language values, one typed by another struct so a reference
// resolves, a method, a constant, and a generic type whose field
// names its type parameter.
const (
	apiFile   = "svc/api/user.zz"
	storeFile = "svc/store/row.zz"
	apiSource = "package svc/api\ntype User string\nmethod Get int\ntype Box T\ntypeparam T\n"
	rowSource = "package svc/store\nimport api svc/api\ntype Row api.User int string\nmethod Put int\nconst rowmax\n"
)

// keyedScripted is the scripted frontend that registers its
// classification keys through its own role.
type keyedScripted struct{ *frontendtest.Scripted }

// Keys registers the scripted classification keys.
func (keyedScripted) Keys(r *meta.Registry) error { return frontendtest.ScriptedKeys(r) }

// The loaded fixture is the suite's entry for a satellite: a tree
// through its frontend, the kernel's keys and the load's stamps.
func TestLoaded(t *testing.T) {
	t.Parallel()

	t.Run("Loaded", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a sealed graph of the tree", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, frontendtest.NewScripted(), scriptedTree(), frontendtest.ScriptedKeys)
			assert.NotNil(t, f.Graph, "the graph is loaded")
			assert.True(t, f.Graph.Frozen(), "the graph is sealed")
		})

		t.Run("registers the kernel's keys", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, frontendtest.NewScripted(), scriptedTree(), frontendtest.ScriptedKeys)
			assert.False(t, f.Keys.IsZero(), "the kernel's keys are registered")
		})

		t.Run("registers the fixture's keys", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, frontendtest.NewScripted(), scriptedTree(), frontendtest.ScriptedKeys)
			_, held := f.Facts.Registry().Resolve(frontendtest.ScriptedTestKey)
			assert.True(t, held, "the language's key is registered")
		})

		t.Run("registers the frontend's keys under its language's spelling", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, keyedScripted{frontendtest.NewScripted()}, scriptedTree())
			claimant, claimed := f.Facts.Registry().Claimant(frontendtest.ScriptedTestKey.Namespace())
			assert.True(t, claimed, "the frontend claims its namespace")
			assert.Equal(t, claimant, string(frontendtest.ScriptedLang), "the language's spelling claims the namespace")
		})

		t.Run("reads a carrier written under the suite's brand", func(t *testing.T) {
			t.Parallel()

			tree := scriptedTree()
			tree[apiFile].Data = []byte("package svc/api\n// +" + string(frontendtest.Brand) +
				":table name=users\ntype User string\n")
			f := rulestest.Loaded(t, frontendtest.NewScripted(), tree, frontendtest.ScriptedKeys)
			attached := 0
			for _, raws := range f.Graph.Directives() {
				attached += len(raws)
			}
			assert.Equal(t, attached, 1, "the carrier attaches")
		})
	})
}

// scriptedTree returns the tree.
func scriptedTree() fstest.MapFS {
	return fstest.MapFS{
		apiFile:   {Data: []byte(apiSource)},
		storeFile: {Data: []byte(rowSource)},
	}
}

// setup loads the tree and binds the scripted rules over it with
// the kernel's keys registered, fresh per call.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(context.Background(), load.Config{
		FS:        scriptedTree(),
		Frontends: []plugin.Frontend{frontendtest.NewScripted()},
		Sink:      sink,
		Brand:     frontendtest.Brand,
	})
	assert.NoError(tb, err, "the scripted tree loads")
	registry := meta.NewRegistry()
	keys, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel keys register")
	return rulestest.Scripted(), &rulestest.Fixture{Graph: g, Facts: meta.NewFacts(registry), Keys: keys}
}
