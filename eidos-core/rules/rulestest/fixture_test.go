// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
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

// These constants are the scripted tree of the suite. It has a struct
// whose fields the language values, one field typed by another struct so
// that a reference resolves, a method, a constant, and a generic type
// whose field has the type of its type parameter.
const (
	apiFile   = "svc/api/user.zz"
	storeFile = "svc/store/row.zz"
	apiSource = "package svc/api\ntype User string\nmethod Get int\ntype Box T\ntypeparam T\n"
	rowSource = "package svc/store\nimport api svc/api\ntype Row api.User int string\nmethod Put int\nconst rowmax\n"
)

// These constants are the tree whose one reference resolves to two
// declarations. The alias dual binds two packages that both declare
// Thing.
const (
	leftFile     = "a/left/l.zz"
	rightFile    = "a/right/r.zz"
	holderFile   = "svc/hold/h.zz"
	leftSource   = "package a/left\ntype Thing string\n"
	rightSource  = "package a/right\ntype Thing string\n"
	holderSource = "package svc/hold\nimport dual a/left a/right\ntype Holder dual.Thing\n"
)

// keyedScripted is the scripted frontend that registers its
// classification keys through its own role.
type keyedScripted struct{ *frontendtest.Scripted }

// Keys registers the scripted classification keys.
func (keyedScripted) Keys(r *meta.Registry) error { return frontendtest.ScriptedKeys(r) }

// Loaded is the entry of a satellite to the suite. It loads a tree
// through the frontend of the satellite, and files the stamps of the
// load under the keys of the kernel.
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

		t.Run("registers nothing for a nil function of keys", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, frontendtest.NewScripted(), scriptedTree(), nil)
			_, held := f.Facts.Registry().Resolve(frontendtest.ScriptedTestKey)
			assert.False(t, held, "the language's key is not registered")
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

		t.Run("returns the Warning of an ambiguous reference", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, frontendtest.NewScripted(), fstest.MapFS{
				leftFile:   {Data: []byte(leftSource)},
				rightFile:  {Data: []byte(rightSource)},
				holderFile: {Data: []byte(holderSource)},
			})
			assert.Length(t, f.Findings, 1, "the load reports one finding")
			assert.Equal(t, f.Findings[0].Code, load.AmbiguousReference,
				"the finding is the ambiguity of the reference to Thing")
		})

		t.Run("returns no finding for a tree that loads clean", func(t *testing.T) {
			t.Parallel()

			f := rulestest.Loaded(t, frontendtest.NewScripted(), scriptedTree(), frontendtest.ScriptedKeys)
			assert.Empty(t, f.Findings, "the load of the scripted tree reports nothing")
		})
	})
}

// scriptedTree returns the scripted tree.
func scriptedTree() fstest.MapFS {
	return fstest.MapFS{
		apiFile:   {Data: []byte(apiSource)},
		storeFile: {Data: []byte(rowSource)},
	}
}

// setup loads the scripted tree under the context of tb, and returns the
// scripted rules with a fixture whose registry contains the kernel's
// keys. Each call loads the tree again.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(assert.Context(tb), load.Config{
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
