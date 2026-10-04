// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// fingerprintAllocs is the copy of the fingerprint a call returns.
const fingerprintAllocs = 1

// A run reads the sealed state only where the live generation records
// the composition's fingerprint, so the fingerprint's determinism and
// its sensitivity to the roster are pinned here.
func TestFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("Fingerprint", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the same bytes for the same composition", func(t *testing.T) {
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

		t.Run("returns a SHA-256 digest", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition composes")
			assert.Length(t, w.Fingerprint(), sha256.Size, "the fingerprint is one digest")
		})

		t.Run("returns a copy the caller can change", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition composes")
			first := w.Fingerprint()
			first[0]++
			assert.False(t, bytes.Equal(first, w.Fingerprint()), "a change to a copy leaves the workspace's own")
		})

		t.Run("returns other bytes for another scheduled roster", func(t *testing.T) {
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
				"two compositions never read one generation")
		})

		// edited returns the fingerprint of the valid composition with a
		// second plan the edit changes, and the checks.
		edited := func(t *testing.T, edit func(*workspace.Plan), checks ...plugin.WorkspaceCheck) []byte {
			t.Helper()

			bindings := planTo("bindings", "fixture", mirror("bindings-mirror"))
			edit(&bindings)
			w, err := valid().Plans(bindings).Checks(checks...).Build()
			assert.NoError(t, err, "the composition composes")
			return w.Fingerprint()
		}
		unchanged := func(*workspace.Plan) {}

		t.Run("returns other bytes for another scope of a plan", func(t *testing.T) {
			t.Parallel()

			scoped := edited(t, func(p *workspace.Plan) {
				p.Sources = workspace.Sources{Packages: []string{"./svc/..."}}
			})
			assert.False(t, bytes.Equal(edited(t, unchanged), scoped), "a change of scope runs cold")
		})

		t.Run("returns the same bytes for one scope's patterns in any order", func(t *testing.T) {
			t.Parallel()

			forward := edited(t, func(p *workspace.Plan) {
				p.Sources = workspace.Sources{Packages: []string{"a", "b"}}
			})
			backward := edited(t, func(p *workspace.Plan) {
				p.Sources = workspace.Sources{Packages: []string{"b", "a"}}
			})
			assert.True(t, bytes.Equal(forward, backward), "the patterns fold sorted")
		})

		t.Run("returns other bytes for another dependency of a plan", func(t *testing.T) {
			t.Parallel()

			depending := edited(t, func(p *workspace.Plan) { p.DependsOn = []string{"plan"} })
			assert.False(t, bytes.Equal(edited(t, unchanged), depending), "a dependency runs cold")
		})

		t.Run("returns other bytes for an added check", func(t *testing.T) {
			t.Parallel()

			checked := edited(t, unchanged, &recordingCheck{name: "stubbed"})
			assert.False(t, bytes.Equal(edited(t, unchanged), checked), "a check runs cold")
		})

		t.Run("returns other bytes for other plans a check reads", func(t *testing.T) {
			t.Parallel()

			one := edited(t, unchanged, &recordingCheck{name: "stubbed", reads: []string{"plan"}})
			other := edited(t, unchanged, &recordingCheck{name: "stubbed", reads: []string{"bindings"}})
			assert.False(t, bytes.Equal(one, other), "the plans a check reads run cold")
		})
	})
}

// A fingerprint allocates its copy in the ordinary run, which runs no
// benchmark. The check runs alone, because AllocsPerRun counts every
// goroutine's allocations and refuses to run beside parallel tests.
func TestFingerprintAllocs(t *testing.T) {
	w, err := valid().Build()
	assert.NoError(t, err, "the composition composes")
	assert.MaxAllocs(t, func() {
		if len(w.Fingerprint()) != sha256.Size {
			t.Fatal("Fingerprint returned another length")
		}
	}, fingerprintAllocs, "Fingerprint allocates the copy it returns")
}

// BenchmarkFingerprint measures the copy of a workspace's fingerprint,
// which a run takes once to key the sealed state.
func BenchmarkFingerprint(b *testing.B) {
	b.Run("Fingerprint", func(b *testing.B) {
		w, err := valid().Build()
		assert.NoError(b, err, "the composition composes")
		c := bench.Start(b).MaxAllocs(fingerprintAllocs)
		defer c.End()
		var got []byte
		for c.Loop() {
			got = w.Fingerprint()
		}
		assert.Length(b, got, sha256.Size, "Fingerprint returns the digest")
	})
}
