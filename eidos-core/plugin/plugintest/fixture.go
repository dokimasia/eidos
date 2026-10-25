// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest

import (
	"errors"
	"slices"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Fixture is a run that a test builds by hand. It has the graph, the fact
// store, the table of validated directives and the scope that the phase
// calls of one plugin see. A test creates it with [New], loads packages,
// stamps facts, and then passes a plugin to [Fixture.Annotate] or
// [Fixture.Generate].
//
// The graph freezes on the first phase call, so every load and attach
// comes before it. A later structural write fails the test with the error
// of the store. A Fixture belongs to one test, and it is not safe for
// concurrent use.
type Fixture struct {
	Graph *store.Graph
	Keys  *meta.Registry
	Facts *meta.Facts
	// Directives is the table of validated directives that the dispatch
	// gates on. Its key is the subject, and the directives of a subject are
	// in position order.
	Directives map[symbol.Identity][]directive.Directive
	// Scope filters what the phase calls see, and a nil scope admits
	// everything.
	Scope store.Scope
	// Rules is the registry of the language rules that a phase call binds
	// the walks of the kernel over. A nil registry binds every language to
	// the absent rules.
	Rules *rules.Registry
	// Kernel contains the key handles of the kernel. [New] registers them
	// under Keys, as a composition registers them before the keys of any
	// plugin.
	Kernel meta.KernelKeys
	// Languages maps each target to the render language against which the
	// template check lints the declared trees. A fixture without languages
	// skips the check.
	Languages map[plugin.Target]render.Language
	// Turn is the turn in the schedule at which the phase calls run.
	Turn int
	// Workers is the number of invocations that each phase call runs at
	// once. Zero and one dispatch one invocation at a time.
	Workers int
	// Select restricts each phase call to the matches that a warm run
	// executes again, and nil runs every match.
	Select *plugin.Selection
	// Journal receives the records of each phase call, and a nil journal
	// keeps none.
	Journal plugin.Journal
	// Target is the target of the plan of the phase call of a generator.
	// Types is the spoke of the backend of the plan, and Policy is the
	// resolved policy of the plan. A fixture without a spoke refuses every
	// type of another language that a generator translates. Where the spoke
	// is a backend with a filename speller and a package rule, a phase call
	// of a generator previews routes over the layout that [Fixture.Layout]
	// returns.
	Target plugin.Target
	Types  plugin.TypeSpeller
	Policy plugin.Policy

	emit    *plugin.Emit
	claimed map[string]bool
	// plugins are the plugins of the source of a fixture that [Loaded]
	// returns. A phase call binds a plugin in the composition of these
	// plugins and the plugin itself.
	plugins []plugin.Plugin
}

// New returns an empty fixture at turn one, with the keys of the kernel
// registered.
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
		Turn:    1,
		claimed: map[string]bool{},
	}
}

// Load adds parsed packages to the graph.
func (f *Fixture) Load(tb assert.TB, pkgs ...*node.Package) *Fixture {
	tb.Helper()

	assert.Total(tb, f.Graph.AddPackage, pkgs, "the fixture package is admitted")
	return f
}

// LoadDependencies adds parsed packages of a dependency round to the graph.
// The packages of a store are such packages. A rule of the generate phase
// does not run on their declarations, and an annotator and a rule of the
// emit phase do. A run treats them the same way.
func (f *Fixture) LoadDependencies(tb assert.TB, pkgs ...*node.Package) *Fixture {
	tb.Helper()

	assert.Total(tb, f.Graph.AddDependencyPackage, pkgs, "the fixture dependency package is admitted")
	return f
}

// Attach records raw directive instances on a subject, before the first
// phase call seals the graph.
func (f *Fixture) Attach(
	tb assert.TB, subject symbol.Identity, raws ...directive.Raw,
) *Fixture {
	tb.Helper()

	assert.NoError(tb, f.Graph.AttachDirectives(subject, raws),
		"the raw instances attach before the seal")
	return f
}

// Validated sets the typed instances of a subject, in position order and
// with canonical names, as the validation returns them. It also attaches
// one raw instance for each directive, because the dispatch routes a gate
// through the index of spelled names of the store, and only raw
// attachments enter that index. A load attaches at the parse and
// validates at the seal, and Validated performs both steps.
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

