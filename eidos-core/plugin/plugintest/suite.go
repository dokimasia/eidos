// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Setup builds the plugin under test together with the fixture it
// runs over, the way a composition builds them: sharing one key
// registry, one schema set, one graph. The suite calls it more
// than once, because declaration stability compares two builds and
// determinism compares two isolated runs, so a Setup returns a
// fresh pair every call.
type Setup func(tb assert.TB) (plugin.Plugin, *Fixture)

// parallelWorkers is how many invocations a phase call runs at once in
// the parallel dispatch check.
const parallelWorkers = 8

// RunPluginSuite runs the conformance checks a fixture needs no
// workspace for: declaration stability, byte-equal emit across
// isolated runs, the same output under parallel dispatch, the same
// output under a selection of every match, annotator idempotence,
// positioned diagnostics, attribution, declared tags, the options
// schema, the template lint, and no panics. It skips the checks for a
// role or a surface the plugin does not implement.
func RunPluginSuite(t *testing.T, setup Setup) {
	t.Helper()

	probe, fixture := setup(t)
	_, annotates := probe.(plugin.Annotator)
	_, generates := probe.(plugin.Generator)
	options, optioned := probe.(plugin.OptionsProvider)
	templated := declaresTree(probe, fixture)

	t.Run("populated fixture", func(t *testing.T) {
		t.Parallel()
		AssertPopulatedFixture(t, setup)
	})
	t.Run("declaration stability", func(t *testing.T) {
		t.Parallel()
		AssertStableDeclaration(t, setup)
	})
	if optioned && options.Options() != nil {
		t.Run("options schema", func(t *testing.T) {
			t.Parallel()
			AssertOptionsSchema(t, setup)
		})
	}
	if templated {
		t.Run("template lint", func(t *testing.T) {
			t.Parallel()
			AssertTemplates(t, setup)
		})
	}
	if generates {
		t.Run("deterministic emit", func(t *testing.T) {
			t.Parallel()
			AssertDeterministicEmit(t, setup)
		})
		t.Run("attribution and declared tags", func(t *testing.T) {
			t.Parallel()
			AssertAttributedEmit(t, setup)
		})
	}
	if annotates {
		t.Run("annotator idempotence", func(t *testing.T) {
			t.Parallel()
			AssertIdempotentAnnotate(t, setup)
		})
	}
	t.Run("parallel dispatch", func(t *testing.T) {
		t.Parallel()
		AssertParallelDispatch(t, setup)
	})
	t.Run("selective dispatch", func(t *testing.T) {
		t.Parallel()
		AssertSelective(t, setup)
	})
	t.Run("positioned diagnostics", func(t *testing.T) {
		t.Parallel()
		AssertPositionedDiagnostics(t, setup)
	})
	t.Run("no structural writes", func(t *testing.T) {
		t.Parallel()
		AssertNoStructuralWrites(t, setup)
	})
	t.Run("no panics", func(t *testing.T) {
		t.Parallel()
		assert.NotPanics(t, func() {
			p, f := setup(t)
			runAll(t, p, f)
		}, "a fixture run never panics")
	})
}

// AssertPopulatedFixture refuses a fixture whose files declare
// nothing: every other check in the suite passes vacuously over an
// empty run and proves nothing, which is the emptiness this kit's
// siblings already refuse. Every declaration a file declares is of
// a kind a trigger matches, so one is enough.
func AssertPopulatedFixture(tb assert.TB, setup Setup) {
	tb.Helper()

	_, f := setup(tb)
	assert.NotNil(tb, f, "the setup returns a fixture")
	assert.NotNil(tb, f.Graph, "the fixture contains a graph")
	// No other check uses this setup's fixture, so sealing it here
	// is the same seal the first phase call would make.
	f.Graph.Freeze()
	declared := 0
	for pkg := range f.Graph.Packages() {
		for _, file := range pkg.Files {
			if file != nil {
				declared += len(file.Decls)
			}
		}
	}
	assert.NotEqual(tb, declared, 0,
		"the fixture graph's files declare something: an empty run passes vacuously and proves nothing")
}

