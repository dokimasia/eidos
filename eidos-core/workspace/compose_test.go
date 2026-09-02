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
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
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

// The two halves compose: source loads into a sealed graph, and a
// run over that very graph writes stamped files into its sink.
func TestCompose(t *testing.T) {
	t.Parallel()

	w, err := workspace.New().
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror("mirror")},
			Backend:    printer(t, "fixture"),
		}).
		Output(output.NewMem(), "eidos").
		Build()
	assert.NoError(t, err, "the writing composition composes")
	assert.Equal(t, w.Brand(), output.Brand("eidos"), "the composition states its brand")

	tree := fstest.MapFS{
		"svc/store/row.zz": {Data: []byte("package svc/store\ntype Row int string\n")},
	}
	sink := diag.NewSink()
	g, report, err := load.Load(t.Context(), load.Config{
		FS:        tree,
		Frontends: []plugin.Frontend{frontendtest.NewScripted()},
		Sink:      sink,
		PluginSet: w.Fingerprint(),
		Brand:     w.Brand(),
	})
	assert.NoError(t, err, "the source loads")
	assert.Length(t, report.Units, 1, "one unit parsed")
	assert.True(t, g.Frozen(), "and the load sealed what it built")

	run, err := w.Run(t.Context(), g)
	assert.NoError(t, err, "the run takes the load's own graph")
	assert.Length(t, run.Written, 1, "and writes the file its plan rendered")

	written := run.Written[0]
	assert.Equal(t, written.Path, "svc/store/gen.txt",
		"addressed by the owning package and the target's own filename")
	assert.Equal(t, written.Action, output.ActionCreated, "as a new file")
}

// The composition writes nothing where a run reports, because half
// a tree is worse than none.
func TestComposeWithholds(t *testing.T) {
	t.Parallel()

	mem := output.NewMem()
	w, err := workspace.New().
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror("mirror")},
			Backend:    printer(t, "fixture"),
		}).
		Output(mem, "eidos").
		Build()
	assert.NoError(t, err, "the writing composition composes")

	g, alphaStruct := alpha(t)
	assert.NoError(t, g.AttachDirectives(alphaStruct.Identity(),
		[]directive.Raw{{Name: "nobody:claims"}}),
		"an unclaimed directive attaches before the seal")

	run, err := w.Run(t.Context(), g)
	assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run reports")
	assert.Length(t, run.Written, 0, "and writes nothing")
	assert.Length(t, mem.Files(), 0, "the staging is discarded whole")
}
