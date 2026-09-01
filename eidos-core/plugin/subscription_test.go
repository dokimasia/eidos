// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

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

func (p gated) Subscriptions() []plugin.Subscription { return p.subs }

// A subscription is one gate tuple as data: what a rule watches is
// what dirtiness routes through, so the zero values of its gate
// fields mean ungated and the phase spelling is API.
func TestSubscription(t *testing.T) {
	t.Parallel()

	t.Run("Subscribed", func(t *testing.T) {
		t.Parallel()

		want := []plugin.Subscription{{
			Rule:      1,
			Kind:      symbol.KindInterface,
			Directive: directive.Name("stubgen:stub"),
			Phase:     plugin.PhaseGenerate,
		}}
		var p plugin.Subscribed = gated{
			name: "stubgen",
			subs: want,
		}
		assert.Equal(t, p.Subscriptions(), want,
			"the engine reads gates as data, never by running a handler")
	})

	t.Run("zero gate fields mean ungated", func(t *testing.T) {
		t.Parallel()

		bare := plugin.Subscription{Rule: 0, Phase: plugin.PhaseAnnotate}
		assert.Equal(t, bare.Kind, symbol.Kind(0),
			"a zero kind is a graph-wide rule")
		assert.Equal(t, bare.Directive, directive.Name(""),
			"an empty directive name is an ungated rule")
		assert.Equal(t, bare.FactKey, meta.KeyID(0),
			"a zero fact key is an ungated rule")
	})

	t.Run("Phase/String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			phase plugin.Phase
			want  string
		}{
			{name: "annotate", phase: plugin.PhaseAnnotate, want: "annotate"},
			{name: "generate", phase: plugin.PhaseGenerate, want: "generate"},
			{name: "emit", phase: plugin.PhaseEmit, want: "emit"},
			{
				name:  "names a phase nothing declares by its number",
				phase: plugin.PhaseEmit + 1,
				want:  "Phase(4)",
			},
			{
				name:  "the zero phase names no phase",
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
