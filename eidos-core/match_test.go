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
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/symbol"
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
					assert.False(t, held, "a graph match has no subject fact")
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

		t.Run("returns a sibling's stamped value", func(t *testing.T) {
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
					assert.True(t, held, "the stamped fact is found")
					assert.True(t, got, "the value is the stamped one")
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
			assert.Equal(t, one.Severity, diag.SeverityError, "the finding is an Error")
			assert.Equal(t, one.Pos, alpha.Pos, "the position is the subject's")
			assert.Equal(t, string(one.Origin), reportingPlugin, "the origin is the reporting plugin")
			assert.Contains(t, one.Msg, alpha.Name, "the message is the handler's formatting")
		})
	})

	t.Run("ErrorfAt", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at the given position", func(t *testing.T) {
			t.Parallel()

			carrier := position.Pos{File: "svc/store/row.go", Line: 41, Col: 3}
			ctx, _ := reportingRun(t, func(m *eidos.StructMatch) {
				m.ErrorfAt(reported, carrier, "the carrier on %s is refused", m.Struct.Name)
			})

			coretest.AssertCodes(t, ctx.Sink, reported)
			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityError, "the finding is an Error")
			assert.Equal(t, one.Pos, carrier, "the position is the given one")
			assert.Equal(t, string(one.Origin), reportingPlugin, "the origin is the reporting plugin")
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
			assert.Equal(t, one.Severity, diag.SeverityInfo, "the finding is at Info severity")
			assert.Equal(t, one.Pos, alpha.Pos, "the position is the subject's")
			assert.False(t, ctx.Sink.Failed(), "the run passes")
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		langs := func(tb assert.TB) (onStruct, onGraph symbol.Lang) {
			tb.Helper()

			g, _, _ := fixtureGraph(tb)
			_, facts := boolKey(tb)
			onStruct, onGraph = "unset", "unset"
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					onStruct = m.Lang()
					return nil
				})).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					onGraph = m.Lang()
					return nil
				})).
				Build()
			assert.NoError(tb, generatorOf(tb, p).Generate(genContext(tb, g, facts, nil)), "the phase call passes")
			return onStruct, onGraph
		}

		t.Run("returns the subject's language", func(t *testing.T) {
			t.Parallel()

			onStruct, _ := langs(t)
			assert.Equal(t, onStruct, coretest.Lang, "the language is the subject's")
		})

		t.Run("returns the zero language on a graph match", func(t *testing.T) {
			t.Parallel()

			_, onGraph := langs(t)
			assert.Equal(t, onGraph, symbol.Lang(""), "the language is zero")
		})
	})

	t.Run("Rules", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the registered rules over the invocation's view", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Rules = rules.NewRegistry()
			assert.NoError(t, ctx.Rules.Register(fixtureLanguage{rulestest.Scripted()}),
				"the fixture language registers")
			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					ran = true
					b := m.Rules()
					assert.Equal(t, b.Source().Lang(), coretest.Lang, "the subject's language binds")
					ref := &node.TypeRef{Spelling: "Beta", Target: beta.ID}
					assert.Equal(t, b.TypeOf(ref).Form, symbol.FormReference, "a reference folds over the view")
					assert.Equal(t, b.TypeOf(&node.TypeRef{Spelling: "int"}).Form, symbol.FormScalar,
						"a builtin folds through the language")
					again := m.Rules()
					assert.Equal(t, again.Source().Lang(), b.Source().Lang(), "the second call returns the binding")
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.True(t, ran, "the handler ran")
			assert.False(t, slices.Contains(coretest.Codes(ctx.Sink), rules.AbsentRules),
				"a registered language reports no warning")
		})

		absentRun := func(tb assert.TB) *plugin.GeneratorContext {
			tb.Helper()

			g, _, _ := fixtureGraph(tb)
			_, facts := boolKey(tb)
			ctx := genContext(tb, g, facts, nil)
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					assert.True(tb, rules.IsAbsent(m.Rules().Source()), "the subject's language binds the absent rules")
					m.Rules()
					assert.True(tb, rules.IsAbsent(m.RulesFor("proto").Source()), "another language binds them too")
					m.RulesFor("proto")
					return nil
				})).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					assert.True(tb, rules.IsAbsent(m.Rules().Source()), "a graph match binds the absent rules")
					return nil
				})).
				Build()
			assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
			return ctx
		}

		t.Run("binds the absent rules without a registry", func(t *testing.T) {
			t.Parallel()

			absentRun(t)
		})

		t.Run("warns once per language without registered rules", func(t *testing.T) {
			t.Parallel()

			ctx := absentRun(t)
			var warned int
			for _, c := range coretest.Codes(ctx.Sink) {
				if c == rules.AbsentRules {
					warned++
				}
			}
			assert.Equal(t, warned, 2, "the two named languages warn once each, and the zero language never")
		})
	})
}

// reportingPlugin is who the reporting cases report as.
const reportingPlugin = "reporter"

// reported is the code the reporting cases report under. One code
// is enough: the cases are about severity, position and origin, not
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

// onlyFinding returns the one finding a sink contains, failing
// unless exactly one arrived.
func onlyFinding(tb assert.TB, s *diag.Sink) diag.Diag {
	tb.Helper()

	found := slices.Collect(s.All())
	assert.Length(tb, found, 1, "the case reported exactly one finding")
	if len(found) != 1 {
		return diag.Diag{}
	}
	return found[0]
}

// fixtureLanguage is the scripted rules registered under the
// fixture's language.
type fixtureLanguage struct{ rules.SourceRules }

// Lang returns the fixture's language.
func (fixtureLanguage) Lang() symbol.Lang { return coretest.Lang }
