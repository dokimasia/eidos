// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored_test

import (
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/authored"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The fixture's names and the brand its composition declares. The
// sample and witness cases share the fixture.
const (
	pkgPath      = coretest.StorePath
	boxName      = "Box"
	alphaName    = "Alpha"
	itemName     = "Item"
	loadName     = "Load"
	paramT       = "T"
	fixtureFile  = "box.go"
	fixtureBrand = "fixture"
)

// sampleAllocs is one construction of the sample annotator: 2 for each
// of the 12 kind rules, the leaf and its handler; 8 for the kernel's
// directive schemas the gate is checked against; 12 for the rules' gate
// spellings and 5 for the growth of the lowered rule list; 5 for the
// growth of the subscriptions; and 6 for the builder, the gate, the
// rule list and the built plugin.
const sampleAllocs = 60

// fixture is the graph the cases run over: a generic Box[T] with
// one field, a plain Alpha a witness can name, and a Load function
// with one parameter and one return.
type fixture struct {
	graph *store.Graph
	box   *node.Struct
	item  *node.Field
	alpha *node.Struct
	load  *node.Function
}

// fakeBackend is a backend by name and target alone.
type fakeBackend struct{}

// Name returns the backend's fixed name.
func (fakeBackend) Name() plugin.ID { return "printer" }

// Target returns the fixture target.
func (fakeBackend) Target() plugin.Target { return "fixture" }

// fixtureRules is the scripted rules registered under the fixture's
// language, resolving a type name as a struct in the fixture
// package.
type fixtureRules struct{}

// Lang returns the fixture's language.
func (fixtureRules) Lang() symbol.Lang { return coretest.Lang }

// Members returns the scripted rules' member policy.
func (fixtureRules) Members() rules.MemberPolicy { return rulestest.Scripted().Members() }

// ParamRole returns the scripted rules' role for a parameter.
func (fixtureRules) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return rulestest.Scripted().ParamRole(p, v)
}

// ReturnRoles returns the scripted rules' roles for the returns.
func (fixtureRules) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return rulestest.Scripted().ReturnRoles(rs, v)
}

// Builtin returns the scripted rules' shape for a builtin.
func (fixtureRules) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return rulestest.Scripted().Builtin(ref, v)
}

// Resolve returns the fixture package's struct named name.
func (fixtureRules) Resolve(
	_ rules.Scope, name string, _ directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	if sym, held := v.Lookup(coretest.ID(pkgPath, name, symbol.KindStruct)); held {
		return sym, nil
	}
	return nil, fmt.Errorf("nothing in %s is named %s", pkgPath, name)
}

// SamplesOf returns the scripted rules' samples.
func (fixtureRules) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return rulestest.Scripted().SamplesOf(ref, hint, v)
}

// ZeroValue returns the scripted rules' zero value.
func (fixtureRules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().ZeroValue(ref, v)
}

// LiteralFor returns the scripted rules' literal for text.
func (fixtureRules) LiteralFor(f *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().LiteralFor(f, ref, text, v)
}

// TypeName returns the scripted rules' type name.
func (fixtureRules) TypeName(word, base string) string {
	return rulestest.Scripted().TypeName(word, base)
}

// The sample annotator stamps the text an author wrote under the
// kernel's sample keys, on every kind whose key admits it.
func TestSample(t *testing.T) {
	t.Parallel()

	t.Run("Sample", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the value on a field", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.item.ID, 4, directive.KernelSample, "value=1", "alternate=2")
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.NoError(t, err, "the run is clean")
			assert.False(t, report.Sink.Failed(), "without a finding")
			v, held := meta.Get(report.Facts, f.item.ID, w.Kernel().Sample)
			assert.True(t, held, "the value is stamped")
			assert.Equal(t, v, "1", "the value arrives as the text the author wrote")
		})

		t.Run("stamps the alternate on a field", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.item.ID, 4, directive.KernelSample, "value=1", "alternate=2")
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.NoError(t, err, "the run is clean")
			alt, held := meta.Get(report.Facts, f.item.ID, w.Kernel().Alternate)
			assert.True(t, held, "the alternate is stamped")
			assert.Equal(t, alt, "2", "the alternate arrives as the text the author wrote")
		})

		t.Run("leaves the alternate absent where none was stated", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.item.ID, 4, directive.KernelSample, "value=1")
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.NoError(t, err, "the run is clean")
			_, held := meta.Get(report.Facts, f.item.ID, w.Kernel().Alternate)
			assert.False(t, held, "a derivation fills what the author left out")
		})

		t.Run("stamps a parameter", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.load.Params[0].ID, 12, directive.KernelSample, "value=in")
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.NoError(t, err, "the run is clean")
			v, _ := meta.Get(report.Facts, f.load.Params[0].ID, w.Kernel().Sample)
			assert.Equal(t, v, "in", "a parameter is a subject the sample reaches")
		})

		t.Run("stamps a return", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.load.Returns[0].ID, 13, directive.KernelSample, "value=out")
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.NoError(t, err, "the run is clean")
			v, _ := meta.Get(report.Facts, f.load.Returns[0].ID, w.Kernel().Sample)
			assert.Equal(t, v, "out", "a return is a subject the sample reaches")
		})

		t.Run("reports RefusedStamp for a sample on a callable", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.load.ID, 11, directive.KernelSample, "value=1")
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.Contains(t, coretest.Codes(report.Sink), eidos.RefusedStamp,
				"the key admits no callable, so the stamp is refused at the subject")
			_, held := meta.Get(report.Facts, f.load.ID, w.Kernel().Sample)
			assert.False(t, held, "and nothing is stamped")
		})

		t.Run("returns an annotator named gen.sample", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, authored.Sample().Name(), authored.SamplePlugin, "the annotator names itself")
		})
	})
}

