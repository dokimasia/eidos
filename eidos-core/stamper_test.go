// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// annContext answers an annotator context over one routing surface.
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

// The stamper binds the claim envelope once, for every plugin: the
// rank fields, the canonical sequence and the derivation are what
// arbitration and attribution stand on, so each is contract.
func TestStamper(t *testing.T) {
	t.Parallel()

	t.Run("Stamp", func(t *testing.T) {
		t.Parallel()

		t.Run("lands with the full envelope", func(t *testing.T) {
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
			assert.False(t, ctx.Sink.Failed(), "nothing was refused")

			got, held := meta.Get(facts, alpha.ID, key)
			assert.True(t, held, "the stamped fact answers")
			assert.True(t, got, "with the stamped value")

			var views []meta.ClaimView
			for v := range facts.Claims(alpha.ID, key.ID()) {
				views = append(views, v)
			}
			assert.Length(t, views, 1, "one claim landed on the first subject")
			claim := views[0].Claim
			assert.Equal(t, claim.Plugin, diag.PluginID("classify"),
				"the rank carries the context's plugin")
			assert.Equal(t, claim.Bucket, 1, "and its bucket")
			assert.Equal(t, claim.Authority, meta.AuthorityPlugin,
				"a stamp speaks with plugin authority")
			assert.Equal(t, claim.Seq, 0,
				"the first match in canonical order carries sequence zero")
			assert.Equal(t, claim.Derived, []meta.Read{{
				Subject: alpha.ID, Key: key.Name(),
			}}, "the derivation names the invocation's reads, the miss included")
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
				assert.Equal(t, v.Claim.Seq, 1,
					"the second subject in identity order carries the next sequence")
			}
		})

		t.Run("reports a refused stamp and continues", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			key, facts := boolKey(t)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")

			var visited int
			p := eidos.NewPlugin("classify").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					visited++
					eidos.Stamp(st, key, false)
					return nil
				})).
				Build()

			ctx := annContext(t, facts, ix)
			assert.NoError(t, annotatorOf(t, p).Annotate(ctx),
				"a refused stamp is not fatal to the phase")
			assert.Equal(t, visited, 2, "every subject still ran")
			assert.True(t, ctx.Sink.Failed(), "the refusal is an Error")

			var found bool
			for d := range ctx.Sink.All() {
				found = true
				assert.Equal(t, d.Code, eidos.RefusedStamp,
					"under the refused-stamp code")
				assert.Equal(t, d.Pos, alpha.Pos,
					"at the first refusing subject's position")
				break
			}
			assert.True(t, found, "the refusal landed in the sink")
		})
	})
}
