// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package workspace_test

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The corpus shape, the benchmarking document's densities at the
// canonical scale: two hundred symbols per package, of which ten
// per hundred carry the mark the annotator stamps, and three of
// those ten reference a symbol in the next package, so the settle
// follows resolved references across packages. The names encode
// the roles, which is what keeps the corpus deterministic without
// a seed; raw directives join the corpus when a frontend exists to
// state them.
const (
	// e2ePackages is the benchmark's package count, the medium
	// size whose symbol total matches the first system's measured
	// envelope.
	e2ePackages = 1_000
	// e2eRefs, e2eMarked and e2eUnmarked split one package's
	// symbols by role: marked and cross-referencing, marked, and
	// present alone.
	e2eRefs       = 6
	e2eMarked     = 14
	e2eUnmarked   = 180
	e2ePathPrefix = "example.com/e2e/pkg"
)

// e2ePath is one corpus package's path.
func e2ePath(p int) string { return e2ePathPrefix + strconv.Itoa(p) }

// e2eGraph builds a corpus of n packages, loaded and left unfrozen
// the way a run takes a graph.
func e2eGraph(tb assert.TB, n int) *store.Graph {
	tb.Helper()

	g := store.New()
	for p := range n {
		path := e2ePath(p)
		decls := make([]symbol.Symbol, 0, e2eRefs+e2eMarked+e2eUnmarked)
		for i := range e2eRefs {
			decls = append(decls, coretest.Struct(path, "mr"+strconv.Itoa(i)))
		}
		for i := range e2eMarked {
			decls = append(decls, coretest.Struct(path, "mk"+strconv.Itoa(i)))
		}
		for i := range e2eUnmarked {
			decls = append(decls, coretest.Struct(path, "pl"+strconv.Itoa(i)))
		}
		assert.NoError(tb, g.AddPackage(coretest.Package(path, decls...)),
			"the corpus package loads")
	}
	return g
}

// e2eWorkspace composes the pipeline over n packages: an annotator
// stamping the mark, a generator mirroring the marked symbols with
// the stated share of cross-package references, and the fixture
// backend the plan settles through.
func e2eWorkspace(tb assert.TB, n int) *workspace.Workspace {
	tb.Helper()

	var mark meta.Key[bool]
	stamper, held := eidos.NewPlugin("marker").
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("e2e", "marker"); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{
				Name: "e2e.mark", Doc: "marks a corpus subject for mirroring",
			})
			mark = k
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			if strings.HasPrefix(m.Struct.Name, "m") {
				eidos.Stamp(st, mark, true)
			}
			return nil
		})).Build().(plugin.Annotator)
	assert.True(tb, held, "the annotator half composes")

	mirror, held := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if _, marked := eidos.Fact(m, mark); !marked {
				return nil
			}
			out := &emit.Struct{
				Origin: m.Struct.Identity(),
				Name:   "for" + m.Struct.Name,
			}
			if strings.HasPrefix(m.Struct.Name, "mr") {
				// The next package's first plain mark, referenced
				// resolved, so the settle follows the name across
				// packages once the respell moves it.
				p := packageIndex(tb, m.Struct.Identity().Package)
				target := coretest.Struct(e2ePath((p+1)%n), "mk0")
				out.Fields.Append(&emit.Field{
					Name: "peer",
					Type: &emit.TypeRef{Target: target.ID, Spelling: "formk0"},
				})
			}
			e.PackageFile().Append(out)
			return nil
		})).Build().(plugin.Generator)
	assert.True(tb, held, "the generator half composes")

	w, err := workspace.New().
		Annotators(stamper).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror},
			Backend:    e2eBackend(),
		}).
		Build()
	assert.NoError(tb, err, "the composition validates")
	return w
}

// packageIndex reads the corpus index back out of a package path.
func packageIndex(tb assert.TB, path string) int {
	tb.Helper()

	p, err := strconv.Atoi(strings.TrimPrefix(path, e2ePathPrefix))
	assert.NoError(tb, err, "a corpus path carries its index")
	return p
}

