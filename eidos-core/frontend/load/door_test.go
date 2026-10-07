// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"crypto/sha256"
	"math"
	"path"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// A door records what a partition or a dependency round read, so a
// warm load can tell whether the door has to run again, and every unit
// the door returned keys on the record.
func TestDoor(t *testing.T) {
	t.Parallel()

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a store file's bytes for a qualified path", func(t *testing.T) {
			t.Parallel()

			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), read: member(libFile)}
			loadTree(t, depTree(), with(probe), stores(depStore()))
			assert.NoError(t, probe.readErr, "the store provides the file")
			assert.Equal(t, probe.bytes, depStore()[libFile].Data, "the bytes are the store's")
		})

		t.Run("records the digest of each file the partition read", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			partition := report.Doors[frontendtest.ScriptedID][0]
			assert.Contains(t, partition.Reads, load.Digested{
				Path: modFile, Digest: sha256.Sum256(stdTree()[modFile].Data),
			}, "the partition's read of the module file is recorded with its digest")
		})

		t.Run("records a read of a path nothing is at with the zero digest", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			delete(tree, modFile)
			_, report, _ := loadTree(t, tree)
			partition := report.Doors[frontendtest.ScriptedID][0]
			assert.Contains(t, partition.Reads, load.Digested{Path: modFile},
				"a file that appears there changes the record")
		})

		t.Run("records a store file a round read in the gate", func(t *testing.T) {
			t.Parallel()

			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), read: member(libFile)}
			_, report, _ := loadTree(t, depTree(), with(probe), stores(depStore()))
			assert.Equal(t, recordOf(t, report, member(libFile)).Digest, sha256.Sum256(depStore()[libFile].Data),
				"the next load's gate proves the file unchanged by its stat")
		})
	})

	t.Run("ReadDir", func(t *testing.T) {
		t.Parallel()

		t.Run("lists a workspace directory", func(t *testing.T) {
			t.Parallel()

			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), list: appPath}
			loadTree(t, depTree(), with(probe))
			assert.NoError(t, probe.listErr, "the workspace provides the directory")
			assert.Equal(t, entryNames(probe.entries), []string{path.Base(appFile)}, "the entries are the tree's")
		})

		t.Run("lists a store's root", func(t *testing.T) {
			t.Parallel()

			probe := &probing{
				ScriptedDependent: frontendtest.NewScriptedDependent(),
				list:              plugin.StorePath(frontendtest.ScriptedStore, ""),
			}
			loadTree(t, depTree(), with(probe), stores(depStore()))
			assert.NoError(t, probe.listErr, "the store has a root")
			assert.Equal(t, entryNames(probe.entries), []string{extRoot}, "the root lists the store's one directory")
		})

		t.Run("records a listing of a path nothing is at with the zero digest", func(t *testing.T) {
			t.Parallel()

			absent := plugin.StorePath(absentStore, libPath)
			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), list: absent}
			_, report, _ := loadTree(t, depTree(), with(probe))
			var reads []load.Digested
			for _, door := range report.Doors[frontendtest.ScriptedID] {
				reads = append(reads, door.Reads...)
			}
			assert.Contains(t, reads, load.Digested{Path: absent + "/"}, "a store that appears changes the record")
		})

		t.Run("returns ErrStoreAbsent naming a store the load does not provide", func(t *testing.T) {
			t.Parallel()

			absent := plugin.StorePath(absentStore, libPath)
			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), list: absent}
			loadTree(t, depTree(), with(probe))
			assert.ErrorIs(t, probe.listErr, plugin.ErrStoreAbsent, "the load provides no such store")
			assert.Contains(t, probe.listErr.Error(), absent, "the error names the path")
		})

		t.Run("records a directory entry apart from a file of its name", func(t *testing.T) {
			t.Parallel()

			flat := depStore()
			flat[dualEntry] = &fstest.MapFile{Data: []byte("not source\n")}
			_, asFile, _ := loadTree(t, depTree(), with(recorded()), stores(flat))
			nested := depStore()
			nested[path.Join(dualEntry, path.Base(noteFile))] = &fstest.MapFile{Data: []byte("not source\n")}
			_, asDir, _ := loadTree(t, depTree(), with(recorded()), stores(nested))
			assert.NotEqual(t, keysOf(asDir)[member(libFile)], keysOf(asFile)[member(libFile)],
				"a listing records which of its entries are directories")
		})

		t.Run("records a listing under its path and a trailing slash", func(t *testing.T) {
			t.Parallel()

			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), list: appPath}
			_, report, _ := loadTree(t, depTree(), with(probe))
			var listed []string
			for _, door := range report.Doors[frontendtest.ScriptedID] {
				for _, r := range door.Reads {
					listed = append(listed, r.Path)
				}
			}
			assert.Contains(t, listed, appPath+"/", "the listing's path ends in a slash")
		})
	})

	t.Run("DoorRecord", func(t *testing.T) {
		t.Parallel()

		t.Run("records the units the partition returned", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			partition := report.Doors[frontendtest.ScriptedID][0]
			assert.Length(t, partition.Units, len(report.Units), "one entry for each unit the partition returned")
		})

		t.Run("records each round after the partition with its needs", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			doors := report.Doors[frontendtest.ScriptedID]
			assert.InRange(t, len(doors), 2, math.Inf(1), "the partition's record comes first, then each round's")
			assert.NotEmpty(t, doors[1].Needs, "the first round's record lists its needs")
		})
	})
}
