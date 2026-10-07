// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"fmt"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The journal methods a recorder lists, in the order a phase call made
// them.
const (
	invokedCall   = "Invoked"
	evaluatedCall = "Evaluated"
)

// The plans whose exports the export case hands a generator, and a
// plan it reads that the generator's plan does not depend on.
const (
	basePlan   = "base"
	corePlan   = "core"
	absentPlan = "absent"
)

// registryTag is the tag of the per-package family the unit case
// touches.
const registryTag = "reg"

// seededUnit is the reference of the unit seed adds.
var seededUnit = plugin.UnitRef{Plugin: "earlier", Key: "a.go"}

// journaled is one invocation a recorder received, with the edges of
// its read set listed in place of the set.
type journaled struct {
	plugin.Invocation
	// read reports whether the invocation handed a read set.
	read bool
	// identities and facts are the set's declaration and fact edges.
	identities []symbol.Identity
	facts      []meta.FactRef
}

// evaluation is one candidate's matches a recorder received.
type evaluation struct {
	subject symbol.Identity
	matches []plugin.MatchKey
}

// recorder is a journal that keeps what a phase call hands it. It copies
// each invocation's slices and lists its read set while Invoked runs,
// because both are the call's own only until Invoked returns.
type recorder struct {
	invoked   []journaled
	evaluated []evaluation
	// calls lists the journal's methods in the order the call made them.
	calls []string
}

// Invoked keeps one invocation, with its slices copied and its read
// set's edges listed.
func (r *recorder) Invoked(inv plugin.Invocation) {
	j := journaled{Invocation: inv, read: inv.Reads != nil}
	j.Exports = slices.Clone(inv.Exports)
	j.Units = slices.Clone(inv.Units)
	j.Hosts = slices.Clone(inv.Hosts)
	j.Claimed = slices.Clone(inv.Claimed)
	j.Findings = slices.Clone(inv.Findings)
	if inv.Reads != nil {
		j.identities = slices.Collect(inv.Reads.Identities())
		for id, key := range inv.Reads.Facts() {
			j.facts = append(j.facts, meta.FactRef{Subject: id, Key: key})
		}
	}
	j.Reads = nil
	r.invoked = append(r.invoked, j)
	r.calls = append(r.calls, invokedCall)
}

// Evaluated keeps one candidate's matches.
func (r *recorder) Evaluated(subject symbol.Identity, matches []plugin.MatchKey) {
	r.evaluated = append(r.evaluated, evaluation{subject: subject, matches: slices.Clone(matches)})
	r.calls = append(r.calls, evaluatedCall)
}

// keys returns the match key of every invocation the recorder received,
// in the order it received them.
func (r *recorder) keys() []plugin.MatchKey {
	out := make([]plugin.MatchKey, len(r.invoked))
	for i, j := range r.invoked {
		out[i] = j.Match
	}
	return out
}

// lines returns one line for each invocation the recorder received: its
// key, the edges it read, what it touched and the messages it reported.
func (r *recorder) lines() []string {
	out := make([]string, len(r.invoked))
	for i, j := range r.invoked {
		out[i] = fmt.Sprintf("%v %v %v %v %v %v %v %v",
			j.Match, j.identities, j.facts, j.Exports, j.Units, j.Hosts, j.Claimed, findingMessages(j.Findings))
	}
	return out
}

// tally is a journal that counts the invocations it receives, the
// least a consumer of the journal does.
type tally struct{ invoked int }

// Invoked counts one invocation.
func (t *tally) Invoked(plugin.Invocation) { t.invoked++ }

// Evaluated counts nothing.
func (*tally) Evaluated(symbol.Identity, []plugin.MatchKey) {}