// AssertStableDeclaration fails unless two builds of one plugin
// declare the same name, gate records, outputs and schemas. A
// declaration that varies between builds breaks every consumer
// that keys on it.
func AssertStableDeclaration(tb assert.TB, setup Setup) {
	tb.Helper()

	first, _ := setup(tb)
	second, _ := setup(tb)
	assert.Equal(tb, second.Name(), first.Name(),
		"the name is the identity everything durable keys on")
	if a, held := first.(plugin.Subscribed); held {
		b, alsoHeld := second.(plugin.Subscribed)
		assert.True(tb, alsoHeld, "both builds declare their gates")
		assert.Equal(tb, b.Subscriptions(), a.Subscriptions(),
			"the gate records are stable across builds")
	}
	if a, held := first.(plugin.OutputProvider); held {
		b, alsoHeld := second.(plugin.OutputProvider)
		assert.True(tb, alsoHeld, "both builds declare their outputs")
		assert.Equal(tb, b.Outputs(), a.Outputs(),
			"the families are stable across builds")
	}
	if a, held := first.(plugin.DirectiveProvider); held {
		b, alsoHeld := second.(plugin.DirectiveProvider)
		assert.True(tb, alsoHeld, "both builds declare their schemas")
		assert.Equal(tb, b.Directives(), a.Directives(),
			"the registered schemas are stable across builds")
	}
}

// declaresTree reports whether the plugin declares a template tree
// for any language the fixture lists. The facade gives every plugin
// the provider's methods, so the suite lints a plugin that declares a
// tree and not every plugin that implements the interface.
func declaresTree(p plugin.Plugin, f *Fixture) bool {
	tp, held := p.(plugin.TemplateProvider)
	if !held || f == nil {
		return false
	}
	for target := range f.Languages {
		if _, declared := tp.Templates(target); declared {
			return true
		}
	}
	return false
}

// AssertTemplates lints every declared template tree through the
// lint the render pass runs, once per language the fixture has,
// with the helpers and overrides the plugin declares for that
// language: a plugin that would fail at CI fails in its own tests
// first. A setup that declares no tree for any fixture language
// fails the check, because a lint over nothing proves nothing, and
// [RunPluginSuite] runs it only for a plugin that declares one.
func AssertTemplates(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	tp, held := p.(plugin.TemplateProvider)
	assert.True(tb, held, "the plugin declares templates, so the check proves something")
	linted := 0
	for _, target := range slices.Sorted(maps.Keys(f.Languages)) {
		tree, declared := tp.Templates(target)
		if !declared {
			continue
		}
		linted++
		pass, err := render.New("lint", f.Languages[target])
		assert.NoError(tb, err, "the fixture language composes")
		for _, finding := range pass.Lint(tree, tp.TemplateFuncs(target), tp.Overrides(target)) {
			expect.NoError(tb, finding, "the tree meets the template rules")
		}
	}
	assert.NotEqual(tb, linted, 0, "a fixture language meets a declared tree, so the check proves something")
}

// AssertOptionsSchema checks the plugin's options struct against
// the tag contract, through the check the composition runs, so a
// plugin that would fail at Build fails in its own tests first. Each
// fault of the struct reports on its own.
func AssertOptionsSchema(tb assert.TB, setup Setup) {
	tb.Helper()

	p, _ := setup(tb)
	for _, err := range plugin.ValidateOptions(p) {
		expect.NoError(tb, err, "the options struct meets the tag contract")
	}
}

// AssertDeterministicEmit runs the plugin of a fresh setup over the
// setup's fixture once per call of [assert.Deterministic], and fails
// unless every run emits the bytes of the first: the byte-identity
// contract, checked before any renderer exists.
func AssertDeterministicEmit(tb assert.TB, setup Setup) {
	tb.Helper()

	assert.Deterministic(tb, func(s Setup) (string, error) {
		p, f := s(tb)
		return string(encodeEmit(tb, generateOnce(tb, p, f))), nil
	}, setup, "isolated runs emit the same bytes")
}