// A construction of the sample annotator allocates within its ceiling
// in the ordinary run, which runs no benchmark.
func TestSampleAllocs(t *testing.T) {
	var got plugin.Annotator
	assert.MaxAllocs(t, func() { got = authored.Sample() }, sampleAllocs,
		"Sample allocates the plugin, its rules and their handlers")
	assert.Equal(t, got.Name(), authored.SamplePlugin, "Sample returns the sample annotator")
}

// BenchmarkSample measures one construction of the sample annotator.
func BenchmarkSample(b *testing.B) {
	b.Run("Sample", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(sampleAllocs)
		defer c.End()
		var got plugin.Annotator
		for c.Loop() {
			got = authored.Sample()
		}
		assert.Equal(b, got.Name(), authored.SamplePlugin, "Sample returns the sample annotator")
	})
}

// build returns an unfrozen fixture graph.
func build(tb assert.TB) fixture {
	tb.Helper()

	box := coretest.Struct(pkgPath, boxName)
	box.Pos = position.Pos{File: fixtureFile, Line: 3, Col: 1}
	box.TypeParams = []*node.TypeParam{{
		ID: symbol.Identity{
			Lang: coretest.Lang, Package: pkgPath, Owner: boxName, Name: paramT, Kind: symbol.KindTypeParam,
		},
		Name: paramT,
		Pos:  position.Pos{File: fixtureFile, Line: 3, Col: 10},
	}}
	item := coretest.Field(pkgPath, boxName, itemName)
	item.Type = &node.TypeRef{Spelling: paramT}
	item.Pos = position.Pos{File: fixtureFile, Line: 4, Col: 2}
	box.Fields = []*node.Field{item}
	alpha := coretest.Struct(pkgPath, alphaName)
	alpha.Pos = position.Pos{File: fixtureFile, Line: 8, Col: 1}
	load := coretest.Function(pkgPath, loadName)
	load.Pos = position.Pos{File: fixtureFile, Line: 12, Col: 1}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(pkgPath, box, alpha, load)),
		"the fixture package is admitted")
	return fixture{graph: g, box: box, item: item, alpha: alpha, load: load}
}

// composed returns a workspace listing both authored annotators,
// the fixture rules and one plan, as a composition would.
func composed(tb assert.TB) *workspace.Workspace {
	tb.Helper()

	gen, held := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
		Build().(plugin.Generator)
	assert.True(tb, held, "an emitter handler makes a generator")
	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(authored.Sample(), authored.Witness()).
		Rules(fixtureRules{}).
		Targets("fixture").
		Plans(workspace.Plan{
			Name: "plan", Generators: []plugin.Generator{gen},
			Backend: fakeBackend{},
		}).
		Build()
	assert.NoError(tb, err, "the composition composes")
	return w
}

// attach attaches one raw kernel directive to a subject: the name,
// then key=value pairs.
func attach(tb assert.TB, g *store.Graph, subject symbol.Identity, line int, name directive.Name, pairs ...string) {
	tb.Helper()

	raw := directive.Raw{Name: name, Pos: position.Pos{File: fixtureFile, Line: line, Col: 1}}
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		raw.Args = append(raw.Args, directive.RawArg{Key: key, Value: directive.RawValue{Text: value}})
	}
	assert.NoError(tb, g.AttachDirectives(subject, []directive.Raw{raw}), "the directive attaches")
}
