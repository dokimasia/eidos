// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The structural triggers position their handlers: an emit match
// names the value and its origin, and neither trigger carries a
// gating instance it was not given.
func TestTrigger(t *testing.T) {
	t.Parallel()

	t.Run("OnEmit", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the value and its origin", func(t *testing.T) {
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
							"the match carries the very emit value")
						assert.Equal(t, m.Origin(), alpha.ID,
							"and the node identity it derives from")
						assert.Nil(t, m.Directive(),
							"a bare match carries no gating instance")
						return nil
					})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.True(t, ran, "the handler ran")
		})

		t.Run("never yields a value without an origin", func(t *testing.T) {
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

		t.Run("carries no gating instance", func(t *testing.T) {
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

		t.Run("reports at the position the handler names", func(t *testing.T) {
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

		t.Run("carries provenance without a verdict", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: "chosen.go", Line: 3, Col: 4}
			ctx := graphRun(t, func(m *eidos.GraphMatch) {
				m.Infof(graphReported, at, "the graph phase ran")
			})

			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityInfo,
				"Infof carries provenance, never a verdict")
			assert.Equal(t, one.Pos, at, "at the position the handler named")
			assert.False(t, ctx.Sink.Failed(),
				"an Info finding leaves the run passing")
		})
	})
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
