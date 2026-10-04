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
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
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
						assert.True(t, m.Value == symbol.Symbol(value),
							"the match has the very emit value")
						assert.Equal(t, m.Origin(), alpha.ID,
							"and the node identity it derives from")
						assert.Nil(t, m.Directive(),
							"a bare match has no gating instance")
						return nil
					})).
				Build()
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

			p := eidos.NewPlugin("t").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						t.Error("a value without an origin is not a subject")
						return nil
					})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
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
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.True(t, ran, "the handler ran")
		})
	})

	t.Run("GraphMatch.Warnf", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a Warning at the position the handler names", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: "chosen.go", Line: 11, Col: 2}
			ctx := graphRun(t, func(m *eidos.GraphMatch) {
				m.Warnf(graphReported, at, "the graph carries %d packages", 1)
			})

			coretest.AssertCodes(t, ctx.Sink, graphReported)
			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityWarning,
				"Warnf reports at Warning severity, which never fails a run")
			assert.Equal(t, one.Pos, at,
				"a graph match has no subject, so the handler's position stands")
			assert.False(t, ctx.Sink.Failed(),
				"a Warning leaves the run passing")
			assert.Contains(t, one.Msg, "1 packages",
				"the handler's own formatting arrives verbatim")
		})
	})

	t.Run("GraphMatch.Infof", func(t *testing.T) {
		t.Parallel()

		t.Run("reports an Info finding at the position the handler names", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: "chosen.go", Line: 3, Col: 4}
			ctx := graphRun(t, func(m *eidos.GraphMatch) {
				m.Infof(graphReported, at, "the graph phase ran")
			})

			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityInfo, "Infof reports at Info severity")
			assert.Equal(t, one.Pos, at, "at the position the handler named")
			assert.False(t, ctx.Sink.Failed(),
				"an Info finding leaves the run passing")
		})
	})

	t.Run("GraphMatch.Errorf", func(t *testing.T) {
		t.Parallel()

		t.Run("reports an Error at the position the handler names", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: "chosen.go", Line: 5, Col: 6}
			ctx := graphRun(t, func(m *eidos.GraphMatch) {
				m.Errorf(graphReported, at, "the graph is refused")
			})

			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityError, "Errorf reports at Error severity")
			assert.Equal(t, one.Pos, at, "at the position the handler named")
			assert.True(t, ctx.Sink.Failed(), "an Error fails the run")
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
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, origin, alpha.ID, "the origin is the node the value derives from")
		})
	})
}

// A trigger allocates its rule, and its matches' methods allocate what
// they report, in the ordinary run, which runs no benchmark. The check
// runs alone, because AllocsPerRun counts every goroutine's allocations
// and refuses to run beside parallel tests.
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
			Build())
	}
	one := func(tb assert.TB, ctx *plugin.GeneratorContext) {
		assert.Length(tb, slices.Collect(ctx.Sink.All()), 1, "the graph invocation reports one finding")
	}
	origins := map[symbol.Identity]bool{}
	for _, s := range structs {
		origins[s.ID] = true
	}
	return []phaseCase{
		{
			name: "GraphMatch.Errorf", allocs: graphReportAllocs, fresh: fresh, check: one,
			gen: reporting(func(m *eidos.GraphMatch) { m.Errorf(graphReported, at, "the graph is refused") }),
		},
		{
			name: "GraphMatch.Warnf", allocs: graphReportAllocs, fresh: fresh, check: one,
			gen: reporting(func(m *eidos.GraphMatch) { m.Warnf(graphReported, at, "the graph is noted") }),
		},
		{
			name: "GraphMatch.Infof", allocs: graphReportAllocs, fresh: fresh, check: one,
			gen: reporting(func(m *eidos.GraphMatch) { m.Infof(graphReported, at, "the graph phase ran") }),
		},
		{
			name: "EmitMatch.Origin", allocs: orderAllocs, fresh: seeded,
			gen: generatorOf(tb, eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, _ *eidos.Emitter) error {
					if !origins[m.Origin()] {
						tb.Fatalf("Origin returned %s, which the fixture does not declare", m.Origin())
					}
					return nil
				})).
				Build()),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.False(tb, ctx.Sink.Failed(), "the emit call reports nothing")
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
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	assert.True(tb, ran, "the graph handler ran")
	return ctx
}
