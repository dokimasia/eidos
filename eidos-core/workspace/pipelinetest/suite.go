// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pipelinetest

import (
	"context"
	"io/fs"
	"os"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/workspace"
)

// Fixture is one plan's end-to-end case: a source tree, the stores its
// load reads, the composition it runs under, and every file the run
// generates.
type Fixture struct {
	// Tree is the source tree. A check copies it into each directory it
	// runs the fixture in.
	Tree fs.FS
	// Stores are the read-only trees the load's dependency units read,
	// keyed by store name.
	Stores map[string]fs.FS
	// Compose builds the workspace over root, the directory a check runs
	// the fixture in: its output is a disk sink at root, and its ledger
	// is the state directory under root. A check composes once per run,
	// and the suite runs its checks in parallel, so Compose builds fresh
	// plugin instances on every call and is safe for concurrent use.
	Compose func(root string) (*workspace.Workspace, error)
	// Want is every file the run generates, frame included, keyed by its
	// workspace-relative, slash-separated path.
	Want map[string][]byte
}

// RunPipelineSuite checks the fixture's one plan against the pipeline
// contract: [AssertClean], [AssertGenerated], [AssertRecorded],
// [AssertIdempotent] and [AssertRelocated], each in a parallel subtest
// over temporary directories of its own.
func RunPipelineSuite(t *testing.T, f Fixture) {
	t.Helper()

	t.Run("runs clean", func(t *testing.T) {
		t.Parallel()
		AssertClean(t, f, t.TempDir())
	})
	t.Run("generates the wanted files", func(t *testing.T) {
		t.Parallel()
		AssertGenerated(t, f, t.TempDir())
	})
	t.Run("records every generated file", func(t *testing.T) {
		t.Parallel()
		AssertRecorded(t, f, t.TempDir())
	})
	t.Run("changes nothing on a second run", func(t *testing.T) {
		t.Parallel()
		AssertIdempotent(t, f, t.TempDir())
	})
	t.Run("generates the same files in another directory", func(t *testing.T) {
		t.Parallel()
		AssertRelocated(t, f, t.TempDir(), t.TempDir())
	})
}

// first copies the fixture's tree into root, an empty directory, and
// runs the fixture there. It stops the check where the fixture states no
// tree or the tree does not copy.
func first(tb assert.TB, f Fixture, root string) (*workspace.Workspace, *workspace.Report, error) {
	tb.Helper()

	assert.NotNil(tb, f.Tree, "the fixture states a tree")
	assert.NoError(tb, os.CopyFS(root, f.Tree), "the fixture's tree copies into the run's directory")
	return run(tb, f, root, false)
}

// run composes the fixture's workspace over root and runs it over the
// tree at root, reading the fixture's stores, and returns the workspace,
// the report and the run's error. Where cold is set, the run ignores the
// sealed state and executes every phase. It stops the check where the
// fixture states no composition, the workspace does not compose, or the
// composition runs other than one plan.
func run(tb assert.TB, f Fixture, root string, cold bool) (*workspace.Workspace, *workspace.Report, error) {
	tb.Helper()

	assert.NotNil(tb, f.Compose, "the fixture states a composition")
	w, err := f.Compose(root)
	assert.NoError(tb, err, "the fixture's workspace composes")
	report, err := w.Run(context.Background(), workspace.Input{Tree: os.DirFS(root), Stores: f.Stores, Cold: cold})
	assert.Length(tb, report.Plans, 1, "the suite checks one plan, and the composition runs one")
	return w, report, err
}
