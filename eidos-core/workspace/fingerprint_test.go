// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"bytes"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
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
}
