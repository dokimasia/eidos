// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

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

// testCode is a code for handler-side reporting cases. The sink
// accepts a code without consulting a registry.
var testCode = diag.Code{Prefix: "tst", Number: 1}

// The two failures the parallel error cases script, at two subjects
// of the many-subject fixture.
var (
	errFirst = errors.New("eidos_test: the earlier failure")
	errLast  = errors.New("eidos_test: the later failure")
)

// The subjects the parallel error cases fail at, by index into the
// many-subject fixture.
const (
	firstFailure = 10
	lastFailure  = 40
)

// failureWait bounds how long the earlier failure waits for the later
// one, so a dispatcher that never runs the later invocation fails the
// case instead of hanging it.
const failureWait = 5 * time.Second

// boolKey returns a registered bool key and a fact store built over
// its registry.
func boolKey(tb assert.TB) (meta.Key[bool], *meta.Facts) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
	key, err := meta.Register[bool](reg, meta.KeySpec{
		Name: "t.flag", Doc: "marks a fixture subject",
	})
	assert.NoError(tb, err, "the key registers")
	return key, meta.NewFacts(reg)
}

// gateName is the directive the gated dispatch cases attach and
// gate on.
const gateName directive.Name = "stub"

// The plugin the generator context runs as, and another plugin the
// opt-out cases compare it with.
const (
	contextPlugin plugin.ID = "weaver"
	otherPlugin   plugin.ID = "audit"
)

// emitHandler is the handler shape every struct-triggered emitter
// rule takes, so a case can vary the rule and keep the handler.
type emitHandler = func(m *eidos.StructMatch, e *eidos.Emitter) error

// fixtureGraph returns a frozen graph that contains two positioned
// structs in the store package.
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
		Plugin: contextPlugin,
		Bucket: 2,
	}
}

// seed adds one earlier-bucket unit with decls to ctx.Emit.
func seed(tb assert.TB, ctx *plugin.GeneratorContext, decls ...symbol.Symbol) {
	tb.Helper()

	assert.NoError(tb, ctx.Emit.Add(plugin.Unit{
		Plugin: "earlier", Per: plugin.PerSource, Word: "impl",
		Key: "a.go", Decls: decls,
	}), "the earlier bucket's unit is added")
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

// visitingStructs returns a plugin whose bare struct rule records
// each subject it visits, and the record.
func visitingStructs(name plugin.ID, rule func(emitHandler) eidos.Rule) (plugin.Plugin, *[]string) {
	var visited []string
	p := eidos.NewPlugin(name).
		Handle(rule(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			visited = append(visited, m.Struct.Name)
			return nil
		})).
		Build()
	return p, &visited
}

// negatedOn returns the validated table with one negated instance of
// the context plugin's stub directive on subject.
func negatedOn(subject symbol.Identity) map[symbol.Identity][]directive.Directive {
	return map[symbol.Identity][]directive.Directive{
		subject: {{Name: directive.Name(string(contextPlugin) + ":stub"), Negated: true}},
	}
}

