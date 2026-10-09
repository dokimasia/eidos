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

// The allocations of a phase call over the invocation fixture whose
// handler calls one method of the match once per invocation. A method
// without a constant allocates nothing per call.
const (
	// readerAllocs is the call's tracked reader, which the lane mints on
	// the call's first tracked read.
	readerAllocs = 1
	// bindAllocs is a call whose invocations each bind the rules: each
	// binding's memo, and once per call the tracked reader and the
	// match's resolver of other languages.
	bindAllocs = invocationStructs + 2
	// reportAllocs is a call whose invocations each report one finding:
	// each finding's message, and the sink's list of findings, which
	// grows eight times to contain 100.
	reportAllocs = invocationStructs + 8
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

		t.Run("returns a sibling's stamped value through a named handle", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"the sibling's fact stamps")

			var got, held bool
			p := eidos.NewPlugin("t").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					got, held = eidos.FactOf(m, alpha.ID, meta.Named[bool](key.Name()))
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.True(t, held, "the name resolves in the registry of the fact store")
			assert.True(t, got, "the value is the stamped one")
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

	t.Run("Export", func(t *testing.T) {
		t.Parallel()

		exportsSeen := func(tb assert.TB, exports map[string]plugin.ExportDoc, plan string) (plugin.ExportDoc, bool) {
			tb.Helper()

			g, _, _ := fixtureGraph(tb)
			_, facts := boolKey(tb)
			ctx := genContext(tb, g, facts, nil)
			ctx.Exports = exports
			var got plugin.ExportDoc
			var held bool
			p := eidos.NewPlugin("t").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					got, held = m.Export(plan)
					return nil
				})).
				Build()
			assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
			return got, held
		}

		t.Run("returns the export of a plan the generator's plan depends on", func(t *testing.T) {
			t.Parallel()

			exports := map[string]plugin.ExportDoc{exportingPlan: {Plan: exportingPlan}}
			got, held := exportsSeen(t, exports, exportingPlan)
			assert.True(t, held, "the export is found")
			assert.Equal(t, got.Plan, exportingPlan, "the export is the plan's")
		})

		t.Run("returns false for a plan the generator's plan does not depend on", func(t *testing.T) {
			t.Parallel()

			_, held := exportsSeen(t, map[string]plugin.ExportDoc{exportingPlan: {Plan: exportingPlan}}, "other")
			assert.False(t, held, "no export is found")
		})

		t.Run("returns false in an annotator's phase call", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")
			held, ran := true, false
			p := eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Stamper) error {
					ran = true
					_, held = m.Export(exportingPlan)
					return nil
				})).
				Build()
			assert.NoError(t, annotatorOf(t, p).Annotate(annContext(t, facts, ix)), "the phase call passes")
			assert.True(t, ran, "the handler ran")
			assert.False(t, held, "an annotator reads no export")
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
			assert.NotContains(t, coretest.Codes(ctx.Sink), rules.AbsentRules,
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
			other := func(c diag.Code) bool { return c != rules.AbsentRules }
			warned := slices.DeleteFunc(coretest.Codes(ctx.Sink), other)
			assert.Length(t, warned, 2, "the two named languages warn once each, and the zero language never")
		})
	})
}

// Each method of a match allocates nothing per invocation beyond what
// it returns or reports, in the ordinary run, which runs no benchmark.
// The check runs alone, because the count includes every goroutine's
// allocations.
func TestMatchAllocs(t *testing.T) {
	checkPhaseAllocs(t, matchMethodCases(t))
}

// BenchmarkMatch measures a phase call over the invocation fixture for
// each method of a match and each fact read, called once per
// invocation.
func BenchmarkMatch(b *testing.B) {
	benchPhases(b, matchMethodCases(b))
}

// matchMethodCases returns a phase call over the invocation fixture for
// each method of a match and each fact read, called once per
// invocation. Each handler checks what the method returned on the
// invocation, so a call that measured another path fails.
func matchMethodCases(tb assert.TB) []phaseCase {
	tb.Helper()

	fresh, key, structs := invocationContexts(tb, false)
	sibling := structs[0].ID
	at := position.Pos{File: "carrier.go", Line: 1, Col: 1}
	visiting := func(h func(*eidos.StructMatch) bool) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
				assert.True(tb, h(m), "the method returns the fixture's value on every invocation")
				return nil
			})).
			Build())
	}
	quiet := func(assert.TB, *plugin.GeneratorContext) {}
	findings := func(tb assert.TB, ctx *plugin.GeneratorContext) {
		assert.Length(tb, slices.Collect(ctx.Sink.All()), invocationStructs, "each invocation reports one finding")
	}
	return []phaseCase{
		{name: "Fact", fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			flagged, _ := eidos.Fact(m, key)
			return flagged
		})},
		{name: "FactOf", fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			flagged, _ := eidos.FactOf(m, sibling, key)
			return flagged
		})},
		{
			name:   "Reader",
			allocs: readerAllocs,
			fresh:  fresh,
			check:  quiet,
			gen: visiting(func(m *eidos.StructMatch) bool {
				return m.Reader() != nil
			}),
		},
		{name: "Lang", fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			return m.Lang() == coretest.Lang
		})},
		{name: "Rules", allocs: bindAllocs, fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			return m.Rules().Lang() == coretest.Lang
		})},
		{
			name:   "RulesFor",
			allocs: bindAllocs,
			fresh:  fresh,
			check:  quiet,
			gen: visiting(func(m *eidos.StructMatch) bool {
				return m.RulesFor(coretest.Lang).Lang() == coretest.Lang
			}),
		},
		{name: "Directive", fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			return m.Directive() == nil
		})},
		{name: "Kernel", fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			return m.Kernel().IsZero()
		})},
		{name: "Export", fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			_, held := m.Export(exportingPlan)
			return held
		})},
		{
			name:   "Errorf",
			allocs: reportAllocs,
			fresh:  fresh,
			check:  findings,
			gen: visiting(func(m *eidos.StructMatch) bool {
				m.Errorf(reported, "the subject is refused")
				return true
			}),
		},
		{
			name:   "Warnf",
			allocs: reportAllocs,
			fresh:  fresh,
			check:  findings,
			gen: visiting(func(m *eidos.StructMatch) bool {
				m.Warnf(reported, "the subject is noted")
				return true
			}),
		},
		{
			name:   "Infof",
			allocs: reportAllocs,
			fresh:  fresh,
			check:  findings,
			gen: visiting(func(m *eidos.StructMatch) bool {
				m.Infof(reported, "the subject was visited")
				return true
			}),
		},
		{
			name:   "ErrorfAt",
			allocs: reportAllocs,
			fresh:  fresh,
			check:  findings,
			gen: visiting(func(m *eidos.StructMatch) bool {
				m.ErrorfAt(reported, at, "the carrier is refused")
				return true
			}),
		},
	}
}

// reportingPlugin is who the reporting cases report as.
const reportingPlugin = "reporter"

// exportingPlan is the plan whose export the export cases hand a
// generator.
const exportingPlan = "stubs"

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
