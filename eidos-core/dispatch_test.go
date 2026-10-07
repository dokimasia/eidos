// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
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

// reuses is how many rounds a reuse case alternates its two phase calls:
// enough that the later call takes the state the earlier one released in
// some round, whatever the parallel cases beside it take from the pool.
const reuses = 8

// gateName is the directive the gated dispatch cases attach and
// gate on.
const gateName directive.Name = "stub"

// The plugin the generator context runs as, and another plugin the
// opt-out cases compare it with.
const (
	contextPlugin plugin.ID = "weaver"
	otherPlugin   plugin.ID = "audit"
)

// invocationStructs is how many structs the invocation fixture
// declares. They are in one file of one package, so a phase call over
// them touches one accumulator per family, however many invocations
// touch it.
const invocationStructs = 100

// emitHandler is the signature of a struct-triggered emitter rule's
// handler, so a case can vary the rule and keep the handler.
type emitHandler = func(m *eidos.StructMatch, e *eidos.Emitter) error

// phaseCase is one generate phase call of a plugin over the invocation
// fixture, named as its benchmark is, and what the call allocates.
// fresh returns a context of the call's own, so a call keeps its units
// and findings apart from the calls before it, and check reads what the
// call left in its context.
type phaseCase struct {
	name   string
	allocs uint64
	gen    plugin.Generator
	fresh  func() *plugin.GeneratorContext
	check  func(tb assert.TB, ctx *plugin.GeneratorContext)
}

// The plugins the dispatch benchmarks run: one whose rules visit
// without a gate or behind a fact gate, and one whose rule a directive
// gates.
const (
	benchPlugin plugin.ID = "bench"
	gatePlugin  plugin.ID = "stubgen"
)

// The workspace TestDispatchAllocs routes over: allocPackages packages
// of allocFiles files, each file declaring allocDecls structs.
const (
	allocPackages = 10
	allocFiles    = 2
	allocDecls    = 4
)

// flushPlaced is how many declarations the flush case's graph rule
// places into its plan file.
const flushPlaced = 10_000

// flushAllocs is the ceiling of a call that flushes one unit into an
// empty store. The flush allocates the unit's declarations, and a graph
// rule's placements name no origin, so the unit lists none. The store's
// acceptance of the unit makes seven: two for the first entry of its set
// of unit references and one for its list of units, then the kind's
// entry in its map of kinds, the kind's map of units with its first
// entry, and the unit's list of the kind.
const flushAllocs = 8

// dispatchRun is a generate phase call that the dispatch benchmarks and
// TestDispatchAllocs repeat into one context: the generator, the context
// a first call warmed, the count of the subjects the rule visited, and
// the count one call visits.
type dispatchRun struct {
	gen     plugin.Generator
	ctx     *plugin.GeneratorContext
	visited *int
	want    int
}

// emitRun returns the call of an emit rule over units of perUnit
// values, whose origins g declares.
func emitRun(tb assert.TB, g *store.Graph, units, perUnit int) dispatchRun {
	tb.Helper()

	_, facts := boolKey(tb)
	visited := 0
	p := eidos.NewPlugin(benchPlugin).
		Handle(eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error {
			visited++
			return nil
		})).
		Build()
	r := dispatchRun{
		gen:     generatorOf(tb, p),
		ctx:     dispatchContext(tb, g, facts, nil, benchEmitStore(tb, units, perUnit), benchPlugin),
		visited: &visited,
		want:    units * perUnit,
	}
	assert.NoError(tb, r.generate(), "the first call warms the context")
	return r
}

// factGatedRun returns the call of an emit rule behind a fact gate over
// units of perUnit values, a multiple of ten units, whose origins g
// declares. It stamps the fact on one origin of every tenth unit.
func factGatedRun(tb assert.TB, g *store.Graph, units, perUnit int) dispatchRun {
	tb.Helper()

	key, facts := boolKey(tb)
	for u := 0; u < units; u += 10 {
		origin := coretest.Struct(coretest.StorePath+"/"+strconv.Itoa(u), "Decl0_0").ID
		assert.NoError(tb, meta.Stamp(facts, key, true, meta.Claim{Subject: origin}), "the origin is stamped")
	}
	visited := 0
	p := eidos.NewPlugin(benchPlugin).
		Handle(eidos.Where(eidos.HasKey(key),
			eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error {
				visited++
				return nil
			}))).
		Build()
	r := dispatchRun{
		gen:     generatorOf(tb, p),
		ctx:     dispatchContext(tb, g, facts, nil, benchEmitStore(tb, units, perUnit), benchPlugin),
		visited: &visited,
		want:    units / 10,
	}
	assert.NoError(tb, r.generate(), "the first call warms the context")
	return r
}

