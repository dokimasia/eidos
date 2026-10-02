// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"bytes"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// The composition fingerprint is what a load keys units by, so its
// determinism and its sensitivity to the roster are pinned here.
func TestFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("derives the same bytes from the same composition", func(t *testing.T) {
		t.Parallel()

		b1, _ := flagged()
		w1, err := b1.Build()
		assert.NoError(t, err, "the composition composes")
		b2, _ := flagged()
		w2, err := b2.Build()
		assert.NoError(t, err, "its twin composes")
		assert.True(t, bytes.Equal(w1.Fingerprint(), w2.Fingerprint()),
			"one composition, one fingerprint")
		assert.True(t, bytes.Equal(w1.Fingerprint(), w1.Fingerprint()),
			"and the derivation repeats")
	})

	t.Run("moves with the scheduled roster", func(t *testing.T) {
		t.Parallel()

		b1, _ := flagged()
		w1, err := b1.Build()
		assert.NoError(t, err, "the flagged composition composes")
		extra, _ := eidos.NewPlugin("extra").
			Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Stamper) error {
				return nil
			})).Build().(plugin.Annotator)
		b2, _ := flagged()
		w2, err := b2.Annotators(extra).Build()
		assert.NoError(t, err, "the widened composition composes")
		assert.False(t, bytes.Equal(w1.Fingerprint(), w2.Fingerprint()),
			"two compositions never share unit keys")
	})

	// edited returns the fingerprint of the valid composition with a
	// second plan the edit shapes, and the checks.
	edited := func(t *testing.T, edit func(*workspace.Plan), checks ...plugin.WorkspaceCheck) []byte {
		t.Helper()

		bindings := planTo("bindings", "fixture", mirror("bindings-mirror"))
		edit(&bindings)
		w, err := valid().Plans(bindings).Checks(checks...).Build()
		assert.NoError(t, err, "the composition composes")
		return w.Fingerprint()
	}
	unchanged := func(*workspace.Plan) {}

	t.Run("moves with a plan's sources", func(t *testing.T) {
		t.Parallel()

		scoped := edited(t, func(p *workspace.Plan) { p.Sources = workspace.Sources{Packages: []string{"./svc/..."}} })
		assert.False(t, bytes.Equal(edited(t, unchanged), scoped), "a change of scope re-keys every unit")
	})

	t.Run("derives the same bytes from one scope's patterns in any order", func(t *testing.T) {
		t.Parallel()

		forward := edited(t, func(p *workspace.Plan) { p.Sources = workspace.Sources{Packages: []string{"a", "b"}} })
		backward := edited(t, func(p *workspace.Plan) { p.Sources = workspace.Sources{Packages: []string{"b", "a"}} })
		assert.True(t, bytes.Equal(forward, backward), "the patterns fold sorted")
	})

	t.Run("moves with a plan's dependencies", func(t *testing.T) {
		t.Parallel()

		depending := edited(t, func(p *workspace.Plan) { p.DependsOn = []string{"plan"} })
		assert.False(t, bytes.Equal(edited(t, unchanged), depending), "a dependency re-keys every unit")
	})

	t.Run("moves with a check", func(t *testing.T) {
		t.Parallel()

		checked := edited(t, unchanged, &recordingCheck{name: "stubbed"})
		assert.False(t, bytes.Equal(edited(t, unchanged), checked), "a check re-keys every unit")
	})

	t.Run("moves with the plans a check reads", func(t *testing.T) {
		t.Parallel()

		one := edited(t, unchanged, &recordingCheck{name: "stubbed", reads: []string{"plan"}})
		other := edited(t, unchanged, &recordingCheck{name: "stubbed", reads: []string{"bindings"}})
		assert.False(t, bytes.Equal(one, other), "the plans a check reads re-key every unit")
	})
}
