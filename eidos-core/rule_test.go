// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// fixtureNamespace is the namespace of the fixture keys of the root
// package.
const fixtureNamespace = "t"

// wantedRank and otherRank are the two values that the equality gate
// distinguishes. The fact store refuses a false bool, because an absent
// fact is the negative, so the cases take a string key with two values.
const (
	wantedRank = "first"
	otherRank  = "second"
)

// chainSlot is the named slot that the cases of the slot gate declare,
// and gatedName the name that the name gate of a record case compares
// with.
const (
	chainSlot = "chain"
	gatedName = "Serve"
)

// These constants are the allocation counts of the scoping wrappers and
// of the predicates.
const (
	// directiveAllocs counts the allocations of a directive wrapper, which
	// are the copy of its schema and its list of rules.
	directiveAllocs = 2
	// gatedAllocs counts the allocation of a kernel gate, which is its list
	// of rules.
	gatedAllocs = 1
	// whereAllocs counts the allocations of a fact gate, which are its list
	// of predicates and its list of rules.
	whereAllocs = 2
	// predAllocs counts the allocation of a predicate, which is the closure
	// of its test over the key.
	predAllocs = 1
	// namedPredAllocs counts the allocations of a predicate on a named
	// handle, which are the closure of its test over the key and the
	// closure that binds the name.
	namedPredAllocs = 2
)

// ruleCall is one construction of a wrapper or a predicate. It has the
// name of its benchmark, its allocation count, and a check of what the
// last construction built.
type ruleCall struct {
	name   string
	allocs uint64
	call   func()
	check  func(tb assert.TB)
}

// TestRule tests that the gates of a rule lower to subscription records.
// The engine reads the records without running a handler, so their shape
// is a contract.
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

		t.Run("returns a rule gated on the spelling under the name of the plugin for a schema without one",
			func(t *testing.T) {
				t.Parallel()

				s := stubSchema("stub")
				s.Plugin = ""
				got := subscriptionsOf(t, eidos.Directive(s, onEmit()))
				assert.Equal(t, got, []plugin.Subscription{{
					Rule: 0, Kind: symbol.KindStruct,
					Directive: directive.Name("t:stub"),
					Phase:     plugin.PhaseEmit,
				}}, "the record names the spelling under the plugin t that the fixture builds")
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

		t.Run("returns one record without a key for a rule whose gates read the value alone", func(t *testing.T) {
			t.Parallel()

			got := subscriptionsOf(t, eidos.Where(eidos.NameEquals(gatedName), onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct, Phase: plugin.PhaseEmit,
			}}, "a value gate does not read a key, so the record is the record of a bare rule")
		})

		t.Run("returns a record for the key gate beside a value gate", func(t *testing.T) {
			t.Parallel()

			key, _ := boolKey(t)
			rule := eidos.Where(eidos.NameEquals(gatedName), eidos.Where(eidos.HasKey(key), onEmit()))
			got := subscriptionsOf(t, rule)
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct, FactKey: key.ID(), Phase: plugin.PhaseEmit,
			}}, "the value gate adds no record")
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

	t.Run("Not", func(t *testing.T) {
		t.Parallel()

		t.Run("visits only a subject on which the key reads absent", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "one subject is flagged")

			visited := gatedVisits(t, g, facts, eidos.Not(eidos.HasKey(key)))
			assert.Equal(t, visited, []symbol.Identity{beta.ID}, "the flagged subject is not visited")
		})

		t.Run("visits only a subject on which the key of a named handle reads absent", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "one subject is flagged")

			visited := gatedVisits(t, g, facts, eidos.Not(eidos.HasKey(meta.Named[bool](key.Name()))))
			assert.Equal(t, visited, []symbol.Identity{beta.ID}, "the bound negated gate visits the unflagged subject")
		})

		t.Run("visits a subject with another value under a negated equality", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t, meta.Stamp(facts, key, wantedRank, meta.Claim{Subject: alpha.ID}),
				"the matching subject is stamped with the wanted value")
			assert.NoError(t, meta.Stamp(facts, key, otherRank, meta.Claim{Subject: beta.ID}),
				"the other subject is stamped with another value")

			visited := gatedVisits(t, g, facts, eidos.Not(eidos.KeyEquals(key, wantedRank)))
			assert.Equal(t, visited, []symbol.Identity{beta.ID}, "only the other value is visited")
		})

		t.Run("visits a subject without a value under a negated equality", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := rankKey(t)
			assert.NoError(t, meta.Stamp(facts, key, wantedRank, meta.Claim{Subject: alpha.ID}),
				"one subject is stamped with the wanted value")

			visited := gatedVisits(t, g, facts, eidos.Not(eidos.KeyEquals(key, wantedRank)))
			assert.Equal(t, visited, []symbol.Identity{beta.ID}, "the unstamped subject is visited")
		})

		t.Run("visits only a subject on which the key reads present under two negations", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			assert.NoError(t, meta.Stamp(facts, key, true, meta.Claim{Subject: alpha.ID}), "one subject is flagged")

			visited := gatedVisits(t, g, facts, eidos.Not(eidos.Not(eidos.HasKey(key))))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID}, "the second negation restores the gate")
		})

		t.Run("returns a gate whose rule records the key", func(t *testing.T) {
			t.Parallel()

			key, _ := boolKey(t)
			got := subscriptionsOf(t, eidos.Where(eidos.Not(eidos.HasKey(key)), onEmit()))
			assert.Equal(t, got, []plugin.Subscription{{
				Rule: 0, Kind: symbol.KindStruct, FactKey: key.ID(), Phase: plugin.PhaseEmit,
			}}, "a negated gate reads its key, so the record contains it")
		})
	})

	t.Run("NameEquals", func(t *testing.T) {
		t.Parallel()

		t.Run("visits only the declaration of the name", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			visited := gatedVisits(t, g, facts, eidos.NameEquals(alpha.Name))
			assert.Equal(t, visited, []symbol.Identity{alpha.ID}, "the declaration of another name is not visited")
		})

		t.Run("visits only the emitted value of the name", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			first, second := emitted(alpha), emitted(beta)
			visited := emitVisits(t, g, symbol.KindStruct, eidos.NameEquals(second.Name), first, second)
			assert.Length(t, visited, 1, "the value of another name is not visited")
			assert.Equal(t, visited[0], symbol.Symbol(second), "the visited value has the name", assert.ByIdentity())
		})

		t.Run("visits every subject of another name under a negation", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			visited := gatedVisits(t, g, facts, eidos.Not(eidos.NameEquals(alpha.Name)))
			assert.Equal(t, visited, []symbol.Identity{beta.ID}, "the declaration of the name is not visited")
		})
	})

	t.Run("HasSlot", func(t *testing.T) {
		t.Parallel()

		t.Run("visits only the emitted function whose body declares the slot", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			declaring := &emit.Function{Origin: alpha.ID, Name: alpha.Name}
			declaring.Body.Declare(chainSlot)
			plain := &emit.Function{Origin: beta.ID, Name: beta.Name}
			visited := emitVisits(t, g, symbol.KindFunction, eidos.HasSlot(chainSlot), declaring, plain)
			assert.Length(t, visited, 1, "a function without the slot is not visited")
			assert.Equal(t, visited[0], symbol.Symbol(declaring), "the visited function declares the slot",
				assert.ByIdentity())
		})

		t.Run("visits only the emitted method whose body declares the slot", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			declaring := &emit.Method{Origin: alpha.ID, Name: alpha.Name}
			declaring.Body.Declare(chainSlot)
			plain := &emit.Method{Origin: beta.ID, Name: beta.Name}
			visited := emitVisits(t, g, symbol.KindMethod, eidos.HasSlot(chainSlot), declaring, plain)
			assert.Length(t, visited, 1, "a method without the slot is not visited")
			assert.Equal(t, visited[0], symbol.Symbol(declaring), "the visited method declares the slot",
				assert.ByIdentity())
		})

		t.Run("visits no emitted value without a body", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			visited := emitVisits(t, g, symbol.KindStruct, eidos.HasSlot(chainSlot), emitted(alpha))
			assert.Empty(t, visited, "a struct has no body")
		})

		t.Run("visits no node declaration", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			assert.Empty(t, gatedVisits(t, g, facts, eidos.HasSlot(chainSlot)), "the node model does not record a body")
		})
	})
}

