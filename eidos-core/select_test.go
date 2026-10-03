// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The subjects the bare-rule selection cases list, by index into the
// many-subject fixture.
const (
	listedEarly = 3
	listedLate  = 10
)

// absentInstance is a gating instance the gated fixture's subject does
// not have.
const absentInstance = 7

// A selection restricts a phase call to what a warm run executes again:
// the listed matches its gates still admit and every match of a
// candidate, in canonical match order, with emit-phase rules running
// whole.
func TestSelect(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the listed matches of a bare rule alone", func(t *testing.T) {
			t.Parallel()

			g, structs := manySubjects(t, manyStructs)
			got := selectedNames(t, g, nil, &plugin.Selection{Matches: []plugin.MatchKey{
				keyOf(structs[listedEarly].ID), keyOf(structs[listedLate].ID),
			}})
			assert.Equal(t, got, []string{structs[listedEarly].Name, structs[listedLate].Name},
				"the two listed subjects run and no other")
		})

		t.Run("runs the listed matches in identity order whatever the listing order", func(t *testing.T) {
			t.Parallel()

			g, structs := manySubjects(t, manyStructs)
			got := selectedNames(t, g, nil, &plugin.Selection{Matches: []plugin.MatchKey{
				keyOf(structs[listedLate].ID), keyOf(structs[listedEarly].ID), keyOf(structs[listedLate].ID),
			}})
			assert.Equal(t, got, []string{structs[listedEarly].Name, structs[listedLate].Name},
				"the selection's order and its repeats do not reach the dispatch")
		})

		t.Run("runs no match for an empty selection", func(t *testing.T) {
			t.Parallel()

			g, _ := manySubjects(t, manyStructs)
			assert.Empty(t, selectedNames(t, g, nil, &plugin.Selection{}), "nothing is selected")
		})

		t.Run("runs no listed match whose subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			ghost := coretest.ID(coretest.StorePath, "Ghost", symbol.KindStruct)
			got := selectedNames(t, g, nil, &plugin.Selection{Matches: []plugin.MatchKey{keyOf(ghost)}})
			assert.Empty(t, got, "a gone subject has no match")
		})

		t.Run("runs no listed match the subject's opt-out refuses", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			got := selectedNames(t, g, negatedOn(alpha.ID), &plugin.Selection{
				Matches: []plugin.MatchKey{keyOf(alpha.ID), keyOf(beta.ID)},
			})
			assert.Equal(t, got, []string{"Beta"}, "the opted-out subject does not run")
		})

		t.Run("runs no listed match the predicate refuses", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: beta.ID}),
				"the second subject's fact stamps")
			p, visited := visitingStructs(contextPlugin, func(h emitHandler) eidos.Rule {
				return eidos.Where(eidos.HasKey(key), eidos.OnStruct(h))
			})
			ctx := genContext(t, g, facts, nil)
			ctx.Select = &plugin.Selection{Matches: []plugin.MatchKey{keyOf(alpha.ID), keyOf(beta.ID)}}
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, *visited, []string{"Beta"}, "the subject without the fact does not run")
		})

		t.Run("runs no listed match of a bare rule above instance zero", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key := keyOf(alpha.ID)
			key.Instance = 1
			got := selectedNames(t, g, nil, &plugin.Selection{Matches: []plugin.MatchKey{key}})
			assert.Empty(t, got, "an ungated rule's one match has instance zero")
		})

		t.Run("runs a listed instance the subject carries", func(t *testing.T) {
			t.Parallel()

			got := selectedInstances(t, func(subject symbol.Identity) *plugin.Selection {
				return &plugin.Selection{Matches: []plugin.MatchKey{
					{Plugin: contextPlugin, Subject: subject, Instance: 2},
				}}
			})
			assert.Equal(t, got, []int{2}, "the listed instance runs alone")
		})

		t.Run("runs two listed instances of one subject", func(t *testing.T) {
			t.Parallel()

			got := selectedInstances(t, func(subject symbol.Identity) *plugin.Selection {
				return &plugin.Selection{Matches: []plugin.MatchKey{
					{Plugin: contextPlugin, Subject: subject, Instance: 0},
					{Plugin: contextPlugin, Subject: subject, Instance: 2},
				}}
			})
			assert.Equal(t, got, []int{0, 2}, "each listed instance runs")
		})

		t.Run("runs a repeated listed instance once", func(t *testing.T) {
			t.Parallel()

			got := selectedInstances(t, func(subject symbol.Identity) *plugin.Selection {
				key := plugin.MatchKey{Plugin: contextPlugin, Subject: subject, Instance: 1}
				return &plugin.Selection{Matches: []plugin.MatchKey{key, key}}
			})
			assert.Equal(t, got, []int{1}, "the repeat does not reach the dispatch")
		})

		t.Run("runs no listed instance the subject does not carry", func(t *testing.T) {
			t.Parallel()

			got := selectedInstances(t, func(subject symbol.Identity) *plugin.Selection {
				return &plugin.Selection{Matches: []plugin.MatchKey{
					{Plugin: contextPlugin, Subject: subject, Instance: absentInstance},
				}}
			})
			assert.Empty(t, got, "a gone instance has no match")
		})

		t.Run("runs every match of a candidate", func(t *testing.T) {
			t.Parallel()

			got := selectedInstances(t, func(subject symbol.Identity) *plugin.Selection {
				return &plugin.Selection{Candidates: []symbol.Identity{subject}}
			})
			assert.Equal(t, got, []int{0, 1, 2}, "each instance of the candidate runs, in instance order")
		})

		t.Run("runs a listed match of a candidate once", func(t *testing.T) {
			t.Parallel()

			got := selectedInstances(t, func(subject symbol.Identity) *plugin.Selection {
				return &plugin.Selection{
					Matches:    []plugin.MatchKey{{Plugin: contextPlugin, Subject: subject, Instance: 1}},
					Candidates: []symbol.Identity{subject},
				}
			})
			assert.Equal(t, got, []int{0, 1, 2}, "the candidate's evaluation covers its listed match")
		})

		t.Run("runs a repeated candidate once", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			got := selectedNames(t, g, nil, &plugin.Selection{Candidates: []symbol.Identity{alpha.ID, alpha.ID}})
			assert.Equal(t, got, []string{"Alpha"}, "the repeat does not reach the dispatch")
		})

		t.Run("runs no candidate the scope excludes", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, func(symbol.Identity) bool { return false })
			assert.NoError(t, err, "the routing surface builds")
			p, visited := visitingStructs(contextPlugin, eidos.OnStruct[*eidos.Emitter])
			ctx := genContext(t, g, facts, nil)
			ctx.Index = ix
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{alpha.ID}}
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Empty(t, *visited, "a subject outside the scope has no match")
		})

		t.Run("runs no candidate of another kind", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			reader := coretest.Interface(coretest.StorePath, coretest.InterfaceName)
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha, reader)),
				"the fixture package is admitted")
			g.Freeze()
			got := selectedNames(t, g, nil, &plugin.Selection{Candidates: []symbol.Identity{reader.ID}})
			assert.Empty(t, got, "an interface is no struct rule's subject")
		})

		t.Run("runs a graph rule the selection lists", func(t *testing.T) {
			t.Parallel()

			calls := graphCalls(t, &plugin.Selection{Matches: []plugin.MatchKey{{Plugin: contextPlugin}}})
			assert.Equal(t, calls, 1, "the listed graph rule runs once")
		})

		t.Run("runs no graph rule the selection does not list", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphCalls(t, &plugin.Selection{}), 0, "an unlisted graph rule does not run")
		})

		t.Run("runs every emit-phase match whatever the selection", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha), emitted(beta))
			ctx.Select = &plugin.Selection{}
			var visited []string
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, _ *eidos.Emitter) error {
					s, held := m.Value.(*emit.Struct)
					assert.True(t, held, "a struct rule receives structs")
					visited = append(visited, s.Name)
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, visited, []string{"GenAlpha", "GenBeta"}, "both values run")
		})

		t.Run("runs a listed match under its own rule alone", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			key := keyOf(alpha.ID)
			key.Rule = 1
			ctx.Select = &plugin.Selection{Matches: []plugin.MatchKey{key}}
			var ran []string
			visit := func(rule string) emitHandler {
				return func(*eidos.StructMatch, *eidos.Emitter) error {
					ran = append(ran, rule)
					return nil
				}
			}
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(visit("first")), eidos.OnStruct(visit("second"))).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, ran, []string{"second"}, "the first rule lists no match")
		})

		t.Run("runs no listed match of another plugin", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key := keyOf(alpha.ID)
			key.Plugin = otherPlugin
			assert.Empty(t, selectedNames(t, g, nil, &plugin.Selection{Matches: []plugin.MatchKey{key}}),
				"another plugin's key selects nothing of this one")
		})

		t.Run("runs no listed match of a rule the plugin does not declare", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			beyond, whole := keyOf(alpha.ID), keyOf(beta.ID)
			beyond.Rule, whole.Rule = 3, plugin.WholeCall
			got := selectedNames(t, g, nil, &plugin.Selection{Matches: []plugin.MatchKey{whole, beyond}})
			assert.Empty(t, got, "an ordinal outside the declaration selects nothing")
		})

		t.Run("numbers the selected matches in canonical match order across rules", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{beta.ID, alpha.ID}}
			warn := func(rule string) emitHandler {
				return func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					m.Warnf(testCode, "%s %s", rule, m.Struct.Name)
					return nil
				}
			}
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(warn("first")), eidos.OnStruct(warn("second"))).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, messages(ctx.Sink), []string{
				"first Alpha", "first Beta", "second Alpha", "second Beta",
			}, "the findings follow rule, then subject")
		})

		t.Run("assembles for a selection of every match the units of a full call", func(t *testing.T) {
			t.Parallel()

			_, structs := manySubjects(t, manyStructs)
			keys := make([]plugin.MatchKey, len(structs))
			for i, s := range structs {
				keys[i] = keyOf(s.ID)
			}
			assert.Equal(t, registryUnits(t, oneWorker, &plugin.Selection{Matches: keys}),
				registryUnits(t, oneWorker, nil), "the selected call places what the full call places")
		})

		t.Run("assembles for a selection of every match on eight workers the units of a full call", func(t *testing.T) {
			t.Parallel()

			_, structs := manySubjects(t, manyStructs)
			keys := make([]plugin.MatchKey, len(structs))
			for i, s := range structs {
				keys[i] = keyOf(s.ID)
			}
			assert.Equal(t, registryUnits(t, eightWorkers, &plugin.Selection{Matches: keys}),
				registryUnits(t, oneWorker, nil), "the selected call does not depend on the worker count")
		})

		t.Run("assembles for every subject as a candidate the units of a full call", func(t *testing.T) {
			t.Parallel()

			_, structs := manySubjects(t, manyStructs)
			candidates := make([]symbol.Identity, len(structs))
			for i, s := range structs {
				candidates[len(structs)-1-i] = s.ID
			}
			assert.Equal(t, registryUnits(t, oneWorker, &plugin.Selection{Candidates: candidates}),
				registryUnits(t, oneWorker, nil), "the candidates' evaluation places what the full call places")
		})
	})

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		// stamped annotates the fixture graph under the selection that the
		// given function builds for its two structs, and returns the
		// stamped subjects.
		stamped := func(tb assert.TB, selection func(alpha, beta symbol.Identity) *plugin.Selection) []symbol.Identity {
			tb.Helper()

			g, alpha, beta := fixtureGraph(tb)
			key, facts := boolKey(tb)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(tb, err, "the routing surface builds")
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, key, true)
					return nil
				})).
				Build()
			ctx := annContext(tb, facts, ix)
			ctx.Plugin = contextPlugin
			ctx.Select = selection(alpha.ID, beta.ID)
			assert.NoError(tb, annotatorOf(tb, p).Annotate(ctx), "the phase call passes")
			return slices.Collect(facts.ByKey(key.ID()))
		}

		t.Run("stamps for a selection what a full call stamps", func(t *testing.T) {
			t.Parallel()

			selected := stamped(t, func(alpha, beta symbol.Identity) *plugin.Selection {
				return &plugin.Selection{
					Matches: []plugin.MatchKey{keyOf(beta)}, Candidates: []symbol.Identity{alpha},
				}
			})
			full := stamped(t, func(_, _ symbol.Identity) *plugin.Selection { return nil })
			assert.Equal(t, selected, full, "a listed match and a candidate cover both subjects")
		})

		t.Run("stamps the listed matches alone", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, key, true)
					return nil
				})).
				Build()
			ctx := annContext(t, facts, ix)
			ctx.Plugin = contextPlugin
			ctx.Select = &plugin.Selection{Matches: []plugin.MatchKey{keyOf(beta.ID)}}
			assert.NoError(t, annotatorOf(t, p).Annotate(ctx), "the phase call passes")
			assert.Equal(t, slices.Collect(facts.ByKey(key.ID())), []symbol.Identity{beta.ID},
				"the unlisted subject is not stamped")
			_, held := meta.Get(facts, alpha.ID, key)
			assert.False(t, held, "and the first subject has no fact")
		})
	})
}

