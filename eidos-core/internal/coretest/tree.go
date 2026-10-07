// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"
)

// CopyTree copies the directory src into a fresh temporary directory
// with [os.CopyFS] and returns that directory. A file of the copy has
// mode 0o666 with the execute bits of its source, and a directory 0o777,
// before the umask.
//
// A case that runs a generator or a command writing into the tree
// it runs inside works on the copy, so no test writes into the
// repository's own files. src is read from the test's working
// directory, which is the package under test.
func CopyTree(tb testing.TB, src string) string {
	tb.Helper()

	root := tb.TempDir()
	assert.NoError(tb, os.CopyFS(root, os.DirFS(src)), "the fixture tree "+src+" copies")
	return root
}
