// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// fixtureNamespace is the namespace the root package's fixture keys
// register under.
const fixtureNamespace = "t"

// The two values the equality gate tells apart. The cases need a
// string key: the fact store refuses a false bool, because absence
// is the negative, so a bool key cannot have two values to compare.
const (
	wantedRank = "first"
	otherRank  = "second"
)

// The allocations of the scoping wrappers and the predicates.
const (
	// directiveAllocs is a directive wrapper: the copy of its schema and
	// its list of rules.
	directiveAllocs = 2
	// gatedAllocs is a kernel gate: its list of rules.
	gatedAllocs = 1
	// whereAllocs is a fact gate: its list of predicates and its list of
	// rules.
	whereAllocs = 2
	// predAllocs is a predicate: its test's closure over the key.
	predAllocs = 1
	// namedPredAllocs is a predicate on a named handle: its test's
	// closure over the key and the closure that binds the name.
	namedPredAllocs = 2
)

// ruleCall is one construction of a wrapper or a predicate, named as
// its benchmark is, what it allocates, and a check of what the last
// construction built.
type ruleCall struct {
	name   string
	allocs uint64
	call   func()
	check  func(tb assert.TB)
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

	t.Run("Directive", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a rule gated on the schema's canonical spelling", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t,
				eidos.Directive(stubSchema("stub"), onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct,
				Directive: directive.Name("stubgen:stub"),
				Phase:     plugin.PhaseEmit,
			}}, "the record names the canonical spelling")
		})
	})

	t.Run("Gated", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a rule gated on the kernel directive's bare name", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, eidos.Gated(directive.KernelSample, onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct,
				Directive: directive.KernelSample,
				Phase:     plugin.PhaseEmit,
			}}, "a kernel directive's canonical spelling is its bare name")
		})
	})

	t.Run("Where", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a rule gated on the predicate's key", func(t *testing.T) {
			t.Parallel()

			key, _ := boolKey(t)
			got := subscriptionsOf(t, eidos.Where(eidos.HasKey(key), onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct,
				FactKey: key.ID(), Phase: plugin.PhaseEmit,
			}}, "the gate tuple is what dirtiness routes through")
		})

		t.Run("returns a rule gated on the key of each nested wrapper", func(t *testing.T) {
			t.Parallel()

			reg := meta.NewRegistry()
			assert.NoError(t, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
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
			assert.Length(t, got, 2, "each gated key is one record")
			assert.Equal(t, got[0].Rule, got[1].Rule, "both records name the one rule")
			assert.Equal(t, got[0].FactKey, first.ID(), "the first record has the outer key")
			assert.Equal(t, got[1].FactKey, second.ID(), "the second record has the inner key")
		})

		t.Run("returns one gated rule per rule it wraps", func(t *testing.T) {
			t.Parallel()

			key, _ := boolKey(t)
			got := subscriptionsOf(t, eidos.Where(eidos.HasKey(key),
				onEmit(),
				eidos.OnEmit(symbol.KindMethod,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil }),
			))
			assert.Length(t, got, 2, "each rule is one record")
			assert.Equal(t, got[0].Rule, plugin.RuleID(0), "the first ordinal is zero")
			assert.Equal(t, got[1].Rule, plugin.RuleID(1), "the ordinals follow declaration order")
			assert.Equal(t, got[1].Kind, symbol.KindMethod, "each record has its own trigger's kind")
			assert.Equal(t, got[1].FactKey, key.ID(), "each record has the shared wrapper's gate")
		})
	})

	t.Run("HasKey", func(t *testing.T) {
		t.Parallel()

		t.Run("visits only a subject on which the key reads present", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"one subject is flagged")

			visited := gatedVisits(t, g, facts, eidos.HasKey(key))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID}, "the unflagged subject is not visited")
		})

		t.Run("visits only a subject on which the key of a named handle reads present", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}),
				"one subject is flagged")

			visited := gatedVisits(t, g, facts, eidos.HasKey(meta.Named[bool](key.Name())))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID}, "the bound gate visits the flagged subject")
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
				"the matching subject is stamped with the wanted value")
			assert.NoError(t,
				meta.Stamp(facts, key, otherRank, meta.Claim{Subject: beta.ID}),
				"the other subject is stamped with another value")

			visited := gatedVisits(t, g, facts, eidos.KeyEquals(key, wantedRank))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID}, "only the equal value is visited")
		})

		t.Run("skips a subject without a value for the key", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, wantedRank, meta.Claim{Subject: alpha.ID}),
				"one subject is stamped")

			visited := gatedVisits(t, g, facts, eidos.KeyEquals(key, wantedRank))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID},
				"the unstamped subject is not compared against a zero value")
		})

		t.Run("visits nothing when no value equals", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, otherRank, meta.Claim{Subject: alpha.ID}),
				"the only stamped subject has another value")

			visited := gatedVisits(t, g, facts, eidos.KeyEquals(key, wantedRank))
			assert.Empty(t, visited, "nothing is visited")
		})

		t.Run("visits only the subject whose value matches through a named handle", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t,
				meta.Stamp(facts, key, wantedRank, meta.Claim{Subject: alpha.ID}),
				"the matching subject is stamped with the wanted value")
			assert.NoError(t,
				meta.Stamp(facts, key, otherRank, meta.Claim{Subject: beta.ID}),
				"the other subject is stamped with another value")

			visited := gatedVisits(t, g, facts, eidos.KeyEquals(meta.Named[string](key.Name()), wantedRank))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID}, "only the equal value is visited")
		})
	})
}

