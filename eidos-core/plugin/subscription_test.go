// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// gated is a declared plugin: its gate tuples are data the engine
// reads without executing any handler.
type gated struct {
	named
	subs []plugin.Subscription
}

// Subscriptions returns the fixture's gates.
func (p gated) Subscriptions() []plugin.Subscription { return p.subs }

// A subscription is one gate tuple as data: what a rule watches is
// what dirtiness routes through, so the zero values of its gate
// fields mean ungated and the phase spelling is API.
func TestSubscription(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		bare := plugin.Subscription{Rule: 0, Phase: plugin.PhaseAnnotate}
		assert.Equal(t, bare.Kind, symbol.Kind(0),
			"a zero kind is a graph-wide rule")
		assert.Equal(t, bare.Directive, directive.Name(""),
			"an empty directive name is an ungated rule")
		assert.Equal(t, bare.FactKey, meta.KeyID(0),
			"a zero fact key is an ungated rule")
	})

	t.Run("Subscriptions", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the gates a plugin declares", func(t *testing.T) {
			t.Parallel()

			want := []plugin.Subscription{{
				Rule:      1,
				Kind:      symbol.KindInterface,
				Directive: directive.Name("stubgen:stub"),
				Phase:     plugin.PhaseGenerate,
			}}
			var p plugin.Subscribed = gated{name: "stubgen", subs: want}
			assert.Equal(t, p.Subscriptions(), want,
				"the engine reads gates as data, never by running a handler")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			phase plugin.Phase
			want  string
		}{
			{name: "returns annotate for PhaseAnnotate", phase: plugin.PhaseAnnotate, want: "annotate"},
			{name: "returns generate for PhaseGenerate", phase: plugin.PhaseGenerate, want: "generate"},
			{name: "returns emit for PhaseEmit", phase: plugin.PhaseEmit, want: "emit"},
			{
				name:  "returns the number of a phase nothing declares",
				phase: plugin.PhaseEmit + 1,
				want:  "Phase(4)",
			},
			{
				name:  "returns the number of the zero phase",
				phase: 0,
				want:  "Phase(0)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.phase.String(), tt.want,
					"a subscription record spells its phase for stats and faults")
			})
		}
	})
}

// A declared phase spells without allocating in the ordinary run,
// which runs no benchmark.
func TestSubscriptionZeroAlloc(t *testing.T) {
	phase := plugin.PhaseEmit
	var got string
	assert.MaxAllocs(t, func() { got = phase.String() }, 0, "String allocates nothing for a declared phase")
	assert.Equal(t, got, "emit", "String spells PhaseEmit")
}

// BenchmarkSubscription measures the spelling of a declared phase,
// which a stats record and a fault name.
func BenchmarkSubscription(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		phase := plugin.PhaseEmit
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = phase.String()
		}
		assert.Equal(b, got, "emit", "String spells PhaseEmit")
	})
}
