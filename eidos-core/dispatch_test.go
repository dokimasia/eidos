// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos_test

import (
	"errors"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// testCode is a code for handler-side reporting cases; the sink
// carries codes without consulting a registry.
var testCode = diag.Code{Prefix: "tst", Number: 1}

// boolKey returns a registered bool key and a fact store built over
// its registry.
func boolKey(tb assert.TB) (meta.Key[bool], *meta.Facts) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace("t", "the test"),
		"the namespace is claimed")
	key, err := meta.Register[bool](reg, meta.KeySpec{
		Name: "t.flag", Doc: "marks a fixture subject",
	})
	assert.NoError(tb, err, "the key registers")
	return key, meta.NewFacts(reg)
}

// fixtureGraph returns a frozen graph holding two positioned structs
// in the store package.
func fixtureGraph(tb assert.TB) (*store.Graph, *node.Struct, *node.Struct) {
	tb.Helper()

	alpha := coretest.Struct(coretest.StorePath, "Alpha")
	alpha.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
	beta := coretest.Struct(coretest.StorePath, "Beta")
	beta.Pos = position.Pos{File: "beta.go", Line: 7, Col: 1}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, alpha, beta)),
		"the fixture package is admitted")
	g.Freeze()
	return g, alpha, beta
}

// genContext returns a generator context over the fixture graph.
func genContext(
	tb assert.TB, g *store.Graph, facts *meta.Facts,
	validated map[symbol.Identity][]directive.Directive,
) *plugin.GeneratorContext {
	tb.Helper()

	ix, err := plugin.NewIndex(g, facts, validated, nil)
	assert.NoError(tb, err, "the routing surface builds")
	return &plugin.GeneratorContext{
		Index:  ix,
		Facts:  facts,
		Emit:   plugin.NewEmit(),
		Sink:   diag.NewSink(),
		Plugin: "weaver",
		Bucket: 2,
	}
}

// seed arrives one earlier-bucket unit holding decls into ctx.Emit.
func seed(tb assert.TB, ctx *plugin.GeneratorContext, decls ...symbol.Symbol) {
	tb.Helper()

	assert.NoError(tb, ctx.Emit.Add(plugin.Unit{
		Plugin: "earlier", Per: plugin.PerSource, Word: "impl",
		Key: "a.go", Decls: decls,
	}), "the earlier bucket's unit arrives")
}

// emitted returns an origined emit struct for one node subject.
func emitted(origin *node.Struct) *emit.Struct {
	return &emit.Struct{Origin: origin.ID, Name: "Gen" + origin.Name}
}

// generatorOf builds and asserts the generator role of a plugin.
func generatorOf(tb assert.TB, p plugin.Plugin) plugin.Generator {
	tb.Helper()

	gen, ok := p.(plugin.Generator)
	assert.True(tb, ok, "emitter rules make the value a generator")
	return gen
}

