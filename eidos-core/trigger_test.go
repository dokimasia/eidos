// Copyright Dokimasia B.V. 2026
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
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The allocations of the triggers and of their matches' methods.
const (
	// triggerAllocs is a trigger rule: its leaf, and its invocation's
	// closure over the handler.
	triggerAllocs = 2
	// graphReportAllocs is a graph phase call whose one invocation
	// reports one finding: the message, and the sink's first slot.
	graphReportAllocs = 2
)

// The structural triggers position their handlers: an emit match
// names the value and its origin, and neither trigger passes a gating
// instance it was not given.
func TestTrigger(t *testing.T) {
	t.Parallel()

	t.Run("OnEmit", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the emit value with its origin to the handler", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			value := emitted(alpha)
			seed(t, ctx, value)

			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						ran = true
						assert.Equal(t, m.Value, symbol.Symbol(value), "the match has the very emit value",
							assert.ByIdentity())
						assert.Equal(t, m.Origin(), alpha.ID,
							"and the node identity it derives from")
						assert.Nil(t, m.Directive(),
							"a bare match has no gating instance")
						return nil
					})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.True(t, ran, "the handler ran")
		})

		t.Run("returns a rule of the kind in the emit phase", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, eidos.OnEmit(symbol.KindStruct,
				func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil }))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct, Phase: plugin.PhaseEmit,
			}}, "the record is in the emit phase")
		})

		t.Run("skips a value without an origin", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, &emit.Struct{Name: "NoOrigin"})

			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						ran = true
						return nil
					})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.False(t, ran, "a value without an origin is not a subject")
		})
	})

	t.Run("OnGraph", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an ungated rule in the generate phase", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, emitNothing())
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Phase: plugin.PhaseGenerate,
			}}, "zero gate fields spell ungated, and a graph rule has no kind")
		})

		t.Run("passes no gating instance to the handler", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					ran = true
					assert.Nil(t, m.Directive(),
						"nothing gated the graph rule, by construction")
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.True(t, ran, "the handler ran")
		})
	})

	t.Run("GraphMatch", func(t *testing.T) {
		t.Parallel()

		t.Run("WarnfAt", func(t *testing.T) {
			t.Parallel()

			t.Run("reports a Warning at the position that the handler passes", func(t *testing.T) {
				t.Parallel()

				at := position.Pos{File: "chosen.go", Line: 11, Col: 2}
				ctx := graphRun(t, func(m *eidos.GraphMatch) {
					m.WarnfAt(graphReported, at, "the graph contains %d packages", 1)
				})

				coretest.AssertCodes(t, ctx.Sink, graphReported)
				one := onlyFinding(t, ctx.Sink)
				assert.Equal(t, one.Severity, diag.SeverityWarning, "WarnfAt reports at Warning severity")
				assert.Equal(t, one.Pos, at, "the finding has the position that the handler passed")
				assert.False(t, ctx.Sink.Failed(), "a Warning leaves the run passing")
				assert.Contains(t, one.Msg, "1 packages", "the message has the formatting of the handler")
			})
		})

		t.Run("InfofAt", func(t *testing.T) {
			t.Parallel()

			t.Run("reports an Info finding at the position that the handler passes", func(t *testing.T) {
				t.Parallel()

				at := position.Pos{File: "chosen.go", Line: 3, Col: 4}
				ctx := graphRun(t, func(m *eidos.GraphMatch) {
					m.InfofAt(graphReported, at, "the graph phase ran")
				})

				one := onlyFinding(t, ctx.Sink)
				assert.Equal(t, one.Severity, diag.SeverityInfo, "InfofAt reports at Info severity")
				assert.Equal(t, one.Pos, at, "the finding has the position that the handler passed")
				assert.False(t, ctx.Sink.Failed(), "an Info finding leaves the run passing")
			})
		})

		t.Run("ErrorfAt", func(t *testing.T) {
			t.Parallel()

			t.Run("reports an Error at the position that the handler passes", func(t *testing.T) {
				t.Parallel()

				at := position.Pos{File: "chosen.go", Line: 5, Col: 6}
				ctx := graphRun(t, func(m *eidos.GraphMatch) {
					m.ErrorfAt(graphReported, at, "the graph is refused")
				})

				one := onlyFinding(t, ctx.Sink)
				assert.Equal(t, one.Severity, diag.SeverityError, "ErrorfAt reports at Error severity")
				assert.Equal(t, one.Pos, at, "the finding has the position that the handler passed")
				assert.True(t, ctx.Sink.Failed(), "an Error fails the run")
			})
		})
	})

	t.Run("OnPackage", func(t *testing.T) {
		t.Parallel()

		t.Run("runs once for each package in scope", func(t *testing.T) {
			t.Parallel()

			store := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Alpha"))
			cache := coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Beta"))
			var visited []symbol.Identity
			dispatched(t, eidos.NewPlugin(subjectPlugin).
				Handle(eidos.OnPackage(func(m *eidos.PackageMatch, _ *eidos.Emitter) error {
					visited = append(visited, m.Package.ID)
					return nil
				})).
				Generator(), store, cache)
			assert.Permutation(t, visited, []symbol.Identity{store.ID, cache.ID},
				"the rule runs once for each package of the graph")
		})

		t.Run("passes the package with its declarations to the handler", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			var files []*node.File
			dispatched(t, eidos.NewPlugin(subjectPlugin).
				Handle(eidos.OnPackage(func(m *eidos.PackageMatch, _ *eidos.Emitter) error {
					files = m.Package.Files
					return nil
				})).
				Generator(), coretest.Package(coretest.StorePath, alpha))
			assert.Length(t, files, 1, "the package has its one file")
			assert.Contains(t, files[0].Decls, symbol.Symbol(alpha), "the file has the declaration of the package",
				assert.ByIdentity())
		})

		t.Run("records a package edge for the package of the match", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnPackage(func(*eidos.PackageMatch, *eidos.Emitter) error { return nil })).
				Generator()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			assert.Length(t, rec.invoked, 1, "the one package has one invocation")
			assert.Equal(t, rec.invoked[0].packages, []symbol.Identity{coretest.PackageID(coretest.StorePath)},
				"a change of any member of the package runs the invocation again")
		})

		t.Run("runs once for each gating instance of a directive on a package", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Alpha"))
			g := store.New()
			assert.NoError(t, g.AddPackage(pkg), "the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(pkg.ID, []directive.Raw{{Name: gateName}, {Name: gateName}}),
				"the gating instances attach to the package")
			g.Freeze()
			_, facts := boolKey(t)
			schema := stubSchema(gateName)
			schema.Repeatable = true
			var instances []int
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.Directive(schema, eidos.OnPackage(func(m *eidos.PackageMatch, _ *eidos.Emitter) error {
					instances = append(instances, m.Directive().Instance)
					return nil
				}))).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, repeated(pkg.ID, schema, 2))),
				"the phase call passes")
			assert.Equal(t, instances, []int{0, 1}, "the handler runs for each instance in instance order")
		})

		t.Run("passes over a value that is not a package", func(t *testing.T) {
			t.Parallel()

			var visited []symbol.Identity
			dispatched(t, eidos.NewPlugin(subjectPlugin).
				Handle(eidos.OnPackage(func(m *eidos.PackageMatch, _ *eidos.Emitter) error {
					visited = append(visited, m.Package.ID)
					return nil
				})).
				Generator(),
				coretest.Package(coretest.StorePath, coretest.Foreign(
					coretest.StorePath, coretest.ForeignName, symbol.KindPackage)))
			assert.Equal(t, visited, []symbol.Identity{coretest.PackageID(coretest.StorePath)},
				"the handler sees the package and not a declaration of another type under its kind")
		})

		t.Run("returns a rule of the package kind in the generate phase", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, eidos.OnPackage(func(*eidos.PackageMatch, *eidos.Emitter) error { return nil }))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindPackage, Phase: plugin.PhaseGenerate,
			}}, "an emitter handler generates")
		})

		t.Run("returns a rule of the package kind in the annotate phase", func(t *testing.T) {
			t.Parallel()

			stamps := eidos.NewPlugin(subjectPlugin).
				Handle(eidos.OnPackage(func(*eidos.PackageMatch, *eidos.Stamper) error { return nil })).
				Annotator()
			subscribed, held := stamps.(plugin.Subscribed)
			assert.True(t, held, "a built plugin declares its gates")
			assert.Equal(t, subscribed.Subscriptions(), []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindPackage, Phase: plugin.PhaseAnnotate,
			}}, "a stamper handler annotates")
		})
	})

	t.Run("EmitMatch.Origin", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identity the emit value derives from", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))
			var origin symbol.Identity
			p := eidos.NewPlugin("t").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						origin = m.Origin()
						return nil
					})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, origin, alpha.ID, "the origin is the node the value derives from")
		})
	})
}