// Dispatch routes every rule: indexed enumeration, skip, gate views,
// per-invocation grain, deterministic flush. Every case here is a
// guarantee a plugin author gets to assume.
func TestDispatch(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		graphRun := func(tb assert.TB) (int, []plugin.Unit) {
			tb.Helper()

			g, _, _ := fixtureGraph(tb)
			_, facts := boolKey(tb)
			ctx := genContext(tb, g, facts, nil)
			var calls int
			p := eidos.NewPlugin(contextPlugin).
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
			assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
			var units []plugin.Unit
			for u := range ctx.Emit.Units() {
				if u.Plugin == contextPlugin {
					units = append(units, u)
				}
			}
			return calls, units
		}

		t.Run("runs a graph rule once", func(t *testing.T) {
			t.Parallel()

			calls, _ := graphRun(t)
			assert.Equal(t, calls, 1, "the handler runs once per phase call")
		})

		t.Run("flushes a graph rule's accumulator as one unit", func(t *testing.T) {
			t.Parallel()

			_, units := graphRun(t)
			assert.Length(t, units, 1, "the touched accumulator flushes one unit")
			assert.Equal(t, units[0].Per, plugin.PerPlan, "the unit has its declared cardinality")
			assert.Equal(t, units[0].Key, "", "a plan unit has no key")
			assert.Length(t, units[0].Decls, 1, "the unit contains what the handler appended")
			assert.Length(t, units[0].Origins, 0, "a graph match has no subject to record")
		})

		t.Run("returns a handler error wrapped with the plugin and the rule", func(t *testing.T) {
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
			assert.ErrorIs(t, err, boom, "the handler's error stops the phase")
			assert.Contains(t, err.Error(), string(contextPlugin), "the error names the plugin")
			assert.Contains(t, err.Error(), "rule 0", "the error names the rule")
		})

		// registry runs a plugin that places one struct per subject into
		// its package's accumulator, on the given workers, and returns
		// the plugin's units.
		registry := func(tb assert.TB, workers int) []plugin.Unit {
			tb.Helper()

			g, _ := manySubjects(tb, manyStructs)
			_, facts := boolKey(tb)
			ctx := on(genContext(tb, g, facts, nil), workers)
			p := eidos.NewPlugin(contextPlugin).
				Output(plugin.Output{Per: plugin.PerPackage, Word: "registry"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.PackageFile().Append(&emit.Struct{Origin: m.Struct.ID, Name: "For" + m.Struct.Name})
					return nil
				})).
				Build()
			assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
			return slices.Collect(ctx.Emit.Units())
		}

		t.Run("assembles a package's accumulator in subject order on one worker", func(t *testing.T) {
			t.Parallel()

			units := registry(t, oneWorker)
			assert.Length(t, units, 1, "the package's matches assemble one unit")
			assert.Length(t, units[0].Decls, manyStructs, "every match placed its struct")
			assert.True(t, slices.IsSortedFunc(units[0].Origins, symbol.Identity.Compare),
				"the declarations follow subject identity")
		})

		t.Run("assembles on eight workers the units one worker assembles", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, registry(t, eightWorkers), registry(t, oneWorker),
				"the units do not depend on the worker count")
		})

		// failAt runs a plugin that warns once per subject and fails at
		// two of them, on the given workers, and returns the findings and
		// the error. On more than one worker the earlier failure waits
		// until the later one has failed, so both failures and the
		// invocations between them have run when the rule stops.
		failAt := func(tb assert.TB, workers int) ([]string, error) {
			tb.Helper()

			g, structs := manySubjects(tb, manyStructs)
			_, facts := boolKey(tb)
			ctx := on(genContext(tb, g, facts, nil), workers)
			later := make(chan struct{})
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					m.Warnf(testCode, "%s", m.Struct.Name)
					switch m.Struct.ID {
					case structs[firstFailure].ID:
						if workers > 1 {
							select {
							case <-later:
							case <-time.After(failureWait):
							}
						}
						return errFirst
					case structs[lastFailure].ID:
						close(later)
						return errLast
					}
					return nil
				})).
				Build()
			err := generatorOf(tb, p).Generate(ctx)
			return messages(ctx.Sink), err
		}

		t.Run("returns the first handler error by sequence on eight workers", func(t *testing.T) {
			t.Parallel()

			_, err := failAt(t, eightWorkers)
			assert.ErrorIs(t, err, errFirst, "the failure earliest in match order stops the phase")
		})

		t.Run("reports the findings up to a failed invocation on one worker", func(t *testing.T) {
			t.Parallel()

			got, _ := failAt(t, oneWorker)
			assert.Length(t, got, firstFailure+1, "the invocations up to the failure reported")
		})

		t.Run("reports the findings of one worker before a failed invocation on eight", func(t *testing.T) {
			t.Parallel()

			parallel, _ := failAt(t, eightWorkers)
			serial, _ := failAt(t, oneWorker)
			assert.Equal(t, parallel, serial, "the findings do not depend on the worker count")
		})

		t.Run("panics with a handler's panic on eight workers", func(t *testing.T) {
			t.Parallel()

			g, structs := manySubjects(t, manyStructs)
			_, facts := boolKey(t)
			ctx := on(genContext(t, g, facts, nil), eightWorkers)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					if m.Struct.ID == structs[firstFailure].ID {
						panic(errFirst)
					}
					return nil
				})).
				Build()
			got := assert.Panics(t, func() { _ = generatorOf(t, p).Generate(ctx) },
				"the panic propagates to the caller")
			assert.Equal(t, got, any(errFirst), "with the handler's own value")
		})

		// Each indexed path resolves its subjects differently, so each
		// has its own error return. A path that swallowed one would
		// finish the phase over a handler that failed.
		schema := stubSchema(gateName)
		paths := []struct {
			name string
			rule func(handler emitHandler) eidos.Rule
		}{
			{
				name: "returns a handler error on the bare path",
				rule: eidos.OnStruct[*eidos.Emitter],
			},
			{
				name: "returns a handler error on the fact-gated path",
				rule: func(h emitHandler) eidos.Rule {
					key, _ := boolKey(t)
					return eidos.Where(eidos.HasKey(key), eidos.OnStruct(h))
				},
			},
			{
				name: "returns a handler error on the directive-gated path",
				rule: func(h emitHandler) eidos.Rule {
					return eidos.Directive(schema, eidos.OnStruct(h))
				},
			},
		}
		for _, tt := range paths {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				alpha := coretest.Struct(coretest.StorePath, "Alpha")
				g := store.New()
				assert.NoError(t,
					g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
					"the fixture package is admitted")
				assert.NoError(t, g.AttachDirectives(alpha.ID,
					[]directive.Raw{{Name: gateName}}),
					"the gating instance attaches")
				g.Freeze()

				key, facts := boolKey(t)
				assert.NoError(t,
					meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
					"the fact gate has a stamped subject to find")
				validated := map[symbol.Identity][]directive.Directive{
					alpha.ID: {{Name: schema.Canonical(), Instance: 0}},
				}

				boom := errors.New("boom")
				p := eidos.NewPlugin("stubgen").
					Handle(tt.rule(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						return boom
					})).
					Build()

				err := generatorOf(t, p).Generate(genContext(t, g, facts, validated))
				assert.ErrorIs(t, err, boom, "the handler's error stops the phase")
			})
		}

		t.Run("skips a stamped subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			ghost := coretest.ID(coretest.StorePath, "Ghost", symbol.KindStruct)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: ghost}),
				"a fact stamps on a subject no package declares")
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"a fact stamps on a subject the graph contains")

			p, visited := visitingStructs("gated", func(h emitHandler) eidos.Rule {
				return eidos.Where(eidos.HasKey(key), eidos.OnStruct(h))
			})
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha"}, "the unresolved subject is not visited")
		})

		t.Run("skips a directive on a declaration of another kind", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			reader := coretest.Interface(coretest.StorePath, coretest.InterfaceName)
			g := store.New()
			assert.NoError(t,
				g.AddPackage(coretest.Package(coretest.StorePath, alpha, reader)),
				"the fixture package is admitted")
			for _, id := range []symbol.Identity{alpha.ID, reader.ID} {
				assert.NoError(t, g.AttachDirectives(id,
					[]directive.Raw{{Name: gateName}}),
					"the directive attaches to both kinds")
			}
			g.Freeze()

			canonical := schema.Canonical()
			validated := map[symbol.Identity][]directive.Directive{
				alpha.ID:  {{Name: canonical, Instance: 0}},
				reader.ID: {{Name: canonical, Instance: 0}},
			}
			_, facts := boolKey(t)

			p, visited := visitingStructs("stubgen", func(h emitHandler) eidos.Rule {
				return eidos.Directive(schema, eidos.OnStruct(h))
			})
			assert.NoError(t,
				generatorOf(t, p).Generate(genContext(t, g, facts, validated)),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha"}, "the struct rule does not visit the interface")
		})

		t.Run("runs a gated emit rule once per instance on the origin", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			g := store.New()
			assert.NoError(t,
				g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
				"the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(alpha.ID, []directive.Raw{
				{Name: gateName}, {Name: gateName},
			}), "two gating instances attach to the origin")
			g.Freeze()

			canonical := schema.Canonical()
			validated := map[symbol.Identity][]directive.Directive{
				alpha.ID: {
					{Name: canonical, Instance: 0},
					{Name: canonical, Instance: 1},
				},
			}
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, validated)
			seed(t, ctx, emitted(alpha))

			instances := []int{}
			p := eidos.NewPlugin("stubgen").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "audit"}).
				Handle(eidos.Directive(schema,
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							gate := m.Directive()
							assert.NotNil(t, gate, "a gated emit match has its instance")
							if gate != nil {
								instances = append(instances, gate.Instance)
							}
							return nil
						}))).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, instances, []int{0, 1}, "the instances run in source order")
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

		ownRun := func(tb assert.TB) int {
			tb.Helper()

			g, alpha, _ := fixtureGraph(tb)
			_, facts := boolKey(tb)
			ctx := genContext(tb, g, facts, nil)
			seed(tb, ctx, emitted(alpha))

			var visited int
			p := eidos.NewPlugin(contextPlugin).
				Output(plugin.Output{Per: plugin.PerPlan, Word: "audit"}).
				Handle(
					eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
						e.PlanFile().Append(&emit.Struct{Origin: alpha.ID, Name: "Own"})
						return nil
					}),
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							visited++
							return nil
						}),
				).
				Build()
			assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
			return visited
		}

		t.Run("visits an earlier bucket's unit", func(t *testing.T) {
			t.Parallel()

			assert.True(t, ownRun(t) >= 1, "the seeded unit is visited")
		})

		t.Run("never visits the plugin's own unit", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ownRun(t), 1, "only the seeded unit is visited")
		})

		t.Run("appends into another plugin's slot", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			other := emitted(alpha)
			seed(t, ctx, other)

			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						s, ok := m.Value.(*emit.Struct)
						assert.True(t, ok, "a struct rule receives structs")
						e.Slot(&s.Methods).Append(&emit.Method{
							Origin: m.Origin(), Name: "Audit",
						})
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, other.Methods.Len(), 1, "the other plugin's slot has the appended method")
		})

		t.Run("evaluates a fact gate on the origin", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"the classifier's fact stamps on one origin")
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha), emitted(beta))

			var origins []string
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.Where(eidos.HasKey(key),
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							origins = append(origins, m.Origin().Name)
							return nil
						}))).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, origins, []string{"Alpha"}, "only the stamped origin's value is visited")
		})

		t.Run("skips an origin under a bare skip", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			validated := map[symbol.Identity][]directive.Directive{
				beta.ID: {{Name: directive.KernelSkip}},
			}
			ctx := genContext(t, g, facts, validated)
			seed(t, ctx, emitted(alpha), emitted(beta))

			var origins []string
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						origins = append(origins, m.Origin().Name)
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, origins, []string{"Alpha"}, "the skipped origin's value is not visited")
		})

		t.Run("skips an origin with a negated directive of the plugin", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, negatedOn(beta.ID))
			seed(t, ctx, emitted(alpha), emitted(beta))

			var origins []string
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						origins = append(origins, m.Origin().Name)
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, origins, []string{"Alpha"}, "the negated origin's value is not visited")
		})

		t.Run("reports at the origin's position", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))

			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						assert.Equal(t, m.Origin(), alpha.ID, "the match names the value's origin")
						m.Warnf(testCode, "flagged")
						return nil
					})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			var found bool
			for d := range ctx.Sink.All() {
				found = true
				assert.Equal(t, d.Pos, alpha.Pos, "the finding is at the origin's position")
				assert.Equal(t, d.Origin, contextPlugin, "the finding is under the reporting plugin")
			}
			assert.True(t, found, "the finding is reported")
		})

		t.Run("orders one accumulator's contributions by origin", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(beta), emitted(alpha))

			p := eidos.NewPlugin(contextPlugin).
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
				if u.Plugin == contextPlugin {
					got, found = u, true
				}
			}
			assert.True(t, found, "the accumulator flushed")
			assert.Equal(t, got.Key, coretest.StorePath, "a per-package unit keys on the origin's package path")

			var names []string
			for _, d := range got.Decls {
				s, ok := d.(*emit.Struct)
				assert.True(t, ok, "the unit contains what the handler appended")
				names = append(names, s.Name)
			}
			assert.Equal(t, names, []string{"ForAlpha", "ForBeta"},
				"the contributions are in origin identity order, not match order")
			assert.Equal(t, got.Origins, []symbol.Identity{alpha.ID, beta.ID}, "the provenance is sorted")
		})
	})

	t.Run("OnStruct", func(t *testing.T) {
		t.Parallel()

		bare := eidos.OnStruct[*eidos.Emitter]

		t.Run("visits every declaration of its kind under a bare rule", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p, visited := visitingStructs(contextPlugin, bare)
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha", "Beta"}, "the declarations are in the graph's order")
		})

		t.Run("runs a directive-gated rule once per instance", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
				"the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(alpha.ID, []directive.Raw{
				{Name: gateName}, {Name: gateName},
			}), "two raw instances attach")
			g.Freeze()

			schema := stubSchema(gateName)
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
						assert.NotNil(t, m.Directive(), "a gated match has its instance")
						instances = append(instances, m.Directive().Instance)
						return nil
					}))).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, validated)),
				"the phase call passes")
			assert.Equal(t, instances, []int{0, 1}, "the instances run in source order")
		})

		t.Run("runs a directive-gated rule for its own directive alone", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
				"the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(alpha.ID, []directive.Raw{{Name: gateName}}),
				"the gating instance attaches")
			g.Freeze()

			schema := stubSchema(gateName)
			validated := map[symbol.Identity][]directive.Directive{
				alpha.ID: {
					{Name: directive.KernelSample, Instance: 0},
					{Name: schema.Canonical(), Instance: 0},
					{Name: directive.KernelDiag, Instance: 0},
				},
			}
			_, facts := boolKey(t)

			var gates []directive.Name
			p := eidos.NewPlugin("stubgen").
				Handle(eidos.Directive(schema,
					eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						gates = append(gates, m.Directive().Name)
						return nil
					}))).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, validated)),
				"the phase call passes")
			assert.Equal(t, gates, []directive.Name{schema.Canonical()}, "only the rule's own instance runs")
		})

		t.Run("runs no directive-gated rule for a negated instance", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
				"the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(alpha.ID, []directive.Raw{{Name: gateName, Negated: true}}),
				"the negated instance attaches")
			g.Freeze()

			schema := stubSchema(gateName)
			validated := map[symbol.Identity][]directive.Directive{
				alpha.ID: {{Name: schema.Canonical(), Negated: true}},
			}
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, validated)
			ctx.Plugin = plugin.ID(schema.Plugin)

			p, visited := visitingStructs(ctx.Plugin, func(h emitHandler) eidos.Rule {
				return eidos.Directive(schema, eidos.OnStruct(h))
			})
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Empty(t, *visited, "the negated instance gates nothing")
		})

		t.Run("runs a directive-gated rule on a subject that negates another directive", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
				"the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(alpha.ID, []directive.Raw{{Name: gateName}}),
				"the gating instance attaches")
			g.Freeze()

			schema := stubSchema(gateName)
			validated := map[symbol.Identity][]directive.Directive{
				alpha.ID: {
					{Name: schema.Canonical(), Instance: 0},
					{Name: directive.Name(schema.Plugin + ":other"), Negated: true},
				},
			}
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, validated)
			ctx.Plugin = plugin.ID(schema.Plugin)

			p, visited := visitingStructs(ctx.Plugin, func(h emitHandler) eidos.Rule {
				return eidos.Directive(schema, eidos.OnStruct(h))
			})
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha"}, "the subject opted in through the gate")
		})

		t.Run("visits only the stamped subjects under a fact-gated rule", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"the fact stamps on one subject")

			p, visited := visitingStructs(contextPlugin, func(h emitHandler) eidos.Rule {
				return eidos.Where(eidos.HasKey(key), eidos.OnStruct(h))
			})
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha"}, "only the stamped subject is visited")
		})

		narrowed := func(beta symbol.Identity) map[symbol.Identity][]directive.Directive {
			return map[symbol.Identity][]directive.Directive{
				beta: {{
					Name: directive.KernelSkip,
					Params: map[directive.ParamKey]directive.Value{
						directive.SkipPlugin: {Kind: directive.TypeString, Str: string(contextPlugin)},
					},
				}},
			}
		}

		t.Run("skips a subject under a skip naming the plugin", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			p, visited := visitingStructs(contextPlugin, bare)
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, narrowed(beta.ID))),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha"}, "the skipped subject is not visited")
		})

		t.Run("visits a subject under a skip naming another plugin", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, narrowed(beta.ID))
			ctx.Plugin = otherPlugin
			p, visited := visitingStructs(otherPlugin, bare)
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha", "Beta"}, "both subjects are visited")
		})

		t.Run("skips a subject with a negated directive of the plugin under a bare rule", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			p, visited := visitingStructs(contextPlugin, bare)
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, negatedOn(beta.ID))),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha"}, "the negated subject is not visited")
		})

		t.Run("skips a subject with a negated directive of the plugin under a fact-gated rule", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			for _, id := range []symbol.Identity{alpha.ID, beta.ID} {
				assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: id}),
					"the fact stamps on both subjects")
			}
			p, visited := visitingStructs(contextPlugin, func(h emitHandler) eidos.Rule {
				return eidos.Where(eidos.HasKey(key), eidos.OnStruct(h))
			})
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, negatedOn(beta.ID))),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha"}, "the negated subject is not visited")
		})

		t.Run("visits a subject with another plugin's negated directive", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, negatedOn(beta.ID))
			ctx.Plugin = otherPlugin
			p, visited := visitingStructs(otherPlugin, bare)
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha", "Beta"}, "both subjects are visited")
		})

		dualRun := func(tb assert.TB) (stamped, emittedCount [2]int) {
			tb.Helper()

			g, _, _ := fixtureGraph(tb)
			key, facts := boolKey(tb)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(tb, err, "the routing surface builds")

			var stamps, emits int
			p := eidos.NewPlugin("dual").
				Handle(
					eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
						stamps++
						eidos.Stamp(st, key, true)
						return nil
					}),
					eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						emits++
						return nil
					}),
				).
				Build()

			assert.NoError(tb, annotatorOf(tb, p).Annotate(annContext(tb, facts, ix)), "the annotate call passes")
			stamped[0], emittedCount[0] = stamps, emits
			assert.NoError(tb, generatorOf(tb, p).Generate(genContext(tb, g, facts, nil)), "the generate call passes")
			stamped[1], emittedCount[1] = stamps, emits
			return stamped, emittedCount
		}

		t.Run("runs only the stamper rules of a dual-role plugin in the annotate phase", func(t *testing.T) {
			t.Parallel()

			stamped, emittedCount := dualRun(t)
			assert.Equal(t, stamped[0], 2, "the stamper rule visits both subjects")
			assert.Equal(t, emittedCount[0], 0, "the emitter rule does not run")
		})

		t.Run("runs only the emitter rules of a dual-role plugin in the generate phase", func(t *testing.T) {
			t.Parallel()

			stamped, emittedCount := dualRun(t)
			assert.Equal(t, emittedCount[1], 2, "the emitter rule visits both subjects")
			assert.Equal(t, stamped[1], 2, "the stamper rule does not run again")
		})
	})

	t.Run("OnGraph", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at the position the handler names", func(t *testing.T) {
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
			assert.True(t, ctx.Sink.Failed(), "the Error fails the run")
			for d := range ctx.Sink.All() {
				assert.Equal(t, d.Pos, alpha.Pos, "the finding is at the named position")
			}
		})

		t.Run("returns a reader over the graph", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("planner").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					_, held := m.Reader().Lookup(alpha.ID)
					assert.True(t, held, "the reader finds the declaration")
					return nil
				})).
				Build()

			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
		})
	})
}