// Dispatch is where the rules meet: indexed enumeration, skip, gate
// views, per-invocation grain, deterministic flush. Every case here
// is a guarantee a plugin author gets to assume.
func TestDispatch(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("runs a graph rule once and flushes its unit", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			var calls int
			p := eidos.NewPlugin("planner").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "registry"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					calls++
					e.PlanFile().Append(&emit.Struct{
						Origin: coretest.Struct(coretest.StorePath, "Alpha").ID,
						Name:   "Registry",
					})
					return nil
				})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, calls, 1, "a graph rule runs once per phase call")

			var units []plugin.Unit
			for u := range ctx.Emit.Units() {
				if u.Plugin == "weaver" {
					units = append(units, u)
				}
			}
			assert.Length(t, units, 1, "one touched accumulator flushes one unit")
			assert.Equal(t, units[0].Per, plugin.PerPlan, "under its declared cardinality")
			assert.Equal(t, units[0].Key, "", "a plan unit has no key")
			assert.Length(t, units[0].Decls, 1, "holding what the handler appended")
			assert.Length(t, units[0].Origins, 0,
				"a graph match has no subject, so no per-subject provenance arrives")
		})

		t.Run("wraps a handler error with the plugin and rule", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			boom := errors.New("boom")
			p := eidos.NewPlugin("planner").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					return boom
				})).
				Build()

			err := generatorOf(t, p).Generate(genContext(t, g, facts, nil))
			assert.ErrorIs(t, err, boom, "a returned error is fatal to the phase")
			assert.Contains(t, err.Error(), "weaver", "naming the plugin")
			assert.Contains(t, err.Error(), "rule 0", "and the rule that failed")
		})

		t.Run("panics on an undeclared tag", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin("planner").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "registry"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					e.PlanFile("nope")
					return nil
				})).
				Build()

			assert.Panics(t, func() {
				_ = generatorOf(t, p).Generate(genContext(t, g, facts, nil))
			}, "addressing a family the plugin never declared is a defect")
		})
	})

	t.Run("OnEmit", func(t *testing.T) {
		t.Parallel()

		t.Run("visits earlier units and never its own", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))

			var visited int
			p := eidos.NewPlugin("weaver").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "audit"}).
				Handle(
					eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
						e.PlanFile().Append(&emit.Struct{
							Origin: alpha.ID, Name: "Own",
						})
						return nil
					}),
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							visited++
							return nil
						}),
				).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, visited, 1,
				"the emit rule sees the seeded unit and not the plugin's own flush")
		})

		t.Run("appends into another plugin's slot", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			other := emitted(alpha)
			seed(t, ctx, other)

			p := eidos.NewPlugin("weaver").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						s, ok := m.Value.(*emit.Struct)
						assert.True(t, ok, "a struct rule receives structs")
						s.Methods.Append(&emit.Method{
							Origin: m.Origin(), Name: "Audit",
						})
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, other.Methods.Len(), 1,
				"the slot is the composition seam between plugins")
		})

		t.Run("gates on the origin's fact", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"the classifier's fact stamps on one origin")
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha), emitted(beta))

			var origins []string
			p := eidos.NewPlugin("weaver").
				Handle(eidos.Where(eidos.HasKey(key),
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							origins = append(origins, m.Origin().Name)
							return nil
						}))).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, origins, []string{"Alpha"},
				"the predicate evaluates against the origin, and only its carrier matches")
		})

		t.Run("skip excludes by origin", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			validated := map[symbol.Identity][]directive.Directive{
				beta.ID: {{Name: directive.KernelSkip}},
			}
			ctx := genContext(t, g, facts, validated)
			seed(t, ctx, emitted(alpha), emitted(beta))

			var origins []string
			p := eidos.NewPlugin("weaver").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						origins = append(origins, m.Origin().Name)
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, origins, []string{"Alpha"},
				"a skipped origin drops out of a bare emit rule")
		})

		t.Run("reports at the origin's position", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))

			p := eidos.NewPlugin("weaver").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						assert.Equal(t, m.Origin(), alpha.ID,
							"the match names whose output it is looking at")
						m.Warnf(testCode, "flagged")
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			var found bool
			for d := range ctx.Sink.All() {
				found = true
				assert.Equal(t, d.Pos, alpha.Pos,
					"an emit match reports at its origin's position")
				assert.Equal(t, d.Origin, diag.Origin("weaver"),
					"under the reporting plugin's identity")
			}
			assert.True(t, found, "the report arrived")
		})

		t.Run("orders one accumulator's contributions by origin", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(beta), emitted(alpha))

			p := eidos.NewPlugin("weaver").
				Output(plugin.Output{Per: plugin.PerPackage, Word: "audit"}).
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						e.PackageFile().Append(&emit.Struct{
							Origin: m.Origin(), Name: "For" + m.Origin().Name,
						})
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			var got plugin.Unit
			var found bool
			for u := range ctx.Emit.Units() {
				if u.Plugin == "weaver" {
					got, found = u, true
				}
			}
			assert.True(t, found, "the accumulator flushed")
			assert.Equal(t, got.Key, coretest.StorePath,
				"a per-package unit keys on the origin's package path")

			var names []string
			for _, d := range got.Decls {
				s, ok := d.(*emit.Struct)
				assert.True(t, ok, "the unit holds what the handler appended")
				names = append(names, s.Name)
			}
			assert.Equal(t, names, []string{"ForAlpha", "ForBeta"},
				"contributions order by origin identity, never by match order")
			assert.Equal(t, got.Origins, []symbol.Identity{alpha.ID, beta.ID},
				"and the provenance is sorted")
		})
	})

	t.Run("node rules", func(t *testing.T) {
		t.Parallel()

		t.Run("a bare rule visits every declaration of its kind", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			var visited []string
			p := eidos.NewPlugin("weaver").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					visited = append(visited, m.Struct.Name)
					return nil
				})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.Equal(t, visited, []string{"Alpha", "Beta"},
				"the full-price path visits everything, in the graph's own order")
		})

		t.Run("a directive-gated rule runs per instance", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
				"the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(alpha.ID, []directive.Raw{
				{Name: "stub"}, {Name: "stub"},
			}), "two raw instances attach")
			g.Freeze()

			schema := stubSchema("stub")
			canonical := schema.Canonical()
			validated := map[symbol.Identity][]directive.Directive{
				alpha.ID: {
					{Name: canonical, Instance: 0},
					{Name: canonical, Instance: 1},
				},
			}
			_, facts := boolKey(t)

			var instances []int
			p := eidos.NewPlugin("stubgen").
				Handle(eidos.Directive(schema,
					eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						assert.NotNil(t, m.Directive(),
							"a gated match carries its instance")
						instances = append(instances, m.Directive().Instance)
						return nil
					}))).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, validated)),
				"the phase call passes")
			assert.Equal(t, instances, []int{0, 1},
				"a repeatable directive runs its handler once per instance, in source order")
		})

		t.Run("a fact-gated rule visits the stamped carriers", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"the fact stamps on one subject")

			var visited []string
			p := eidos.NewPlugin("weaver").
				Handle(eidos.Where(eidos.HasKey(key),
					eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						visited = append(visited, m.Struct.Name)
						return nil
					}))).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.Equal(t, visited, []string{"Alpha"},
				"the fact index routes the rule to its carriers alone")
		})

		t.Run("a narrowed skip excludes one plugin alone", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			validated := map[symbol.Identity][]directive.Directive{
				beta.ID: {{
					Name: directive.KernelSkip,
					Params: map[directive.ParamKey]directive.Value{
						directive.SkipPlugin: {
							Kind: directive.TypeString, Str: "weaver",
						},
					},
				}},
			}

			counting := func() (plugin.Plugin, *[]string) {
				var visited []string
				p := eidos.NewPlugin("weaver").
					Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						visited = append(visited, m.Struct.Name)
						return nil
					})).
					Build()
				return p, &visited
			}

			named, visited := counting()
			ctx := genContext(t, g, facts, validated)
			assert.NoError(t, generatorOf(t, named).Generate(ctx), "the named plugin runs")
			assert.Equal(t, *visited, []string{"Alpha"},
				"the named plugin loses the skipped subject")

			other, otherVisited := counting()
			otherCtx := genContext(t, g, facts, validated)
			otherCtx.Plugin = "audit"
			assert.NoError(t, generatorOf(t, other).Generate(otherCtx),
				"another plugin runs")
			assert.Equal(t, *otherVisited, []string{"Alpha", "Beta"},
				"every other plugin still matches the subject")
		})

		t.Run("a dual-role plugin runs each phase's rules alone", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")

			var stamped, emitted int
			p := eidos.NewPlugin("dual").
				Handle(
					eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
						stamped++
						eidos.Stamp(st, key, true)
						return nil
					}),
					eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						emitted++
						return nil
					}),
				).
				Build()

			assert.NoError(t, annotatorOf(t, p).Annotate(annContext(t, facts, ix)),
				"the annotate call passes")
			assert.Equal(t, stamped, 2, "the annotate call ran the stamper rule")
			assert.Equal(t, emitted, 0, "and never the emitter rule")

			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the generate call passes")
			assert.Equal(t, emitted, 2, "the generate call ran the emitter rule")
			assert.Equal(t, stamped, 2, "and never the stamper rule again")
		})
	})

	t.Run("OnGraph", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at the position it names", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("planner").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					m.Errorf(testCode, alpha.Pos, "graph-wide finding")
					return nil
				})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.True(t, ctx.Sink.Failed(), "an Error fails the run")
			for d := range ctx.Sink.All() {
				assert.Equal(t, d.Pos, alpha.Pos,
					"a graph match demands the position it reports at")
			}
		})

		t.Run("reads the graph through the match", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("planner").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					_, held := m.Reader().Lookup(alpha.ID)
					assert.True(t, held, "the reader returns the held declaration")
					return nil
				})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
		})
	})
}

