// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// printer is a kit backend spelling one kind and framing its files
// through the fixture language's comment syntax, which is what the
// output contract stamps through.
func printer(tb assert.TB, target plugin.Target) plugin.Backend {
	tb.Helper()

	return backend.New("printer", target,
		plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{.Name}} struct{}\n",
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the fixture spells no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: totalCoverage()}).
		Build()
}

// totalCoverage renders every fact, which the fixture's one
// template kind states none of.
func totalCoverage() map[symbol.Fact]render.Verdict {
	out := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		out[f] = render.Renders
	}
	return out
}

// The two halves compose: source loads into a sealed graph under
// the composition's brand, and a run over that very graph writes
// stamped files into its sink.
func TestCompose(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the file a loaded graph's plan renders", func(t *testing.T) {
			t.Parallel()

			var o opener
			w := writing(t, o.open)
			tree := fstest.MapFS{
				"svc/store/row.zz": {Data: []byte("package svc/store\ntype Row int string\n")},
			}
			g, report, err := load.Load(t.Context(), load.Config{
				FS:        tree,
				Frontends: []plugin.Frontend{frontendtest.NewScripted()},
				Sink:      diag.NewSink(),
				PluginSet: w.Fingerprint(),
				Brand:     w.Brand(),
			})
			assert.NoError(t, err, "the source loads")
			assert.Length(t, report.Units, 1, "one unit is parsed")
			assert.True(t, g.Frozen(), "the load seals what it built")

			run, err := w.Run(t.Context(), g)
			assert.NoError(t, err, "the run takes the load's own graph")
			assert.Length(t, run.Written, 1, "the run writes the rendered file")
			written := run.Written[0]
			assert.Equal(t, written.Path, "svc/store/gen.txt",
				"the path is the package path and the target's filename")
			assert.Equal(t, written.Action, output.ActionCreated, "the file is new")

			stamped := o.opened()[0].Files()[written.Path]
			p, framed := output.Read(stamped)
			assert.True(t, framed, "the file has a frame")
			assert.Equal(t, p.Brand, w.Brand(), "the frame claims the composition's brand")
		})
	})
}
