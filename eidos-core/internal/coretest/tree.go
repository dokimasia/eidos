// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
)

// Directory and file modes a copied fixture tree is written with.
const (
	treeDirPerm  fs.FileMode = 0o750
	treeFilePerm fs.FileMode = 0o600
)

// CopyTree copies the directory src into a fresh temporary
// directory and returns that directory.
//
// A case that runs a generator or a command writing into the tree
// it runs inside works on the copy, so no test writes into the
// repository's own files. src is read from the test's working
// directory, which is the package under test.
func CopyTree(tb testing.TB, src string) string {
	tb.Helper()

	root := tb.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), treeDirPerm); err != nil {
			return err
		}
		return os.WriteFile(target, content, treeFilePerm)
	})
	assert.NoError(tb, err, "the fixture tree "+src+" copies")
	return root
}
