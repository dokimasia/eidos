// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// The structural triggers position their handlers: an emit match
// names the value and its origin, and neither trigger carries a
// gating instance it was not given.
func TestTrigger(t *testing.T) {
	t.Parallel()

	t.Run("OnEmit", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the value and its origin", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			value := emitted(alpha)
			seed(t, ctx, value)

			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						ran = true
						assert.True(t, m.Value == symbol.Symbol(value),
							"the match carries the very emit value")
						assert.Equal(t, m.Origin(), alpha.ID,
							"and the node identity it derives from")
						assert.Nil(t, m.Directive(),
							"a bare match carries no gating instance")
						return nil
					})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.True(t, ran, "the handler ran")
		})

		t.Run("never yields a value without an origin", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, &emit.Struct{Name: "NoOrigin"})

			p := eidos.NewPlugin("t").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						t.Error("a value without an origin is not a subject")
						return nil
					})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
		})
	})

	t.Run("OnGraph", func(t *testing.T) {
		t.Parallel()

		t.Run("carries no gating instance", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ran := false
			p := eidos.NewPlugin("t").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					ran = true
					assert.Nil(t, m.Directive(),
						"nothing gated the graph rule, by construction")
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)),
				"the phase call passes")
			assert.True(t, ran, "the handler ran")
		})
	})
}
