// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugintest

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// Setup builds the plugin under test together with the fixture it
// runs over, the way a composition builds them: sharing one key
// registry, one schema set, one graph. The suite calls it more
// than once, because declaration stability compares two builds and
// determinism compares two isolated runs, so a Setup returns a
// fresh pair every call.
type Setup func(tb assert.TB) (plugin.Plugin, *Fixture)

// RunPluginSuite holds a plugin to the conformance checks a fixture
// can check without a workspace: declaration stability, byte-equal
// emit across isolated runs, annotator idempotence, positioned
// diagnostics, attribution, declared tags, the options schema, the
// template lint, and no panics. Checks for roles or surfaces the
// plugin does not hold are skipped.
func RunPluginSuite(t *testing.T, setup Setup) {
	t.Helper()

	probe, fixture := setup(t)
	_, annotates := probe.(plugin.Annotator)
	_, generates := probe.(plugin.Generator)
	options, optioned := probe.(plugin.OptionsProvider)
	templated := declaresTree(probe, fixture)

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

// AssertStableDeclaration holds two builds of one plugin to the
// same declaration: the name, the gate records, the outputs and
// the owned schemas. A declaration that varies between builds
// breaks every consumer that keys on it.
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
			"the owned schemas are stable across builds")
	}
}

// declaresTree reports whether the plugin declares a template tree
// for any language the fixture carries. The facade gives every
// plugin the provider's shape, so the shape alone proves nothing:
// the suite gates its lint on a declared tree, never on the
// interface.
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

// AssertTemplates holds every declared template tree to the static
// template rules, through the same lint the render pass runs, per
// language the fixture carries: a plugin failing at CI fails in
// its own tests first. A setup declaring no tree for any fixture
// language fails the check, because a lint over nothing proves
// nothing; [RunPluginSuite] runs it only for a plugin declaring
// one.
func AssertTemplates(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	tp, held := p.(plugin.TemplateProvider)
	if !held {
		tb.Errorf("the plugin declares no templates, and the check proves nothing")
		return
	}
	linted := 0
	for _, target := range slices.Sorted(maps.Keys(f.Languages)) {
		tree, declared := tp.Templates(target)
		if !declared {
			continue
		}
		linted++
		pass, err := render.New("lint", f.Languages[target])
		assert.NoError(tb, err, "the fixture language composes")
		for _, finding := range pass.Lint(tree, tp.TemplateFuncs(target), tp.Overrides()) {
			assert.NoError(tb, finding, "the tree holds the template rules")
		}
	}
	assert.True(tb, linted > 0,
		"no fixture language meets a declared tree, and the check proves nothing")
}

// AssertOptionsSchema holds the plugin's options struct to the tag
// contract, through the same check the composition runs, so a
// plugin failing at Build fails in its own tests first.
func AssertOptionsSchema(tb assert.TB, setup Setup) {
	tb.Helper()

	p, _ := setup(tb)
	for _, err := range plugin.ValidateOptions(p) {
		assert.NoError(tb, err, "the options struct holds the tag contract")
	}
}

// AssertDeterministicEmit runs one plugin over two isolated
// fixtures and holds the emitted bytes equal: the byte-identity
// contract, checked before any renderer exists.
func AssertDeterministicEmit(tb assert.TB, setup Setup) {
	tb.Helper()

	firstPlugin, firstFixture := setup(tb)
	secondPlugin, secondFixture := setup(tb)
	assert.Equal(tb,
		string(encodeEmit(tb, generateOnce(tb, secondPlugin, secondFixture))),
		string(encodeEmit(tb, generateOnce(tb, firstPlugin, firstFixture))),
		"two isolated runs emit the same bytes")
}

// AssertIdempotentAnnotate runs one plugin's annotate phase twice
// over one fixture and holds both passes clean. A stamp that
// depends on run state arrives a second value from the same rank
// source, which the fact store refuses, and the refusal fails this
// check.
func AssertIdempotentAnnotate(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	first := f.Annotate(tb, p)
	assert.NoError(tb, first.Err, "the first pass runs whole")
	assert.False(tb, first.Sink.Failed(), "and stamps clean")
	second := f.Annotate(tb, p)
	assert.NoError(tb, second.Err, "the second pass runs whole")
	assert.False(tb, second.Sink.Failed(),
		"a repeated pass re-stamps identical claims, never new values")
}

// AssertPositionedDiagnostics runs every phase the plugin holds and
// refuses a finding without a position: a diagnostic nobody can
// jump to is a defect in whatever reported it.
func AssertPositionedDiagnostics(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	for _, r := range runAll(tb, p, f) {
		for d := range r.Sink.All() {
			assert.False(tb, d.Pos.IsZero(),
				"every finding names the position it is about")
		}
	}
}

// AssertAttributedEmit runs the generate phase and holds every unit
// to its plugin's name and its declared families: output nobody can
// attribute, or under a tag nothing declared, is a routing hole.
func AssertAttributedEmit(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	declared := map[string]bool{}
	if outputs, held := p.(plugin.OutputProvider); held {
		for _, o := range outputs.Outputs() {
			declared[o.Tag] = true
		}
	}
	for u := range generateOnce(tb, p, f).Units() {
		assert.Equal(tb, u.Plugin, p.Name(),
			"every unit names the plugin that emitted it")
		assert.True(tb, declared[u.Tag],
			"every unit arrives under a declared family")
	}
}

// AssertTwins holds two spellings of one plugin, usually a facade
// build and a hand-rolled SPI twin, to byte-equal emit: the
// lowering guarantee, checked from the outside. Each setup returns
// its own spelling over an equivalent fixture.
func AssertTwins(tb assert.TB, facade, twin Setup) {
	tb.Helper()

	facadePlugin, facadeFixture := facade(tb)
	twinPlugin, twinFixture := twin(tb)
	assert.Equal(tb,
		string(encodeEmit(tb, generateOnce(tb, twinPlugin, twinFixture))),
		string(encodeEmit(tb, generateOnce(tb, facadePlugin, facadeFixture))),
		"both spellings of the plugin emit the same bytes")
}

// runAll runs every phase the plugin holds, annotate first, over
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

// generateOnce runs annotate where the plugin holds it, then
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
		for _, d := range u.Decls {
			encoded, err := emit.EncodeJSON(d)
			assert.NoError(tb, err, "every emitted declaration encodes")
			out.Write(encoded)
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

// AssertNoStructuralWrites runs every phase the plugin holds and
// holds the graph's bytes still: annotators and generators read
// declarations through shared pointers, so mutating one in place
// is the structural write the sealed store cannot refuse, and this
// check is what catches it.
func AssertNoStructuralWrites(tb assert.TB, setup Setup) {
	tb.Helper()

	p, f := setup(tb)
	f.Graph.Freeze()
	before := encodeGraph(tb, f)
	runAll(tb, p, f)
	assert.Equal(tb, string(encodeGraph(tb, f)), string(before),
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