// The journal is what a warm run keys re-execution on: one record for
// each invocation in canonical match order, with what the invocation
// read and touched, and each candidate's matches.
func TestJournal(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("records one invocation for each match in canonical match order", func(t *testing.T) {
			t.Parallel()

			g, structs := manySubjects(t, manyStructs)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Build()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			want := make([]plugin.MatchKey, len(structs))
			for i, s := range structs {
				want[i] = plugin.MatchKey{Plugin: contextPlugin, Subject: s.ID}
			}
			assert.Equal(t, rec.keys(), want, "each subject's match is recorded once, in identity order")
		})

		t.Run("records the invocations in identity order where the store enumerates in another", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(beta), emitted(alpha))
			var origins []symbol.Identity
			for v := range ctx.Emit.ByKind(symbol.KindStruct) {
				origin, _ := emit.OriginOf(v)
				origins = append(origins, origin)
			}
			assert.Equal(t, origins, []symbol.Identity{beta.ID, alpha.ID}, "the store enumerates in the unit's order")
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })).
				Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.keys(), []plugin.MatchKey{
				{Plugin: contextPlugin, Subject: alpha.ID, Host: plugin.EmitRef{Unit: seededUnit, Index: 1}},
				{Plugin: contextPlugin, Subject: beta.ID, Host: plugin.EmitRef{Unit: seededUnit, Index: 0}},
			}, "the journal receives the records in identity order")
		})

		t.Run("records each rule's invocations after the rule before it", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(nothing[*eidos.Emitter]), eidos.OnStruct(nothing[*eidos.Emitter])).
				Build()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			assert.Equal(t, rec.keys(), []plugin.MatchKey{
				{Plugin: contextPlugin, Rule: 0, Subject: alpha.ID},
				{Plugin: contextPlugin, Rule: 0, Subject: beta.ID},
				{Plugin: contextPlugin, Rule: 1, Subject: alpha.ID},
				{Plugin: contextPlugin, Rule: 1, Subject: beta.ID},
			}, "the rule orders before the subject")
		})

		t.Run("records the gating instance of each directive-gated match", func(t *testing.T) {
			t.Parallel()

			g, alpha := gatedStruct(t, scheduled)
			_, facts := boolKey(t)
			schema := stubSchema(gateName)
			schema.Repeatable = true
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.Directive(schema, eidos.OnStruct(nothing[*eidos.Emitter]))).
				Build()
			rec := journaledGenerate(t, genContext(t, g, facts, repeated(alpha.ID, schema, scheduled)), p)
			assert.Equal(t, rec.keys(), []plugin.MatchKey{
				{Plugin: contextPlugin, Subject: alpha.ID, Instance: 0},
				{Plugin: contextPlugin, Subject: alpha.ID, Instance: 1},
				{Plugin: contextPlugin, Subject: alpha.ID, Instance: 2},
			}, "each instance's match is recorded with its instance")
		})

		t.Run("records the host of an emit-phase match", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha), emitted(beta))
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })).
				Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.keys(), []plugin.MatchKey{
				{Plugin: contextPlugin, Subject: alpha.ID, Host: plugin.EmitRef{Unit: seededUnit, Index: 0}},
				{Plugin: contextPlugin, Subject: beta.ID, Host: plugin.EmitRef{Unit: seededUnit, Index: 1}},
			}, "each value's match names its origin and its place")
		})

		t.Run("records an origin's emit-phase matches by instance before host", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			schema := stubSchema(gateName)
			schema.Repeatable = true
			ctx := genContext(t, g, facts, repeated(alpha.ID, schema, 2))
			seed(t, ctx, emitted(alpha), &emit.Struct{Origin: alpha.ID, Name: "GenAlphaToo"})
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.Directive(schema,
					eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }))).
				Build()
			rec := journaledGenerate(t, ctx, p)
			key := func(instance, place int) plugin.MatchKey {
				return plugin.MatchKey{
					Plugin: contextPlugin, Subject: alpha.ID, Instance: instance,
					Host: plugin.EmitRef{Unit: seededUnit, Index: place},
				}
			}
			assert.Equal(t, rec.keys(), []plugin.MatchKey{key(0, 0), key(0, 1), key(1, 0), key(1, 1)},
				"the store enumerates each value's instances, and the journal orders the instances first")
		})

		t.Run("records an origin's emit-phase matches in reference order of their hosts", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			earlier := plugin.Unit{
				Plugin: "earlier", Tag: "late", Per: plugin.PerSource, Word: "impl", Key: "a.go",
				Decls: []symbol.Symbol{emitted(alpha)},
			}
			later := plugin.Unit{
				Plugin: "earlier", Tag: "early", Per: plugin.PerSource, Word: "impl", Key: "b.go",
				Decls: []symbol.Symbol{&emit.Struct{Origin: alpha.ID, Name: "GenAlphaToo"}},
			}
			assert.NoError(t, ctx.Emit.Add(earlier), "the unit the store enumerates first is added")
			assert.NoError(t, ctx.Emit.Add(later), "the unit whose reference sorts first is added")
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })).
				Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.keys(), []plugin.MatchKey{
				{Plugin: contextPlugin, Subject: alpha.ID, Host: plugin.EmitRef{Unit: later.Ref()}},
				{Plugin: contextPlugin, Subject: alpha.ID, Host: plugin.EmitRef{Unit: earlier.Ref()}},
			}, "the tag orders the references before the key")
		})

		t.Run("records the declarations an invocation looked up", func(t *testing.T) {
			t.Parallel()

			rec, _, beta := lookupRun(t)
			assert.Equal(t, rec.invoked[0].identities, []symbol.Identity{beta.ID},
				"the first subject's record lists the sibling it looked up")
		})

		t.Run("records no read set for an invocation that read nothing", func(t *testing.T) {
			t.Parallel()

			rec, _, _ := lookupRun(t)
			assert.False(t, rec.invoked[1].read, "the second subject's handler read nothing")
		})

		t.Run("records each invocation's own reads", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					other := beta.ID
					if m.Struct.ID == beta.ID {
						other = alpha.ID
					}
					m.Reader().Lookup(other)
					return nil
				})).
				Build()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			assert.Length(t, rec.invoked, 2, "both subjects' invocations are recorded")
			expect.Equal(t, rec.invoked[0].identities, []symbol.Identity{beta.ID},
				"the first record lists its own read alone")
			expect.Equal(t, rec.invoked[1].identities, []symbol.Identity{alpha.ID},
				"the second record lists its own read alone")
		})

		t.Run("records the facts an invocation read", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					eidos.Fact(m, key)
					return nil
				})).
				Build()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			assert.Equal(t, rec.invoked[0].facts, []meta.FactRef{{Subject: alpha.ID, Key: key.Name()}},
				"the record lists the fact read, a miss included")
		})

		t.Run("records the plans whose export an invocation read", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Exports = map[string]plugin.ExportDoc{basePlan: {Plan: basePlan}, corePlan: {Plan: corePlan}}
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, _ *eidos.Emitter) error {
					m.Export(corePlan)
					m.Export(basePlan)
					m.Export(basePlan)
					m.Export(absentPlan)
					return nil
				})).
				Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.invoked[0].Exports, []string{basePlan, corePlan},
				"the record lists each export read once, sorted, and no plan without an export")
		})

		t.Run("records the units an invocation touched", func(t *testing.T) {
			t.Parallel()

			rec, _ := touchRun(t)
			pkg := coretest.PackageID(coretest.StorePath)
			assert.Equal(t, rec.invoked[0].Units, []plugin.UnitRef{
				{Plugin: contextPlugin, Pkg: pkg, Key: "alpha.go"},
				{Plugin: contextPlugin, Tag: registryTag, Pkg: pkg, Key: coretest.StorePath},
			}, "the record lists each touched unit once, sorted")
		})

		t.Run("records the references of the units the flush added", func(t *testing.T) {
			t.Parallel()

			rec, ctx := touchRun(t)
			var flushed []plugin.UnitRef
			for u := range ctx.Emit.Units() {
				flushed = append(flushed, u.Ref())
			}
			assert.Permutation(t, rec.invoked[0].Units, flushed, "each unit the record names is a unit of the store")
		})

		t.Run("records the host whose slots an invocation appended into", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
					s, held := m.Value.(*emit.Struct)
					assert.True(t, held, "a struct rule receives structs")
					e.Slot(&s.Fields).Append(&emit.Field{Name: "audited"})
					return nil
				})).
				Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.invoked[0].Hosts, []plugin.EmitRef{{Unit: seededUnit, Index: 0}},
				"the record names the value whose slot received the append")
		})

		t.Run("records the findings an invocation reported in report order", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					if m.Struct.ID == alpha.ID {
						m.Warnf(testCode, "second")
						m.Warnf(testCode, "first")
					}
					return nil
				})).
				Build()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			assert.Length(t, rec.invoked, 2, "both subjects' invocations are recorded")
			expect.Equal(t, findingMessages(rec.invoked[0].Findings), []string{"second", "first"},
				"the first record lists its findings as reported")
			expect.Empty(t, rec.invoked[1].Findings, "the second record lists none of the first's findings")
		})

		t.Run("records a finding on its invocation after invocations without effects", func(t *testing.T) {
			t.Parallel()

			g, structs := manySubjects(t, scheduled)
			_, facts := boolKey(t)
			last := structs[len(structs)-1].ID
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					if m.Struct.ID == last {
						m.Warnf(testCode, "last")
					}
					return nil
				})).
				Build()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			got := make([][]string, len(rec.invoked))
			for i, j := range rec.invoked {
				got[i] = findingMessages(j.Findings)
			}
			assert.Equal(t, got, [][]string{nil, nil, {"last"}}, "the finding is the last invocation's alone")
		})

		t.Run("records the absent-rules warning on the invocation that reported it", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					m.Rules()
					return nil
				})).
				Build()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			assert.Length(t, rec.invoked, 2, "both subjects' invocations are recorded")
			expect.Equal(t, findingCodes(rec.invoked[0].Findings), []diag.Code{rules.AbsentRules},
				"the first invocation that bound the rules reports the warning")
			expect.Empty(t, rec.invoked[1].Findings, "the second reports it no more")
		})

		t.Run("records on eight workers what one worker records", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, readingRun(t, eightWorkers, manyStructs), readingRun(t, oneWorker, manyStructs),
				"the records do not depend on the worker count")
		})

		t.Run("records on eight workers what one worker records across collected chunks", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, readingRun(t, eightWorkers, chunkedStructs), readingRun(t, oneWorker, chunkedStructs),
				"the records follow the sequence from one chunk to the next")
		})

		t.Run("leaves the units a call without a journal leaves", func(t *testing.T) {
			t.Parallel()

			placeAll := func(tb assert.TB, journal plugin.Journal) []plugin.Unit {
				tb.Helper()

				g, _ := manySubjects(tb, manyStructs)
				key, facts := boolKey(tb)
				ctx := genContext(tb, g, facts, nil)
				ctx.Journal = journal
				p := eidos.NewPlugin(contextPlugin).
					Output(plugin.Output{Per: plugin.PerPackage, Word: "registry"}).
					Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						eidos.Fact(m, key)
						e.PackageFile().Append(&emit.Struct{Origin: m.Struct.ID, Name: "For" + m.Struct.Name})
						return nil
					})).
					Build()
				assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
				return slices.Collect(ctx.Emit.Units())
			}
			assert.Equal(t, placeAll(t, &recorder{}), placeAll(t, nil), "journaling changes no output")
		})

		t.Run("delivers nothing for a call whose handler fails", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			rec := &recorder{}
			ctx.Journal = rec
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					m.Warnf(testCode, "visited")
					return errFirst
				})).
				Build()
			assert.ErrorIs(t, generatorOf(t, p).Generate(ctx), errFirst, "the handler's error stops the call")
			assert.Empty(t, rec.calls, "a failed call journals nothing")
		})

		t.Run("records each candidate's matches after every invocation", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Select = &plugin.Selection{
				Matches:    []plugin.MatchKey{{Plugin: contextPlugin, Subject: alpha.ID}},
				Candidates: []symbol.Identity{beta.ID},
			}
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.calls, []string{invokedCall, invokedCall, evaluatedCall},
				"the candidates' records follow the invocations")
		})

		t.Run("records the matches a candidate has", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{beta.ID}}
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.evaluated, []evaluation{{
				subject: beta.ID, matches: []plugin.MatchKey{{Plugin: contextPlugin, Subject: beta.ID}},
			}}, "the candidate's one match is recorded")
		})

		t.Run("records no match for a candidate the rules do not admit", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ghost := coretest.ID(coretest.StorePath, "Ghost", symbol.KindStruct)
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{ghost}}
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.evaluated, []evaluation{{subject: ghost}}, "a gone candidate has no match")
		})

		t.Run("records each candidate's matches across rules", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{alpha.ID, beta.ID}}
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(nothing[*eidos.Emitter]), eidos.OnStruct(nothing[*eidos.Emitter])).
				Build()
			rec := journaledGenerate(t, ctx, p)
			keys := func(subject symbol.Identity) []plugin.MatchKey {
				return []plugin.MatchKey{
					{Plugin: contextPlugin, Rule: 0, Subject: subject},
					{Plugin: contextPlugin, Rule: 1, Subject: subject},
				}
			}
			assert.Equal(t, rec.evaluated, []evaluation{
				{subject: alpha.ID, matches: keys(alpha.ID)},
				{subject: beta.ID, matches: keys(beta.ID)},
			}, "each candidate's record lists its matches of both rules")
		})

		t.Run("records a repeated candidate once", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{beta.ID, beta.ID}}
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.evaluated, []evaluation{{
				subject: beta.ID, matches: []plugin.MatchKey{{Plugin: contextPlugin, Subject: beta.ID}},
			}}, "the repeat has no record of its own")
		})

		t.Run("records nil for a candidate without matches after one with matches", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ghost := coretest.ID(coretest.StorePath, "Ghost", symbol.KindStruct)
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{alpha.ID, ghost}}
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Length(t, rec.evaluated, 2, "both candidates have a record")
			assert.Nil(t, rec.evaluated[1].matches, "the gone candidate's record lists no match")
		})

		t.Run("records no match for a candidate of another kind", func(t *testing.T) {
			t.Parallel()

			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			reader := coretest.Interface(coretest.StorePath, coretest.InterfaceName)
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, alpha, reader)),
				"the fixture package is admitted")
			g.Freeze()
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{reader.ID}}
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.evaluated, []evaluation{{subject: reader.ID}},
				"an interface has no struct rule's match")
		})

		t.Run("records a candidate's matches across rules in canonical match order", func(t *testing.T) {
			t.Parallel()

			g, alpha := gatedStruct(t, 2)
			_, facts := boolKey(t)
			schema := stubSchema(gateName)
			schema.Repeatable = true
			ctx := genContext(t, g, facts, repeated(alpha.ID, schema, 2))
			ctx.Select = &plugin.Selection{Candidates: []symbol.Identity{alpha.ID}}
			p := eidos.NewPlugin(contextPlugin).
				Handle(
					eidos.OnStruct(nothing[*eidos.Emitter]),
					eidos.Directive(schema, eidos.OnStruct(nothing[*eidos.Emitter])),
				).
				Build()
			rec := journaledGenerate(t, ctx, p)
			assert.Equal(t, rec.evaluated, []evaluation{{subject: alpha.ID, matches: []plugin.MatchKey{
				{Plugin: contextPlugin, Rule: 0, Subject: alpha.ID},
				{Plugin: contextPlugin, Rule: 1, Subject: alpha.ID, Instance: 0},
				{Plugin: contextPlugin, Rule: 1, Subject: alpha.ID, Instance: 1},
			}}}, "the bare rule's match precedes the gated rule's instances")
		})
	})

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		t.Run("records the facts an invocation stamped", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			first, second, facts := twoKeys(t)
			param := symbol.Identity{
				Lang: alpha.ID.Lang, Package: alpha.ID.Package, Owner: alpha.ID.Name,
				Name: "T", Kind: symbol.KindTypeParam,
			}
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					if m.Struct.ID == alpha.ID {
						eidos.Stamp(st, second, true)
						eidos.StampOn(st, param, first, true)
						eidos.Stamp(st, first, true)
						eidos.Stamp(st, first, true)
					}
					return nil
				})).
				Build()
			rec := journaledAnnotate(t, g, facts, p)
			want := slices.SortedFunc(slices.Values([]meta.FactRef{
				{Subject: alpha.ID, Key: first.Name()},
				{Subject: alpha.ID, Key: second.Name()},
				{Subject: param, Key: first.Name()},
			}), meta.FactRef.Compare)
			assert.Equal(t, rec.invoked[0].Claimed, want, "the record lists each stamped fact once, sorted")
		})

		t.Run("records no claim the fact store refused", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, key, false)
					return nil
				})).
				Build()
			rec := journaledAnnotate(t, g, facts, p)
			assert.Empty(t, rec.invoked[0].Claimed, "a refused stamp claims nothing")
			assert.Equal(t, findingCodes(rec.invoked[0].Findings), []diag.Code{eidos.RefusedStamp},
				"and the record lists the refusal")
		})

		t.Run("records one invocation for each match in canonical match order", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).Handle(eidos.OnStruct(nothing[*eidos.Stamper])).Build()
			rec := journaledAnnotate(t, g, facts, p)
			assert.Equal(t, rec.keys(), []plugin.MatchKey{
				{Plugin: contextPlugin, Subject: alpha.ID},
				{Plugin: contextPlugin, Subject: beta.ID},
			}, "an annotator's call journals as a generator's does")
		})
	})
}