// Seed adds units of earlier turns to the emit store that the next
// [Fixture.Generate] call reads. The fixture of a weaver seeds the units of
// the plugins that run before the weaver.
func (f *Fixture) Seed(tb assert.TB, units ...plugin.Unit) *Fixture {
	tb.Helper()

	assert.Total(tb, f.store().Add, units, "the seeded unit is added")
	return f
}

// Key registers a typed key in the registry of the fixture. The first key
// of a namespace claims the namespace.
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

// Stamp records one fact on a subject at the authority of a plugin, as the
// stamp of an earlier annotator records it.
func Stamp[T meta.FactValue](
	tb assert.TB, f *Fixture, k meta.Key[T], subject symbol.Identity, v T,
) {
	tb.Helper()

	assert.NoError(tb, meta.Stamp(f.Facts, k, v, meta.Claim{Subject: subject}),
		"the fixture fact stamps")
}

// Result is what one phase call leaves. It has the emit store of the plan,
// the findings, and the fatal error where the phase returned one.
type Result struct {
	Emit *plugin.Emit
	Sink *diag.Sink
	Err  error
}

// Annotate runs the annotate phase of one plugin over the fixture. It
// first binds the gates of the plugin on named handles in the registry of
// the fixture, and then runs the bound plugin. A plugin that does not
// implement [plugin.Annotator] fails the test. A panic of the phase fails
// the test through [assert.NotPanics], so the other tests of the binary
// still run.
func (f *Fixture) Annotate(tb assert.TB, p plugin.Plugin) Result {
	tb.Helper()

	ann, implemented := f.bind(tb, p).(plugin.Annotator)
	assert.True(tb, implemented, "the plugin implements the annotator role")
	ix := f.index(tb)
	sink := diag.NewSink()
	var err error
	assert.NotPanics(tb, func() {
		err = ann.Annotate(&plugin.AnnotatorContext{
			Index:   ix,
			Reader:  mintReader(tb, ix),
			Facts:   f.Facts,
			Sink:    sink,
			Rules:   f.Rules,
			Kernel:  f.Kernel,
			Plugin:  p.Name(),
			Turn:    f.Turn,
			Workers: f.Workers,
			Select:  f.Select,
			Journal: f.Journal,
		})
	}, "the annotate phase runs without a panic")
	return Result{Emit: f.store(), Sink: sink, Err: err}
}

// Generate runs the generate phase of one plugin over the fixture. It
// first binds the gates of the plugin on named handles in the registry of
// the fixture, and then runs the bound plugin. A plugin that does not
// implement [plugin.Generator] fails the test. A panic of the phase fails
// the test through [assert.NotPanics], so the other tests of the binary
// still run. The emit store persists across calls, so a later Generate
// call sees the earlier flushes, as a later turn does. The call
// translates types with the target, the spoke and the policy of the
// fixture. Where the spoke is also a filename speller and a package rule,
// the call previews a route through them over the layout that
// [Fixture.Layout] returns for p and the plugins of the source of the
// fixture.
func (f *Fixture) Generate(tb assert.TB, p plugin.Plugin) Result {
	tb.Helper()

	gen, implemented := f.bind(tb, p).(plugin.Generator)
	assert.True(tb, implemented, "the plugin implements the generator role")
	var placer plugin.Placer
	speller, spells := f.Types.(plugin.FileSpeller)
	packager, packs := f.Types.(plugin.Packager)
	if spells && packs {
		in := f.Layout(tb, slices.Concat(f.plugins, []plugin.Plugin{p})...)
		in.Speller, in.Packager = speller, packager
		placer = layout.NewPlacer(*in)
	}
	ix := f.index(tb)
	sink := diag.NewSink()
	var err error
	assert.NotPanics(tb, func() {
		err = gen.Generate(&plugin.GeneratorContext{
			Index:   ix,
			Reader:  mintReader(tb, ix),
			Facts:   f.Facts,
			Emit:    f.store(),
			Sink:    sink,
			Rules:   f.Rules,
			Kernel:  f.Kernel,
			Plugin:  p.Name(),
			Turn:    f.Turn,
			Workers: f.Workers,
			Select:  f.Select,
			Journal: f.Journal,
			Target:  f.Target,
			Types:   f.Types,
			Policy:  f.Policy,
			Placer:  placer,
		})
	}, "the generate phase runs without a panic")
	return Result{Emit: f.store(), Sink: sink, Err: err}
}

