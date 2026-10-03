// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
)

// The files the statistics cases add to the sealed-state cases' tree: a
// second package the scripted frontend claims, and a file no frontend
// claims.
const (
	apiSource = "svc/api/user.zz"
	readme    = "README.md"
)

// The counts the cases expect of the tree: every file the gate stats,
// and the claimed files a cold gate hashes.
const (
	treeFiles    = 3
	claimedFiles = 2
)

// The files of the coupled tree: one package the files of two
// directories declare.
const (
	firstShared  = "a/one.zz"
	secondShared = "b/two.zz"
)

// editTime is the modification time an edited file takes, as an
// editor's write moves it.
var editTime = time.Unix(1, 0)

// Stats are what a probe asserts about a run, so each count states what
// the run executed and nothing else.
func TestStats(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("counts every unit of a cold run as parsed", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Parsed, len(report.Load.Units), "the run parses every unit")
			assert.Equal(t, report.Stats.Kept+report.Stats.Restored, 0, "and keeps or restores none")
		})

		t.Run("counts the files the gate statted", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Statted, treeFiles, "the gate stats every file of the tree")
		})

		t.Run("counts the claimed files a cold run hashed", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Hashed, claimedFiles, "the gate reads every claimed file")
		})

		t.Run("counts no hash for a file the record proves unchanged", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Hashed, 0, "every stat matches its record")
		})

		t.Run("counts every unit of an unchanged second run as kept", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Kept, len(report.Load.Units), "the generation keeps every unit")
			assert.Equal(t, report.Stats.Parsed, 0, "and nothing parses")
		})

		t.Run("counts a unit an edit couples to as reparsed", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: coupledTree("type Other int\n", time.Time{})})
			report := sealedRun(t, w, workspace.Input{Tree: coupledTree("type Other int\ntype Twin int\n", editTime)})
			assert.Equal(t, report.Stats.Reparsed, 1, "the unit that declares Twin parses again")
		})

		t.Run("counts the regions the run decoded", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.True(t, report.Stats.Decoded > 0, "the run's phases read the kept units' regions")
		})

		t.Run("counts the bytes the commit wrote", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.True(t, report.Stats.Written > 0, "the commit wrote the generation")
			assert.True(t, report.Stats.Size > 0, "and the live state has its bytes")
		})

		t.Run("reports no generation for a commit that fails", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, crashing{Mem: ledger.NewMem()}, "plan")
			report, err := w.Run(t.Context(), workspace.Input{Tree: statsTree()})
			assert.ErrorIs(t, err, errRecord, "the commit fails")
			assert.False(t, report.Stats.Generation, "and makes no generation live")
		})

		t.Run("reports a run over a graph as cold", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"),
				workspace.Input{Graph: routedIn(t, coretest.StorePath)})
			assert.True(t, report.Stats.Cold, "the run reads no generation")
			assert.Equal(t, report.Stats.Statted, 0, "and stats nothing")
		})
	})
}

// coupledTree returns one package two directories' files declare,
// which the scripted language makes two units: the first declares Twin,
// and the second the given body, written at the given time.
func coupledTree(second string, written time.Time) fstest.MapFS {
	return fstest.MapFS{
		firstShared:  {Data: []byte("package shared\ntype Twin string\n")},
		secondShared: {Data: []byte("package shared\n" + second), ModTime: written},
	}
}

// statsTree returns the tree the statistics cases load.
func statsTree() fstest.MapFS {
	tree := sealedTree()
	tree[apiSource] = &fstest.MapFile{Data: []byte("package svc/api\ntype User string\n")}
	tree[readme] = &fstest.MapFile{Data: []byte("# svc\n")}
	return tree
}
