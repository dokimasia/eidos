// Copyright Dokimasia B.V. 2026
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
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/history"

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

// The sink failures that the write cases inject, and the unclaimed
// directive that makes a run report an Error.
var (
	errDiskFull   = errors.New("disk full")
	errNoDevice   = errors.New("no device")
	errReadOnly   = errors.New("read-only file system")
	unclaimedName = directive.Name("nobody:claims")
)

// partialSink stages every write and commits the first staged file
// before it fails, as a disk commit fails part of the way through a
// tree.
type partialSink struct{ staged []string }

// Write stages the path.
func (s *partialSink) Write(path string, _ []byte) error {
	s.staged = append(s.staged, path)
	return nil
}

// Delete stages nothing.
func (*partialSink) Delete(string) error { return nil }

// Overwrite allows every verdict.
func (*partialSink) Overwrite(...output.Found) error { return nil }

// Prepare reports every staged path as a file to create.
func (s *partialSink) Prepare() ([]output.Change, error) {
	changes := make([]output.Change, 0, len(s.staged))
	for _, p := range s.staged {
		changes = append(changes, output.Change{
			Path:   p,
			Action: output.ActionCreated,
			Found:  output.FoundNothing,
			Hash:   "sha256:" + strings.Repeat("ab", 32),
		})
	}
	return changes, nil
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

// Delete returns errReadOnly.
func (*refusingSink) Delete(string) error { return errReadOnly }

// Overwrite returns errReadOnly.
func (*refusingSink) Overwrite(...output.Found) error { return errReadOnly }

// Prepare prepares nothing.
func (*refusingSink) Prepare() ([]output.Change, error) { return nil, nil }

// Commit commits nothing.
func (*refusingSink) Commit() ([]output.Written, error) { return nil, nil }

// Discard records the discard.
func (s *refusingSink) Discard() error {
	s.discarded = true
	return nil
}

// opener records the sinks that the output of a composition opens, a
// new memory sink for each call. It is safe for concurrent runs.
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

// wordHelper is the name of the shared helper through which the struct
// template of the vocal backend spells a name. An override replaces the
// helper.
const wordHelper = "word"

// clauseHelper is the name of the helper through which the struct template
// of the clause printer writes the name that a package declares.
const clauseHelper = "clause"

// cacheClause is the name that the package clause of the cache package
// declares in the cases of the declared names, and renamedClause the name
// that an edit gives it.
const (
	cacheClause   = "caching"
	renamedClause = "cached"
)

// alphaFile is the source file of Alpha in the routing fixture, in the
// directory of its package.
const alphaFile = coretest.StorePath + "/alpha.go"

// TestWrite tests the write, the last step of a plan. The stamped files
// of the render go to a sink that the plan opens, and the report records
// what the commit of each plan wrote to the destination.
func TestWrite(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the files a commit refused part-way wrote", func(t *testing.T) {
			t.Parallel()

			w := writing(t, func() (output.Sink, error) { return &partialSink{}, nil })
			g := routedIn(t, coretest.StorePath)
			cache := coretest.Struct(coretest.CachePath, "Beta")
			cache.Pos = position.Pos{File: coretest.CachePath + "/beta.go", Line: 3, Col: 1}
			pkg := coretest.Package(coretest.CachePath, cache)
			pkg.Files[0].Path = coretest.CachePath + "/beta.go"
			assert.NoError(t, g.AddPackage(pkg), "the second package is admitted")
			run, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, errDiskFull, "the refused commit fails the run")
			assert.Equal(t, run.Plans[0].Status, workspace.PlanFailed, "the plan fails")
			assert.Length(t, run.Plans[0].Changes, 1, "the report records the one file written to its destination")
			assert.Equal(t, run.Plans[0].Changes[0].Path, cacheGen, "the file the commit wrote before the refusal")
		})

		t.Run("opens one sink for each committing run", func(t *testing.T) {
			t.Parallel()

			var o opener
			w := writing(t, o.open)
			for range 2 {
				g, _ := alpha(t)
				_, err := w.Run(t.Context(), workspace.Input{Graph: g})
				assert.NoError(t, err, "the run is clean")
			}
			sinks := o.opened()
			assert.Length(t, sinks, 2, "each run opens one sink for its one plan")
			for _, s := range sinks {
				assert.Length(t, s.Files(), 1, "each sink has its own run's file")
			}
		})

		t.Run("opens a sink of its own for each concurrent run", func(t *testing.T) {
			t.Parallel()

			var o opener
			w := writing(t, o.open)
			graphs := make([]*store.Graph, 4)
			for i := range graphs {
				graphs[i], _ = alpha(t)
			}
			outcomes := history.Concurrently(len(graphs), time.Minute, func(run int) (any, error) {
				return w.Run(t.Context(), workspace.Input{Graph: graphs[run]})
			})
			for _, out := range outcomes {
				expect.True(t, out.Finished, "each run finishes")
				expect.NoError(t, out.Error, "each run is clean")
			}
			assert.Length(t, o.opened(), len(graphs), "each run opens its own sink")
		})

		t.Run("opens no sink for a run that reports an Error", func(t *testing.T) {
			t.Parallel()

			var o opener
			w := writing(t, o.open)
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{{Name: unclaimedName}}),
				"an unclaimed directive attaches before the seal")
			run, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run reports an Error")
			assert.Equal(t, run.Plans[0].Status, workspace.PlanFailed, "the shared phase's Error fails the plan")
			assert.Empty(t, run.Plans[0].Changes, "the report records no file")
			assert.Empty(t, o.opened(), "no sink is opened")
		})

		t.Run("writes nothing for a composition without output", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition builds")
			g, _ := alpha(t)
			run, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			assert.Empty(t, run.Plans[0].Changes, "the report records no file")
		})

		t.Run("returns an error for an output that fails to open", func(t *testing.T) {
			t.Parallel()

			w := writing(t, func() (output.Sink, error) { return nil, errNoDevice })
			g, _ := alpha(t)
			_, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, errNoDevice, "the open's error fails the run")
		})

		t.Run("returns an error for an open function that returns two nils", func(t *testing.T) {
			t.Parallel()

			w := writing(t, func() (output.Sink, error) { return nil, nil })
			g, _ := alpha(t)
			_, err := w.Run(t.Context(), workspace.Input{Graph: g})
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

		t.Run("renders the declared name of a package outside the sources of the plan", func(t *testing.T) {
			t.Parallel()

			var o opener
			g, _ := routedAlpha(t)
			cache := coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Beta"))
			cache.Name = cacheClause
			cache.Files[0].Path = coretest.CachePath + "/beta.go"
			assert.NoError(t, g.AddPackage(cache), "the cache package is admitted")
			_, err := clauseWriting(t, o.open, coretest.CachePath).Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			body := string(o.opened()[0].Files()[coretest.StorePath+"/gen.txt"])
			assert.Contains(t, body, "type ForAlpha "+cacheClause,
				"the render reads the name that the package clause of the cache declares")
		})

		t.Run("renders no declared name for a package that the run did not load", func(t *testing.T) {
			t.Parallel()

			var o opener
			g, _ := routedAlpha(t)
			_, err := clauseWriting(t, o.open, coretest.CachePath).Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			body := string(o.opened()[0].Files()[coretest.StorePath+"/gen.txt"])
			assert.Contains(t, body, "type ForAlpha \n", "the render has no name for the cache package")
		})

		t.Run("discards the staging after a refused write", func(t *testing.T) {
			t.Parallel()

			refusing := &refusingSink{}
			w := writing(t, func() (output.Sink, error) { return refusing, nil })
			g, _ := alpha(t)
			run, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, errReadOnly, "the refused write fails the run")
			assert.True(t, refusing.discarded, "the staging is discarded")
			assert.Empty(t, run.Plans[0].Changes, "the report records no file")
		})

		t.Run("writes a file beside the source it derives from", func(t *testing.T) {
			t.Parallel()

			var o opener
			g, _ := routedAlpha(t)
			assert.Permutation(t, writtenPaths(t, routing(t, layout.Config{}, o.open), g, &o),
				[]string{coretest.StorePath + "/gen.txt"}, "the package's file is in its directory")
		})

		t.Run("writes a file of a centralised plan under the output directory", func(t *testing.T) {
			t.Parallel()

			var o opener
			g, _ := routedAlpha(t)
			cfg := layout.Config{Policy: layout.PolicyCentralised, Dir: "out"}
			assert.Permutation(t, writtenPaths(t, routing(t, cfg, o.open), g, &o),
				[]string{"out/" + coretest.StorePath + "/gen.txt"}, "the source directory under the output directory")
		})

		t.Run("opens no sink for a run whose layout rejects a declaration", func(t *testing.T) {
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
			run, err := routing(t, layout.Config{}, o.open).Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refusal fails the run")
			coretest.AssertCodes(t, run.Sink, layout.EscapingPath)
			assert.Empty(t, o.opened(), "no sink is opened")
		})
	})
}