// TestRuleAllocs checks in the ordinary run, which runs no benchmark, that
// each wrapper allocates its lists and each predicate its test. A
// predicate on a named handle also allocates its bind. The check runs
// alone, because the count includes the allocations of every goroutine.
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

// BenchmarkRule measures the construction of each scoping wrapper around
// one rule and of each predicate. The constructor of a plugin declares
// each of them once.
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
// rule, and of each predicate, with a check of the record that the last
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
		{name: "Not", allocs: predAllocs, check: keyed(flag.ID()), call: func() { p = eidos.Not(pred) }},
		{name: "NameEquals", allocs: predAllocs, check: keyed(0), call: func() { p = eidos.NameEquals(gatedName) }},
		{name: "HasSlot", allocs: predAllocs, check: keyed(0), call: func() { p = eidos.HasSlot(chainSlot) }},
	}
}

// rankKey returns a registered string key and the fact store of its
// registry.
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

// subscriptionsOf builds a generator of rules and returns its gate
// records.
func subscriptionsOf(tb assert.TB, rules ...eidos.Rule) []plugin.Subscription {
	tb.Helper()

	p := eidos.NewPlugin("t").Handle(rules...).Generator()
	subscribed, ok := p.(plugin.Subscribed)
	assert.True(tb, ok, "a built plugin declares its gates as data")
	return subscribed.Subscriptions()
}

// gatedVisits returns the subjects that a struct rule under pred visits,
// in visit order. It first binds the gates of the plugin in the registry
// of the fact store, as a workspace binds them when it builds, and then
// runs the bound plugin.
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
		Generator()
	binder, binds := p.(plugin.KeyBinder)
	assert.True(tb, binds, "a built plugin binds its gates")
	bound, err := binder.BindKeys(facts.Registry())
	assert.NoError(tb, err, "the gates bind")
	assert.NoError(tb, generatorOf(tb, bound).Generate(genContext(tb, g, facts, nil)),
		"the phase call passes")
	return visited
}

// emitVisits returns the emitted values of kind k that an emit rule under
// pred visits, in visit order. An earlier turn placed values in the store
// of the plan over the graph g.
func emitVisits(
	tb assert.TB, g *store.Graph, k symbol.Kind, pred eidos.Pred, values ...symbol.Symbol,
) []symbol.Symbol {
	tb.Helper()

	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	seed(tb, ctx, values...)
	visited := []symbol.Symbol{}
	p := eidos.NewPlugin("gated").
		Handle(eidos.Where(pred, eidos.OnEmit(k, func(m *eidos.EmitMatch, _ *eidos.Emitter) error {
			visited = append(visited, m.Value)
			return nil
		}))).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	return visited
}
