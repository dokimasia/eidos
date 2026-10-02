// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pipelinetest

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
)

// digestPrefix names the digest a record's entry states its file's hash
// in, ahead of the hex digits.
const digestPrefix = "sha256:"

// aged is the instant the idempotence check sets every file's times to
// before the second run, so a rewrite moves an mtime on a file system
// of any timestamp resolution.
var aged = time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)

// AssertClean runs the fixture in root, an empty directory, and checks
// the run: it reports no Error, positions every finding at a file, and
// returns no error. Each Error and each finding without a position fails
// the check on a line of its own, naming the finding's code, before the
// run's error does.
func AssertClean(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	_, report, err := first(tb, f, root)
	for d := range report.Sink.All() {
		if d.Severity == diag.SeverityError {
			tb.Errorf("the run reports the Error %s at %s: %s", d.Code, d.Pos, d.Msg)
		}
		if d.Pos.File == "" {
			tb.Errorf("finding %s %q has no position", d.Code, d.Msg)
		}
	}
	if err != nil {
		tb.Errorf("the run returns an error: %v", err)
	}
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
	assert.Equal(tb, framed(tb, root, w.Brand()), texts(f.Want),
		"the files under the brand's frame are the fixture's wanted files, byte for byte")
}

// AssertRecorded runs the fixture in root, an empty directory, and
// checks the record the run left in the state directory: it lists
// exactly the fixture's wanted paths, each under the composition's one
// plan and with the digest of the file at the path, frame included.
func AssertRecorded(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	w, report, _ := first(tb, f, root)
	plan := report.Plans[0].Name
	want := make(map[string]entry, len(f.Want))
	for path := range f.Want {
		want[path] = entry{plan: plan, hash: digestAt(tb, root, path)}
	}
	got := map[string]entry{}
	for _, e := range recordAt(tb, root, w.Brand()).Files {
		got[e.Path] = entry{plan: e.Plan, hash: e.Hash}
	}
	assert.Equal(tb, got, want,
		"the record lists every wanted file and nothing else, under the plan and with the digest of its bytes")
}

// AssertIdempotent runs the fixture twice in root, an empty directory,
// and checks the second run: it changes the bytes of no file under root
// and moves the mtime of none, the source tree's and the record's
// included. Between the runs every file's times are set to an instant in
// the past, so a rewrite of unchanged bytes moves its mtime.
func AssertIdempotent(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	_, _, _ = first(tb, f, root)
	for _, path := range files(tb, root) {
		assert.NoError(tb, os.Chtimes(at(root, path), aged, aged), "a file of the run's directory ages")
	}
	before := states(tb, root)
	_, _, _ = run(tb, f, root)
	assert.Equal(tb, states(tb, root), before, "a second run changes no byte and moves no mtime")
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
	assert.Equal(tb, framed(tb, two, w2.Brand()), framed(tb, one, w1.Brand()),
		"a run in another directory generates the same bytes")
	assert.Equal(tb, recordAt(tb, two, w2.Brand()).Files, recordAt(tb, one, w1.Brand()).Files,
		"a run in another directory records the same files")
}

// entry is what the record check compares per path: the plan the record
// lists the file under, and the digest it states.
type entry struct {
	plan string
	hash string
}

// state is what the idempotence check compares per file: its bytes, and
// its mtime spelled in UTC to the nanosecond, so a diff shows both.
type state struct {
	body  string
	mtime string
}

// files returns the slash-separated path of every file under root,
// relative to root, in lexical order.
func files(tb assert.TB, root string) []string {
	tb.Helper()

	var out []string
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, path)
		}
		return err
	})
	assert.NoError(tb, err, "the run's directory walks")
	return out
}

// at returns the operating system's spelling of a slash-separated path
// under root.
func at(root, path string) string {
	return filepath.Join(root, filepath.FromSlash(path))
}

// framed returns the bytes of every file under root that has the
// brand's frame, keyed by its slash-separated path relative to root.
func framed(tb assert.TB, root string, brand output.Brand) map[string]string {
	tb.Helper()

	out := map[string]string{}
	for _, path := range files(tb, root) {
		b, err := os.ReadFile(at(root, path))
		assert.NoError(tb, err, "a file of the run's directory reads")
		if p, held := output.Read(b); held && p.Brand == brand {
			out[path] = string(b)
		}
	}
	return out
}

// texts returns the wanted files' bytes as text, keyed as the fixture
// keys them, so a mismatch shows in a diff.
func texts(want map[string][]byte) map[string]string {
	out := make(map[string]string, len(want))
	for path, b := range want {
		out[path] = string(b)
	}
	return out
}

// states returns the state of every file under root, keyed by its
// slash-separated path relative to root.
func states(tb assert.TB, root string) map[string]state {
	tb.Helper()

	out := map[string]state{}
	for _, path := range files(tb, root) {
		b, err := os.ReadFile(at(root, path))
		assert.NoError(tb, err, "a file of the run's directory reads")
		info, err := os.Stat(at(root, path))
		assert.NoError(tb, err, "a file of the run's directory states its times")
		out[path] = state{body: string(b), mtime: info.ModTime().UTC().Format(time.RFC3339Nano)}
	}
	return out
}

// digestAt returns the digest of the file at a slash-separated path
// under root, spelled the way a record's entry states it.
func digestAt(tb assert.TB, root, path string) string {
	tb.Helper()

	b, err := os.ReadFile(at(root, path))
	assert.NoError(tb, err, "a wanted file reads")
	sum := sha256.Sum256(b)
	return digestPrefix + hex.EncodeToString(sum[:])
}

// recordAt returns the record in the state directory under root. It
// stops the check where the directory contains no record or the record
// does not decode.
func recordAt(tb assert.TB, root string, brand output.Brand) manifest.Manifest {
	tb.Helper()

	b, err := os.ReadFile(at(root, ledger.ManifestPath(brand)))
	assert.NoError(tb, err, "the state directory contains the run's record")
	m, err := manifest.Decode(b)
	assert.NoError(tb, err, "the record decodes")
	return m
}
