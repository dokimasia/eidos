// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// The extension of the files that the fixture generates, and one that no
// file of the fixture has.
const (
	textExt = ".txt"
	goExt   = ".go"
)

// A run leaves its files and its record on disk, and Generated turns them
// into the input of a toolchain check.
func TestGenerated(t *testing.T) {
	t.Parallel()

	t.Run("Generated", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every file that the record lists", func(t *testing.T) {
			t.Parallel()

			f := fixture(t)
			root := t.TempDir()
			workspacetest.AssertGenerated(t, f, root)
			got := workspacetest.Generated(t, root, fixtureBrand, f.Tree)
			expect.Equal(t, got.Files, f.Want, "the output has the files of the run")
			expect.Equal(t, got.Sources, f.Tree, "the output has the sources", assert.ByIdentity())
		})

		t.Run("returns the files with one of the extensions", func(t *testing.T) {
			t.Parallel()

			f := fixture(t)
			root := t.TempDir()
			workspacetest.AssertGenerated(t, f, root)
			got := workspacetest.Generated(t, root, fixtureBrand, nil, goExt, textExt)
			assert.Equal(t, got.Files, f.Want, "the output has each file with one of the extensions")
		})

		t.Run("leaves out the files with another extension", func(t *testing.T) {
			t.Parallel()

			f := fixture(t)
			root := t.TempDir()
			workspacetest.AssertGenerated(t, f, root)
			got := workspacetest.Generated(t, root, fixtureBrand, nil, goExt)
			assert.Empty(t, got.Files, "the output has no file of the fixture")
		})

		t.Run("stops the test where the record lists no file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a directory without a run", func(tb assert.TB) {
				workspacetest.Generated(tb, root, fixtureBrand, nil)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the record of the run in " + root + " lists its files"}, "the rejection names the record")
		})
	})
}