// bareRun returns the call of a bare struct rule over g, which declares
// structs structs.
func bareRun(tb assert.TB, g *store.Graph, structs int) dispatchRun {
	tb.Helper()

	_, facts := boolKey(tb)
	visited := 0
	p := eidos.NewPlugin(benchPlugin).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
			visited++
			return nil
		})).
		Build()
	r := dispatchRun{
		gen:     generatorOf(tb, p),
		ctx:     dispatchContext(tb, g, facts, nil, plugin.NewEmit(), benchPlugin),
		visited: &visited,
		want:    structs,
	}
	assert.NoError(tb, r.generate(), "the first call warms the context")
	return r
}

// directiveGatedRun returns the call of a directive-gated struct rule
// over a workspace of packages packages, each declaring decls structs
// with the gate's directive attached.
func directiveGatedRun(tb assert.TB, packages, decls int) dispatchRun {
	tb.Helper()

	schema := stubSchema(gateName)
	g, validated := gatedWorkspace(tb, schema.Canonical(), packages, 1, decls)
	_, facts := boolKey(tb)
	visited := 0
	p := eidos.NewPlugin(gatePlugin).
		Handle(eidos.Directive(schema, eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
			visited++
			return nil
		}))).
		Build()
	r := dispatchRun{
		gen:     generatorOf(tb, p),
		ctx:     dispatchContext(tb, g, facts, validated, plugin.NewEmit(), gatePlugin),
		visited: &visited,
		want:    packages * decls,
	}
	assert.NoError(tb, r.generate(), "the first call warms the context")
	return r
}

// generate resets the count of visited subjects, runs the phase call
// once and returns its error.
func (r dispatchRun) generate() error {
	*r.visited = 0
	return r.gen.Generate(r.ctx)
}

