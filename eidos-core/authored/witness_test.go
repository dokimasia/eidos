// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/authored"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/workspace"
)

// witnessAllocs is one construction of the witness annotator: 2 for
// each of the 6 kind rules, the leaf and its handler; 8 for the
// kernel's directive schemas the gate is checked against; 6 for the
// rules' gate spellings and 4 for the growth of the lowered rule list;
// 4 for the growth of the subscriptions; and 6 for the builder, the
// gate, the rule list and the built plugin.
const witnessAllocs = 40

// The witness annotator stamps each key's resolved type on the type
// parameter the key names, and refuses a key that names none.
func TestWitness(t *testing.T) {
	t.Parallel()

	t.Run("Witness", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps each key's resolved type on the parameter it names", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "T="+alphaName)
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.NoError(t, err, "the run is clean")
			assert.False(t, report.Sink.Failed(), "without a finding")
			got, held := meta.Get(report.Facts, f.box.TypeParams[0].ID, w.Kernel().Witness)
			assert.True(t, held, "the witness is on the type parameter")
			assert.Equal(t, got, f.alpha.ID, "as the identity the resolver bound")
		})

		t.Run("stamps the witness at directive authority", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "T="+alphaName)
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.NoError(t, err, "the run is clean")
			views := slices.Collect(report.Facts.Claims(f.box.TypeParams[0].ID, w.Kernel().Witness.ID()))
			assert.Length(t, views, 1, "the instance makes one claim")
			expect.Equal(t, views[0].Claim.Authority, meta.AuthorityDirective, "the claim has directive authority")
			expect.Equal(t, views[0].Claim.Pos, position.Pos{File: fixtureFile, Line: 2, Col: 1},
				"the claim has the position of the instance")
		})

		t.Run("reports UnknownWitnessParam for a key naming no parameter", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "U="+alphaName, "T="+alphaName)
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			var refusal diag.Diag
			for d := range report.Sink.All() {
				if d.Code == authored.UnknownWitnessParam {
					refusal = d
				}
			}
			assert.Equal(t, refusal.Code, authored.UnknownWitnessParam, "under the witness's own code")
			assert.Equal(t, refusal.Pos.Line, 2, "positioned at the carrier")
			assert.Contains(t, refusal.Msg, "U", "naming the key")
			assert.Contains(t, refusal.Msg, paramT, "and the parameters the declaration has")
		})

		t.Run("stamps the keys that name a parameter beside a refused key", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "U="+alphaName, "T="+alphaName)
			w := composed(t)
			report, _ := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			_, held := meta.Get(report.Facts, f.box.TypeParams[0].ID, w.Kernel().Witness)
			assert.True(t, held, "the key that names a parameter still stamps")
		})

		t.Run("reports UnknownWitnessParam for a declaration without type parameters", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.alpha.ID, 7, directive.KernelWitness, "T="+boxName)
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			var msgs []string
			for d := range report.Sink.All() {
				if d.Code == authored.UnknownWitnessParam {
					msgs = append(msgs, d.Msg)
				}
			}
			assert.Length(t, msgs, 1, "one refusal")
			assert.Contains(t, msgs[0], "none", "which says the declaration has no parameters")
		})

		t.Run("stamps nothing for a witness naming a type nothing resolves", func(t *testing.T) {
			t.Parallel()

			f := build(t)
			attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "T=Ghost")
			w := composed(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: f.graph})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.Contains(t, coretest.Codes(report.Sink), directive.UnresolvedReference,
				"validation refused the reference before the annotator ran")
			_, held := meta.Get(report.Facts, f.box.TypeParams[0].ID, w.Kernel().Witness)
			assert.False(t, held, "so nothing is stamped")
		})

		t.Run("returns an annotator named gen.witness", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, authored.Witness().Name(), authored.WitnessPlugin, "the annotator names itself")
		})
	})
}

// A construction of the witness annotator allocates within its ceiling
// in the ordinary run, which runs no benchmark.
func TestWitnessAllocs(t *testing.T) {
	var got plugin.Annotator
	assert.MaxAllocs(t, func() { got = authored.Witness() }, witnessAllocs,
		"Witness allocates the plugin, its rules and their handlers")
	assert.Equal(t, got.Name(), authored.WitnessPlugin, "Witness returns the witness annotator")
}

// BenchmarkWitness measures one construction of the witness annotator.
func BenchmarkWitness(b *testing.B) {
	b.Run("Witness", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(witnessAllocs)
		defer c.End()
		var got plugin.Annotator
		for c.Loop() {
			got = authored.Witness()
		}
		assert.Equal(b, got.Name(), authored.WitnessPlugin, "Witness returns the witness annotator")
	})
}
