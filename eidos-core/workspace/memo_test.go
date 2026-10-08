// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"maps"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
)

// memoDir is the ledger directory the memo's entries are under.
const memoDir = "memo"

// The memo's own blobs in its directory: its size in bytes, and when it
// was last listed whole.
const (
	memoTotal   = "memo/total"
	memoTrimmed = "memo/trimmed"
)

// memoLimit is a memo cap above every case's total.
const memoLimit = 1 << 20

// The names that the api package's one struct takes in the cap cases.
// Each is as long as the others, so the memo's entries for the three
// versions of the package have one size.
const (
	firstName  = "User"
	secondName = "Uzer"
	thirdName  = "Uxer"
)

// errMemoLedger is the failure the memo's ledger returns from its open.
var errMemoLedger = errors.New("the memo's directory does not open")

// The memo restores a unit an earlier run parsed, so when a run reads
// it, when it writes it, where its entries are, and which entries its cap
// removes, are each pinned.
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

		t.Run("removes no entry of the memo for Input.Cold", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			w := built(t, sealingBuilder(t, l, "plan").Memo(workspace.Memo{Limit: memoLimit}))
			sealedRun(t, w, workspace.Input{Tree: apiTree(firstName, editTime)})
			sealedRun(t, w, workspace.Input{Tree: apiTree(secondName, editTime.Add(1))})
			before := slices.Sorted(maps.Keys(entriesIn(t, l)))
			assert.Length(t, before, 2, "the memo has an entry for each of the two names")
			sealedRun(t, w, workspace.Input{Tree: apiTree(firstName, editTime.Add(2)), Cold: true})
			assert.Equal(t, slices.Sorted(maps.Keys(entriesIn(t, l))), before,
				"the cold run keeps the entry of the name that its tree does not declare")
		})

		t.Run("removes the least recently used entry at the commit of a run over the memo's cap", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			ample := built(t, sealingBuilder(t, l, "plan").Memo(workspace.Memo{Limit: memoLimit}))
			sealedRun(t, ample, workspace.Input{Tree: apiTree(firstName, editTime)})
			sealedRun(t, ample, workspace.Input{Tree: apiTree(secondName, editTime.Add(1))})
			reverted := sealedRun(t, ample, workspace.Input{Tree: apiTree(firstName, editTime.Add(2))})
			assert.Equal(t, reverted.Stats.Restored, 1, "the revert restores the first name, which touches its entry")
			entries := entriesIn(t, l)
			assert.Length(t, entries, 2, "the memo has an entry for each of the two names")
			var pair int64
			for _, size := range entries {
				pair += size
			}
			// A cap of two and a half entries keeps two entries and no third.
			capped := built(t, sealingBuilder(t, l, "plan").Memo(workspace.Memo{Limit: pair * 5 / 4}))
			sealedRun(t, capped, workspace.Input{Tree: apiTree(thirdName, editTime.Add(3))})
			first := sealedRun(t, capped, workspace.Input{Tree: apiTree(firstName, editTime.Add(4))})
			expect.Equal(t, first.Stats.Restored, 1, "the entry that the revert used remains")
			second := sealedRun(t, capped, workspace.Input{Tree: apiTree(secondName, editTime.Add(5))})
			expect.Equal(t, second.Stats.Parsed, 1, "the entry that no run used since its write is gone")
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

// apiTree returns a tree of the api package alone, whose one struct has a
// name, written at an instant.
func apiTree(name string, written time.Time) fstest.MapFS {
	return fstest.MapFS{
		apiSource: {Data: []byte("package svc/api\ntype " + name + " string\n"), ModTime: written},
	}
}

// entriesIn returns the size of each entry of the memo in a ledger, keyed
// by the entry's name, without the memo's own blobs.
func entriesIn(t *testing.T, l ledger.Ledger) map[string]int64 {
	t.Helper()

	blobs, err := l.List(t.Context(), memoDir)
	assert.NoError(t, err, "the memo lists")
	out := map[string]int64{}
	for _, b := range blobs {
		if b.Name != memoTotal && b.Name != memoTrimmed {
			out[b.Name] = b.Size
		}
	}
	return out
}