// check fails unless the last call visited every subject the rule routes
// to.
func (r dispatchRun) check(tb assert.TB) {
	tb.Helper()

	assert.Equal(tb, *r.visited, r.want, "the rule visits every subject it routes to")
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
			assert.Empty(t, units[0].Origins, "a graph match has no subject to record")
		})

		t.Run("wraps a handler's error with the plugin's rule", func(t *testing.T) {
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
			assert.Pairwise(t, units[0].Origins, func(earlier, later symbol.Identity) bool {
				return earlier.Compare(later) < 0
			}, "the declarations follow subject identity")
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
			assert.Equal(t, got, any(errFirst), "with the handler's own value", assert.ByIdentity())
		})

		t.Run("records no unit of the call before", func(t *testing.T) {
			t.Parallel()

			for range reuses {
				touchRun(t)
				rec, _, _ := lookupRun(t)
				for _, j := range rec.invoked {
					assert.Empty(t, j.Units, "no record names a unit the earlier call touched")
				}
			}
		})

		t.Run("records no read of the call before", func(t *testing.T) {
			t.Parallel()

			for range reuses {
				readingRun(t, oneWorker, manyStructs)
				rec, _, _ := lookupRun(t)
				assert.False(t, rec.invoked[1].read, "the second subject read nothing")
			}
		})

		t.Run("reads the graph of its own call through a reader", func(t *testing.T) {
			t.Parallel()

			gamma := coretest.Struct(coretest.CachePath, "Gamma")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.CachePath, gamma)),
				"the second graph's package is admitted")
			g.Freeze()
			_, facts := boolKey(t)
			for range reuses {
				lookupRun(t)
				found := false
				p := eidos.NewPlugin(contextPlugin).
					Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
						_, found = m.Reader().Lookup(gamma.ID)
						return nil
					})).
					Build()
				assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)), "the phase call passes")
				assert.True(t, found, "the reader reads this call's graph and not the earlier call's")
			}
		})

		t.Run("runs every rule of a plugin with more rules than the call before", func(t *testing.T) {
			t.Parallel()

			for range reuses {
				lookupRun(t)
				g, _, _ := fixtureGraph(t)
				_, facts := boolKey(t)
				p := eidos.NewPlugin(contextPlugin).
					Handle(
						eidos.OnStruct(nothing[*eidos.Emitter]),
						eidos.OnStruct(nothing[*eidos.Emitter]),
						eidos.OnStruct(nothing[*eidos.Emitter]),
					).
					Build()
				rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
				assert.Length(t, rec.invoked, 6, "three rules each match two subjects")
			}
		})

		t.Run("runs every match after a call under a selection", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			for range reuses {
				selectedNames(t, g, nil, &plugin.Selection{Candidates: []symbol.Identity{alpha.ID}})
				p, visited := visitingStructs(contextPlugin, eidos.OnStruct[*eidos.Emitter])
				assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)), "the phase call passes")
				assert.Equal(t, *visited, []string{"Alpha", "Beta"}, "a call without a selection visits both subjects")
			}
		})

		t.Run("assembles a package's accumulator after a call that failed", func(t *testing.T) {
			t.Parallel()

			for range reuses {
				_, err := failAt(t, eightWorkers)
				assert.ErrorIs(t, err, errFirst, "the earlier call fails")
				units := registry(t, oneWorker)
				assert.Length(t, units, 1, "the next call assembles one unit")
				assert.Length(t, units[0].Decls, manyStructs, "of every match it ran")
			}
		})

		t.Run("reports nothing of a call that panicked", func(t *testing.T) {
			t.Parallel()

			g, structs := manySubjects(t, manyStructs)
			_, facts := boolKey(t)
			panicking := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					m.Warnf(testCode, "%s", m.Struct.Name)
					if m.Struct.ID == structs[firstFailure].ID {
						panic(errFirst)
					}
					return nil
				})).
				Build()
			for range reuses {
				assert.Panics(t, func() {
					_ = generatorOf(t, panicking).Generate(on(genContext(t, g, facts, nil), eightWorkers))
				}, "the earlier call panics")
				ctx := genContext(t, g, facts, nil)
				p, visited := visitingStructs(contextPlugin, eidos.OnStruct[*eidos.Emitter])
				assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
				assert.Length(t, *visited, manyStructs, "the next call visits every subject")
				assert.Empty(t, messages(ctx.Sink), "and reports no finding of the call that panicked")
			}
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

			assert.NotEqual(t, ownRun(t), 0, "the seeded unit is visited")
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
			findings := slices.Collect(ctx.Sink.All())
			assert.Length(t, findings, 1, "the finding is reported")
			expect.Equal(t, findings[0].Pos, alpha.Pos, "the finding is at the origin's position")
			expect.Equal(t, findings[0].Origin, contextPlugin, "the finding is under the reporting plugin")
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
			var flushed []plugin.Unit
			for u := range ctx.Emit.Units() {
				if u.Plugin == contextPlugin {
					flushed = append(flushed, u)
				}
			}
			assert.Length(t, flushed, 1, "the accumulator flushed one unit")
			got := flushed[0]
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
			assert.Equal(t, *visited, []string{"Alpha", "Beta"}, "the declarations are in identity order")
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

		t.Run("runs a directive-gated rule once on a subject that writes both spellings", func(t *testing.T) {
			t.Parallel()

			schema := stubSchema(gateName)
			canonical := schema.Canonical()
			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			beta := coretest.Struct(coretest.StorePath, "Beta")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha, beta)),
				"the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(alpha.ID, []directive.Raw{{Name: canonical}, {Name: gateName}}),
				"the first subject writes both spellings")
			assert.NoError(t, g.AttachDirectives(beta.ID, []directive.Raw{{Name: gateName}}),
				"the second subject writes the bare spelling")
			g.Freeze()
			validated := map[symbol.Identity][]directive.Directive{
				alpha.ID: {{Name: canonical, Instance: 0}},
				beta.ID:  {{Name: canonical, Instance: 0}},
			}
			_, facts := boolKey(t)

			p, visited := visitingStructs("stubgen", func(h emitHandler) eidos.Rule {
				return eidos.Directive(schema, eidos.OnStruct(h))
			})
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, validated)),
				"the phase call passes")
			assert.Equal(t, *visited, []string{"Alpha", "Beta"}, "each carrier runs once, whatever spellings it writes")
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

