// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

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
			assert.Equal(t, claim.Seq, 0, "the first match in canonical order has sequence zero")
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

		t.Run("assigns the sequence in canonical match order", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			key, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")

			p := eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, key, true)
					return nil
				})).
				Build()
			assert.NoError(t, annotatorOf(t, p).Annotate(annContext(t, facts, ix)),
				"the phase call passes")

			for v := range facts.Claims(beta.ID, key.ID()) {
				assert.Equal(t, v.Claim.Seq, 1, "the second subject in identity order has the next sequence")
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
}
