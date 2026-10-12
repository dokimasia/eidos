// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
)

// The fixture is the plan in the form that the renderer receives. Every
// check renders through the context of the fixture, so no check
// exercises a part that the context leaves out.
func TestFixture(t *testing.T) {
	t.Parallel()

	t.Run("Context", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a context with every part of the fixture", func(t *testing.T) {
			t.Parallel()

			f := &backendtest.Fixture{
				Emit:      plugin.NewEmit(),
				Schedule:  []plugin.ID{"gen"},
				Trees:     map[plugin.ID]fs.FS{"gen": fstest.MapFS{}},
				Funcs:     map[plugin.ID]template.FuncMap{"gen": {}},
				Overrides: map[plugin.ID][]string{"gen": {"up"}},
			}
			sink := diag.NewSink()
			got := f.Context(wellBackend(t), sink)
			expect.Equal(t, got.Emit, f.Emit, "the context has the store of the fixture", assert.ByIdentity())
			expect.Equal(t, got.Schedule, f.Schedule, "the context has the schedule of the fixture")
			expect.Equal(t, got.Trees["gen"], f.Trees["gen"], "the context has the trees of the fixture",
				assert.ByIdentity())
			expect.Equal(t, got.Funcs["gen"], f.Funcs["gen"], "the context has the helpers of the fixture",
				assert.ByIdentity())
			expect.Equal(t, got.Overrides["gen"], []string{"up"},
				"the context has the override declarations of the fixture")
			expect.Equal(t, got.Sink, sink, "the context reports into the sink", assert.ByIdentity())
			expect.NotEmpty(t, got.Plugin, "the context renders under the identity of the suite")
		})
	})
}
