// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// stampAllocs is an annotate phase call over the invocation fixture
// whose invocations each stamp a second key on a target with a claim:
// each target's map of keys with its first group and the claim's state,
// three allocations, and the key's index, which allocates twelve times
// to contain 100 members. A first claim on a target also inserts into
// the fact store's sync.Map, whose trie grows by a number of nodes the
// hashes decide, so the cases stamp a second key, whose count is exact.
const stampAllocs = 3*invocationStructs + 12

// stampCase is one annotate phase call over the invocation fixture,
// named as its benchmark is. fresh returns a context of the call's
// own, whose fact store has the first key's claim on every target, and
// targets lists the identities the call stamps the second key on.
type stampCase struct {
	name    string
	ann     plugin.Annotator
	fresh   func() *plugin.AnnotatorContext
	targets []symbol.Identity
	second  meta.Key[bool]
}

// check fails unless the call stamped the second key on every target
// without a refusal.
func (c stampCase) check(tb assert.TB, ctx *plugin.AnnotatorContext) {
	tb.Helper()

	assert.False(tb, ctx.Sink.Failed(), "no stamp is refused")
	for _, target := range c.targets {
		_, held := meta.Get(ctx.Facts, target, c.second)
		assert.True(tb, held, "the second key is stamped on the target")
	}
}

// annContext returns an annotator context over one routing surface.
func annContext(tb assert.TB, facts *meta.Facts, ix *plugin.Index) *plugin.AnnotatorContext {
	tb.Helper()

	return &plugin.AnnotatorContext{
		Index:  ix,
		Facts:  facts,
		Sink:   diag.NewSink(),
		Plugin: "classify",
		Bucket: 1,
	}
}

// annotatorOf builds and asserts the annotator role of a plugin.
func annotatorOf(tb assert.TB, p plugin.Plugin) plugin.Annotator {
	tb.Helper()

	ann, ok := p.(plugin.Annotator)
	assert.True(tb, ok, "stamper rules make the value an annotator")
	return ann
}

// refusingRun annotates the fixture graph with a plugin whose every
// invocation stamps a false boolean, and returns the context, the
// visit count, the first subject's position and the phase's error.
func refusingRun(tb assert.TB) (*plugin.AnnotatorContext, int, position.Pos, error) {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	key, facts := boolKey(tb)
	ix, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(tb, err, "the routing surface builds")

	var visited int
	p := eidos.NewPlugin("classify").
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			visited++
			eidos.Stamp(st, key, false)
			return nil
		})).
		Build()
	ctx := annContext(tb, facts, ix)
	err = annotatorOf(tb, p).Annotate(ctx)
	return ctx, visited, alpha.Pos, err
}

