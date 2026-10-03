// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest_test

import (
	"os"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
)

// rewritingBrand is the brand the rewriting fixture records under.
const rewritingBrand output.Brand = "fixture"

// The rewriting fixture's plans and kept path.
const (
	ownPlan   = "plan"
	otherPlan = "other"
	keptPath  = "kept.go"
)

// fileEntry returns a well-formed entry for one path under the plan.
func fileEntry(path string) manifest.Entry {
	return manifest.Entry{Path: path, Plan: ownPlan, Hash: "sha256:" + strings.Repeat("ab", 32)}
}

// twoFiles returns a record of two files.
func twoFiles() manifest.Manifest {
	return manifest.Manifest{Version: manifest.Version, Files: []manifest.Entry{fileEntry("a.go"), fileEntry("b.go")}}
}

// onDisk returns the record the state directory under root contains.
func onDisk(t *testing.T, root string) manifest.Manifest {
	t.Helper()

	d, err := ledger.OpenDir(root, rewritingBrand)
	assert.NoError(t, err, "the ledger opens")
	m, _, err := state.ReadManifest(t.Context(), d)
	assert.NoError(t, err, "the record reads")
	return m
}

// underOther records every entry under the other plan.
func underOther(_ int, m manifest.Manifest) manifest.Manifest {
	for i := range m.Files {
		m.Files[i].Plan = otherPlan
	}
	return m
}

// keeping returns the record with keptPath's entry added where it is
// missing.
func keeping(_ int, m manifest.Manifest) manifest.Manifest {
	for _, e := range m.Files {
		if e.Path == keptPath {
			return m
		}
	}
	m.Files = append(m.Files, fileEntry(keptPath))
	return m
}

// The rewriting ledger leaves an edited record on disk once a commit
// ends, whatever documents the commit wrote or removed.
func TestLedger(t *testing.T) {
	t.Parallel()

	t.Run("NewRewriting", func(t *testing.T) {
		t.Parallel()

		t.Run("applies the edit to the record the directory contains", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			d, err := ledger.OpenDir(root, rewritingBrand)
			assert.NoError(t, err, "the plain ledger opens")
			_, err = state.WriteManifest(t.Context(), d, twoFiles(), nil)
			assert.NoError(t, err, "a run's record is written")
			_, err = coretest.NewRewriting(t.Context(), root, rewritingBrand, 2, underOther)
			assert.NoError(t, err, "the rewriting ledger opens")
			for _, e := range onDisk(t, root).Files {
				assert.Equal(t, e.Plan, otherPlan, "the record is edited before the run reads it")
			}
		})

		t.Run("leaves a directory without a record alone", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			_, err := coretest.NewRewriting(t.Context(), root, rewritingBrand, 1, keeping)
			assert.NoError(t, err, "the rewriting ledger opens")
			assert.Empty(t, onDisk(t, root).Files, "no record is invented")
		})

		t.Run("returns an error for a record that does not read", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			d, err := ledger.OpenDir(root, rewritingBrand)
			assert.NoError(t, err, "the plain ledger opens")
			assert.NoError(t, d.Write(t.Context(), "manifest/ea.json", []byte("not a document")), "the record breaks")
			_, err = coretest.NewRewriting(t.Context(), root, rewritingBrand, 1, keeping)
			assert.HasError(t, err, "the record does not read")
		})

		t.Run("returns an error for an invalid brand", func(t *testing.T) {
			t.Parallel()

			_, err := coretest.NewRewriting(t.Context(), t.TempDir(), "Fixture", 1, underOther)
			assert.HasError(t, err, "the brand is refused")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("records the edited copy of the record a commit writes", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			l, err := coretest.NewRewriting(t.Context(), root, rewritingBrand, 1, underOther)
			assert.NoError(t, err, "the ledger opens")
			_, err = state.WriteManifest(t.Context(), l, twoFiles(), nil)
			assert.NoError(t, err, "the record commits")
			for _, e := range onDisk(t, root).Files {
				assert.Equal(t, e.Plan, otherPlan, "every entry is recorded under the other plan")
			}
		})

		t.Run("hands the edit the run's ordinal", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			var seen []int
			recording := func(run int, m manifest.Manifest) manifest.Manifest {
				seen = append(seen, run)
				return m
			}
			l, err := coretest.NewRewriting(t.Context(), root, rewritingBrand, 3, recording)
			assert.NoError(t, err, "the ledger opens")
			_, err = state.WriteManifest(t.Context(), l, twoFiles(), nil)
			assert.NoError(t, err, "the record commits")
			assert.True(t, len(seen) > 0, "the edit runs")
			for _, run := range seen {
				assert.Equal(t, run, 3, "with the ledger's run")
			}
		})

		t.Run("writes a blob that is no manifest document as given", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			l, err := coretest.NewRewriting(t.Context(), root, rewritingBrand, 1, underOther)
			assert.NoError(t, err, "the ledger opens")
			assert.NoError(t, l.Write(t.Context(), "state/CURRENT", []byte("gen\n")), "the blob is written")
			got, err := l.Read(t.Context(), "state/CURRENT")
			assert.NoError(t, err, "the blob reads")
			assert.Equal(t, string(got), "gen\n", "with its bytes")
		})

		t.Run("returns an error for a root removed after the ledger opened", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			l, err := coretest.NewRewriting(t.Context(), root, rewritingBrand, 1, underOther)
			assert.NoError(t, err, "the ledger opens")
			assert.NoError(t, os.RemoveAll(root), "the root is removed")
			assert.HasError(t, l.Write(t.Context(), "manifest/ea.json", nil), "the write is refused")
		})

		t.Run("returns an error for a record that does not read back", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			l, err := coretest.NewRewriting(t.Context(), root, rewritingBrand, 1, underOther)
			assert.NoError(t, err, "the ledger opens")
			assert.HasError(t, l.Write(t.Context(), "manifest/ea.json", []byte("not a document")),
				"the record does not read back")
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		t.Run("records the edited copy after a document is removed", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			l, err := coretest.NewRewriting(t.Context(), root, rewritingBrand, 1, keeping)
			assert.NoError(t, err, "the ledger opens")
			_, err = state.WriteManifest(t.Context(), l, twoFiles(), nil)
			assert.NoError(t, err, "the record commits")
			assert.NoError(t, l.Remove(t.Context(), "manifest/"+manifest.BucketOf(keptPath)+".json"),
				"the kept file's document is removed")
			paths := []string{}
			for _, e := range onDisk(t, root).Files {
				paths = append(paths, e.Path)
			}
			assert.Contains(t, paths, keptPath, "the edit records the kept file again")
		})

		t.Run("returns an error for an invalid name", func(t *testing.T) {
			t.Parallel()

			l, err := coretest.NewRewriting(t.Context(), t.TempDir(), rewritingBrand, 1, keeping)
			assert.NoError(t, err, "the ledger opens")
			assert.HasError(t, l.Remove(t.Context(), "../x"), "the name is refused")
		})
	})
}
