// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
)

// memoDir is the ledger directory the memo's entries are under.
const memoDir = "memo"

// memoLimit is a memo cap above every case's total.
const memoLimit = 1 << 20

// errMemoLedger is the failure the memo's ledger returns from its open.
var errMemoLedger = errors.New("the memo's directory does not open")

// The memo restores a unit an earlier run parsed, so when a run reads
// it, when it writes it, and where its entries are, are each pinned.
func TestMemo(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("restores a unit an earlier run parsed", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Memo(workspace.Memo{Limit: memoLimit}))
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			edited := statsTree()
			edited[apiSource] = &fstest.MapFile{
				Data: []byte("package svc/api\ntype Account string\n"), ModTime: editTime,
			}
			sealedRun(t, w, workspace.Input{Tree: edited})
			reverted := statsTree()
			reverted[apiSource].ModTime = editTime.Add(1)
			report := sealedRun(t, w, workspace.Input{Tree: reverted})
			assert.Equal(t, report.Stats.Restored, 1, "the reverted unit restores without a parse")
			assert.Equal(t, report.Stats.Parsed, 0, "and nothing parses")
		})

		t.Run("restores nothing for Input.Cold", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Memo(workspace.Memo{Limit: memoLimit}))
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree(), Cold: true})
			assert.Equal(t, report.Stats.Restored, 0, "a cold run reads no entry")
			assert.Equal(t, report.Stats.Parsed, len(report.Load.Units), "and parses every unit")
		})

		t.Run("writes no entry for a dry run", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := built(t, sealingBuilder(t, mem, "plan").Memo(workspace.Memo{Limit: memoLimit}))
			sealedRun(t, w, workspace.Input{Tree: statsTree(), Dry: true})
			entries, err := mem.List(t.Context(), memoDir)
			assert.NoError(t, err, "the ledger lists")
			assert.Empty(t, entries, "a dry run writes nothing")
		})

		t.Run("keeps the memo in the ledger the composition opens for it", func(t *testing.T) {
			t.Parallel()

			own, shared := ledger.NewMem(), ledger.NewMem()
			memo := workspace.Memo{Limit: memoLimit, Open: func() (ledger.Ledger, error) { return shared, nil }}
			sealedRun(t, built(t, sealingBuilder(t, own, "plan").Memo(memo)), workspace.Input{Tree: statsTree()})
			ownEntries, err := own.List(t.Context(), memoDir)
			assert.NoError(t, err, "the workspace's ledger lists")
			assert.Empty(t, ownEntries, "the workspace's ledger keeps no entry")
			sharedEntries, err := shared.List(t.Context(), memoDir)
			assert.NoError(t, err, "the memo's ledger lists")
			assert.NotEmpty(t, sharedEntries, "and the memo's ledger keeps them")
		})

		t.Run("restores the units another workspace over one memo ledger parsed", func(t *testing.T) {
			t.Parallel()

			shared := ledger.NewMem()
			memo := workspace.Memo{Limit: memoLimit, Open: func() (ledger.Ledger, error) { return shared, nil }}
			sealedRun(t, built(t, sealingBuilder(t, ledger.NewMem(), "plan").Memo(memo)),
				workspace.Input{Tree: statsTree()})
			report := sealedRun(t, built(t, sealingBuilder(t, ledger.NewMem(), "plan").Memo(memo)),
				workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Restored, len(report.Load.Units), "the second workspace parses nothing")
		})

		t.Run("returns the error of a memo ledger that fails to open", func(t *testing.T) {
			t.Parallel()

			memo := workspace.Memo{Limit: memoLimit, Open: func() (ledger.Ledger, error) { return nil, errMemoLedger }}
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Memo(memo))
			_, err := w.Run(t.Context(), workspace.Input{Tree: statsTree()})
			assert.ErrorIs(t, err, errMemoLedger, "the open's own error returns")
		})
	})
}
