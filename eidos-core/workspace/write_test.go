// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The sink failures the write cases inject, and the unclaimed
// directive that makes a run report an Error.
var (
	errDiskFull   = errors.New("disk full")
	errNoDevice   = errors.New("no device")
	errReadOnly   = errors.New("read-only file system")
	unclaimedName = directive.Name("nobody:claims")
)

// partialSink stages every write and commits the first staged file
// before failing: the shape of a disk commit refused part-way
// through a tree.
type partialSink struct{ staged []string }

// Write stages the path.
func (s *partialSink) Write(path string, _ []byte) error {
	s.staged = append(s.staged, path)
	return nil
}

// Commit records the first staged path and returns errDiskFull.
func (s *partialSink) Commit() ([]output.Written, error) {
	return []output.Written{{Path: s.staged[0], Action: output.ActionCreated}}, errDiskFull
}

// Discard drops nothing.
func (*partialSink) Discard() error { return nil }

// refusingSink refuses every write and records whether the staging
// was discarded.
type refusingSink struct{ discarded bool }

// Write returns errReadOnly.
func (*refusingSink) Write(string, []byte) error { return errReadOnly }

// Commit commits nothing.
func (*refusingSink) Commit() ([]output.Written, error) { return nil, nil }

// Discard records the discard.
func (s *refusingSink) Discard() error {
	s.discarded = true
	return nil
}

// opener counts the sinks a composition's output opens, one fresh
// in-memory sink per call. It is safe for concurrent runs.
type opener struct {
	mu    sync.Mutex
	sinks []*output.Mem
}

// open returns a fresh in-memory sink and records it.
func (o *opener) open() (output.Sink, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	m := output.NewMem()
	o.sinks = append(o.sinks, m)
	return m, nil
}

// opened returns the sinks opened so far, in open order.
func (o *opener) opened() []*output.Mem {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]*output.Mem(nil), o.sinks...)
}

// wordHelper is the shared helper the vocal backend's struct
// template spells a name through, and the name an override
// replaces.
const wordHelper = "word"

// vocal is a kit backend with one shared helper: its struct template
// spells the name through wordHelper, which returns the name as it
// is.
func vocal(tb assert.TB, target plugin.Target) plugin.Backend {
	tb.Helper()

	return backend.New("vocal", target, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{" + wordHelper + " .Name}} struct{}\n",
		}).
		Funcs(func(*render.ImportSet) template.FuncMap {
			return template.FuncMap{wordHelper: func(s string) string { return s }}
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

// shouting returns a composition whose one plan renders through the
// vocal backend, with one generator that overrides wordHelper for
// the given target, into the output open returns.
func shouting(tb assert.TB, overridden plugin.Target, open func() (output.Sink, error)) *workspace.Workspace {
	tb.Helper()

	shouter, held := eidos.NewPlugin("shouter").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		For(overridden, eidos.Overrides(template.FuncMap{wordHelper: strings.ToUpper})).
		Handle(eidos.OnStruct(mirrored)).
		Build().(plugin.Generator)
	assert.True(tb, held, "an emitter rule lowers to the generator role")
	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{shouter},
			Backend:    vocal(tb, "fixture"),
		}).
		Output(open).
		Build()
	assert.NoError(tb, err, "the shouting composition composes")
	return w
}

// renderedBody runs a composition over the one-struct fixture and
// returns the one file its one sink committed.
func renderedBody(t *testing.T, w *workspace.Workspace, o *opener) string {
	t.Helper()

	g, _ := alpha(t)
	_, err := w.Run(t.Context(), g)
	assert.NoError(t, err, "the run is clean")
	files := o.opened()[0].Files()
	assert.Length(t, files, 1, "the run writes one file")
	for _, body := range files {
		return string(body)
	}
	return ""
}

// writing returns a composition whose one plan renders through the
// printer backend into the output open returns.
func writing(tb assert.TB, open func() (output.Sink, error)) *workspace.Workspace {
	tb.Helper()

	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror("mirror")},
			Backend:    printer(tb, "fixture"),
		}).
		Output(open).
		Build()
	assert.NoError(tb, err, "the writing composition composes")
	return w
}

// alphaFile is the source file the routing fixture declares Alpha in,
// inside its package's own directory.
const alphaFile = coretest.StorePath + "/alpha.go"

// routedAlpha returns an unfrozen one-package graph whose one file
// sits in the package's directory, and the struct it declares.
func routedAlpha(tb assert.TB) (*store.Graph, *node.Struct) {
	tb.Helper()

	s := coretest.Struct(coretest.StorePath, "Alpha")
	s.Pos = position.Pos{File: alphaFile, Line: 3, Col: 1}
	pkg := coretest.Package(coretest.StorePath, s)
	pkg.Files[0].Path = alphaFile
	g := store.New()
	assert.NoError(tb, g.AddPackage(pkg), "the fixture package is admitted")
	return g, s
}

// routing returns a composition whose one plan mirrors every struct
// through the printer backend under the given layout, into the output
// open returns.
func routing(tb assert.TB, cfg layout.Config, open func() (output.Sink, error)) *workspace.Workspace {
	tb.Helper()

	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror("mirror")},
			Backend:    printer(tb, "fixture"),
			Layout:     cfg,
		}).
		Output(open).
		Build()
	assert.NoError(tb, err, "the routing composition composes")
	return w
}

// writtenPaths runs a composition over g and returns the paths its one
// sink committed, sorted.
func writtenPaths(t *testing.T, w *workspace.Workspace, g *store.Graph, o *opener) []string {
	t.Helper()

	_, err := w.Run(t.Context(), g)
	assert.NoError(t, err, "the run is clean")
	return slices.Sorted(maps.Keys(o.opened()[0].Files()))
}