// benchEmitStore seeds units of origined structs whose origins the
// graph contains, so the dispatch path performs its position lookups.
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

// gatedWorkspace returns a frozen workspace whose every struct has
// one raw instance of the gate directive, and the validated table
// naming it by its canonical spelling.
func gatedWorkspace(
	tb assert.TB, canonical directive.Name, packages, files, decls int,
) (*store.Graph, map[symbol.Identity][]directive.Directive) {
	tb.Helper()

	g := store.New()
	validated := map[symbol.Identity][]directive.Directive{}
	for _, pkg := range coretest.Workspace(packages, files, decls) {
		assert.NoError(tb, g.AddPackage(pkg), "the bench package loads")
		for decl := range node.Declarations(pkg) {
			if decl.Kind() != symbol.KindStruct {
				continue
			}
			assert.NoError(tb, g.AttachDirectives(decl.Identity(), []directive.Raw{{Name: gateName}}),
				"the directive attaches")
			validated[decl.Identity()] = []directive.Directive{{Name: canonical}}
		}
	}
	g.Freeze()
	return g, validated
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

	b.Run("fact-gated emit rule, one origin in ten", func(b *testing.B) {
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

	b.Run("directive-gated rule over 20k subjects", func(b *testing.B) {
		b.ReportAllocs()
		const gatedSubjects = 1_000 * 20
		schema := stubSchema(gateName)
		gated, validated := gatedWorkspace(b, schema.Canonical(), 1_000, 1, 20)
		_, facts := boolKey(b)
		ix, err := plugin.NewIndex(gated, facts, validated, nil)
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		ctx := &plugin.GeneratorContext{
			Index: ix, Facts: facts, Emit: plugin.NewEmit(),
			Sink: diag.NewSink(), Plugin: "stubgen", Bucket: 1,
		}
		var visited int
		p := eidos.NewPlugin("stubgen").
			Handle(eidos.Directive(schema,
				eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
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
			if visited != gatedSubjects {
				b.Fatalf("visited %d subjects", visited)
			}
		}
	})

	b.Run("annotate stamps three facts after three reads per subject", func(b *testing.B) {
		b.ReportAllocs()
		reg := meta.NewRegistry()
		if err := reg.ClaimNamespace(fixtureNamespace); err != nil {
			b.Fatalf("ClaimNamespace: unexpected error: %v", err)
		}
		var keys []meta.Key[bool]
		for _, name := range []meta.KeyName{"t.first", "t.second", "t.third"} {
			key, err := meta.Register[bool](reg, meta.KeySpec{Name: name, Doc: "marks a bench subject"})
			if err != nil {
				b.Fatalf("Register: unexpected error: %v", err)
			}
			keys = append(keys, key)
		}
		p := eidos.NewPlugin("bench").
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
				for _, key := range keys {
					eidos.Fact(m, key)
				}
				for _, key := range keys {
					eidos.Stamp(st, key, true)
				}
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

	b.Run("annotate stamps 200k subjects", func(b *testing.B) {
		b.ReportAllocs()
		reg := meta.NewRegistry()
		if err := reg.ClaimNamespace(fixtureNamespace); err != nil {
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