// AssertParallelDispatch runs every phase the plugin implements over
// two isolated fixtures, one dispatching sequentially and one on eight
// workers, and fails unless both runs emit the same bytes, end with the
// same fact values and report the same findings in the same order: the
// output of a phase call does not depend on its worker count. Run under
// the race detector, the parallel run also exposes state a handler
// writes outside its effects.
func AssertParallelDispatch(tb assert.TB, setup Setup) {
	tb.Helper()

	serialPlugin, serialFixture := setup(tb)
	parallelPlugin, parallelFixture := setup(tb)
	serialFixture.Workers, parallelFixture.Workers = 1, parallelWorkers
	serial := runAll(tb, serialPlugin, serialFixture)
	parallel := runAll(tb, parallelPlugin, parallelFixture)
	expect.Equal(tb,
		string(encodeEmit(tb, parallelFixture.store())),
		string(encodeEmit(tb, serialFixture.store())),
		"a parallel phase call emits what a sequential one does")
	expect.Equal(tb, presentFacts(parallelFixture), presentFacts(serialFixture),
		"and it ends with the fact values a sequential call ends with")
	expect.Equal(tb, findingsOf(parallel), findingsOf(serial),
		"and it reports the same findings in the same order")
}

// AssertSelective runs every phase the plugin implements twice, with one
// plugin over two isolated fixtures: whole with a journal, then under a
// selection that lists every match the whole run journaled for the
// phase. It fails unless both runs emit the same bytes, end with the
// same fact values and report the same findings, and unless each phase's
// journal lists every match once, in canonical match order. The second
// run takes the fixture of a second setup and keeps the plugin of the
// first, as a workspace runs one plugin cold and then warm, so a handler
// whose output depends on an earlier invocation fails the check. The
// selected run executes its matches in canonical match order, which an
// enumeration in another order does not, so the check compares the
// findings in [diag.Diag.Compare] order. A plugin that journals nothing
// runs whole both times and passes.
func AssertSelective(tb assert.TB, setup Setup) {
	tb.Helper()

	p, wholeFixture := setup(tb)
	_, selectedFixture := setup(tb)
	whole, wholeKeys := journaledRun(tb, p, wholeFixture, nil)
	selected, selectedKeys := journaledRun(tb, p, selectedFixture, wholeKeys)
	for _, keys := range wholeKeys {
		expect.Pairwise(tb, keys, func(earlier, later plugin.MatchKey) bool { return earlier.Compare(later) < 0 },
			"the journal lists every match once, in canonical match order")
	}
	expect.Equal(tb, selectedKeys, wholeKeys, "the selected run journals the matches the whole run journaled")
	expect.Equal(tb,
		string(encodeEmit(tb, selectedFixture.store())),
		string(encodeEmit(tb, wholeFixture.store())),
		"a selection of every match emits what the whole call does")
	expect.Equal(tb, presentFacts(selectedFixture), presentFacts(wholeFixture),
		"and it ends with the fact values the whole call ends with")
	expect.Equal(tb, canonicalFindings(selected), canonicalFindings(whole),
		"and it reports the same findings")
}

// keyJournal keeps the match keys one phase call journals, in the order
// the call passes them to Invoked.
type keyJournal struct{ keys []plugin.MatchKey }

// Invoked keeps the invocation's key.
func (j *keyJournal) Invoked(inv plugin.Invocation) { j.keys = append(j.keys, inv.Match) }

// Evaluated keeps nothing: neither run of [AssertSelective] names a
// candidate.
func (*keyJournal) Evaluated(symbol.Identity, []plugin.MatchKey) {}