// benchEmitStore seeds units of origined structs whose origins the
// graph holds, so the dispatch path performs its position lookups.
func benchEmitStore(tb assert.TB, units, perUnit int) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	for u := range units {
		path := coretest.StorePath + "/" + strconv.Itoa(u)
		decls := make([]symbol.Symbol, 0, perUnit)
		for i := range perUnit {
			origin := coretest.Struct(path, "Decl0_"+strconv.Itoa(i)).ID
			decls = append(decls, &emit.Struct{
				Origin: origin, Name: "Gen" + strconv.Itoa(i),
			})
		}
		err := e.Add(plugin.Unit{
			Plugin: "earlier", Per: plugin.PerSource, Word: "impl",
			Key: "unit" + strconv.Itoa(u) + ".go", Decls: decls,
		})
		if err != nil {
			tb.Fatalf("Add: unexpected error: %v", err)
		}
	}
	return e
}

func BenchmarkDispatch(b *testing.B) {
	const packages, files, decls = 1_000, 10, 20
	g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)

	b.Run("emit rule over 20k values", func(b *testing.B) {
		b.ReportAllocs()
		_, facts := boolKey(b)
		const units, perUnit = 1_000, 20
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		ctx := &plugin.GeneratorContext{
			Index: ix, Facts: facts, Emit: benchEmitStore(b, units, perUnit),
			Sink: diag.NewSink(), Plugin: "bench", Bucket: 1,
		}
		var visited int
		p := eidos.NewPlugin("bench").
			Handle(eidos.OnEmit(symbol.KindStruct,
				func(m *eidos.EmitMatch, e *eidos.Emitter) error {
					visited++
					return nil
				})).
			Build()
		gen, ok := p.(plugin.Generator)
		if !ok {
			b.Fatal("the bench plugin must generate")
		}
		for b.Loop() {
			visited = 0
			if err := gen.Generate(ctx); err != nil {
				b.Fatalf("Generate: unexpected error: %v", err)
			}
			if visited != units*perUnit {
				b.Fatalf("visited %d values", visited)
			}
		}
	})

	b.Run("fact-gated emit rule, one carrier in ten", func(b *testing.B) {
		b.ReportAllocs()
		key, facts := boolKey(b)
		const units, perUnit = 1_000, 20
		for u := 0; u < units; u += 10 {
			path := coretest.StorePath + "/" + strconv.Itoa(u)
			origin := coretest.Struct(path, "Decl0_0").ID
			if err := meta.Stamp(facts, key, true, meta.Claim{Subject: origin}); err != nil {
				b.Fatalf("Stamp: unexpected error: %v", err)
			}
		}
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		ctx := &plugin.GeneratorContext{
			Index: ix, Facts: facts, Emit: benchEmitStore(b, units, perUnit),
			Sink: diag.NewSink(), Plugin: "bench", Bucket: 1,
		}
		var visited int
		p := eidos.NewPlugin("bench").
			Handle(eidos.Where(eidos.HasKey(key),
				eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						visited++
						return nil
					}))).
			Build()
		gen, ok := p.(plugin.Generator)
		if !ok {
			b.Fatal("the bench plugin must generate")
		}
		for b.Loop() {
			visited = 0
			if err := gen.Generate(ctx); err != nil {
				b.Fatalf("Generate: unexpected error: %v", err)
			}
			if visited != units/10 {
				b.Fatalf("visited %d values", visited)
			}
		}
	})

	b.Run("flush of 10k placed declarations", func(b *testing.B) {
		b.ReportAllocs()
		_, facts := boolKey(b)
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		const placed = 10_000
		payload := make([]symbol.Symbol, placed)
		for i := range payload {
			payload[i] = &emit.Struct{
				Origin: coretest.Struct(coretest.StorePath+"/0", "Decl0_0").ID,
				Name:   "Gen" + strconv.Itoa(i),
			}
		}
		p := eidos.NewPlugin("bench").
			Output(plugin.Output{Per: plugin.PerPlan, Word: "registry"}).
			Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
				e.PlanFile().Append(payload...)
				return nil
			})).
			Build()
		gen, ok := p.(plugin.Generator)
		if !ok {
			b.Fatal("the bench plugin must generate")
		}
		for b.Loop() {
			ctx := &plugin.GeneratorContext{
				Index: ix, Facts: facts, Emit: plugin.NewEmit(),
				Sink: diag.NewSink(), Plugin: "bench", Bucket: 1,
			}
			if err := gen.Generate(ctx); err != nil {
				b.Fatalf("Generate: unexpected error: %v", err)
			}
		}
	})
}

