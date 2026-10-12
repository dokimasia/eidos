// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// missingDir is a directory of goldens that a case does not create.
const missingDir = "missing"

// The goldens of a fixture are files on disk, so Want reads them and
// AssertGolden matches a run against them.
func TestGolden(t *testing.T) {
	t.Parallel()

	t.Run("Want", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every file under the directory by its path", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{genFile: files.Text(genBody), stubFile: files.Text(stubBody)})
			assert.Equal(t, workspacetest.Want(t, filepath.ToSlash(dir)), map[string][]byte{
				genFile: []byte(genBody), stubFile: []byte(stubBody),
			}, "Want returns each file by its slash-separated path under the directory")
		})

		t.Run("stops the test where the directory does not exist", func(t *testing.T) {
			t.Parallel()

			missing := filepath.ToSlash(filepath.Join(t.TempDir(), missingDir))
			records := assert.Rejects(t, "a directory of goldens that does not exist", func(tb assert.TB) {
				workspacetest.Want(tb, missing)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the goldens under " + missing + " read"},
				"the rejection names the directory")
		})
	})

	t.Run("AssertGolden", func(t *testing.T) {
		t.Parallel()

		t.Run("passes goldens that match the files of the run", func(t *testing.T) {
			t.Parallel()

			f := fixture(t)
			dir := files.Workspace(t, files.Tree{
				genFile: files.Bytes(f.Want[genFile]), stubFile: files.Bytes(f.Want[stubFile]),
			})
			workspacetest.AssertGolden(t, f, t.TempDir(), filepath.ToSlash(dir))
		})

		t.Run("passes the goldens of a fixture of one plan", func(t *testing.T) {
			t.Parallel()

			f := fixture(t)
			f.Plans = func() []workspace.Plan { return []workspace.Plan{mirrors()} }
			dir := files.Workspace(t, files.Tree{genFile: files.Bytes(f.Want[genFile])})
			workspacetest.AssertGolden(t, f, t.TempDir(), filepath.ToSlash(dir))
		})

		t.Run("stops the test where the fixture has no plans", func(t *testing.T) {
			t.Parallel()

			planless := fixture(t)
			planless.Plans = nil
			root, dir := t.TempDir(), t.TempDir()
			records := assert.Rejects(t, "a fixture with no plan to run", func(tb assert.TB) {
				workspacetest.AssertGolden(tb, planless, root, filepath.ToSlash(dir))
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has plans"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("fails a golden whose bytes differ from the file of the run", func(t *testing.T) {
			t.Parallel()

			f := fixture(t)
			dir := files.Workspace(t, files.Tree{
				genFile: files.Bytes(f.Want[stubFile]), stubFile: files.Bytes(f.Want[stubFile]),
			})
			root := t.TempDir()
			records := assert.Rejects(t, "a golden of another file", func(tb assert.TB) {
				workspacetest.AssertGolden(tb, f, root, filepath.ToSlash(dir))
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the golden file " + filepath.Join(dir, filepath.FromSlash(genFile)) +
					" matches the output, and -update writes it",
			}, "the rejection names the golden that differs")
		})
	})
}