// journaledRun runs every phase the plugin implements, annotate first,
// each with a journal of its own, and under a selection of the phase's
// keys where selected lists keys for the phase. It returns the results
// and the keys each phase journaled, both in phase order.
func journaledRun(
	tb assert.TB, p plugin.Plugin, f *Fixture, selected [][]plugin.MatchKey,
) ([]Result, [][]plugin.MatchKey) {
	tb.Helper()

	var results []Result
	var keys [][]plugin.MatchKey
	run := func(call func(assert.TB, plugin.Plugin) Result) {
		j := &keyJournal{}
		f.Journal, f.Select = j, nil
		if phase := len(keys); phase < len(selected) {
			f.Select = &plugin.Selection{Matches: selected[phase]}
		}
		r := call(tb, p)
		assert.NoError(tb, r.Err, "the phase runs whole")
		results = append(results, r)
		keys = append(keys, j.keys)
	}
	if _, held := p.(plugin.Annotator); held {
		run(f.Annotate)
	}
	if _, held := p.(plugin.Generator); held {
		run(f.Generate)
	}
	f.Journal, f.Select = nil, nil
	return results, keys
}

// findingsOf returns every finding of a sequence of phase results, in
// phase order and then report order.
func findingsOf(results []Result) []diag.Diag {
	var out []diag.Diag
	for _, r := range results {
		out = slices.AppendSeq(out, r.Sink.All())
	}
	return out
}

// canonicalFindings returns every finding of a sequence of phase
// results in [diag.Diag.Compare] order.
func canonicalFindings(results []Result) []diag.Diag {
	out := findingsOf(results)
	slices.SortStableFunc(out, diag.Diag.Compare)
	return out
}

// AssertIdempotentAnnotate runs one plugin's annotate phase twice
// over one fixture, through [assert.Idempotent]. It fails unless both
// passes run whole and stamp clean, and the second pass leaves every
// fact's value as the first left it. A stamp that depends on run state
// either claims a second value from the same rank source, which the
// fact store refuses, or changes the value that ranks first, which the
// comparison refuses.
func AssertIdempotentAnnotate(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	assert.Idempotent(tb, func(annotator plugin.Plugin) error {
		r := f.Annotate(tb, annotator)
		assert.False(tb, r.Sink.Failed(), "a pass stamps clean, and a repeated pass re-stamps identical claims")
		return r.Err
	}, p, func() []presentFact { return presentFacts(f) },
		"a repeated pass leaves every fact's value as the first pass left it")
}

// presentFact is one present fact: its subject, its key and the value
// of its claim that ranks first.
type presentFact struct {
	subject symbol.Identity
	key     meta.KeyName
	value   any
}

// presentFacts returns every present fact in the fixture's store, in
// key registration order and then identity order.
func presentFacts(f *Fixture) []presentFact {
	var out []presentFact
	for name := range f.Keys.Keys() {
		id, _ := f.Keys.Resolve(name)
		for subject := range f.Facts.ByKey(id) {
			for view := range f.Facts.Claims(subject, id) {
				out = append(out, presentFact{subject: subject, key: name, value: view.Value})
				break
			}
		}
	}
	return out
}

// AssertPositionedDiagnostics runs every phase the plugin implements and
// refuses a finding without a position: a diagnostic nobody can
// jump to is a defect in whatever reported it. Each finding reports on
// its own.
func AssertPositionedDiagnostics(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	for _, r := range runAll(tb, p, f) {
		for d := range r.Sink.All() {
			expect.NotEqual(tb, d.Pos, position.Pos{}, "every finding names the position it is about: "+d.Msg)
		}
	}
}

// AssertAttributedEmit runs the generate phase and checks every
// unit the plugin flushed against the plugin's name and its
// declared families: output nobody can attribute, or under a tag
// nothing declared, is a routing hole. The check skips the units
// the fixture seeded, which are other plugins' output.
func AssertAttributedEmit(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	declared := map[string]bool{}
	if outputs, held := p.(plugin.OutputProvider); held {
		for _, o := range outputs.Outputs() {
			declared[o.Tag] = true
		}
	}
	seeded := map[plugin.UnitRef]bool{}
	for u := range f.store().Units() {
		seeded[u.Ref()] = true
	}
	for u := range generateOnce(tb, p, f).Units() {
		if seeded[u.Ref()] {
			continue
		}
		expect.Equal(tb, u.Plugin, p.Name(), "every unit names the plugin that emitted it")
		expect.Contains(tb, declared, u.Tag, "every unit arrives under a declared family")
	}
}