// fixturePrinter returns the printer of the target fixture. Every call
// returns one value, so the plans of a composition share the backend of
// their target, as a target renders through one backend. A kit backend
// keeps no state of a run, so parallel tests share the value.
var fixturePrinter = sync.OnceValue(func() plugin.Backend {
	return printerAs("printer", "fixture", "")
})

// printerAs returns a kit backend of target that spells one kind and
// frames its files through the comment syntax of the fixture language,
// which the output contract stamps through. It names every file file,
// or after its family word where file is empty. Each call returns a new
// backend.
func printerAs(name plugin.ID, target plugin.Target, file string) plugin.Backend {
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

// totalCoverage returns a verdict of Renders for every fact. The one
// kind template of the fixture states no fact.
func totalCoverage() map[symbol.Fact]render.Verdict {
	out := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		out[f] = render.Renders
	}
	return out
}

// vocal returns a kit backend with one shared helper. Its struct
// template spells the name through wordHelper, which returns the name
// unchanged.
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

// clausePrinter returns a kit backend of target whose struct template
// writes the name that the package of path declares, as the import set of
// the file reads it, after the name of the struct. Each call returns a new
// backend.
func clausePrinter(target plugin.Target, path string) plugin.Backend {
	return backend.New("clauses", target, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{.Name}} {{" + clauseHelper + "}}\n",
		}).
		Funcs(func(set *render.ImportSet) template.FuncMap {
			return template.FuncMap{clauseHelper: func() string {
				name, _ := set.Declared(path)
				return name
			}}
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

// clauseWriting returns a composition whose one plan mirrors the structs
// of the store package into the output that open returns. The plan
// renders through the clause printer of the fixtures' language, which
// writes the name that the package of path declares.
func clauseWriting(tb assert.TB, open func() (output.Sink, error), path string) *workspace.Workspace {
	tb.Helper()

	target := plugin.Target(coretest.Lang)
	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets(target).
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror("mirror")},
			Backend:    clausePrinter(target, path),
			Sources:    workspace.Sources{Packages: []string{coretest.StorePath}},
		}).
		Output(open).
		Build()
	assert.NoError(tb, err, "the clause writing composition composes")
	return w
}

