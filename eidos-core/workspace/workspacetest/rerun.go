// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest

import (
	"maps"
	"os"
	"slices"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace/internal/rundir"
)

// rerunInstant is the modification time that [AssertRerunUnchanged]
// gives every file before the rerun. It is far enough in the past that a
// rewrite moves the time on a file system of any timestamp resolution.
var rerunInstant = time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)

// fileState is the state of one file that the rerun check compares. It
// contains the bytes and the modification time in UTC to the nanosecond,
// so a diff shows both.
type fileState struct {
	body  string
	mtime string
}

// AssertRerunUnchanged sets the modification time of every file under
// root outside the sealed state of brand to one instant in the past,
// calls rerun, and then compares each file outside the sealed state with
// its state before the call. It fails on each file whose bytes or
// modification time rerun changed, and on each file that rerun added. The
// sealed state is the state directory of brand outside the documents of
// its manifest, which a run rewrites as it records what it read. The
// comparison includes the manifest. Each changed file and each added file
// fails the check with a record of its own, and the contract of the
// record contains the path.
func AssertRerunUnchanged(tb assert.TB, root string, brand output.Brand, rerun func()) {
	tb.Helper()

	for _, path := range rundir.Files(tb, root) {
		if rundir.Sealed(brand, path) {
			continue
		}
		assert.NoError(tb, os.Chtimes(rundir.Path(root, path), rerunInstant, rerunInstant),
			"the modification time of "+path+" moves into the past")
	}
	before := fileStates(tb, root, brand)
	rerun()
	after := fileStates(tb, root, brand)
	paths := slices.Sorted(maps.Keys(before))
	for _, path := range paths {
		expect.Equal(tb, after[path].body, before[path].body, "a second run changes no byte of "+path)
		expect.Equal(tb, after[path].mtime, before[path].mtime, "a second run moves the mtime of no file: "+path)
	}
	for _, path := range slices.Sorted(maps.Keys(after)) {
		expect.Contains(tb, paths, path, "a second run writes no file the first did not: "+path)
	}
}

// fileStates returns the state of every file under root outside the
// sealed state of brand, keyed by its slash-separated path relative to
// root. It stops the check where the bytes or the modification time of a
// file do not read.
func fileStates(tb assert.TB, root string, brand output.Brand) map[string]fileState {
	tb.Helper()

	out := map[string]fileState{}
	for _, path := range rundir.Files(tb, root) {
		if rundir.Sealed(brand, path) {
			continue
		}
		b, err := os.ReadFile(rundir.Path(root, path))
		assert.NoError(tb, err, "a file of the run's directory reads")
		info, err := os.Stat(rundir.Path(root, path))
		assert.NoError(tb, err, "the times of a file of the run's directory read")
		out[path] = fileState{body: string(b), mtime: info.ModTime().UTC().Format(time.RFC3339Nano)}
	}
	return out
}