// BenchmarkJournal measures a journaled phase call over the 200,000
// structs of the bench workspace, each invocation reading one fact. The
// call allocates nothing: its records, its read log and the delivery's
// buffers are the state an earlier call released, which the warm-up call
// grew. The context, the emit store and the sink that each iteration
// hands the call are built outside the measurement.
func BenchmarkJournal(b *testing.B) {
	const packages, files, decls = 1_000, 10, 20
	g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)

	b.Run("Generate", func(b *testing.B) {
		key, facts := boolKey(b)
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		assert.NoError(b, err, "the routing surface builds")
		p := eidos.NewPlugin("bench").
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
				eidos.Fact(m, key)
				return nil
			})).
			Build()
		gen := generatorOf(b, p)
		count := &tally{}
		var ctx *plugin.GeneratorContext
		fresh := func() {
			count.invoked = 0
			ctx = &plugin.GeneratorContext{
				Index: ix, Facts: facts, Emit: plugin.NewEmit(), Sink: diag.NewSink(),
				Plugin: "bench", Bucket: 1, Journal: count,
			}
		}
		// The warm-up call builds the graph's index of the kind and grows
		// the state the measured calls take.
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			c.Excluding(fresh)
			err = gen.Generate(ctx)
		}
		assert.NoError(b, err, "the phase call passes")
		assert.Equal(b, count.invoked, packages*files*decls, "the journal receives one invocation per struct")
	})
}

