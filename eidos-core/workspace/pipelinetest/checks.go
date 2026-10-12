// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pipelinetest

import (
	"maps"
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/workspace/internal/rundir"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// AssertClean runs the fixture in root, an empty directory, and checks
// the run: it reports no Error, every finding has a position that
// [diag.Diag.Positioned] accepts, and it returns no error. Each Error
// and each finding without a position fails the check with a record of
// its own, and the contract of the record contains the code of the
// finding. The error of the run fails the check after them.
func AssertClean(tb assert.TB, f workspacetest.Fixture, root string) {
	tb.Helper()

	_, report, err := first(tb, f, root)
	for d := range report.Sink.All() {
		finding := d.Code.String() + " " + d.Msg
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			"the run reports no Error: "+finding+" at "+d.Pos.String())
		expect.True(tb, d.Positioned(), "the finding has a file: "+finding)
	}
	expect.NoError(tb, err, "the run returns no error")
}

// AssertGenerated runs the fixture in root, an empty directory, and
// checks what the run generated: the files under root that have the
// brand's frame are the fixture's wanted files, byte for byte. A wanted
// file the run did not generate, a generated file the fixture does not
// list, and a file whose bytes differ each show in the diff. A file in
// the frame of another brand is not an output of the run, and the check
// passes over it.
func AssertGenerated(tb assert.TB, f workspacetest.Fixture, root string) {
	tb.Helper()

	w, _, _ := first(tb, f, root)
	assert.Equal(tb, rundir.Framed(tb, root, w.Brand()), rundir.Texts(f.Want),
		"the files under the brand's frame are the fixture's wanted files, byte for byte")
}

// AssertRecorded runs the fixture in root, an empty directory, and
// checks the record the run left in the state directory: it lists
// exactly the fixture's wanted paths, each under the composition's one
// plan and with the digest of the file at the path, frame included.
// Each entry the fixture does not want, each entry under another plan
// or with another digest, and each wanted path the record omits fails
// the check with a record of its own, and the contract of the record
// contains the path.
func AssertRecorded(tb assert.TB, f workspacetest.Fixture, root string) {
	tb.Helper()

	w, report, _ := first(tb, f, root)
	plan := report.Plans[0].Name
	wanted := slices.Sorted(maps.Keys(f.Want))
	var listed []string
	for _, e := range rundir.Record(tb, root, w.Brand()).Files {
		listed = append(listed, e.Path)
		expect.Contains(tb, wanted, e.Path, "the fixture wants "+e.Path+", which the record lists")
		if _, held := f.Want[e.Path]; !held {
			continue
		}
		expect.Equal(tb, e.Plan, plan, "the record lists "+e.Path+" under the composition's plan")
		expect.Equal(tb, e.Hash, rundir.Digest(tb, root, e.Path), "the record has the digest of "+e.Path)
	}
	for _, path := range wanted {
		expect.Contains(tb, listed, path, "the record lists "+path+", which the fixture wants")
	}
}

// AssertIdempotent runs the fixture twice in root, an empty directory,
// and checks the second run through [workspacetest.AssertRerunUnchanged].
// The second run ignores the sealed state, so it executes every phase
// again. It changes the bytes of no file under root outside the sealed
// state and moves the modification time of none, the source tree's and
// the manifest's included.
func AssertIdempotent(tb assert.TB, f workspacetest.Fixture, root string) {
	tb.Helper()

	w, _, _ := first(tb, f, root)
	workspacetest.AssertRerunUnchanged(tb, root, w.Brand(), func() { _, _, _ = run(tb, f, root, true) })
}

// AssertRelocated runs the fixture in one and in two, two empty
// directories, and checks that the runs agree: the files under the
// brand's frame are the same bytes at the same paths, and the records
// list the same files. The names of the workspaces in the records may
// differ, because a disk ledger derives the name of a workspace from its
// directory.
func AssertRelocated(tb assert.TB, f workspacetest.Fixture, one, two string) {
	tb.Helper()

	w1, _, _ := first(tb, f, one)
	w2, _, _ := first(tb, f, two)
	expect.Equal(tb, rundir.Framed(tb, two, w2.Brand()), rundir.Framed(tb, one, w1.Brand()),
		"a run in another directory generates the same bytes")
	expect.Equal(tb, rundir.Record(tb, two, w2.Brand()).Files, rundir.Record(tb, one, w1.Brand()).Files,
		"a run in another directory records the same files")
}
