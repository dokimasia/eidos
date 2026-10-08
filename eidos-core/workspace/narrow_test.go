// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// storePattern is the pattern of the narrowing cases. It names the store
// package's directory of the rounds tree.
const storePattern = "svc/store"

// The mirrors that the edit of the narrowing cases adds to the generated
// file of the store package and to the generated file of the api package.
const (
	mirroredCol    = "type ForCol struct{}"
	mirroredReader = "type ForReader struct{}"
)

// The stale entry of the case whose source does not parse: its path, and
// the source that its record lists.
const (
	garbledPath   = coretest.StorePath + "/old.txt"
	garbledSource = "not an identity"
)

// A run with patterns commits the changes inside them, and a prune
// commits the removals of stale files alone. Both leave every other change
// for a later run and write no generation, so the next run finds the
// changes that they left.
func TestNarrow(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("commits the change of a package that a pattern admits", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			grown(t, root)
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			assert.Contains(t, files.Read(t, filepath.Join(root, filepath.FromSlash(storeGenerated))), mirroredCol,
				"the store package's file has the new mirror")
		})

		t.Run("leaves the file of a package outside the patterns as it was", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			api := filepath.Join(root, filepath.FromSlash(apiGenerated))
			before := files.Read(t, api)
			grown(t, root)
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			files.HasContent(t, api, before, "the api package's file has the bytes of the first run")
		})

		t.Run("lists the change of a package outside the patterns in Withheld", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			grown(t, root)
			report := sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			withheld := report.Plans[0].Withheld
			assert.Length(t, withheld, 1, "one change is withheld")
			expect.Equal(t, withheld[0].Path, apiGenerated, "the change of the api package's file")
			expect.Equal(t, withheld[0].Action, output.ActionUpdated, "which a commit would update")
		})

		t.Run("lists the changes inside the patterns in Changes", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			grown(t, root)
			report := sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			changes := report.Plans[0].Changes
			assert.Length(t, changes, 1, "one change commits")
			assert.Equal(t, changes[0].Path, storeGenerated, "the change of the store package's file")
		})

		t.Run("keeps the record's entry of a file whose change it withholds", func(t *testing.T) {
			t.Parallel()

			root, w, l := onDiskRun(t)
			before := recordIn(t, l).Files
			grown(t, root)
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			after := recordIn(t, l).Files
			assert.Length(t, after, 2, "the record lists both files")
			assert.Equal(t, after[0], before[0], "the api package's entry is the entry of the first run")
		})

		t.Run("records no entry for a new file whose write it withholds", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.CopyFS(root, roundsTree(rowLine, userLine)),
				"the tree copies into the run's directory")
			sealedRun(t, built(t, sealingOnDisk(t, root)),
				workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			assert.Equal(t, paths(recorded(t, root)), []string{storeGenerated}, "the record lists the committed file")
		})

		t.Run("writes no generation for a run with patterns", func(t *testing.T) {
			t.Parallel()

			root, w, l := onDiskRun(t)
			first := liveGeneration(t, l)
			grown(t, root)
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			assert.Equal(t, liveGeneration(t, l), first, "the generation of the first run is still live")
		})

		t.Run("commits on the next run the change that a run with patterns withheld", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			grown(t, root)
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			assert.Contains(t, files.Read(t, filepath.Join(root, filepath.FromSlash(apiGenerated))), mirroredReader,
				"the api package's file has the new mirror")
		})

		t.Run("commits the write of a file whose previous record lists a source inside the patterns",
			func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				moved := manifest.Entry{
					Path: cacheGen, Plan: "plan", Hash: "sha256:" + strings.Repeat("ab", 32),
					Plugins: []plugin.ID{"plan-mirror"},
					Sources: []string{coretest.Struct(coretest.StorePath, "Alpha").ID.String()},
				}
				mem := recording(t, moved)
				w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
					Ledger(func() (ledger.Ledger, error) { return mem, nil }))
				_, err := w.Run(t.Context(), workspace.Input{
					Graph: routedIn(t, coretest.StorePath, coretest.CachePath), Patterns: []string{storePattern},
				})
				assert.NoError(t, err, "the run is clean")
				files.IsFile(t, filepath.Join(root, cacheGen), "the cache package's file is written")
			})

		t.Run("withholds the removal of a stale file outside the patterns", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			assert.NoError(t, os.Remove(filepath.Join(root, filepath.FromSlash(apiSource))),
				"the api package's source is deleted")
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			files.IsFile(t, filepath.Join(root, filepath.FromSlash(apiGenerated)), "the api package's file remains")
		})

		t.Run("lists the removal of a stale file outside the patterns in Withheld", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			assert.NoError(t, os.Remove(filepath.Join(root, filepath.FromSlash(apiSource))),
				"the api package's source is deleted")
			report := sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			assert.Equal(t, report.Plans[0].Withheld, []output.Change{{
				Path: apiGenerated, Action: output.ActionDeleted, Found: output.FoundIntact,
			}}, "the removal that a commit would make")
		})

		t.Run("removes a stale file whose plugin left its plan", func(t *testing.T) {
			t.Parallel()

			root, _, _ := onDiskRun(t)
			w := built(t, sealingOf(t, nil, "plan", columnMirror()).
				Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
				Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) }))
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Patterns: []string{storePattern}})
			files.Absent(t, filepath.Join(root, filepath.FromSlash(apiGenerated)), "the mirror's file is removed")
		})

		t.Run("withholds the removal of a stale entry whose source does not parse as an identity", func(t *testing.T) {
			t.Parallel()

			garbled := manifest.Entry{
				Path: garbledPath, Plan: "plan", Hash: "sha256:" + strings.Repeat("ab", 32),
				Plugins: []plugin.ID{"plan-mirror"}, Sources: []string{garbledSource},
			}
			mem := recording(t, garbled)
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }))
			report, err := w.Run(t.Context(), workspace.Input{
				Graph: routedIn(t, coretest.StorePath), Patterns: []string{storePattern},
			})
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Plans[0].Withheld, []output.Change{{
				Path: garbledPath, Action: output.ActionUnchanged, Found: output.FoundNothing,
			}}, "the removal is withheld")
		})

		t.Run("removes a stale file under Prune", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			assert.NoError(t, os.Remove(filepath.Join(root, filepath.FromSlash(apiSource))),
				"the api package's source is deleted")
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Prune: true})
			files.Absent(t, filepath.Join(root, filepath.FromSlash(apiGenerated)), "the api package's file is removed")
		})

		t.Run("withholds the write of a changed file under Prune", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			store := filepath.Join(root, filepath.FromSlash(storeGenerated))
			before := files.Read(t, store)
			grown(t, root)
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Prune: true})
			files.HasContent(t, store, before, "the store package's file has the bytes of the first run")
		})

		t.Run("lists every write of a prune in Withheld", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			grown(t, root)
			report := sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Prune: true})
			withheld := report.Plans[0].Withheld
			assert.Length(t, withheld, 2, "the write of each package's file is withheld")
			for i, path := range []string{apiGenerated, storeGenerated} {
				expect.Equal(t, withheld[i].Path, path, "the withheld changes are in path order")
				expect.Equal(t, withheld[i].Action, output.ActionUpdated, "each one an update")
			}
		})

		t.Run("writes no generation for a prune", func(t *testing.T) {
			t.Parallel()

			root, w, l := onDiskRun(t)
			first := liveGeneration(t, l)
			assert.NoError(t, os.Remove(filepath.Join(root, filepath.FromSlash(apiSource))),
				"the api package's source is deleted")
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root), Prune: true})
			assert.Equal(t, liveGeneration(t, l), first, "the generation of the first run is still live")
		})
	})
}

// grown edits the rounds tree under root the way a person edits it: the
// store package declares Col after the row, and the api package declares
// Reader after the user, so the generated file of each package gains a
// mirror.
func grown(t *testing.T, root string) {
	t.Helper()

	place(t, root, sealedSource, "package svc/store\n"+rowLine+colLine)
	place(t, root, apiSource, "package svc/api\n"+userLine+readerLine)
}
