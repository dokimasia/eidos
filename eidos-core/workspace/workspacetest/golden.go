// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/core/workspace/internal/rundir"
)

// Want returns every file under dir, keyed by its slash-separated path
// relative to dir: the files that the plans of a fixture generate. dir is
// a slash-separated directory relative to the directory of the test.
// Where dir does not exist and the test runs with -update, Want returns
// an empty map, because the update creates the directory. It stops the
// test where dir or a file under it does not read.
func Want(tb assert.TB, dir string) map[string][]byte {
	tb.Helper()

	root := filepath.FromSlash(dir)
	_, err := os.Stat(root)
	if errors.Is(err, fs.ErrNotExist) && golden.ShouldUpdate() {
		return map[string][]byte{}
	}
	tree := os.DirFS(root)
	out := map[string][]byte{}
	err = fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		out[path], err = fs.ReadFile(tree, path)
		return err
	})
	assert.NoError(tb, err, "the goldens under "+dir+" read")
	return out
}

// AssertGolden runs the fixture's plans in root, an empty directory, and
// matches each file that the record of the run lists against the file at
// its path under dir through [golden.MatchAt], in path order. dir is a
// slash-separated directory relative to the directory of the test. The
// fixture may have one plan or more, so a pipeline fixture matches its
// goldens too. Under -update the match writes each file into dir.
// AssertGolden deletes no golden that the run no longer generates. It
// stops the test where the fixture has no plans, where the run returns
// an error, and at the first file that does not match its golden.
func AssertGolden(tb assert.TB, f Fixture, root, dir string) {
	tb.Helper()

	assert.NotNil(tb, f.Plans, "the fixture has plans")
	copied(tb, f, root)
	w := built(tb, f, root, f.Plans(), nil)
	_, err := ran(tb, f, w, root)
	assert.NoError(tb, err, "the fixture's plans run clean")
	for _, e := range rundir.Record(tb, root, w.Brand()).Files {
		got, err := os.ReadFile(rundir.Path(root, e.Path))
		assert.NoError(tb, err, "the run writes "+e.Path)
		golden.MatchAt(tb, filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(e.Path)), got,
			golden.ShouldUpdate())
	}
}
