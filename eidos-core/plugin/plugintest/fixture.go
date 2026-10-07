// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest

import (
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Fixture is a hand-built run: the graph, the fact store, the
// validated directive table and the scope one plugin's phase calls
// see. Build it, load packages, stamp facts, then hand a plugin to
// [Fixture.Annotate] or [Fixture.Generate].
//
// The graph freezes on the first phase call, so every load and
// attach comes before it. A later structural write fails the test
// through the store's error. A Fixture belongs to one test and is
// not safe for concurrent use.
type Fixture struct {
	Graph *store.Graph
	Keys  *meta.Registry
	Facts *meta.Facts
	// Directives is the validated table dispatch gates on, keyed by
	// subject, position order per subject.
	Directives map[symbol.Identity][]directive.Directive
	// Scope filters what phase calls see. A nil scope admits
	// everything.
	Scope store.Scope
	// Rules is the registry of language rules a phase call binds
	// the kernel's walks over. A nil registry binds every language
	// to the absent rules.
	Rules *rules.Registry
	// Kernel is the kernel's key handles. [New] registers them
	// under Keys, the way a composition registers them before any
	// plugin's.
	Kernel meta.KernelKeys
	// Languages maps each target to the render language the template
	// check lints declared trees against. A fixture without one skips
	// the check.
	Languages map[plugin.Target]render.Language
	// Bucket is the priority bucket phase calls claim to run in.
	Bucket int
	// Workers is how many invocations each phase call runs at once.
	// Zero and one dispatch sequentially.
	Workers int
	// Select restricts each phase call to what a warm run executes
	// again, and nil runs every match.
	Select *plugin.Selection
	// Journal receives each phase call's records, and nil keeps none.
	Journal plugin.Journal

	emit    *plugin.Emit
	claimed map[string]bool
}

// New returns an empty fixture at bucket one, with the kernel's
// keys registered.
func New(tb assert.TB) *Fixture {
	tb.Helper()

	keys := meta.NewRegistry()
	kernel, err := meta.Kernel(keys)
	assert.NoError(tb, err, "the kernel's keys register")
	return &Fixture{
		Graph:   store.New(),
		Keys:    keys,
		Facts:   meta.NewFacts(keys),
		Kernel:  kernel,
		Bucket:  1,
		claimed: map[string]bool{},
	}
}

// Load adds parsed packages to the graph.
func (f *Fixture) Load(tb assert.TB, pkgs ...*node.Package) *Fixture {
	tb.Helper()

	assert.Total(tb, f.Graph.AddPackage, pkgs, "the fixture package is admitted")
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

// Validated sets a subject's typed instances, as validation returns
// them: position order, canonical names. It also attaches one raw
// instance per directive, because dispatch routes a gate through the
// store's spelled-name index, and only raw attachments feed that
// index. A real load attaches at parse and validates at the seal.
// Validated performs both steps.
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

// Seed adds earlier-bucket units to the emit store that the next
// [Fixture.Generate] call reads. A weaver's fixture seeds the units
// of the plugins that run before the weaver.
func (f *Fixture) Seed(tb assert.TB, units ...plugin.Unit) *Fixture {
	tb.Helper()

	assert.Total(tb, f.store().Add, units, "the seeded unit is added")
	return f
}

// Key registers a typed key under the fixture's registry, claiming
// the name's namespace on first use.
func Key[T meta.FactValue](
	tb assert.TB, f *Fixture, name meta.KeyName, doc string,
) meta.Key[T] {
	tb.Helper()

	ns := name.Namespace()
	assert.NotEmpty(tb, ns, "a key name spells its namespace first")
	if !f.claimed[ns] {
		assert.NoError(tb, f.Keys.ClaimNamespace(ns), "the namespace is claimed")
		f.claimed[ns] = true
	}
	key, err := meta.Register[T](f.Keys, meta.KeySpec{Name: name, Doc: doc})
	assert.NoError(tb, err, "the key registers")
	return key
}

// Stamp records one subject-bound fact at plugin authority, as an
// earlier annotator's stamp records it.
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

// Annotate runs one plugin's annotate phase over the fixture. A
// plugin that does not implement [plugin.Annotator] fails the test.
func (f *Fixture) Annotate(tb assert.TB, p plugin.Plugin) Result {
	tb.Helper()

	ann, implemented := p.(plugin.Annotator)
	assert.True(tb, implemented, "the plugin implements the annotator role")
	ix := f.index(tb)
	sink := diag.NewSink()
	err := ann.Annotate(&plugin.AnnotatorContext{
		Index:   ix,
		Reader:  mintReader(tb, ix),
		Facts:   f.Facts,
		Sink:    sink,
		Rules:   f.Rules,
		Kernel:  f.Kernel,
		Plugin:  p.Name(),
		Bucket:  f.Bucket,
		Workers: f.Workers,
		Select:  f.Select,
		Journal: f.Journal,
	})
	return Result{Emit: f.store(), Sink: sink, Err: err}
}

// Generate runs one plugin's generate phase over the fixture. A
// plugin that does not implement [plugin.Generator] fails the test.
// The emit store persists across calls, so successive Generate calls
// see earlier flushes the way later buckets do.
func (f *Fixture) Generate(tb assert.TB, p plugin.Plugin) Result {
	tb.Helper()

	gen, implemented := p.(plugin.Generator)
	assert.True(tb, implemented, "the plugin implements the generator role")
	ix := f.index(tb)
	sink := diag.NewSink()
	err := gen.Generate(&plugin.GeneratorContext{
		Index:   ix,
		Reader:  mintReader(tb, ix),
		Facts:   f.Facts,
		Emit:    f.store(),
		Sink:    sink,
		Rules:   f.Rules,
		Kernel:  f.Kernel,
		Plugin:  p.Name(),
		Bucket:  f.Bucket,
		Workers: f.Workers,
		Select:  f.Select,
		Journal: f.Journal,
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
