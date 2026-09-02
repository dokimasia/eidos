// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest

import (
	"strings"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Fixture is a hand-built run: the graph, the fact store, the
// validated directive table and the scope one plugin's phase calls
// see. Build it, load packages, stamp facts, then hand a plugin to
// [Fixture.Annotate] or [Fixture.Generate].
//
// The graph freezes on the first phase call, so every load and
// attach comes before it; a later structural write fails the test
// through the store's own refusal. A Fixture belongs to one test
// and is not safe for concurrent use.
type Fixture struct {
	Graph *store.Graph
	Keys  *meta.Registry
	Facts *meta.Facts
	// Directives is the validated table dispatch gates on, keyed by
	// subject, position order per subject.
	Directives map[symbol.Identity][]directive.Directive
	// Scope filters what phase calls see; nil admits everything.
	Scope store.Scope
	// Languages holds a render language per target, what the
	// template check lints declared trees against; a fixture that
	// carries none skips the check.
	Languages map[plugin.Target]render.Language
	// Bucket is the priority bucket phase calls claim to run in.
	Bucket int

	emit    *plugin.Emit
	claimed map[string]bool
}

// New returns a fixture holding nothing, bucket one.
func New(tb assert.TB) *Fixture {
	tb.Helper()

	keys := meta.NewRegistry()
	return &Fixture{
		Graph:   store.New(),
		Keys:    keys,
		Facts:   meta.NewFacts(keys),
		Bucket:  1,
		claimed: map[string]bool{},
	}
}

// Load adds parsed packages to the graph.
func (f *Fixture) Load(tb assert.TB, pkgs ...*node.Package) *Fixture {
	tb.Helper()

	for _, p := range pkgs {
		assert.NoError(tb, f.Graph.AddPackage(p), "the fixture package is admitted")
	}
	return f
}

// Attach records raw directive instances on a subject, before the
// first phase call seals the graph.
func (f *Fixture) Attach(
	tb assert.TB, subject symbol.Identity, raws ...directive.Raw,
) *Fixture {
	tb.Helper()

	assert.NoError(tb, f.Graph.AttachDirectives(subject, raws),
		"the raw instances attach before the seal")
	return f
}

// Validated sets a subject's typed instances, as validation would
// have returned them: position order, canonical names. It also
// attaches one raw instance per directive, because dispatch routes
// a gate through the store's spelled-name index, which only raw
// attachments feed; a real load attaches at parse and validates at
// the seal, and the fixture stands in for both.
func (f *Fixture) Validated(
	tb assert.TB, subject symbol.Identity, ds ...directive.Directive,
) *Fixture {
	tb.Helper()

	raws := make([]directive.Raw, 0, len(ds))
	for _, d := range ds {
		raws = append(raws, directive.Raw{Name: d.Name, Pos: d.Pos})
	}
	f.Attach(tb, subject, raws...)
	if f.Directives == nil {
		f.Directives = map[symbol.Identity][]directive.Directive{}
	}
	f.Directives[subject] = ds
	return f
}

// Seed arrives earlier-bucket units into the emit store the next
// [Fixture.Generate] call reads, which is how a weaver's fixture
// stands in for the plugins that ran before it.
func (f *Fixture) Seed(tb assert.TB, units ...plugin.Unit) *Fixture {
	tb.Helper()

	for _, u := range units {
		assert.NoError(tb, f.store().Add(u), "the seeded unit arrives")
	}
	return f
}

// Key registers a typed key under the fixture's registry, claiming
// the name's namespace on first use.
func Key[T meta.FactValue](
	tb assert.TB, f *Fixture, name meta.KeyName, doc string,
) meta.Key[T] {
	tb.Helper()

	ns, _, split := strings.Cut(string(name), ".")
	assert.True(tb, split, "a key name spells its namespace first")
	if !f.claimed[ns] {
		assert.NoError(tb, f.Keys.ClaimNamespace(ns, "the fixture"),
			"the namespace is claimed")
		f.claimed[ns] = true
	}
	key, err := meta.Register[T](f.Keys, meta.KeySpec{Name: name, Doc: doc})
	assert.NoError(tb, err, "the key registers")
	return key
}

// Stamp records one fact at plugin authority, subject bound: the
// shortest spelling of "an earlier annotator concluded this".
func Stamp[T meta.FactValue](
	tb assert.TB, f *Fixture, k meta.Key[T], subject symbol.Identity, v T,
) {
	tb.Helper()

	assert.NoError(tb, meta.Stamp(f.Facts, k, v, meta.Claim{Subject: subject}),
		"the fixture fact stamps")
}

// Result is what one phase call left behind: the plan's emit store,
// the findings, and the fatal error where the phase returned one.
type Result struct {
	Emit *plugin.Emit
	Sink *diag.Sink
	Err  error
}

// Annotate runs one plugin's annotate phase over the fixture. The
// plugin must hold the annotator role; a fixture handing the wrong
// role over is its own defect and fails the test.
func (f *Fixture) Annotate(tb assert.TB, p plugin.Plugin) Result {
	tb.Helper()

	ann, held := p.(plugin.Annotator)
	assert.True(tb, held, "the plugin holds the annotator role")
	ix := f.index(tb)
	sink := diag.NewSink()
	err := ann.Annotate(&plugin.AnnotatorContext{
		Index:  ix,
		Reader: mintReader(tb, ix),
		Facts:  f.Facts,
		Sink:   sink,
		Plugin: p.Name(),
		Bucket: f.Bucket,
	})
	return Result{Emit: f.store(), Sink: sink, Err: err}
}

// Generate runs one plugin's generate phase over the fixture. The
// emit store persists across calls, so successive Generate calls
// see earlier flushes the way later buckets do.
func (f *Fixture) Generate(tb assert.TB, p plugin.Plugin) Result {
	tb.Helper()

	gen, held := p.(plugin.Generator)
	assert.True(tb, held, "the plugin holds the generator role")
	ix := f.index(tb)
	sink := diag.NewSink()
	err := gen.Generate(&plugin.GeneratorContext{
		Index:  ix,
		Reader: mintReader(tb, ix),
		Facts:  f.Facts,
		Emit:   f.store(),
		Sink:   sink,
		Plugin: p.Name(),
		Bucket: f.Bucket,
	})
	return Result{Emit: f.store(), Sink: sink, Err: err}
}

// index seals the graph on first use and returns the routing
// surface a phase call dispatches through.
func (f *Fixture) index(tb assert.TB) *plugin.Index {
	tb.Helper()

	f.Graph.Freeze()
	ix, err := plugin.NewIndex(f.Graph, f.Facts, f.Directives, f.Scope)
	assert.NoError(tb, err, "the routing surface builds")
	return ix
}

// mintReader returns the phase call's tracked read handle: the
// whole-call grain a hand-rolled plugin reads at.
func mintReader(tb assert.TB, ix *plugin.Index) *store.Reader {
	tb.Helper()

	r, err := ix.Reader(store.NewReadSet())
	assert.NoError(tb, err, "the tracked handle mints")
	return r
}

// store returns the fixture's emit store, created on first use.
func (f *Fixture) store() *plugin.Emit {
	if f.emit == nil {
		f.emit = plugin.NewEmit()
	}
	return f.emit
}