// shouting returns a composition whose one plan renders through the
// vocal backend into the output that open returns. The one generator of
// the plan overrides wordHelper for the target overridden.
func shouting(tb assert.TB, overridden plugin.Target, open func() (output.Sink, error)) *workspace.Workspace {
	tb.Helper()

	shouter := eidos.NewPlugin("shouter").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		For(overridden, eidos.Overrides(template.FuncMap{wordHelper: strings.ToUpper})).
		Handle(eidos.OnStruct(mirrored)).
		Generator()
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

// renderedBody runs a composition over the fixture of one struct, and
// returns the one file that its one sink committed.
func renderedBody(t *testing.T, w *workspace.Workspace, o *opener) string {
	t.Helper()

	g, _ := alpha(t)
	_, err := w.Run(t.Context(), workspace.Input{Graph: g})
	assert.NoError(t, err, "the run is clean")
	files := o.opened()[0].Files()
	assert.Length(t, files, 1, "the run writes one file")
	for _, body := range files {
		return string(body)
	}
	return ""
}

// writing returns a composition whose one plan renders through the
// printer of the target fixture into the output that open returns.
func writing(tb assert.TB, open func() (output.Sink, error)) *workspace.Workspace {
	tb.Helper()

	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror("mirror")},
			Backend:    fixturePrinter(),
		}).
		Output(open).
		Build()
	assert.NoError(tb, err, "the writing composition composes")
	return w
}

// routedAlpha returns an unfrozen graph of one package whose one file
// is in the directory of the package, and the struct of the file.
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
// through the printer of the target fixture under the layout cfg, into
// the output that open returns.
func routing(tb assert.TB, cfg layout.Config, open func() (output.Sink, error)) *workspace.Workspace {
	tb.Helper()

	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror("mirror")},
			Backend:    fixturePrinter(),
			Layout:     cfg,
		}).
		Output(open).
		Build()
	assert.NoError(tb, err, "the routing composition composes")
	return w
}

// writtenPaths runs a composition over g, and returns the paths that its
// one sink committed in the order of the map of the sink.
func writtenPaths(t *testing.T, w *workspace.Workspace, g *store.Graph, o *opener) []string {
	t.Helper()

	_, err := w.Run(t.Context(), workspace.Input{Graph: g})
	assert.NoError(t, err, "the run is clean")
	return slices.Collect(maps.Keys(o.opened()[0].Files()))
}