// e2eBackend builds the fixture backend: one struct spelling, an
// upper-first respell so the settle rewrites every name and
// follows every reference, and a coverage rendering every fact so
// the guard walks each declaration.
func e2eBackend() plugin.Backend {
	facts := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		facts[f] = render.Renders
	}
	return eidos.NewBackend("printer", "fixture",
		plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{.Name}} {\n" +
				"{{- range .Fields.Items}}\n\t{{.Name}} {{spell .Type}}\n{{- end}}\n}\n",
		}).
		Funcs(template.FuncMap{
			"spell": func(t *emit.TypeRef) string {
				if t == nil {
					return ""
				}
				return t.Spelling
			},
		}).
		Coverage(render.Coverage{Facts: facts}).
		Respell(func(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
			return strings.ToUpper(name[:1]) + name[1:], nil
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return []byte("\tnoop()\n"), nil
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Build()
}

// e2ePipeline runs the corpus through every stage the kernel owns:
// annotate, generate and settle inside the run, then render, stamp
// and the sink's commit. The renderer arrives composed, the way a
// host holds it. It returns the committed files.
func e2ePipeline(
	tb assert.TB, w *workspace.Workspace, g *store.Graph, r plugin.Renderer,
) []output.Written {
	tb.Helper()

	report, err := w.Run(context.Background(), g)
	assert.NoError(tb, err, "the run completes")
	assert.False(tb, report.Sink.Failed(), "and reports no errors")

	files, err := r.Render(&plugin.RenderContext{
		Emit: report.Emits["plan"], Sink: report.Sink, Plugin: "printer",
	})
	assert.NoError(tb, err, "the render completes")
	assert.False(tb, report.Sink.Failed(), "and reports no findings that fail")

	c, err := output.NewContract("e2e", plugin.CommentSyntax{Line: []string{"//"}})
	assert.NoError(tb, err, "the contract composes")
	sink := output.NewMem()
	for _, f := range files {
		body, stampErr := c.Stamp(f)
		assert.NoError(tb, stampErr, "every rendered file stamps")
		// Routing a file under its owning package is the layout's
		// half of the address; the fixture routes by package path.
		assert.NoError(tb, sink.Write(f.Pkg.Package+"/"+f.Name, body),
			"and stages")
	}
	written, err := sink.Commit()
	assert.NoError(tb, err, "the staging commits")
	return written
}

// peakRSS reads the process's high-water resident set in bytes,
// zero where the platform does not expose it.
func peakRSS() uint64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		if !strings.HasPrefix(line, "VmHWM:") {
			continue
		}
		kb, err := strconv.ParseUint(strings.TrimSuffix(
			strings.TrimSpace(strings.TrimPrefix(line, "VmHWM:")), " kB"), 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}

// BenchmarkPipeline measures the kernel's whole pipeline over the
// corpus: annotate, generate, settle, render, stamp and commit,
// the corpus, composition and renderer built outside the
// measurement, under an allocation ceiling pinned from measurement
// with headroom. The peak resident set reports as a metric beside
// the numbers, so the envelope carries memory as well as work.
func BenchmarkPipeline(b *testing.B) {
	c := bench.Start(b).MaxAllocs(520_000)
	defer c.End()
	for c.Loop() {
		var w *workspace.Workspace
		var g *store.Graph
		var r plugin.Renderer
		c.Excluding(func() {
			w = e2eWorkspace(b, e2ePackages)
			g = e2eGraph(b, e2ePackages)
			renderer, held := e2eBackend().(plugin.Renderer)
			if !held {
				b.Fatal("the fixture backend renders")
			}
			r = renderer
		})
		written := e2ePipeline(b, w, g, r)
		if len(written) != e2ePackages {
			b.Fatalf("the envelope commits one file per package, got %d",
				len(written))
		}
	}
	if rss := peakRSS(); rss > 0 {
		b.ReportMetric(float64(rss)/(1<<20), "peak-RSS-MB")
	}
}

// A second run costs a first run's work and writes a first run's
// bytes: nothing a run leaves behind feeds the next one, which is
// the pathology the first system had, whose warm runs re-ingested
// their own outputs. The corpus is a share of the benchmark's, so
// the gate stays quick.
func TestPipelineWarmEqualsCold(t *testing.T) {
	t.Parallel()

	const n = 50
	run := func() map[string]string {
		w := e2eWorkspace(t, n)
		g := e2eGraph(t, n)
		r, held := e2eBackend().(plugin.Renderer)
		assert.True(t, held, "the fixture backend renders")
		out := map[string]string{}
		for _, f := range e2ePipeline(t, w, g, r) {
			out[f.Path] = f.Hash
		}
		return out
	}
	cold := run()
	warm := run()
	assert.Length(t, warm, n, "one file per package commits")
	assert.Equal(t, warm, cold,
		"a warm run writes a cold run's bytes, hash for hash")
}
