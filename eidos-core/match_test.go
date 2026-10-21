// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
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

// The spellings of the directive cases: the bare name of a directive of
// the fixture plugin, its canonical spelling, and the canonical spelling of
// the directive of the same name of another plugin.
const (
	excuseName      directive.Name = "excuse"
	excuseSpelling  directive.Name = directive.Name(contextPlugin) + ":excuse"
	foreignSpelling directive.Name = "other:excuse"
)

// absentKey is the name of a key that the fixture registry does not
// contain.
const absentKey meta.KeyName = "t.absent"

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
				Generator()
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
				Generator()
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
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.True(t, held, "the name resolves in the registry of the fact store")
			assert.True(t, got, "the value is the stamped one")
		})
	})

	t.Run("Subjects", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the subjects on which the key reads present in identity order", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			for _, id := range []symbol.Identity{beta.ID, alpha.ID} {
				assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: id}), "the fact stamps")
			}
			got := subjectsSeen(t, genContext(t, g, facts, nil), key)
			assert.Equal(t, got, []symbol.Identity{alpha.ID, beta.ID}, "the two subjects in identity order")
		})

		t.Run("returns the subjects through a named handle", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "the fact stamps")
			got := subjectsSeen(t, genContext(t, g, facts, nil), meta.Named[bool](key.Name()))
			assert.Equal(t, got, []symbol.Identity{alpha.ID}, "the name resolves in the registry of the fact store")
		})

		t.Run("returns the subjects through a named handle of another value type", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "the fact stamps")
			got := subjectsSeen(t, genContext(t, g, facts, nil), meta.Named[string](key.Name()))
			assert.Equal(t, got, []symbol.Identity{alpha.ID}, "the presence of a key does not depend on its value type")
		})

		t.Run("returns no subject for a name that the registry does not contain", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "the fact stamps")
			got := subjectsSeen(t, genContext(t, g, facts, nil), meta.Named[bool](absentKey))
			assert.Empty(t, got, "a key that nothing registered has no subject")
		})

		t.Run("returns no subject for the zero key", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "the fact stamps")
			got := subjectsSeen(t, genContext(t, g, facts, nil), meta.Key[bool]{})
			assert.Empty(t, got, "the zero key names no key")
		})

		t.Run("returns no subject outside the scope of the index", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "the fact stamps")
			ctx := genContext(t, g, facts, nil)
			ix, err := plugin.NewIndex(g, facts, nil, func(symbol.Identity) bool { return false })
			assert.NoError(t, err, "the routing surface builds")
			ctx.Index = ix
			assert.Empty(t, subjectsSeen(t, ctx, key), "the scope admits no package")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			for _, id := range []symbol.Identity{alpha.ID, beta.ID} {
				assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: id}), "the fact stamps")
			}
			seen := 0
			p := eidos.NewPlugin("t").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					for range eidos.Subjects(m, key) {
						seen++
						break
					}
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)), "the phase call passes")
			assert.Equal(t, seen, 1, "the iteration stops when the range stops")
		})

		t.Run("records the membership edge on the key", func(t *testing.T) {
			t.Parallel()

			rec := keysRecorded(t, func(k meta.Key[bool]) meta.Key[bool] { return k })
			for _, j := range rec.invoked {
				assert.Equal(t, j.keys, []meta.KeyName{"t.flag"}, "each invocation read the membership of the key")
				assert.Empty(t, j.facts, "the enumeration reads no value")
			}
		})

		t.Run("records the edge of a name that the registry does not contain", func(t *testing.T) {
			t.Parallel()

			rec := keysRecorded(t, func(meta.Key[bool]) meta.Key[bool] { return meta.Named[bool](absentKey) })
			for _, j := range rec.invoked {
				assert.Equal(t, j.keys, []meta.KeyName{absentKey}, "the edge records the name")
			}
		})

		t.Run("records nothing for the zero key", func(t *testing.T) {
			t.Parallel()

			rec := keysRecorded(t, func(meta.Key[bool]) meta.Key[bool] { return meta.Key[bool]{} })
			for _, j := range rec.invoked {
				assert.False(t, j.read, "the invocation read nothing")
			}
		})
	})

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the instances of a directive of the plugin in source order", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, directivesSeen(t, excuseName), []int{0, 1},
				"the two instances of the directive of the plugin, in source order")
		})

		t.Run("returns the instances for the canonical spelling of the directive", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, directivesSeen(t, excuseSpelling), []int{0, 1},
				"the canonical spelling addresses the directive of the plugin")
		})

		t.Run("returns no instance of a directive of another plugin", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, directivesSeen(t, foreignSpelling), "another plugin's directive is invisible")
		})

		t.Run("returns no instance of a kernel directive", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, directivesSeen(t, directive.KernelMeta), "a kernel directive is not the plugin's")
		})

		t.Run("returns no instance on a graph match", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			validated := plugin.ValidatedMap{alpha.ID: {{Name: excuseSpelling}}}
			var got []*directive.Directive
			ran := false
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					ran = true
					got = slices.Collect(m.Directives(excuseName))
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, validated)), "the phase call passes")
			assert.True(t, ran, "the handler ran")
			assert.Empty(t, got, "a graph match has no subject")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			validated := plugin.ValidatedMap{alpha.ID: {{Name: excuseSpelling}, {Name: excuseSpelling, Instance: 1}}}
			seen := 0
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					for range m.Directives(excuseName) {
						seen++
						break
					}
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, validated)), "the phase call passes")
			assert.Equal(t, seen, 1, "the iteration stops when the range stops")
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

	t.Run("WarnfAt", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a Warning at the given position", func(t *testing.T) {
			t.Parallel()

			carrier := position.Pos{File: "svc/store/row.go", Line: 42, Col: 3}
			ctx, _ := reportingRun(t, func(m *eidos.StructMatch) {
				m.WarnfAt(reported, carrier, "the carrier on %s is noted", m.Struct.Name)
			})

			coretest.AssertCodes(t, ctx.Sink, reported)
			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityWarning, "the finding is a Warning")
			assert.Equal(t, one.Pos, carrier, "the position is the given one")
			assert.False(t, ctx.Sink.Failed(), "the run passes")
		})
	})

	t.Run("InfofAt", func(t *testing.T) {
		t.Parallel()

		t.Run("reports an Info finding at the given position", func(t *testing.T) {
			t.Parallel()

			carrier := position.Pos{File: "svc/store/row.go", Line: 43, Col: 3}
			ctx, _ := reportingRun(t, func(m *eidos.StructMatch) {
				m.InfofAt(reported, carrier, "the carrier on %s was visited", m.Struct.Name)
			})

			coretest.AssertCodes(t, ctx.Sink, reported)
			one := onlyFinding(t, ctx.Sink)
			assert.Equal(t, one.Severity, diag.SeverityInfo, "the finding is at Info severity")
			assert.Equal(t, one.Pos, carrier, "the position is the given one")
			assert.False(t, ctx.Sink.Failed(), "the run passes")
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

	t.Run("SubjectMatch", func(t *testing.T) {
		t.Parallel()

		t.Run("lets one function report at the subjects of two kinds", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			alpha.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
			store := coretest.Interface(coretest.StorePath, "Store")
			store.Pos = position.Pos{File: "store.go", Line: 9, Col: 1}
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, alpha, store))
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			refuse := func(m eidos.SubjectMatch) error {
				m.Errorf(reported, "the subject in %s is refused", m.Lang())
				return nil
			}
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error { return refuse(m) })).
				Handle(eidos.OnInterface(func(m *eidos.InterfaceMatch, _ *eidos.Emitter) error { return refuse(m) })).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			var at []position.Pos
			for d := range ctx.Sink.All() {
				at = append(at, d.Pos)
			}
			assert.Permutation(t, at, []position.Pos{alpha.Pos, store.Pos},
				"the function reports at the subject of each match that it receives")
		})

		t.Run("is not satisfied by a graph match", func(t *testing.T) {
			t.Parallel()

			var m any = &eidos.GraphMatch{}
			_, satisfied := m.(eidos.SubjectMatch)
			assert.False(t, satisfied, "a graph match has no subject")
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
				Generator()
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
				Generator()
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
				Annotator()
			assert.NoError(t, p.Annotate(annContext(t, facts, ix)), "the phase call passes")
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
				Generator()
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
				Generator()
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
	directed := directiveContexts(tb)
	sibling := structs[0].ID
	at := position.Pos{File: "carrier.go", Line: 1, Col: 1}
	visiting := func(h func(*eidos.StructMatch) bool) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
				assert.True(tb, h(m), "the method returns the fixture's value on every invocation")
				return nil
			})).
			Generator())
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
		{name: "Subjects", fresh: fresh, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			n := 0
			for range eidos.Subjects(m, key) {
				n++
			}
			return n == invocationStructs
		})},
		{name: "Directives", fresh: directed, check: quiet, gen: visiting(func(m *eidos.StructMatch) bool {
			n := 0
			for range m.Directives(excuseName) {
				n++
			}
			return n == 2
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
		{
			name:   "WarnfAt",
			allocs: reportAllocs,
			fresh:  fresh,
			check:  findings,
			gen: visiting(func(m *eidos.StructMatch) bool {
				m.WarnfAt(reported, at, "the carrier is noted")
				return true
			}),
		},
		{
			name:   "InfofAt",
			allocs: reportAllocs,
			fresh:  fresh,
			check:  findings,
			gen: visiting(func(m *eidos.StructMatch) bool {
				m.InfofAt(reported, at, "the carrier was visited")
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
		Generator()
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

// subjectsSeen runs a graph rule over ctx that collects the subjects of k,
// and returns them.
func subjectsSeen[T meta.FactValue](tb assert.TB, ctx *plugin.GeneratorContext, k meta.Key[T]) []symbol.Identity {
	tb.Helper()

	var got []symbol.Identity
	ran := false
	p := eidos.NewPlugin("t").
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
			ran = true
			got = slices.Collect(eidos.Subjects(m, k))
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	assert.True(tb, ran, "the handler ran")
	return got
}

// keysRecorded runs a struct rule over the fixture graph with a journal. The
// handler ranges over the subjects of the key that handle returns for the
// fixture key. keysRecorded returns the journal.
func keysRecorded(tb assert.TB, handle func(meta.Key[bool]) meta.Key[bool]) *recorder {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	key, facts := boolKey(tb)
	assert.NoError(tb, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "the fact stamps")
	ctx := genContext(tb, g, facts, nil)
	rec := &recorder{}
	ctx.Journal = rec
	k := handle(key)
	p := eidos.NewPlugin("t").
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			for range eidos.Subjects(m, k) { // ranging is what records the edge
			}
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	assert.Length(tb, rec.invoked, 2, "the journal records the invocation of each struct")
	return rec
}

// directivesSeen runs a struct rule over the fixture graph. Its first struct
// has two instances of the directive of the fixture plugin, an instance of
// the directive of the same name of another plugin, and an instance of a
// kernel directive. directivesSeen returns the instance numbers that
// Directives yields for name on that struct.
func directivesSeen(tb assert.TB, name directive.Name) []int {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	validated := plugin.ValidatedMap{alpha.ID: {
		{Name: excuseSpelling},
		{Name: foreignSpelling},
		{Name: directive.KernelMeta},
		{Name: excuseSpelling, Instance: 1},
	}}
	var got []int
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if m.Struct.Identity() != alpha.ID {
				return nil
			}
			for d := range m.Directives(name) {
				got = append(got, d.Instance)
			}
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(genContext(tb, g, facts, validated)), "the phase call passes")
	return got
}

// directiveContexts returns contexts over the invocation fixture whose
// validated table gives each struct two instances of the directive of the
// fixture plugin and one instance of the directive of another plugin.
func directiveContexts(tb assert.TB) func() *plugin.GeneratorContext {
	tb.Helper()

	g, structs := invocationGraph(tb)
	_, facts := boolKey(tb)
	validated := plugin.ValidatedMap{}
	for _, s := range structs {
		validated[s.ID] = []directive.Directive{
			{Name: excuseSpelling}, {Name: foreignSpelling}, {Name: excuseSpelling, Instance: 1},
		}
	}
	ix, err := plugin.NewIndex(g, facts, validated, nil)
	assert.NoError(tb, err, "the routing surface builds")
	return func() *plugin.GeneratorContext {
		return &plugin.GeneratorContext{
			Index: ix, Facts: facts, Emit: plugin.NewEmit(), Sink: diag.NewSink(), Plugin: contextPlugin, Turn: 2,
		}
	}
}

// fixtureLanguage is the scripted rules registered under the
// fixture's language.
type fixtureLanguage struct{ rules.SourceRules }

// Lang returns the fixture's language.
func (fixtureLanguage) Lang() symbol.Lang { return coretest.Lang }
