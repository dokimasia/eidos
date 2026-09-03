// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/authored"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/workspace"
)

func TestSample(t *testing.T) {
	t.Parallel()

	t.Run("stamps the value and the alternate on a field", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.item.ID, 4, directive.KernelSample, "value=1", "alternate=2")
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
		assert.NoError(t, err, "the run is clean")
		assert.False(t, report.Sink.Failed(), "without a finding")
		v, held := meta.Get(report.Facts, f.item.ID, w.Kernel().Sample)
		assert.True(t, held && v == "1", "the value arrives as the text the author wrote")
		alt, held := meta.Get(report.Facts, f.item.ID, w.Kernel().Alternate)
		assert.True(t, held && alt == "2", "and so does the alternate")
	})

	t.Run("leaves the alternate absent where none was stated", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.item.ID, 4, directive.KernelSample, "value=1")
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
		assert.NoError(t, err, "the run is clean")
		_, held := meta.Get(report.Facts, f.item.ID, w.Kernel().Alternate)
		assert.False(t, held, "a derivation fills what the author left out")
	})

	t.Run("stamps a parameter and a return", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.load.Params[0].ID, 12, directive.KernelSample, "value=in")
		attach(t, f.graph, f.load.Returns[0].ID, 13, directive.KernelSample, "value=out")
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
		assert.NoError(t, err, "the run is clean")
		v, _ := meta.Get(report.Facts, f.load.Params[0].ID, w.Kernel().Sample)
		assert.Equal(t, v, "in", "a parameter is a subject the sample reaches")
		v, _ = meta.Get(report.Facts, f.load.Returns[0].ID, w.Kernel().Sample)
		assert.Equal(t, v, "out", "and so is a return")
	})

	t.Run("refuses a sample on a callable", func(t *testing.T) {
		t.Parallel()

		f := build(t)
		attach(t, f.graph, f.load.ID, 11, directive.KernelSample, "value=1")
		w := composed(t)
		report, err := w.Run(t.Context(), f.graph)
		assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
		assert.True(t, slices.Contains(coretest.Codes(report.Sink), eidos.RefusedStamp),
			"the key admits no callable, so the stamp is refused at the subject")
		_, held := meta.Get(report.Facts, f.load.ID, w.Kernel().Sample)
		assert.False(t, held, "and nothing is stamped")
	})

	t.Run("carries the kernel's identity", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, authored.Sample().Name(), authored.SamplePlugin, "the annotator names itself")
	})
}