// nothing is a handler that does nothing, for a case about what the
// dispatch records and not about what a handler does.
func nothing[E eidos.Effect](*eidos.StructMatch, E) error { return nil }

// journaledGenerate runs p's generate phase over ctx with a recorder as
// its journal, and returns the recorder.
func journaledGenerate(tb assert.TB, ctx *plugin.GeneratorContext, p plugin.Plugin) *recorder {
	tb.Helper()

	rec := &recorder{}
	ctx.Journal = rec
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	return rec
}

// journaledAnnotate runs p's annotate phase over g with a recorder as
// its journal, as the context plugin, and returns the recorder.
func journaledAnnotate(tb assert.TB, g *store.Graph, facts *meta.Facts, p plugin.Plugin) *recorder {
	tb.Helper()

	ix, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(tb, err, "the routing surface builds")
	ctx := annContext(tb, facts, ix)
	ctx.Plugin = contextPlugin
	rec := &recorder{}
	ctx.Journal = rec
	assert.NoError(tb, annotatorOf(tb, p).Annotate(ctx), "the phase call passes")
	return rec
}

// lookupRun journals a generator whose handler looks the second fixture
// struct up from the first, and returns the recorder and both structs.
func lookupRun(tb assert.TB) (*recorder, *node.Struct, *node.Struct) {
	tb.Helper()

	g, alpha, beta := fixtureGraph(tb)
	_, facts := boolKey(tb)
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
			if m.Struct.ID == alpha.ID {
				m.Reader().Lookup(beta.ID)
			}
			return nil
		})).
		Build()
	return journaledGenerate(tb, genContext(tb, g, facts, nil), p), alpha, beta
}