// BenchmarkSelect measures a selected phase call over the 200,000
// structs of the bench workspace: the cost of a warm run's call that
// runs one match, which enumerates no index. Each iteration allocates
// its context, its emit store and its sink, five allocations, and the
// call allocates its state, its first lane's matches and the match it
// runs, three more. A candidate's match costs one more, the buffer of
// the matches it has.
func BenchmarkSelect(b *testing.B) {
	const packages, files, decls = 1_000, 10, 20
	g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)
	_, facts := boolKey(b)
	ix, err := plugin.NewIndex(g, facts, nil, nil)
	if err != nil {
		b.Fatalf("NewIndex: unexpected error: %v", err)
	}
	subject := coretest.Struct(coretest.StorePath+"/500", "Decl5_10").ID
	visited := 0
	p := eidos.NewPlugin("bench").
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
			visited++
			return nil
		})).
		Build()
	gen, ok := p.(plugin.Generator)
	if !ok {
		b.Fatal("the bench plugin must generate")
	}
	run := func(b *testing.B, sel *plugin.Selection, ceiling uint64) {
		b.Helper()

		c := bench.Start(b).MaxAllocs(ceiling)
		defer c.End()
		for c.Loop() {
			visited = 0
			ctx := &plugin.GeneratorContext{
				Index: ix, Facts: facts, Emit: plugin.NewEmit(), Sink: diag.NewSink(),
				Plugin: "bench", Bucket: 1, Select: sel,
			}
			if err := gen.Generate(ctx); err != nil {
				b.Fatalf("Generate: unexpected error: %v", err)
			}
		}
		if visited != 1 {
			b.Fatalf("the selected call ran %d matches", visited)
		}
	}

	b.Run("Generate a listed match", func(b *testing.B) {
		run(b, &plugin.Selection{Matches: []plugin.MatchKey{{Plugin: "bench", Subject: subject}}}, 8)
	})

	b.Run("Generate a candidate", func(b *testing.B) {
		run(b, &plugin.Selection{Candidates: []symbol.Identity{subject}}, 9)
	})
}