// A trigger allocates its rule, and its matches' methods allocate what
// they report, in the ordinary run, which runs no benchmark. The check
// runs alone, because the count includes every goroutine's allocations.
func TestTriggerAllocs(t *testing.T) {
	checkPhaseAllocs(t, triggerCases(t))

	onGraph := func(*eidos.GraphMatch, *eidos.Emitter) error { return nil }
	onEmit := func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }
	var rule eidos.Rule
	assert.MaxAllocs(t, func() { rule = eidos.OnGraph(onGraph) }, triggerAllocs,
		"OnGraph allocates the rule's leaf and its invocation")
	assert.Length(t, subscriptionsOf(t, rule), 1, "OnGraph returns one rule")
	assert.MaxAllocs(t, func() { rule = eidos.OnEmit(symbol.KindStruct, onEmit) }, triggerAllocs,
		"OnEmit allocates the rule's leaf and its invocation")
	assert.Length(t, subscriptionsOf(t, rule), 1, "OnEmit returns one rule")
	onPackage := func(*eidos.PackageMatch, *eidos.Emitter) error { return nil }
	assert.MaxAllocs(t, func() { rule = eidos.OnPackage(onPackage) }, triggerAllocs,
		"OnPackage allocates the rule's leaf and its invocation")
	assert.Length(t, subscriptionsOf(t, rule), 1, "OnPackage returns one rule")
}