// The stamper binds the claim envelope once, for every plugin: the
// rank fields, the canonical sequence and the derivation are what
// arbitration and attribution read, so each is contract.
func TestStamper(t *testing.T) {
	t.Parallel()

	t.Run("Stamp", func(t *testing.T) {
		t.Parallel()

		t.Run("records the claim with the full envelope", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")

			p := eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					_, _ = eidos.Fact(m, key)
					eidos.Stamp(st, key, true)
					return nil
				})).
				Build()

			ctx := annContext(t, facts, ix)
			assert.NoError(t, annotatorOf(t, p).Annotate(ctx), "the phase call passes")
			assert.False(t, ctx.Sink.Failed(), "nothing is refused")

			got, held := meta.Get(facts, alpha.ID, key)
			assert.True(t, held, "the stamped fact is found")
			assert.True(t, got, "the value is the stamped one")

			var views []meta.ClaimView
			for v := range facts.Claims(alpha.ID, key.ID()) {
				views = append(views, v)
			}
			assert.Length(t, views, 1, "one claim is on the first subject")
			claim := views[0].Claim
			assert.Equal(t, claim.Plugin, plugin.ID("classify"), "the claim has the context's plugin")
			assert.Equal(t, claim.Bucket, 1, "the claim has the context's bucket")
			assert.Equal(t, claim.Authority, meta.AuthorityPlugin, "the claim has plugin authority")
			assert.Equal(t, claim.Order, meta.Order{Subject: alpha.ID},
				"the claim's order is the first rule and the subject, without a gating instance")
			assert.Equal(t, claim.Derived, []meta.Read{{
				Subject: alpha.ID, Key: key.Name(),
			}}, "the derivation names the invocation's reads, the miss included")
		})

		t.Run("derives each claim from its own invocation alone", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")

			p := eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Fact(m, key)
					eidos.Stamp(st, key, true)
					return nil
				})).
				Build()
			assert.NoError(t, annotatorOf(t, p).Annotate(annContext(t, facts, ix)),
				"the phase call passes")

			for _, subject := range []symbol.Identity{alpha.ID, beta.ID} {
				var views []meta.ClaimView
				for v := range facts.Claims(subject, key.ID()) {
					views = append(views, v)
				}
				assert.Length(t, views, 1, "each subject has one claim")
				assert.Equal(t, views[0].Claim.Derived, []meta.Read{{
					Subject: subject, Key: key.Name(),
				}}, "the derivation has this invocation's read alone")
			}
		})

		t.Run("derives each claim from the reads made before it", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			reg := meta.NewRegistry()
			assert.NoError(t, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
			var keys []meta.Key[bool]
			for _, name := range []meta.KeyName{"t.first", "t.second", "t.third"} {
				key, err := meta.Register[bool](reg, meta.KeySpec{Name: name, Doc: "a fixture key"})
				assert.NoError(t, err, "the key registers")
				keys = append(keys, key)
			}
			facts := meta.NewFacts(reg)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")

			p := eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					if m.Struct.Identity() != alpha.ID {
						return nil
					}
					eidos.FactOf(m, beta.ID, keys[0])
					eidos.Stamp(st, keys[0], true)
					eidos.Stamp(st, keys[1], true)
					eidos.FactOf(m, beta.ID, keys[1])
					eidos.Stamp(st, keys[2], true)
					return nil
				})).
				Build()
			assert.NoError(t, annotatorOf(t, p).Annotate(annContext(t, facts, ix)),
				"the phase call passes")

			derived := func(k meta.Key[bool]) []meta.Read {
				for v := range facts.Claims(alpha.ID, k.ID()) {
					return v.Claim.Derived
				}
				return nil
			}
			first := []meta.Read{{Subject: beta.ID, Key: keys[0].Name()}}
			assert.Equal(t, derived(keys[0]), first, "the first claim has the read before it")
			assert.Equal(t, derived(keys[1]), first,
				"a claim without a read since the last has the same derivation")
			assert.Equal(t, derived(keys[2]), []meta.Read{
				{Subject: beta.ID, Key: keys[0].Name()},
				{Subject: beta.ID, Key: keys[1].Name()},
			}, "a claim after another read has that read too")
		})

		t.Run("orders each claim by its rule before its invocation's subject", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			reg := meta.NewRegistry()
			assert.NoError(t, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
			var keys []meta.Key[bool]
			for _, name := range []meta.KeyName{"t.first", "t.second"} {
				key, err := meta.Register[bool](reg, meta.KeySpec{Name: name, Doc: "a fixture key"})
				assert.NoError(t, err, "the key registers")
				keys = append(keys, key)
			}
			facts := meta.NewFacts(reg)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")

			p := eidos.NewPlugin("classify").
				Handle(
					eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
						eidos.Stamp(st, keys[0], true)
						return nil
					}),
					eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
						eidos.Stamp(st, keys[1], true)
						return nil
					}),
				).
				Build()
			assert.NoError(t, annotatorOf(t, p).Annotate(annContext(t, facts, ix)),
				"the phase call passes")

			for i, key := range keys {
				for v := range facts.Claims(beta.ID, key.ID()) {
					assert.Equal(t, v.Claim.Order, meta.Order{Rule: i, Subject: beta.ID},
						"the claim names its rule and the subject its invocation ran on")
				}
			}
		})

		t.Run("reports RefusedStamp at the refusing subject's position", func(t *testing.T) {
			t.Parallel()

			ctx, _, first, err := refusingRun(t)
			assert.NoError(t, err, "the refused stamp does not stop the phase")
			assert.True(t, ctx.Sink.Failed(), "the refusal is an Error")
			var found bool
			for d := range ctx.Sink.All() {
				found = true
				assert.Equal(t, d.Code, eidos.RefusedStamp, "the code is RefusedStamp")
				assert.Equal(t, d.Pos, first, "the first finding is at the first subject's position")
				break
			}
			assert.True(t, found, "the refusal is reported")
		})

		t.Run("runs every subject after a refused stamp", func(t *testing.T) {
			t.Parallel()

			_, visited, _, err := refusingRun(t)
			assert.NoError(t, err, "the refused stamp does not stop the phase")
			assert.Equal(t, visited, 2, "both subjects run")
		})
	})

	t.Run("StampOn", func(t *testing.T) {
		t.Parallel()

		// stampOn annotates the fixture graph with a plugin whose handler
		// stamps the flag on the identity target returns, through the
		// first subject's stamper, and returns the context and the facts.
		stampOn := func(tb assert.TB, target func(alpha, beta *node.Struct) symbol.Identity) (
			*plugin.AnnotatorContext, *meta.Facts, meta.Key[bool],
		) {
			tb.Helper()

			g, alpha, beta := fixtureGraph(tb)
			key, facts := boolKey(tb)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(tb, err, "the routing surface builds")
			p := eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					if m.Struct.Identity() == alpha.ID {
						eidos.StampOn(st, target(alpha, beta), key, true)
					}
					return nil
				})).
				Build()
			ctx := annContext(tb, facts, ix)
			assert.NoError(tb, annotatorOf(tb, p).Annotate(ctx), "the phase call passes")
			return ctx, facts, key
		}

		t.Run("records the claim on a type parameter the subject declares", func(t *testing.T) {
			t.Parallel()

			var owned symbol.Identity
			ctx, facts, key := stampOn(t, func(alpha, _ *node.Struct) symbol.Identity {
				owned = symbol.Identity{
					Lang: alpha.ID.Lang, Package: alpha.ID.Package, Owner: alpha.ID.Name,
					Name: "T", Kind: symbol.KindTypeParam,
				}
				return owned
			})
			assert.False(t, ctx.Sink.Failed(), "nothing is refused")
			got, held := meta.Get(facts, owned, key)
			assert.True(t, held, "the type parameter is stamped")
			assert.True(t, got, "the value is the stamped one")
		})

		t.Run("reports RefusedStamp at the subject's position for an identity it does not declare", func(t *testing.T) {
			t.Parallel()

			var sibling symbol.Identity
			var at position.Pos
			ctx, facts, key := stampOn(t, func(alpha, beta *node.Struct) symbol.Identity {
				sibling, at = beta.ID, alpha.Pos
				return sibling
			})
			var reported []diag.Diag
			for d := range ctx.Sink.All() {
				reported = append(reported, d)
			}
			assert.Length(t, reported, 1, "the refusal is reported once")
			assert.Equal(t, reported[0].Code, eidos.RefusedStamp, "the code is RefusedStamp")
			assert.Equal(t, reported[0].Pos, at, "the finding is at the subject's position")
			_, held := meta.Get(facts, sibling, key)
			assert.False(t, held, "nothing is stamped on the sibling")
		})
	})
}

