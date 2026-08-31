// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backendtest_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backendtest"
	"go.dokimi.dev/eidos/core/plugin"
)

// The fixture is the plan as the renderer sees it, so the context
// the suite hands over has to carry it whole: a field the lowering
// drops is a surface no check can exercise.
func TestFixture(t *testing.T) {
	t.Parallel()

	t.Run("the render context carries the fixture whole", func(t *testing.T) {
		t.Parallel()

		f := &backendtest.Fixture{
			Emit:      plugin.NewEmit(),
			Schedule:  []plugin.ID{"gen"},
			Trees:     map[plugin.ID]fs.FS{"gen": fstest.MapFS{}},
			Funcs:     map[plugin.ID]template.FuncMap{"gen": {}},
			Overrides: map[plugin.ID][]string{"gen": {"up"}},
		}
		var seen *plugin.RenderContext
		probe := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			return &fake{render: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
				seen = ctx
				return nil, nil
			}}, f
		}

		backendtest.AssertSpeltKinds(t, probe)
		assert.True(t, seen != nil, "the check rendered")
		assert.True(t, seen.Emit == f.Emit, "the same store")
		assert.Equal(t, seen.Schedule, f.Schedule, "the schedule as data")
		assert.True(t, seen.Trees["gen"] != nil, "the trees reach the context")
		assert.True(t, seen.Funcs["gen"] != nil, "the helpers reach the context")
		assert.Equal(t, seen.Overrides["gen"], []string{"up"},
			"the override declarations reach the context")
		assert.True(t, seen.Sink != nil, "the suite supplies a fresh sink")
		assert.True(t, seen.Plugin != "", "the suite renders under its own identity")
	})
}
