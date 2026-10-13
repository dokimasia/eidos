// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/output"
)

// The files that the stream cases stage: two files out of path order, and
// a removal.
const (
	streamFirst      = "a.txt"
	streamSecond     = "b.txt"
	streamRemoved    = "old.txt"
	streamFirstBody  = "first\n"
	streamSecondBody = "second\n"
)

// errStreamWrite is the error of a writer that fails every write.
var errStreamWrite = errors.New("output_test: the stream refuses the write")

// failing is a writer that fails every write and counts the writes.
type failing struct {
	writes int
}

// Write counts the write and returns errStreamWrite.
func (f *failing) Write([]byte) (int, error) {
	f.writes++
	return 0, errStreamWrite
}

// A stream writes the files that its sinks commit to one writer, each under
// a header line, in path order.
func TestStream(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a new sink on each call", func(t *testing.T) {
			t.Parallel()

			s := output.NewStream(&bytes.Buffer{}, "")
			first, err := s.Open()
			assert.NoError(t, err, "the stream opens a sink")
			second, err := s.Open()
			assert.NoError(t, err, "the stream opens a second sink")
			assert.NotEqual(t, first, second, "each call returns a sink of its own", assert.ByIdentity())
		})

		t.Run("returns a sink whose commit writes each file under its header in path order", func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			streamed(t, output.NewStream(&out, ""), streamSecond, streamSecondBody, streamFirst, streamFirstBody)
			assert.Equal(t, out.String(),
				"// === a.txt ===\n"+streamFirstBody+"\n// === b.txt ===\n"+streamSecondBody+"\n",
				"the stream has each file under its header, in path order")
		})

		t.Run("returns a sink whose commit writes the prefix before each path", func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			streamed(t, output.NewStream(&out, "tools/"), streamFirst, streamFirstBody)
			assert.Equal(t, out.String(), "// === tools/a.txt ===\n"+streamFirstBody+"\n", "the header has the prefix")
		})

		t.Run("returns a sink whose commit returns a created record with the digest of each file", func(t *testing.T) {
			t.Parallel()

			records := streamed(t, output.NewStream(&bytes.Buffer{}, ""), streamFirst, streamFirstBody)
			sum := sha256.Sum256([]byte(streamFirstBody))
			assert.Equal(t, records, []output.Written{{
				Path: streamFirst, Action: output.ActionCreated, Hash: "sha256:" + hex.EncodeToString(sum[:]),
			}}, "the commit creates the file")
		})

		t.Run("returns a sink whose commit writes nothing for a removal", func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			sink := sinkOf(t, output.NewStream(&out, ""))
			assert.NoError(t, sink.Delete(streamRemoved), "the removal stages")
			records, err := sink.Commit()
			assert.NoError(t, err, "the commit succeeds")
			assert.Empty(t, records, "the commit removes nothing")
			assert.Empty(t, out.String(), "the stream has nothing")
		})

		t.Run("returns a sink whose prepare reports the empty destination", func(t *testing.T) {
			t.Parallel()

			sink := sinkOf(t, output.NewStream(&bytes.Buffer{}, ""))
			assert.NoError(t, sink.Write(streamFirst, []byte(streamFirstBody)), "the file stages")
			assert.NoError(t, sink.Delete(streamRemoved), "the removal stages")
			changes, err := sink.Prepare()
			assert.NoError(t, err, "the staging prepares")
			sum := sha256.Sum256([]byte(streamFirstBody))
			assert.Equal(t, changes, []output.Change{
				{
					Path: streamFirst, Action: output.ActionCreated, Found: output.FoundNothing,
					Hash: "sha256:" + hex.EncodeToString(sum[:]),
				},
				{Path: streamRemoved, Action: output.ActionUnchanged, Found: output.FoundNothing},
			}, "the write creates its file, and the removal finds nothing")
		})

		t.Run("returns a sink whose discard writes nothing", func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			sink := sinkOf(t, output.NewStream(&out, ""))
			assert.NoError(t, sink.Write(streamFirst, []byte(streamFirstBody)), "the file stages")
			assert.NoError(t, sink.Discard(), "the staging discards")
			assert.Empty(t, out.String(), "the stream has nothing")
		})

		t.Run("returns a sink whose commit joins the error of each write", func(t *testing.T) {
			t.Parallel()

			w := &failing{}
			sink := sinkOf(t, output.NewStream(w, ""))
			assert.NoError(t, sink.Write(streamFirst, []byte(streamFirstBody)), "the first file stages")
			assert.NoError(t, sink.Write(streamSecond, []byte(streamSecondBody)), "the second file stages")
			_, err := sink.Commit()
			assert.ErrorIs(t, err, errStreamWrite, "the commit returns the error of the writer")
			assert.Equal(t, w.writes, 2, "the commit continues with the second file")
		})

		t.Run("returns a sink whose second commit returns ErrFinished", func(t *testing.T) {
			t.Parallel()

			sink := sinkOf(t, output.NewStream(&bytes.Buffer{}, ""))
			_, err := sink.Commit()
			assert.NoError(t, err, "the first commit succeeds")
			_, err = sink.Commit()
			assert.ErrorIs(t, err, output.ErrFinished, "the sink serves one staging")
		})

		t.Run("returns sinks whose commits write one after another", func(t *testing.T) {
			t.Parallel()

			const sinks, files = 4, 16
			var out bytes.Buffer
			s := output.NewStream(&out, "")
			var wg sync.WaitGroup
			for i := range sinks {
				sink := sinkOf(t, s)
				for j := range files {
					path := strconv.Itoa(i) + "/" + strconv.Itoa(j) + ".txt"
					assert.NoError(t, sink.Write(path, []byte(strconv.Itoa(i)+"\n")), "the file stages")
				}
				wg.Go(func() {
					_, err := sink.Commit()
					expect.NoError(t, err, "the commit succeeds")
				})
			}
			wg.Wait()
			var commits []string
			for line := range strings.Lines(out.String()) {
				if path, header := strings.CutPrefix(line, "// === "); header {
					commit, _, _ := strings.Cut(path, "/")
					commits = append(commits, commit)
				}
			}
			assert.Length(t, commits, sinks*files, "the stream has each file of each commit")
			assert.Length(t, slices.Compact(commits), sinks, "the files of one commit follow each other")
		})
	})
}

// sinkOf opens a sink of the stream s, and fails the test on an error.
func sinkOf(tb testing.TB, s *output.Stream) output.Sink {
	tb.Helper()

	sink, err := s.Open()
	assert.NoError(tb, err, "the stream opens a sink")
	return sink
}

// streamed stages each pair of a path and a body of files into a new sink
// of the stream s, commits the sink, and returns its records.
func streamed(tb testing.TB, s *output.Stream, files ...string) []output.Written {
	tb.Helper()

	sink := sinkOf(tb, s)
	for i := 0; i < len(files); i += 2 {
		assert.NoError(tb, sink.Write(files[i], []byte(files[i+1])), "the file stages")
	}
	records, err := sink.Commit()
	assert.NoError(tb, err, "the commit succeeds")
	return records
}