// keyOf returns the context plugin's first-rule key for a subject.
func keyOf(subject symbol.Identity) plugin.MatchKey {
	return plugin.MatchKey{Plugin: contextPlugin, Subject: subject}
}

// selectedNames runs a bare struct rule of the context plugin over g
// under the selection and the validated table, and returns the names of
// the subjects it visited, in visit order.
func selectedNames(
	tb assert.TB, g *store.Graph, validated map[symbol.Identity][]directive.Directive, sel *plugin.Selection,
) []string {
	tb.Helper()

	_, facts := boolKey(tb)
	p, visited := visitingStructs(contextPlugin, eidos.OnStruct[*eidos.Emitter])
	ctx := genContext(tb, g, facts, validated)
	ctx.Select = sel
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	return *visited
}

// selectedInstances runs a directive-gated struct rule over one subject
// with three instances of a repeatable directive, under the selection
// that the given function builds for the subject, and returns the
// instances the rule visited, in visit order.
func selectedInstances(tb assert.TB, selection func(subject symbol.Identity) *plugin.Selection) []int {
	tb.Helper()

	g, alpha := gatedStruct(tb, scheduled)
	_, facts := boolKey(tb)
	schema := stubSchema(gateName)
	schema.Repeatable = true
	ctx := genContext(tb, g, facts, repeated(alpha.ID, schema, scheduled))
	ctx.Select = selection(alpha.ID)
	var visited []int
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.Directive(schema, eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
			visited = append(visited, m.Directive().Instance)
			return nil
		}))).
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	return visited
}

// graphCalls runs a graph rule of the context plugin under the
// selection, and returns how often its handler ran.
func graphCalls(tb assert.TB, sel *plugin.Selection) int {
	tb.Helper()

	g, _, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	ctx.Select = sel
	calls := 0
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnGraph(func(*eidos.GraphMatch, *eidos.Emitter) error {
			calls++
			return nil
		})).
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	return calls
}

// registryUnits runs a plugin that places one struct per subject of the
// many-subject fixture into its package's accumulator and reports one
// finding per subject, on the given workers under the selection, and
// returns the store's units.
func registryUnits(tb assert.TB, workers int, sel *plugin.Selection) []plugin.Unit {
	tb.Helper()

	g, _ := manySubjects(tb, manyStructs)
	_, facts := boolKey(tb)
	ctx := on(genContext(tb, g, facts, nil), workers)
	ctx.Select = sel
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