// A generate phase call allocates nothing of its own on the routes a run
// takes: an emit rule, a fact gate, a bare rule and a directive gate,
// each into a context a call before the count warmed. A flush allocates
// the store's acceptance of its one unit. The ordinary run, which runs
// no benchmark, checks the benchmarks' ceilings here, over a workspace
// of 80 structs. The check runs alone, because the count includes every
// goroutine's allocations.
func TestDispatchAllocs(t *testing.T) {
	g := coretest.Frozen(t, coretest.Workspace(allocPackages, allocFiles, allocDecls)...)
	runs := []struct {
		name string
		run  dispatchRun
	}{
		{name: "an emit rule", run: emitRun(t, g, allocPackages, allocDecls)},
		{name: "a fact-gated emit rule", run: factGatedRun(t, g, allocPackages, allocDecls)},
		{name: "a bare rule", run: bareRun(t, g, allocPackages*allocFiles*allocDecls)},
		{name: "a directive-gated rule", run: directiveGatedRun(t, allocPackages, allocDecls)},
	}
	for _, tt := range runs {
		msg := "Generate allocates nothing for " + tt.name
		var err error
		assert.MaxAllocs(t, func() { err = tt.run.generate() }, 0, msg)
		assert.NoError(t, err, "the phase call passes for "+tt.name)
		tt.run.check(t)
	}
	checkPhaseAllocs(t, []phaseCase{flushCase(t, g)})
}

// BenchmarkDispatch measures generate-phase calls on the emit side of
// the canonical workspace: an emit rule over 20,000 values, a fact gate
// on their origins, and the flush of 10,000 declarations a graph rule
// places. A call allocates nothing of its own, because it takes the
// state the warm-up call released. The flush's ceiling is
// the store's acceptance of the one unit, which [flushAllocs]
// decomposes. TestDispatchAllocs checks every ceiling in the ordinary
// run.
func BenchmarkDispatch(b *testing.B) {
	const packages, files, decls = 1_000, 10, 20
	g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)

	b.Run("emit rule over 20k values", func(b *testing.B) {
		benchDispatch(b, emitRun(b, g, packages, decls))
	})

	b.Run("fact-gated emit rule over one origin in ten", func(b *testing.B) {
		benchDispatch(b, factGatedRun(b, g, packages, decls))
	})

	benchPhases(b, []phaseCase{flushCase(b, g)})
}

// BenchmarkNodeDispatch measures phase calls on the node side of the
// canonical workspace: a bare rule over 200,000 structs, a
// directive-gated rule over 20,000, and annotate calls that stamp facts
// on every struct into an empty store. A phase call allocates nothing of
// its own, because it takes the state the warm-up call released, and
// TestDispatchAllocs checks that zero ceiling in the ordinary run. The
// annotate cases measure the fact store's claims, which the store keeps,
// and each case's ceiling decomposes them. Only -bench checks those
// ceilings: one annotate call over 200,000 subjects takes 0.12 and 0.40
// seconds, and a count repeats it 101 times.
func BenchmarkNodeDispatch(b *testing.B) {
	const packages, files, decls = 1_000, 10, 20
	g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)

	// The annotate cases' ceilings. Each subject's first claim allocates
	// its bag, the boxed identity and the sync.Map entry, and a boolean
	// boxes without allocating. A subject's second and third keys
	// allocate the bag's key map with its first group, and a state each.
	// The three reads before the stamps make one derivation, which the
	// three claims share. Each key's index allocates 1,047 times as its
	// map grows to 200,000 members, and the store's sync.Map allocates its
	// root. sync.Map also adds trie nodes at random where two hashes share
	// a prefix: 75,426 on average with a standard deviation of 148 over 40
	// runs of 200,000 insertions. Each ceiling allows 76,610 trie nodes,
	// eight standard deviations above the mean.
	const (
		annotateSubjects    = packages * files * decls
		annotateIndex       = 1_047
		annotateTrie        = 76_610
		annotateOneAllocs   = 3*annotateSubjects + annotateIndex + 1 + annotateTrie
		annotateThreeAllocs = 8*annotateSubjects + 3*annotateIndex + 1 + annotateTrie
	)

	b.Run("bare rule over 200k structs", func(b *testing.B) {
		benchDispatch(b, bareRun(b, g, packages*files*decls))
	})

	b.Run("directive-gated rule over 20k subjects", func(b *testing.B) {
		benchDispatch(b, directiveGatedRun(b, packages, decls))
	})

	b.Run("annotate stamps three facts after three reads per subject", func(b *testing.B) {
		reg, keys := annotateKeys(b, "t.first", "t.second", "t.third")
		p := eidos.NewPlugin(benchPlugin).
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
		ann := annotatorOf(b, p)
		var ctx *plugin.AnnotatorContext
		fresh := func() { ctx = annotateContext(b, g, reg) }
		c := bench.Start(b).Warmup(1).MaxAllocs(annotateThreeAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			c.Excluding(fresh)
			err = ann.Annotate(ctx)
		}
		assert.NoError(b, err, "the annotate call passes")
		assert.False(b, ctx.Sink.Failed(), "no stamp is refused")
	})

	b.Run("annotate stamps 200k subjects", func(b *testing.B) {
		reg, keys := annotateKeys(b, "t.flag")
		p := eidos.NewPlugin(benchPlugin).
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
				eidos.Stamp(st, keys[0], true)
				return nil
			})).
			Build()
		ann := annotatorOf(b, p)
		var ctx *plugin.AnnotatorContext
		fresh := func() { ctx = annotateContext(b, g, reg) }
		c := bench.Start(b).Warmup(1).MaxAllocs(annotateOneAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			c.Excluding(fresh)
			err = ann.Annotate(ctx)
		}
		assert.NoError(b, err, "the annotate call passes")
		assert.False(b, ctx.Sink.Failed(), "no stamp is refused")
	})
}

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