func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()
	key, _ := boolKey(b)
	for b.Loop() {
		p := eidos.NewPlugin("bench").
			Output(plugin.Output{Per: plugin.PerPlan, Word: "registry"}).
			Handle(
				eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					return nil
				}),
				eidos.Where(eidos.HasKey(key),
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							return nil
						}),
					eidos.OnEmit(symbol.KindMethod,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							return nil
						}),
				),
			).
			Build()
		if p.Name() != "bench" {
			b.Fatal("the declaration must build")
		}
	}
}

func BenchmarkNodeDispatch(b *testing.B) {
	const packages, files, decls = 1_000, 10, 20
	g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)

	b.Run("bare rule over 200k structs", func(b *testing.B) {
		b.ReportAllocs()
		_, facts := boolKey(b)
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		ctx := &plugin.GeneratorContext{
			Index: ix, Facts: facts, Emit: plugin.NewEmit(),
			Sink: diag.NewSink(), Plugin: "bench", Bucket: 1,
		}
		var visited int
		p := eidos.NewPlugin("bench").
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
				visited++
				return nil
			})).
			Build()
		gen, ok := p.(plugin.Generator)
		if !ok {
			b.Fatal("the bench plugin must generate")
		}
		for b.Loop() {
			visited = 0
			if err := gen.Generate(ctx); err != nil {
				b.Fatalf("Generate: unexpected error: %v", err)
			}
			if visited != packages*files*decls {
				b.Fatalf("visited %d subjects", visited)
			}
		}
	})

	b.Run("annotate stamps 200k subjects", func(b *testing.B) {
		b.ReportAllocs()
		reg := meta.NewRegistry()
		if err := reg.ClaimNamespace("t", "the bench"); err != nil {
			b.Fatalf("ClaimNamespace: unexpected error: %v", err)
		}
		benchKey, err := meta.Register[bool](reg, meta.KeySpec{
			Name: "t.flag", Doc: "marks a bench subject",
		})
		if err != nil {
			b.Fatalf("Register: unexpected error: %v", err)
		}
		p := eidos.NewPlugin("bench").
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
				eidos.Stamp(st, benchKey, true)
				return nil
			})).
			Build()
		ann, ok := p.(plugin.Annotator)
		if !ok {
			b.Fatal("the bench plugin must annotate")
		}
		for b.Loop() {
			facts := meta.NewFacts(reg)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			if err != nil {
				b.Fatalf("NewIndex: unexpected error: %v", err)
			}
			ctx := &plugin.AnnotatorContext{
				Index: ix, Facts: facts, Sink: diag.NewSink(),
				Plugin: "bench", Bucket: 1,
			}
			if err := ann.Annotate(ctx); err != nil {
				b.Fatalf("Annotate: unexpected error: %v", err)
			}
			if ctx.Sink.Failed() {
				b.Fatal("no stamp may be refused")
			}
		}
	})
}
