// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pipelinetest

import (
	"maps"
	"os"
	"slices"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace/internal/rundir"
)

// aged is the instant the idempotence check sets every file's times to
// before the second run, so a rewrite moves an mtime on a file system
// of any timestamp resolution.
var aged = time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)

// AssertClean runs the fixture in root, an empty directory, and checks
// the run: it reports no Error, positions every finding at a file, and
// returns no error. Each Error and each finding without a position
// fails the check on a record of its own, whose contract names the
// finding's code, and the run's error fails it after them.
func AssertClean(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	_, report, err := first(tb, f, root)
	for d := range report.Sink.All() {
		finding := d.Code.String() + " " + d.Msg
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			"the run reports no Error: "+finding+" at "+d.Pos.String())
		expect.NotEqual(tb, d.Pos.File, "", "the finding states a file: "+finding)
	}
	expect.NoError(tb, err, "the run returns no error")
}

// AssertGenerated runs the fixture in root, an empty directory, and
// checks what the run generated: the files under root that have the
// brand's frame are the fixture's wanted files, byte for byte. A wanted
// file the run did not generate, a generated file the fixture does not
// list, and a file whose bytes differ each show in the diff. A file
// another brand framed is not the run's, and the check reads past it.
func AssertGenerated(tb assert.TB, f Fixture, root string) {
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
// the check on a record of its own, whose contract names the path.
func AssertRecorded(tb assert.TB, f Fixture, root string) {
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
		expect.Equal(tb, e.Hash, rundir.Digest(tb, root, e.Path), "the record states the digest of "+e.Path)
	}
	for _, path := range wanted {
		expect.Contains(tb, listed, path, "the record lists "+path+", which the fixture wants")
	}
}

// AssertIdempotent runs the fixture twice in root, an empty directory,
// and checks the second run: it changes the bytes of no file under root
// and moves the mtime of none, the source tree's and the manifest's
// included. Between the runs every file's times are set to an instant in
// the past, so a rewrite of unchanged bytes moves its mtime. The sealed
// state outside the manifest is the run's record of what it read, and
// the second run records the files' new times there. Each changed body,
// each moved mtime and each added file fails the check on a record of
// its own, whose contract names the path.
func AssertIdempotent(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	w, _, _ := first(tb, f, root)
	for _, path := range rundir.Files(tb, root) {
		assert.NoError(tb, os.Chtimes(rundir.Path(root, path), aged, aged), "a file of the run's directory ages")
	}
	before := states(tb, root, w.Brand())
	_, _, _ = run(tb, f, root)
	after := states(tb, root, w.Brand())
	paths := slices.Sorted(maps.Keys(before))
	for _, path := range paths {
		expect.Equal(tb, after[path].body, before[path].body, "a second run changes no byte of "+path)
		expect.Equal(tb, after[path].mtime, before[path].mtime, "a second run moves the mtime of no file: "+path)
	}
	for _, path := range slices.Sorted(maps.Keys(after)) {
		expect.Contains(tb, paths, path, "a second run writes no file the first did not: "+path)
	}
}

// AssertRelocated runs the fixture in one and in two, two empty
// directories, and checks that the runs agree: the files under the
// brand's frame are the same bytes at the same paths, and the records
// list the same files. The records' workspace names may differ, because
// a disk ledger names the workspace after its directory.
func AssertRelocated(tb assert.TB, f Fixture, one, two string) {
	tb.Helper()

	w1, _, _ := first(tb, f, one)
	w2, _, _ := first(tb, f, two)
	expect.Equal(tb, rundir.Framed(tb, two, w2.Brand()), rundir.Framed(tb, one, w1.Brand()),
		"a run in another directory generates the same bytes")
	expect.Equal(tb, rundir.Record(tb, two, w2.Brand()).Files, rundir.Record(tb, one, w1.Brand()).Files,
		"a run in another directory records the same files")
}

// state is what the idempotence check compares per file: its bytes, and
// its mtime spelled in UTC to the nanosecond, so a diff shows both.
type state struct {
	body  string
	mtime string
}

// states returns the state of every file under root outside the brand's
// sealed state, keyed by its slash-separated path relative to root.
func states(tb assert.TB, root string, brand output.Brand) map[string]state {
	tb.Helper()

	out := map[string]state{}
	for _, path := range rundir.Files(tb, root) {
		if rundir.Sealed(brand, path) {
			continue
		}
		b, err := os.ReadFile(rundir.Path(root, path))
		assert.NoError(tb, err, "a file of the run's directory reads")
		info, err := os.Stat(rundir.Path(root, path))
		assert.NoError(tb, err, "a file of the run's directory states its times")
		out[path] = state{body: string(b), mtime: info.ModTime().UTC().Format(time.RFC3339Nano)}
	}
	return out
}