// flushCase returns the generate phase call whose graph rule places
// flushPlaced declarations into its plan file, each call over a fresh
// context on g.
func flushCase(tb assert.TB, g *store.Graph) phaseCase {
	tb.Helper()

	_, facts := boolKey(tb)
	ix, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(tb, err, "the routing surface builds")
	payload := make([]symbol.Symbol, flushPlaced)
	for i := range payload {
		payload[i] = &emit.Struct{
			Origin: coretest.Struct(coretest.StorePath+"/0", "Decl0_0").ID,
			Name:   "Gen" + strconv.Itoa(i),
		}
	}
	p := eidos.NewPlugin(benchPlugin).
		Output(plugin.Output{Per: plugin.PerPlan, Word: "registry"}).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
			e.PlanFile().Append(payload...)
			return nil
		})).
		Build()
	return phaseCase{
		name:   "flush of 10k placed declarations",
		allocs: flushAllocs,
		gen:    generatorOf(tb, p),
		fresh: func() *plugin.GeneratorContext {
			return &plugin.GeneratorContext{
				Index: ix, Facts: facts, Emit: plugin.NewEmit(),
				Sink: diag.NewSink(), Plugin: benchPlugin, Bucket: 1,
			}
		},
		check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
			units := slices.Collect(ctx.Emit.Units())
			assert.Length(tb, units, 1, "the flush adds one unit")
			assert.Length(tb, units[0].Decls, flushPlaced, "of every placed declaration")
		},
	}
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
		assert.NoError(tb, e.Add(plugin.Unit{
			Plugin: "earlier", Per: plugin.PerSource, Word: "impl",
			Key: "unit" + strconv.Itoa(u) + ".go", Decls: decls,
		}), "the earlier unit is added")
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

// annotateKeys returns a registry with one boolean key registered under
// each name, and the keys in the order of the names.
func annotateKeys(tb assert.TB, names ...meta.KeyName) (*meta.Registry, []meta.Key[bool]) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
	keys := make([]meta.Key[bool], 0, len(names))
	for _, name := range names {
		key, err := meta.Register[bool](reg, meta.KeySpec{Name: name, Doc: "marks a bench subject"})
		assert.NoError(tb, err, "the key registers")
		keys = append(keys, key)
	}
	return reg, keys
}

// annotateContext returns an annotator context over g with an empty
// fact store over reg.
func annotateContext(tb assert.TB, g *store.Graph, reg *meta.Registry) *plugin.AnnotatorContext {
	tb.Helper()

	facts := meta.NewFacts(reg)
	ix, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(tb, err, "the routing surface builds")
	return annContext(tb, facts, ix)
}

// invocationGraph returns a frozen graph of the invocation fixture's
// structs, without positions, in one file of the store package, and
// the structs.
func invocationGraph(tb assert.TB) (*store.Graph, []*node.Struct) {
	tb.Helper()

	structs := make([]*node.Struct, 0, invocationStructs)
	decls := make([]symbol.Symbol, 0, invocationStructs)
	for i := range invocationStructs {
		s := coretest.Struct(coretest.StorePath, "Subject"+strconv.Itoa(i))
		structs = append(structs, s)
		decls = append(decls, s)
	}
	return coretest.Frozen(tb, coretest.Package(coretest.StorePath, decls...)), structs
}

