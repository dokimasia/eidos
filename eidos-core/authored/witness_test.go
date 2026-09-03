// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/authored"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/workspace"
)

func TestWitness(t *testing.T) {
	t.Parallel()

	t.Run("stamps each key's resolved type on the parameter it names", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "T="+alphaName)
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
		assert.NoError(t, err, "the run is clean")
		assert.False(t, report.Sink.Failed(), "without a finding")
		got, held := meta.Get(report.Facts, f.box.TypeParams[0].ID, w.Kernel().Witness)
		assert.True(t, held, "the witness sits on the type parameter")
		assert.Equal(t, got, f.alpha.ID, "as the identity the resolver bound")
	})

	t.Run("refuses a key naming no parameter and stamps the rest", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "U="+alphaName, "T="+alphaName)
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
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
		_, held := meta.Get(report.Facts, f.box.TypeParams[0].ID, w.Kernel().Witness)
		assert.True(t, held, "the key that names a parameter still stamps")
	})

	t.Run("refuses a witness on a declaration without parameters", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.alpha.ID, 7, directive.KernelWitness, "T="+boxName)
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
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

	t.Run("refuses a witness naming a type nothing resolves", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.box.ID, 2, directive.KernelWitness, "T=Ghost")
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
		assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
		codes := make([]diag.Code, 0)
		for d := range report.Sink.All() {
			codes = append(codes, d.Code)
		}
		assert.True(t, slices.Contains(codes, directive.UnresolvedReference),
			"validation refused the reference before the annotator ran")
		_, held := meta.Get(report.Facts, f.box.TypeParams[0].ID, w.Kernel().Witness)
		assert.False(t, held, "so nothing is stamped")
	})

	t.Run("carries the kernel's identity", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, authored.Witness().Name(), authored.WitnessPlugin, "the annotator names itself")
	})
}