// Layout returns the input of the layout of a plan over the fixture, as a
// workspace routes the store of a plan whose generators are plugins. The
// input has the index of the fixture, the residents and the modules of its
// graph, the families that each plugin declares, and the sealed registry
// of the schemas of the kernel and of each plugin, which scopes the routing
// keys of a directive. Its configuration is the zero [layout.Config], which
// writes each file beside its source. The render of a spec sets the store,
// the halves of the backend, the target and the sink of the input. Layout
// seals the graph, as the first phase call does, and it stops the test
// where a schema does not register.
func (f *Fixture) Layout(tb assert.TB, plugins ...plugin.Plugin) *layout.Input {
	tb.Helper()

	outputs := make(map[plugin.ID][]plugin.Output, len(plugins))
	for _, p := range plugins {
		outputs[p.Name()] = nil
		if op, declares := p.(plugin.OutputProvider); declares {
			outputs[p.Name()] = op.Outputs()
		}
	}
	ix := f.index(tb)
	residents := layout.Residents(f.Graph)
	return &layout.Input{
		Index:      ix,
		Outputs:    outputs,
		Directives: schemasOf(tb, plugins),
		Residents:  func(dir string) []plugin.Resident { return residents[dir] },
		Modules:    layout.Modules(f.Graph, f.Facts, f.Kernel),
	}
}

// bind returns the plugin that p binds in the registry of the fixture, as
// a workspace binds it when it builds, and p itself where p does not
// implement [plugin.KeyBinder]. A name that the registry does not contain
// fails the test. The composition of the bind is p and the plugins of the
// source of [Loaded]. Where p runs after a capability that no plugin of the
// composition provides, a name whose namespace no registrant claims passes.
// The gate on such a name does not select a subject.
func (f *Fixture) bind(tb assert.TB, p plugin.Plugin) plugin.Plugin {
	tb.Helper()

	kb, binds := p.(plugin.KeyBinder)
	if !binds {
		return p
	}
	bound, err := kb.BindKeys(f.Keys)
	if err != nil && runsWithout(p, slices.Concat(f.plugins, []plugin.Plugin{p})) {
		var joined interface{ Unwrap() []error }
		errs := []error{err}
		if errors.As(err, &joined) {
			errs = joined.Unwrap()
		}
		err = errors.Join(slices.DeleteFunc(slices.Clone(errs), func(e error) bool {
			return errors.Is(e, plugin.ErrUnclaimedNamespace)
		})...)
	}
	assert.NoError(tb, err, "the gates of the plugin on named handles bind")
	return bound
}

// index seals the graph on its first call, and returns the routing surface
// through which a phase call dispatches. The index reads the table of
// validated directives of the fixture.
func (f *Fixture) index(tb assert.TB) *plugin.Index {
	tb.Helper()

	f.Graph.Freeze()
	ix, err := plugin.NewIndex(f.Graph, f.Facts, plugin.ValidatedMap(f.Directives), f.Scope)
	assert.NoError(tb, err, "the routing surface builds")
	return ix
}

// mintReader returns the tracked read handle of a phase call. Its grain is
// the whole call, at which a hand-rolled plugin reads.
func mintReader(tb assert.TB, ix *plugin.Index) *store.Reader {
	tb.Helper()

	r, err := ix.Reader(store.NewReadSet())
	assert.NoError(tb, err, "the tracked handle mints")
	return r
}

// store returns the emit store of the fixture, and creates it on its first
// call.
func (f *Fixture) store() *plugin.Emit {
	if f.emit == nil {
		f.emit = plugin.NewEmit()
	}
	return f.emit
}

// runsWithout reports whether p runs after a capability that no plugin of
// composition provides in a role that the plugin implements. composition
// contains p, as the roster of a workspace does.
func runsWithout(p plugin.Plugin, composition []plugin.Plugin) bool {
	cp, declares := p.(plugin.CapabilityProvider)
	if !declares {
		return false
	}
	return slices.ContainsFunc(cp.After(), func(c plugin.Capability) bool {
		return !slices.ContainsFunc(composition, func(q plugin.Plugin) bool {
			provider, provides := q.(plugin.CapabilityProvider)
			if !provides {
				return false
			}
			_, annotates := q.(plugin.Annotator)
			_, generates := q.(plugin.Generator)
			return annotates && slices.Contains(provider.Provides(plugin.RoleAnnotator), c) ||
				generates && slices.Contains(provider.Provides(plugin.RoleGenerator), c)
		})
	})
}