// AssertTwins fails unless two spellings of one plugin, usually a
// facade build and a hand-rolled SPI twin, emit the same bytes: the
// lowering guarantee, checked from outside the facade. Each setup
// returns its own spelling over an equivalent fixture.
func AssertTwins(tb assert.TB, facade, twin Setup) {
	tb.Helper()

	facadePlugin, facadeFixture := facade(tb)
	twinPlugin, twinFixture := twin(tb)
	assert.Equal(tb,
		string(encodeEmit(tb, generateOnce(tb, twinPlugin, twinFixture))),
		string(encodeEmit(tb, generateOnce(tb, facadePlugin, facadeFixture))),
		"both spellings of the plugin emit the same bytes")
}

// runAll runs every phase the plugin implements, annotate first, over
// one fixture, and returns the results in phase order.
func runAll(tb assert.TB, p plugin.Plugin, f *Fixture) []Result {
	tb.Helper()

	var out []Result
	if _, held := p.(plugin.Annotator); held {
		r := f.Annotate(tb, p)
		assert.NoError(tb, r.Err, "the annotate phase runs whole")
		out = append(out, r)
	}
	if _, held := p.(plugin.Generator); held {
		r := f.Generate(tb, p)
		assert.NoError(tb, r.Err, "the generate phase runs whole")
		out = append(out, r)
	}
	return out
}

// generateOnce runs annotate where the plugin implements it, then
// generate, and returns the fixture's emit store.
func generateOnce(tb assert.TB, p plugin.Plugin, f *Fixture) *plugin.Emit {
	tb.Helper()

	if _, held := p.(plugin.Annotator); held {
		r := f.Annotate(tb, p)
		assert.NoError(tb, r.Err, "the annotate phase runs whole")
	}
	r := f.Generate(tb, p)
	assert.NoError(tb, r.Err, "the generate phase runs whole")
	return r.Emit
}

// encodeEmit renders an emit store as deterministic bytes: units in
// their total order, each declaration through the emit codec.
func encodeEmit(tb assert.TB, e *plugin.Emit) []byte {
	tb.Helper()

	var out bytes.Buffer
	for u := range e.Units() {
		fmt.Fprintf(&out, "unit %s %q %d %q %q %s\n",
			u.Plugin, u.Tag, u.Per, u.Word, u.Key, u.Pkg)
		for _, id := range u.Origins {
			fmt.Fprintf(&out, "origin %s\n", id)
		}
		for _, p := range u.Contributors {
			fmt.Fprintf(&out, "contributor %s\n", p)
		}
		for _, d := range u.Decls {
			encoded, err := emit.EncodeJSON(d)
			assert.NoError(tb, err, "every emitted declaration encodes")
			out.Write(encoded)
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

// AssertNoStructuralWrites runs every phase the plugin implements
// and fails unless the graph's bytes are unchanged afterwards:
// annotators and generators read declarations through shared
// pointers, so mutating one in place is the structural write the
// sealed store cannot refuse, and this check catches it.
func AssertNoStructuralWrites(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	f.Graph.Freeze()
	assert.Pure(tb, func() string { return string(encodeGraph(tb, f)) }, func() { runAll(tb, p, f) },
		"the graph is input truth, and no phase call rewrites it")
}

// encodeGraph renders every loaded package as deterministic bytes.
func encodeGraph(tb assert.TB, f *Fixture) []byte {
	tb.Helper()

	var out bytes.Buffer
	for pkg := range f.Graph.ByKind(symbol.KindPackage) {
		encoded, err := node.EncodeJSON(pkg)
		assert.NoError(tb, err, "every loaded package encodes")
		out.Write(encoded)
		out.WriteByte('\n')
	}
	return out.Bytes()
}
