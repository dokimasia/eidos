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
		PluginSet: []byte("rulestest"),
	})
	assert.NoError(tb, err, "the scripted tree loads")
	registry := meta.NewRegistry()
	keys, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel keys register")
	return rulestest.Scripted(), &rulestest.Fixture{Graph: g, Facts: meta.NewFacts(registry), Keys: keys}
}

// The loaded fixture is the suite's entry for a satellite: a tree
// through its frontend, the kernel's keys and the load's stamps.
func TestLoaded(t *testing.T) {
	t.Parallel()

	t.Run("Loaded", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a tree into a fixture with the kernel's keys and the load's stamps", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, frontendtest.NewScripted(), scriptedTree(), frontendtest.ScriptedKeys)
			assert.NotNil(t, f.Graph, "the graph loaded")
			assert.True(t, f.Graph.Frozen(), "and sealed")
			assert.False(t, f.Keys.IsZero(), "the kernel's keys registered")
			assert.NotNil(t, f.Facts, "over a fact store")
			assert.NotNil(t, f.Facts.Registry(), "whose registry the language's keys joined")
		})
	})
}