// A stamp allocates what the fact store keeps for the claim in the
// ordinary run, which runs no benchmark. Each counted call takes a
// context built before the count, because a call leaves its claims in
// its fact store. The check runs alone, because AllocsPerRun counts
// every goroutine's allocations and refuses to run beside parallel
// tests.
func TestStamperAllocs(t *testing.T) {
	for _, tt := range stampCases(t) {
		contexts := make([]*plugin.AnnotatorContext, allocRuns)
		for i := range contexts {
			contexts[i] = tt.fresh()
		}
		at := 0
		msg := tt.name + " allocates the second key's claim on each target"
		assert.MaxAllocs(t, func() {
			if err := tt.ann.Annotate(contexts[at]); err != nil {
				t.Fatalf("Annotate: unexpected error: %v", err)
			}
			at++
		}, stampAllocs, msg)
		tt.check(t, contexts[allocRuns-1])
	}
}

// BenchmarkStamper measures an annotate phase call over the invocation
// fixture that stamps the second key once per invocation, through each
// stamping function.
func BenchmarkStamper(b *testing.B) {
	for _, tt := range stampCases(b) {
		b.Run(tt.name, func(b *testing.B) {
			ctx := tt.fresh()
			assert.NoError(b, tt.ann.Annotate(ctx), "the call before the measurement passes")
			c := bench.Start(b).MaxAllocs(stampAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(func() { ctx = tt.fresh() })
				err = tt.ann.Annotate(ctx)
			}
			assert.NoError(b, err, "the annotate call passes")
			tt.check(b, ctx)
		})
	}
}

// stampCases returns an annotate phase call over the invocation fixture
// for each stamping function: Stamp on each subject, and StampOn on a
// type parameter each subject declares.
func stampCases(tb assert.TB) []stampCase {
	tb.Helper()

	g, structs := invocationGraph(tb)
	reg, keys := annotateKeys(tb, "t.first", "t.second")
	first, second := keys[0], keys[1]
	owned := func(s *node.Struct) symbol.Identity {
		return symbol.Identity{
			Lang: s.ID.Lang, Package: s.ID.Package, Owner: s.ID.Name, Name: "T", Kind: symbol.KindTypeParam,
		}
	}
	subjects := make([]symbol.Identity, 0, len(structs))
	params := make([]symbol.Identity, 0, len(structs))
	for _, s := range structs {
		subjects = append(subjects, s.ID)
		params = append(params, owned(s))
	}
	claimed := func(targets []symbol.Identity) func() *plugin.AnnotatorContext {
		return func() *plugin.AnnotatorContext {
			ctx := annotateContext(tb, g, reg)
			for _, target := range targets {
				assert.NoError(tb, meta.Stamp(ctx.Facts, first, true, meta.Claim{Subject: target}),
					"the first key's claim is stamped")
			}
			return ctx
		}
	}
	return []stampCase{
		{
			name: "Stamp", fresh: claimed(subjects), targets: subjects, second: second,
			ann: annotatorOf(tb, eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, second, true)
					return nil
				})).
				Build()),
		},
		{
			name: "StampOn", fresh: claimed(params), targets: params, second: second,
			ann: annotatorOf(tb, eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.StampOn(st, owned(m.Struct), second, true)
					return nil
				})).
				Build()),
		},
	}
}