// BenchmarkTrigger measures the two triggers' constructors, a graph
// phase call reporting one finding through each reporting method of a
// graph match, and an emit phase call reading each value's origin.
func BenchmarkTrigger(b *testing.B) {
	benchPhases(b, triggerCases(b))

	b.Run("OnGraph", func(b *testing.B) {
		handler := func(*eidos.GraphMatch, *eidos.Emitter) error { return nil }
		c := bench.Start(b).MaxAllocs(triggerAllocs)
		defer c.End()
		var rule eidos.Rule
		for c.Loop() {
			rule = eidos.OnGraph(handler)
		}
		assert.Length(b, subscriptionsOf(b, rule), 1, "OnGraph returns one rule")
	})

	b.Run("OnEmit", func(b *testing.B) {
		handler := func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }
		c := bench.Start(b).MaxAllocs(triggerAllocs)
		defer c.End()
		var rule eidos.Rule
		for c.Loop() {
			rule = eidos.OnEmit(symbol.KindStruct, handler)
		}
		assert.Length(b, subscriptionsOf(b, rule), 1, "OnEmit returns one rule")
	})

	b.Run("OnPackage", func(b *testing.B) {
		handler := func(*eidos.PackageMatch, *eidos.Emitter) error { return nil }
		c := bench.Start(b).MaxAllocs(triggerAllocs)
		defer c.End()
		var rule eidos.Rule
		for c.Loop() {
			rule = eidos.OnPackage(handler)
		}
		assert.Length(b, subscriptionsOf(b, rule), 1, "OnPackage returns one rule")
	})
}

// triggerCases returns a graph phase call over the invocation fixture
// for each reporting method of a graph match, and an emit phase call
// that reads each value's origin.
func triggerCases(tb assert.TB) []phaseCase {
	tb.Helper()

	fresh, _, _ := invocationContexts(tb, false)
	seeded, _, structs := invocationContexts(tb, true)
	at := position.Pos{File: "chosen.go", Line: 1, Col: 1}
	reporting := func(report func(*eidos.GraphMatch)) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Handle(eidos.OnGraph(func(m *eidos.GraphMatch, _ *eidos.Emitter) error {
				report(m)
				return nil
			})).
			Generator())
	}
	one := func(tb assert.TB, ctx *plugin.GeneratorContext) {
		assert.Length(tb, slices.Collect(ctx.Sink.All()), 1, "the graph invocation reports one finding")
	}
	origins := map[symbol.Identity]bool{}
	for _, s := range structs {
		origins[s.ID] = true
	}
	// visited is the package of the last package match, which the check of
	// the package case reads after the count.
	var visited symbol.Identity
	return []phaseCase{
		{
			name: "GraphMatch.ErrorfAt", allocs: graphReportAllocs, fresh: fresh, check: one,
			gen: reporting(func(m *eidos.GraphMatch) { m.ErrorfAt(graphReported, at, "the graph is refused") }),
		},
		{
			name: "GraphMatch.WarnfAt", allocs: graphReportAllocs, fresh: fresh, check: one,
			gen: reporting(func(m *eidos.GraphMatch) { m.WarnfAt(graphReported, at, "the graph is noted") }),
		},
		{
			name: "GraphMatch.InfofAt", allocs: graphReportAllocs, fresh: fresh, check: one,
			gen: reporting(func(m *eidos.GraphMatch) { m.InfofAt(graphReported, at, "the graph phase ran") }),
		},
		{
			name: "EmitMatch.Origin", allocs: orderAllocs, fresh: seeded,
			gen: generatorOf(tb, eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, _ *eidos.Emitter) error {
					assert.True(tb, origins[m.Origin()], "Origin returns an identity the fixture declares")
					return nil
				})).
				Generator()),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.False(tb, ctx.Sink.Failed(), "the emit call reports nothing")
			},
		},
		{
			name: "PackageMatch.Package", allocs: readerAllocs, fresh: fresh,
			gen: generatorOf(tb, eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnPackage(func(m *eidos.PackageMatch, _ *eidos.Emitter) error {
					visited = m.Package.ID
					return nil
				})).
				Generator()),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.Equal(tb, visited, coretest.PackageID(coretest.StorePath),
					"the match has the package of the fixture")
				assert.False(tb, ctx.Sink.Failed(), "the package call reports nothing")
			},
		},
	}
}

// graphReported is the code the graph reporting cases carry.
var graphReported = diag.MustRegister(diag.Prefix("TRIGGERTEST"), diag.CodeSpec{
	Number:  1,
	Meaning: "a fixture finding a graph reporting case asserts over",
})

// graphRun dispatches report from one graph rule and returns the
// context it reported into.
func graphRun(
	tb assert.TB, report func(m *eidos.GraphMatch),
) *plugin.GeneratorContext {
	tb.Helper()

	g, _, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)

	ran := false
	p := eidos.NewPlugin("grapher").
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
			ran = true
			report(m)
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	assert.True(tb, ran, "the graph handler ran")
	return ctx
}
