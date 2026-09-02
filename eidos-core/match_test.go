// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
)

// The match's fact reads are the sanctioned channel between
// plugins, so what they return and what they refuse are contract.
func TestMatch(t *testing.T) {
	t.Parallel()

	t.Run("Fact", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false on a graph match", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					ran = true
					_, held := eidos.Fact(m, key)
					assert.False(t, held,
						"a graph match has no subject, so no subject fact returns")
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.True(t, ran, "the handler ran")
		})
	})

	t.Run("FactOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a sibling's winning value", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"the sibling's fact stamps")

			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					ran = true
					got, held := eidos.FactOf(m, alpha.ID, key)
					assert.True(t, held, "the stamped fact returns")
					assert.True(t, got, "with its winning value")
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.True(t, ran, "the handler ran")
		})
	})

	t.Run("Errorf", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at the subject's position", func(t *testing.T) {
			t.Parallel()

			ctx, alpha := reportingRun(t, func(m *eidos.StructMatch) {
				m.Errorf(reported, "the subject %s is refused", m.Struct.Name)
			})

			coretest.AssertCodes(t, ctx.Sink, reported)
			coretest.AssertPositioned(t, ctx.Sink)
			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityError,
				"Errorf reports at Error severity, which fails the run")
			assert.Equal(t, one.Pos, alpha.Pos,
				"the subject's position is pre-bound, so a handler names none")
			assert.Equal(t, string(one.Origin), reportingPlugin,
				"and the reporting plugin is pre-bound as the origin")
			assert.Contains(t, one.Msg, alpha.Name,
				"the handler's own formatting arrives verbatim")
		})
	})

	t.Run("Infof", func(t *testing.T) {
		t.Parallel()

		t.Run("reports without failing the run", func(t *testing.T) {
			t.Parallel()

			ctx, alpha := reportingRun(t, func(m *eidos.StructMatch) {
				m.Infof(reported, "the subject %s was visited", m.Struct.Name)
			})

			coretest.AssertCodes(t, ctx.Sink, reported)
			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityInfo,
				"Infof carries provenance, never a verdict")
			assert.Equal(t, one.Pos, alpha.Pos,
				"at the subject's pre-bound position")
			assert.False(t, ctx.Sink.Failed(),
				"an Info finding leaves the run passing")
		})
	})
}

// reportingPlugin is who the reporting cases report as.
const reportingPlugin = "reporter"

// reported is the code the reporting cases carry. One code is
// enough: the cases are about severity, position and origin, not
// about telling two findings apart.
var reported = diag.MustRegister(diag.Prefix("MATCHTEST"), diag.CodeSpec{
	Number:  1,
	Meaning: "a fixture finding a reporting case asserts over",
})

// reportingRun dispatches report over the fixture graph's first
// struct and returns the context it reported into, with that
// subject.
func reportingRun(
	tb assert.TB, report func(m *eidos.StructMatch),
) (*plugin.GeneratorContext, *node.Struct) {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	ctx.Plugin = reportingPlugin

	ran := false
	p := eidos.NewPlugin(reportingPlugin).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if m.Struct.Identity() != alpha.ID {
				return nil
			}
			ran = true
			report(m)
			return nil
		})).
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	assert.True(tb, ran, "the reporting handler ran")
	return ctx, alpha
}

// onlyFinding returns the one finding a sink holds, failing unless
// exactly one arrived.
func onlyFinding(tb assert.TB, s *diag.Sink) diag.Diag {
	tb.Helper()

	held := slices.Collect(s.All())
	assert.Length(tb, held, 1, "the case reported exactly one finding")
	if len(held) != 1 {
		return diag.Diag{}
	}
	return held[0]
}
