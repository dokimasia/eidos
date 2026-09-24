// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// subscriptionsOf builds the plugin and returns its gate records.
func subscriptionsOf(tb assert.TB, rules ...eidos.Rule) []plugin.Subscription {
	tb.Helper()

	p := eidos.NewPlugin("t").Handle(rules...).Build()
	subscribed, ok := p.(plugin.Subscribed)
	assert.True(tb, ok, "a built plugin declares its gates as data")
	return subscribed.Subscriptions()
}

// A rule's gates lower to subscription records, which is the whole
// point of declarative gating: the engine reads them without running
// a handler, so their shape is contract.
func TestRule(t *testing.T) {
	t.Parallel()

	onEmit := func() eidos.Rule {
		return eidos.OnEmit(symbol.KindStruct,
			func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil })
	}

	t.Run("Subscriptions", func(t *testing.T) {
		t.Parallel()

		t.Run("a graph rule watches everything", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, emitNothing())
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Phase: plugin.PhaseGenerate,
			}}, "zero gate fields spell ungated, and a graph rule has no kind")
		})

		t.Run("an emit rule watches its kind in the emit phase", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, onEmit())
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct, Phase: plugin.PhaseEmit,
			}}, "the emit phase is the record's own, not the generate phase's")
		})

		t.Run("a directive gate records the canonical name", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t,
				eidos.Directive(stubSchema("stub"), onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct,
				Directive: directive.Name("stubgen:stub"),
				Phase:     plugin.PhaseEmit,
			}}, "the record carries the canonical spelling, whatever the author writes")
		})

		t.Run("a kernel gate records the kernel's name", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, eidos.Gated(directive.KernelSample, onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct,
				Directive: directive.KernelSample,
				Phase:     plugin.PhaseEmit,
			}}, "a kernel directive's canonical spelling is its bare name")
		})

		t.Run("a fact gate records its key", func(t *testing.T) {
			t.Parallel()

			key, _ := boolKey(t)
			got := subscriptionsOf(t, eidos.Where(eidos.HasKey(key), onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct,
				FactKey: key.ID(), Phase: plugin.PhaseEmit,
			}}, "the gate tuple is what dirtiness routes through")
		})

		t.Run("two fact gates record twice under one rule", func(t *testing.T) {
			t.Parallel()

			reg := meta.NewRegistry()
			assert.NoError(t, reg.ClaimNamespace("t", "the test"),
				"the namespace is claimed")
			first, err := meta.Register[bool](reg, meta.KeySpec{
				Name: "t.first", Doc: "the first gate",
			})
			assert.NoError(t, err, "the first key registers")
			second, err := meta.Register[bool](reg, meta.KeySpec{
				Name: "t.second", Doc: "the second gate",
			})
			assert.NoError(t, err, "the second key registers")

			got := subscriptionsOf(t,
				eidos.Where(eidos.HasKey(first),
					eidos.Where(eidos.HasKey(second), onEmit())))
			assert.Length(t, got, 2, "one record per gated key")
			assert.Equal(t, got[0].Rule, got[1].Rule,
				"both records name the one rule")
			assert.Equal(t, got[0].FactKey, first.ID(), "the outer key records")
			assert.Equal(t, got[1].FactKey, second.ID(), "and the inner key records")
		})

		t.Run("wrappers gate every rule they hold", func(t *testing.T) {
			t.Parallel()

			key, _ := boolKey(t)
			got := subscriptionsOf(t, eidos.Where(eidos.HasKey(key),
				onEmit(),
				eidos.OnEmit(symbol.KindMethod,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil }),
			))
			assert.Length(t, got, 2, "two rules produce two records")
			assert.Equal(t, got[0].Rule, plugin.RuleID(0), "ordinals follow declaration order")
			assert.Equal(t, got[1].Rule, plugin.RuleID(1), "one per rule")
			assert.Equal(t, got[1].Kind, symbol.KindMethod,
				"each record carries its own trigger's kind")
			assert.Equal(t, got[1].FactKey, key.ID(),
				"and the shared wrapper's gate")
		})
	})

	t.Run("KeyEquals", func(t *testing.T) {
		t.Parallel()

		t.Run("visits only the subject whose value matches", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, wantedRank, meta.Claim{Subject: alpha.ID}),
				"the matching subject carries the wanted value")
			assert.NoError(t,
				meta.Stamp(facts, key, otherRank, meta.Claim{Subject: beta.ID}),
				"and the other a different one, so the gate has something to reject")

			visited := gatedVisits(t, g, facts, eidos.KeyEquals(key, wantedRank))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID},
				"the gate admits the equal value and rejects the unequal one")
		})

		t.Run("rejects a subject carrying no value for the key", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, wantedRank, meta.Claim{Subject: alpha.ID}),
				"one subject is stamped and the other never is")

			visited := gatedVisits(t, g, facts, eidos.KeyEquals(key, wantedRank))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID},
				"an unstamped subject holds no value, so the gate rejects it "+
					"rather than comparing against a zero")
		})

		t.Run("visits nothing when no value equals", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, otherRank, meta.Claim{Subject: alpha.ID}),
				"the only stamped subject holds a different value")

			visited := gatedVisits(t, g, facts, eidos.KeyEquals(key, wantedRank))
			assert.Length(t, visited, 0, "a gate nothing satisfies visits nothing")
		})
	})
}

// The two values the equality gate tells apart. A string key is what
// the cases need: the fact store refuses a false bool, holding that
// absence is the negative, so a bool key cannot carry two values to
// compare.
const (
	wantedRank = "first"
	otherRank  = "second"
)

// rankKey returns a registered string key and the fact store it was
// registered in.
func rankKey(tb assert.TB) (meta.Key[string], *meta.Facts) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace("t", "the test"),
		"the namespace is claimed")
	key, err := meta.Register[string](reg, meta.KeySpec{
		Name: "t.rank", Doc: "ranks a fixture subject",
	})
	assert.NoError(tb, err, "the key registers")
	return key, meta.NewFacts(reg)
}

// gatedVisits returns the subjects a struct rule under pred visited,
// in visit order.
func gatedVisits(
	tb assert.TB, g *store.Graph, facts *meta.Facts, pred eidos.Pred,
) []symbol.Identity {
	tb.Helper()

	visited := []symbol.Identity{}
	p := eidos.NewPlugin("gated").
		Handle(eidos.Where(pred,
			eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
				visited = append(visited, m.Struct.Identity())
				return nil
			}))).
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(genContext(tb, g, facts, nil)),
		"the phase call passes")
	return visited
}
