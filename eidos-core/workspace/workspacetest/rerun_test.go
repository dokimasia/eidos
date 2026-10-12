// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// The file outside the state directory that a rerun case keeps, the file
// that a rerun adds, and the bytes that a rerun writes.
const (
	keptFile    = "svc/kept.txt"
	addedFile   = "svc/added.txt"
	keptBody    = "kept\n"
	changedBody = "changed\n"
)

// The contracts of the rerun check, each before the path that it names.
const (
	rerunChanges = "a second run changes no byte of "
	rerunMoves   = "a second run moves the mtime of no file: "
	rerunAdds    = "a second run writes no file the first did not: "
)

// A segment of the sealed state, which a run rewrites as it records what
// it read, and a document of the manifest, which the check compares.
var (
	segmentFile = ledger.StateDir(fixtureBrand) + "/state/seg/0001"
	manifestDoc = ledger.ManifestPath(fixtureBrand) + "/00.json"
)

// A rerun over an unchanged tree rewrites no file, so the reruns that
// write are simulated and the check's own failure is asserted.
func TestRerun(t *testing.T) {
	t.Parallel()

	t.Run("AssertRerunUnchanged", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a rerun that writes nothing", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertRerunUnchanged(t, rerunRoot(t), fixtureBrand, func() {})
		})

		t.Run("passes a rerun that rewrites the sealed state", func(t *testing.T) {
			t.Parallel()

			root := rerunRoot(t)
			workspacetest.AssertRerunUnchanged(t, root, fixtureBrand, func() {
				files.Write(t, root, files.Tree{segmentFile: files.Text(changedBody)})
			})
		})

		tests := []struct {
			name  string
			rerun func(tb assert.TB, root string)
			want  []string
		}{
			{
				name: "fails a rerun that writes the same bytes again",
				rerun: func(tb assert.TB, root string) {
					files.Write(tb, root, files.Tree{keptFile: files.Text(keptBody)})
				},
				want: []string{rerunMoves + keptFile},
			},
			{
				name: "fails a rerun that changes the bytes of a file",
				rerun: func(tb assert.TB, root string) {
					files.Write(tb, root, files.Tree{keptFile: files.Text(changedBody)})
				},
				want: []string{rerunChanges + keptFile, rerunMoves + keptFile},
			},
			{
				name: "fails a rerun that removes a file",
				rerun: func(tb assert.TB, root string) {
					assert.NoError(tb, os.Remove(filepath.Join(root, filepath.FromSlash(keptFile))), "the file removes")
				},
				want: []string{rerunChanges + keptFile, rerunMoves + keptFile},
			},
			{
				name: "fails a rerun that adds a file",
				rerun: func(tb assert.TB, root string) {
					files.Write(tb, root, files.Tree{addedFile: files.Text(keptBody)})
				},
				want: []string{rerunAdds + addedFile},
			},
			{
				name: "fails a rerun that rewrites the manifest",
				rerun: func(tb assert.TB, root string) {
					files.Write(tb, root, files.Tree{manifestDoc: files.Text(changedBody)})
				},
				want: []string{rerunChanges + manifestDoc, rerunMoves + manifestDoc},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := rerunRoot(t)
				records := assert.Rejects(t, "a rerun that writes outside the sealed state", func(tb assert.TB) {
					workspacetest.AssertRerunUnchanged(tb, root, fixtureBrand, func() { tt.rerun(tb, root) })
				})
				assert.Equal(t, coretest.Contracts(records), tt.want,
					"the rejection names each file that the rerun wrote")
			})
		}
	})
}

// rerunRoot returns a new directory with a file outside the state
// directory, a segment of the sealed state and a document of the
// manifest.
func rerunRoot(t *testing.T) string {
	t.Helper()

	return files.Workspace(t, files.Tree{
		keptFile:    files.Text(keptBody),
		segmentFile: files.Text(keptBody),
		manifestDoc: files.Text(keptBody),
	})
}
