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
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// printer is a kit backend spelling one kind and framing its files
// through the fixture language's comment syntax, which is what the
// output contract stamps through.
func printer(tb assert.TB, target plugin.Target) plugin.Backend {
	tb.Helper()

	return printerAs(tb, "printer", target, "")
}

// printerAs is the printer under a name of its own, for a composition
// of more than one plan. It names every file file, or after its family
// word where file is empty.
func printerAs(tb assert.TB, name plugin.ID, target plugin.Target, file string) plugin.Backend {
	tb.Helper()

	return backend.New(name, target,
		plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{.Name}} struct{}\n",
		}).
		Naming(func(u plugin.Unit) string {
			if file != "" {
				return file
			}
			return u.Word + ".txt"
		}).
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

// The two halves compose: the composition's frontend loads a tree
// under the composition's brand, and the run over the graph it loaded
// writes stamped files into its sink.
func TestCompose(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the file a loaded tree's plan renders", func(t *testing.T) {
			t.Parallel()

			var o opener
			w, err := workspace.New().
				Brand(fixtureBrand).
				Frontends(frontendtest.NewScripted()).
				Annotators(stamper("noter", quiet)).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{mirror("mirror")},
					Backend:    printer(t, "fixture"),
				}).
				Output(o.open).
				Build()
			assert.NoError(t, err, "the loading composition composes")
			tree := fstest.MapFS{
				"svc/store/row.zz": {Data: []byte("package svc/store\ntype Row int string\n")},
			}
			run, err := w.Run(t.Context(), workspace.Input{Tree: tree})
			assert.NoError(t, err, "the run loads the tree and writes")
			assert.Length(t, run.Load.Units, 1, "one unit is parsed")
			changes := run.Plans[0].Changes
			assert.Length(t, changes, 1, "the run writes the rendered file")
			assert.Equal(t, changes[0].Path, "svc/store/gen.txt",
				"the path is the package's directory and the target's filename")
			assert.Equal(t, changes[0].Action, output.ActionCreated, "the file is new")

			stamped := o.opened()[0].Files()[changes[0].Path]
			p, framed := output.Read(stamped)
			assert.True(t, framed, "the file has a frame")
			assert.Equal(t, p.Brand, w.Brand(), "the frame claims the composition's brand")
		})
	})
}