// invocationContexts returns a function that returns a generator
// context over the invocation fixture with an empty sink, the flag key,
// and the fixture's structs. Every struct reads the flag present, the
// fixture's language has rules, and the context reads the export of
// exportingPlan. Where seeded is true, the emit store contains one
// earlier unit of one emit struct per subject, which an emit rule
// visits, and an empty store otherwise.
func invocationContexts(
	tb assert.TB, seeded bool,
) (func() *plugin.GeneratorContext, meta.Key[bool], []*node.Struct) {
	tb.Helper()

	g, structs := invocationGraph(tb)
	key, facts := boolKey(tb)
	for _, s := range structs {
		assert.NoError(tb, meta.Stamp(facts, key, true, meta.Claim{Subject: s.ID}), "the subject is flagged")
	}
	ix, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(tb, err, "the routing surface builds")
	registry := rules.NewRegistry()
	assert.NoError(tb, registry.Register(fixtureLanguage{rulestest.Scripted()}), "the fixture language registers")
	exports := map[string]plugin.ExportDoc{exportingPlan: {Plan: exportingPlan}}
	fresh := func() *plugin.GeneratorContext {
		ctx := &plugin.GeneratorContext{
			Index: ix, Facts: facts, Emit: plugin.NewEmit(), Sink: diag.NewSink(),
			Plugin: contextPlugin, Bucket: 2, Rules: registry, Exports: exports,
		}
		if seeded {
			values := make([]symbol.Symbol, 0, len(structs))
			for _, s := range structs {
				values = append(values, emitted(s))
			}
			seed(tb, ctx, values...)
		}
		return ctx
	}
	return fresh, key, structs
}

// checkPhaseAllocs checks each case's ceiling in the ordinary run, which
// runs no benchmark. Each counted call takes a context built outside the
// count, because a call leaves its units and findings in its context.
// The count's first call warms the pooled state of a phase call. Each
// count keeps the first error of its calls, which cmp.Or returns
// without allocating.
func checkPhaseAllocs(t *testing.T, cases []phaseCase) {
	t.Helper()

	for _, tt := range cases {
		var (
			last *plugin.GeneratorContext
			err  error
		)
		msg := tt.name + " allocates within its ceiling over the invocation fixture"
		assert.MaxAllocsWithSetup(t, tt.fresh, func(ctx *plugin.GeneratorContext) {
			err = cmp.Or(err, tt.gen.Generate(ctx))
			last = ctx
		}, tt.allocs, msg)
		assert.NoError(t, err, "every counted call of "+tt.name+" passes")
		tt.check(t, last)
	}
}

// benchPhases measures each case's phase call under its ceiling. Each
// call takes a fresh context outside the measurement, and a warm-up call
// fills the pooled state of a phase call.
func benchPhases(b *testing.B, cases []phaseCase) {
	b.Helper()

	for _, tt := range cases {
		b.Run(tt.name, func(b *testing.B) {
			var ctx *plugin.GeneratorContext
			c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(func() { ctx = tt.fresh() })
				err = tt.gen.Generate(ctx)
			}
			assert.NoError(b, err, "the phase call passes")
			tt.check(b, ctx)
		})
	}
}

// benchDispatch measures a run's phase call under a zero ceiling, each
// call into the context the run's first call warmed.
func benchDispatch(b *testing.B, r dispatchRun) {
	b.Helper()

	c := bench.Start(b).MaxAllocs(0)
	defer c.End()
	var err error
	for c.Loop() {
		err = r.generate()
	}
	assert.NoError(b, err, "the phase call passes")
	r.check(b)
}

// dispatchContext returns a generator context over g in the first
// bucket, running as name, with its own sink and the emit store e.
func dispatchContext(
	tb assert.TB, g *store.Graph, facts *meta.Facts,
	validated map[symbol.Identity][]directive.Directive, e *plugin.Emit, name plugin.ID,
) *plugin.GeneratorContext {
	tb.Helper()

	ix, err := plugin.NewIndex(g, facts, validated, nil)
	assert.NoError(tb, err, "the routing surface builds")
	return &plugin.GeneratorContext{Index: ix, Facts: facts, Emit: e, Sink: diag.NewSink(), Plugin: name, Bucket: 1}
}
