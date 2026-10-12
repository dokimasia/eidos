// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pipelinetest

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// RunPipelineSuite checks the fixture's one plan against the pipeline
// contract: [AssertClean], [AssertGenerated], [AssertRecorded],
// [AssertIdempotent] and [AssertRelocated], each in a parallel subtest
// over temporary directories of its own. It reads the tree, the stores,
// the composition, the plans and the wanted files of the fixture, and
// does not read its edit. The plans of the fixture are one plan.
func RunPipelineSuite(t *testing.T, f workspacetest.Fixture) {
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
// runs the fixture there. It stops the check where the fixture has no
// tree or the tree does not copy.
func first(tb assert.TB, f workspacetest.Fixture, root string) (*workspace.Workspace, *workspace.Report, error) {
	tb.Helper()

	assert.NotNil(tb, f.Tree, "the fixture has a tree")
	assert.NoError(tb, os.CopyFS(root, f.Tree), "the fixture's tree copies into the run's directory")
	return run(tb, f, root, false)
}

// run composes the fixture's workspace over root with the fixture's
// plans, runs it over the tree at root under the context of tb, reading
// the fixture's stores, and returns the workspace, the report and the
// run's error. Where cold is set, the run ignores the sealed state and
// executes every phase. It stops the check where the fixture has no
// composition or no plans, the composition returns an error, the
// workspace does not build, or the composition runs other than one plan.
func run(
	tb assert.TB, f workspacetest.Fixture, root string, cold bool,
) (*workspace.Workspace, *workspace.Report, error) {
	tb.Helper()

	assert.NotNil(tb, f.Compose, "the fixture has a composition")
	assert.NotNil(tb, f.Plans, "the fixture has plans")
	b, err := f.Compose(root)
	assert.NoError(tb, err, "the fixture's composition configures")
	w, err := b.Plans(f.Plans()...).Build()
	assert.NoError(tb, err, "the fixture's workspace composes")
	report, err := w.Run(assert.Context(tb), workspace.Input{Tree: os.DirFS(root), Stores: f.Stores, Cold: cold})
	assert.Length(tb, report.Plans, 1, "the suite checks one plan, and the composition runs one")
	return w, report, err
}
