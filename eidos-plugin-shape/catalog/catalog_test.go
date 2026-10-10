// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
)

// The names of the declarations of the fixtures of the package.
const (
	storeName    = "Store"
	beginName    = "Begin"
	commitName   = "Commit"
	rollbackName = "Rollback"
	putName      = "Put"
	getName      = "Get"
)

// The catalog composes as two annotators that pass the conformance checks
// of a plugin.
func TestCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Annotators", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the plugin shape and the plugin shapecheck", func(t *testing.T) {
			t.Parallel()

			got := catalog.Annotators()
			assert.Length(t, got, 2, "the catalog has two annotators")
			expect.Equal(t, got[0].Name(), catalog.ShapeID, "the first annotator is the plugin shape")
			expect.Equal(t, got[1].Name(), catalog.CheckID, "the second annotator is the plugin shapecheck")
		})

		t.Run("returns new plugins on each call", func(t *testing.T) {
			t.Parallel()

			first, second := catalog.Annotators(), catalog.Annotators()
			expect.NotEqual(t, first[0], second[0], "two calls return two plugins shape", assert.ByIdentity())
			expect.NotEqual(t, first[1], second[1], "two calls return two plugins shapecheck", assert.ByIdentity())
		})

		t.Run("returns plugins that declare no template helper", func(t *testing.T) {
			t.Parallel()

			for _, a := range catalog.Annotators() {
				name := string(a.Name())
				templates, provides := a.(plugin.TemplateProvider)
				assert.True(t, provides, "the plugin "+name+" states its presentation")
				expect.Empty(t, templates.TemplateTargets(), "the plugin "+name+" declares no template tree")
				expect.Empty(t, templates.TemplateFuncs(plugin.Target(shapetest.Lang)),
					"the plugin "+name+" declares no template helper")
			}
		})

		t.Run("returns a plugin shape that passes the conformance checks of a plugin", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				r := suiteRun(tb)
				return r.Annotators[0], r.Fixture
			})
		})

		t.Run("returns a plugin shapecheck that passes the conformance checks of a plugin", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				r := suiteRun(tb)
				res := r.Annotate(tb, r.Annotators[0])
				assert.NoError(tb, res.Err, "the plugin shape classifies the fixture")
				return r.Annotators[1], r.Fixture
			})
		})
	})
}

// suiteRun returns the run of the conformance checks. Its package declares
// a transaction whose instance is complete, a detected writer and reader,
// and a method with a mixin, so a run of either plugin reports no finding.
func suiteRun(tb assert.TB) *shapetest.Run {
	tb.Helper()

	str := &node.TypeRef{Spelling: shapetest.String}
	failure := &node.TypeRef{Spelling: shapetest.Error}
	begin := shapetest.Method(storeName, beginName, nil, failure)
	commit := shapetest.Method(storeName, commitName, nil, failure)
	rollback := shapetest.Method(storeName, rollbackName, nil, failure)
	put := shapetest.Method(storeName, putName, []*node.TypeRef{str}, failure)
	get := shapetest.Method(storeName, getName, []*node.TypeRef{str}, str, failure)
	r := shapetest.New(tb, shapetest.Package(begin, commit, rollback, put, get))
	r.Declare(tb, begin.ID, shapetest.Instance{Contract: shape.Tx, Role: shape.TxBegin})
	r.Declare(tb, commit.ID,
		shapetest.Instance{Contract: shape.Tx, Role: shape.TxCommit},
		shapetest.Instance{Mixin: shape.Atomic, Params: map[directive.ParamKey]directive.Value{
			shape.AtomicRead: {Kind: directive.TypeReference, Ref: getName, Target: get.ID},
		}})
	r.Declare(tb, rollback.ID, shapetest.Instance{Contract: shape.Tx, Role: shape.TxRollback})
	return r
}