// The write is the run's last step: the render's staged files go to
// a sink the run opens for itself, and the report records what was
// written to the destination.
func TestWrite(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the files a commit refused part-way wrote", func(t *testing.T) {
			t.Parallel()

			w := writing(t, func() (output.Sink, error) { return &partialSink{}, nil })
			g, _ := alpha(t)
			run, err := w.Run(t.Context(), g)
			assert.ErrorIs(t, err, errDiskFull, "the refused commit fails the run")
			assert.Length(t, run.Written, 1, "the report records the file written to its destination")
		})

		t.Run("opens one sink for each committing run", func(t *testing.T) {
			t.Parallel()

			var o opener
			w := writing(t, o.open)
			for range 2 {
				g, _ := alpha(t)
				_, err := w.Run(t.Context(), g)
				assert.NoError(t, err, "the run is clean")
			}
			sinks := o.opened()
			assert.Length(t, sinks, 2, "each run opens one sink")
			for _, s := range sinks {
				assert.Length(t, s.Files(), 1, "each sink has its own run's file")
			}
		})

		t.Run("opens a sink of its own for each concurrent run", func(t *testing.T) {
			t.Parallel()

			var o opener
			w := writing(t, o.open)
			errs := make([]error, 4)
			var wg sync.WaitGroup
			for i := range errs {
				g, _ := alpha(t)
				wg.Go(func() {
					_, errs[i] = w.Run(t.Context(), g)
				})
			}
			wg.Wait()
			for _, err := range errs {
				assert.NoError(t, err, "each run is clean")
			}
			assert.Length(t, o.opened(), len(errs), "each run opens its own sink")
		})

		t.Run("opens no sink for a run that reports an Error", func(t *testing.T) {
			t.Parallel()

			var o opener
			w := writing(t, o.open)
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{{Name: unclaimedName}}),
				"an unclaimed directive attaches before the seal")
			run, err := w.Run(t.Context(), g)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run reports an Error")
			assert.Empty(t, run.Written, "the report records no file")
			assert.Empty(t, o.opened(), "no sink is opened")
		})

		t.Run("writes nothing for a composition without output", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition builds")
			g, _ := alpha(t)
			run, err := w.Run(t.Context(), g)
			assert.NoError(t, err, "the run is clean")
			assert.Empty(t, run.Written, "the report records no file")
		})

		t.Run("returns an error for an output that fails to open", func(t *testing.T) {
			t.Parallel()

			w := writing(t, func() (output.Sink, error) { return nil, errNoDevice })
			g, _ := alpha(t)
			_, err := w.Run(t.Context(), g)
			assert.ErrorIs(t, err, errNoDevice, "the open's error fails the run")
		})

		t.Run("returns an error for an open function that returns (nil, nil)", func(t *testing.T) {
			t.Parallel()

			w := writing(t, func() (output.Sink, error) { return nil, nil })
			g, _ := alpha(t)
			_, err := w.Run(t.Context(), g)
			assert.HasError(t, err, "the run fails")
			assert.Contains(t, err.Error(), "returned (nil, nil)", "the error names the fault")
		})

		t.Run("renders an override a generator declares for the plan's target", func(t *testing.T) {
			t.Parallel()

			var o opener
			body := renderedBody(t, shouting(t, "fixture", o.open), &o)
			assert.Contains(t, body, "type FORALPHA struct{}", "the override replaces the shared helper")
		})

		t.Run("renders the shared helper for an override declared for another target", func(t *testing.T) {
			t.Parallel()

			var o opener
			body := renderedBody(t, shouting(t, "other", o.open), &o)
			assert.Contains(t, body, "type ForAlpha struct{}", "the shared helper spells the name")
		})

		t.Run("discards the staging after a refused write", func(t *testing.T) {
			t.Parallel()

			refusing := &refusingSink{}
			w := writing(t, func() (output.Sink, error) { return refusing, nil })
			g, _ := alpha(t)
			run, err := w.Run(t.Context(), g)
			assert.ErrorIs(t, err, errReadOnly, "the refused write fails the run")
			assert.True(t, refusing.discarded, "the staging is discarded")
			assert.Empty(t, run.Written, "the report records no file")
		})

		t.Run("writes a file beside the source it derives from", func(t *testing.T) {
			t.Parallel()

			var o opener
			g, _ := routedAlpha(t)
			assert.Equal(t, writtenPaths(t, routing(t, layout.Config{}, o.open), g, &o),
				[]string{coretest.StorePath + "/gen.txt"}, "the package's file is in its directory")
		})

		t.Run("writes a file of a centralised plan under the output directory", func(t *testing.T) {
			t.Parallel()

			var o opener
			g, _ := routedAlpha(t)
			cfg := layout.Config{Policy: layout.PolicyCentralised, Dir: "out"}
			assert.Equal(t, writtenPaths(t, routing(t, cfg, o.open), g, &o),
				[]string{"out/" + coretest.StorePath + "/gen.txt"}, "the source directory under the output directory")
		})

		t.Run("opens no sink for a run whose layout refuses a declaration", func(t *testing.T) {
			t.Parallel()

			var o opener
			g, s := routedAlpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{{
				Name: directive.KernelOut,
				Args: []directive.RawArg{{
					Key: string(directive.OutPath), Value: directive.RawValue{Text: "/etc/gen.txt", Quoted: true},
				}},
				Pos: position.Pos{File: alphaFile, Line: 2, Col: 1},
			}}), "the escaping override attaches before the seal")
			run, err := routing(t, layout.Config{}, o.open).Run(t.Context(), g)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refusal fails the run")
			coretest.AssertCodes(t, run.Sink, layout.EscapingPath)
			assert.Empty(t, o.opened(), "no sink is opened")
		})
	})
}