// Each wrapper allocates its lists, and each predicate its test, in the
// ordinary run, which runs no benchmark. A predicate on a named handle
// also allocates its bind. The check runs alone, because the count
// includes every goroutine's allocations.
func TestRuleAllocs(t *testing.T) {
	for _, tt := range ruleCalls(t) {
		msg := tt.name + " allocates its rule"
		assert.MaxAllocs(t, tt.call, tt.allocs, msg)
		tt.check(t)
	}

	leaf := eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })
	flag := meta.Named[bool]("t.flag")
	rank := meta.Named[string]("t.rank")
	var p eidos.Pred
	assert.MaxAllocs(t, func() { p = eidos.HasKey(flag) }, namedPredAllocs,
		"HasKey allocates the test and the bind of a named handle")
	assert.Equal(t, subscriptionsOf(t, eidos.Where(p, leaf))[0].FactKey, meta.KeyID(0),
		"HasKey returns a gate that waits for the workspace to bind it")
	assert.MaxAllocs(t, func() { p = eidos.KeyEquals(rank, wantedRank) }, namedPredAllocs,
		"KeyEquals allocates the test and the bind of a named handle")
	assert.Equal(t, subscriptionsOf(t, eidos.Where(p, leaf))[0].FactKey, meta.KeyID(0),
		"KeyEquals returns a gate that waits for the workspace to bind it")
}

// BenchmarkRule measures each scoping wrapper around one rule and each
// predicate's construction: what a plugin's constructor declares once.
func BenchmarkRule(b *testing.B) {
	for _, tt := range ruleCalls(b) {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			for c.Loop() {
				tt.call()
			}
			tt.check(b)
		})
	}
}

// ruleCalls returns one construction of each wrapper around one emit
// rule, and of each predicate, with a check of the record the last
// construction lowers to.
func ruleCalls(tb assert.TB) []ruleCall {
	tb.Helper()

	leaf := eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })
	schema := stubSchema("stub")
	flag, _ := boolKey(tb)
	rank, _ := rankKey(tb)
	pred := eidos.HasKey(flag)
	var (
		rule eidos.Rule
		p    eidos.Pred
	)
	gatedOn := func(name directive.Name) func(assert.TB) {
		return func(tb assert.TB) {
			assert.Equal(tb, subscriptionsOf(tb, rule)[0].Directive, name, "the rule is gated on the directive")
		}
	}
	keyed := func(id meta.KeyID) func(assert.TB) {
		return func(tb assert.TB) {
			assert.Equal(tb, subscriptionsOf(tb, eidos.Where(p, leaf))[0].FactKey, id, "the predicate gates on the key")
		}
	}
	return []ruleCall{
		{
			name: "Directive", allocs: directiveAllocs, check: gatedOn(schema.Canonical()),
			call: func() { rule = eidos.Directive(schema, leaf) },
		},
		{
			name: "Gated", allocs: gatedAllocs, check: gatedOn(directive.KernelSample),
			call: func() { rule = eidos.Gated(directive.KernelSample, leaf) },
		},
		{
			name: "Where", allocs: whereAllocs, call: func() { rule = eidos.Where(pred, leaf) },
			check: func(tb assert.TB) {
				assert.Equal(tb, subscriptionsOf(tb, rule)[0].FactKey, flag.ID(), "the rule is gated on the key")
			},
		},
		{name: "HasKey", allocs: predAllocs, check: keyed(flag.ID()), call: func() { p = eidos.HasKey(flag) }},
		{
			name: "KeyEquals", allocs: predAllocs, check: keyed(rank.ID()),
			call: func() { p = eidos.KeyEquals(rank, wantedRank) },
		},
	}
}

// rankKey returns a registered string key and the fact store it was
// registered in.
func rankKey(tb assert.TB) (meta.Key[string], *meta.Facts) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
	key, err := meta.Register[string](reg, meta.KeySpec{
		Name: "t.rank", Doc: "ranks a fixture subject",
	})
	assert.NoError(tb, err, "the key registers")
	return key, meta.NewFacts(reg)
}

// subscriptionsOf builds the plugin and returns its gate records.
func subscriptionsOf(tb assert.TB, rules ...eidos.Rule) []plugin.Subscription {
	tb.Helper()

	p := eidos.NewPlugin("t").Handle(rules...).Build()
	subscribed, ok := p.(plugin.Subscribed)
	assert.True(tb, ok, "a built plugin declares its gates as data")
	return subscribed.Subscriptions()
}

// gatedVisits returns the subjects a struct rule under pred visited,
// in visit order. The plugin binds its gates in the registry of the
// fact store first, as a workspace binds them when it builds.
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
	binder, binds := p.(plugin.KeyBinder)
	assert.True(tb, binds, "a built plugin binds its gates")
	assert.NoError(tb, binder.BindKeys(facts.Registry()), "the gates bind")
	assert.NoError(tb, generatorOf(tb, p).Generate(genContext(tb, g, facts, nil)),
		"the phase call passes")
	return visited
}