// touchRun journals a generator whose handler, on the first fixture
// struct, touches its package's registry and twice appends to its
// source file's stub, and returns the recorder and the context.
func touchRun(tb assert.TB) (*recorder, *plugin.GeneratorContext) {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	p := eidos.NewPlugin(contextPlugin).
		Output(plugin.Output{Per: plugin.PerSource, Word: "stub"}).
		Output(plugin.Output{Tag: registryTag, Per: plugin.PerPackage, Word: "registry"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if m.Struct.ID != alpha.ID {
				return nil
			}
			e.PackageFile(registryTag)
			e.File().Append(&emit.Struct{Origin: m.Struct.ID, Name: "Stub"})
			e.File().Append(&emit.Struct{Origin: m.Struct.ID, Name: "Mock"})
			return nil
		})).
		Build()
	return journaledGenerate(tb, ctx, p), ctx
}

// readingRun journals a generator over n subjects on the given workers,
// whose handler looks up the next subject, reads a fact and reports a
// warning, and returns the recorder's lines.
func readingRun(tb assert.TB, workers, n int) []string {
	tb.Helper()

	g, structs := manySubjects(tb, n)
	index := indexOf(structs)
	key, facts := boolKey(tb)
	ctx := on(genContext(tb, g, facts, nil), workers)
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
			m.Reader().Lookup(structs[(index[m.Struct.ID]+1)%len(structs)].ID)
			eidos.Fact(m, key)
			m.Warnf(testCode, "%s", m.Struct.Name)
			return nil
		})).
		Build()
	return journaledGenerate(tb, ctx, p).lines()
}

// twoKeys returns two registered bool keys and a fact store built over
// their registry.
func twoKeys(tb assert.TB) (meta.Key[bool], meta.Key[bool], *meta.Facts) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
	first, err := meta.Register[bool](reg, meta.KeySpec{Name: "t.first", Doc: "a fixture key"})
	assert.NoError(tb, err, "the first key registers")
	second, err := meta.Register[bool](reg, meta.KeySpec{Name: "t.second", Doc: "a fixture key"})
	assert.NoError(tb, err, "the second key registers")
	return first, second, meta.NewFacts(reg)
}

// findingMessages returns the messages of findings, in order, and nil
// for none.
func findingMessages(found []diag.Diag) []string {
	var out []string
	for _, d := range found {
		out = append(out, d.Msg)
	}
	return out
}

// findingCodes returns the codes of findings, in order, and nil for
// none.
func findingCodes(found []diag.Diag) []diag.Code {
	var out []diag.Code
	for _, d := range found {
		out = append(out, d.Code)
	}
	return out
}
