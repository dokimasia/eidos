// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// The header of a file in a stream is headerOpen, the prefix and the path
// of the file, and headerClose.
const (
	headerOpen  = "// === "
	headerClose = " ===\n"
)

// Stream writes the files that its sinks commit to one writer, such as
// standard output. Each file is a header line, // === <path> ===, then the
// body and a newline. A sink of the stream commits its files in path order
// under the lock of the stream, so the files of two commits never mix. A
// run commits its plans one after another in commit order, so the output
// follows that order too.
//
// # Concurrency
//
// A Stream is safe for concurrent use. Each sink that [Stream.Open]
// returns belongs to one goroutine, as every [Sink] does.
//
// # Allocation contract
//
// [NewStream] allocates the stream, and Open allocates the sink. A sink
// stages and prepares as [Mem] does, and its commit allocates what Mem's
// commit allocates and what the formatting of each header allocates.
type Stream struct {
	mu     sync.Mutex
	w      io.Writer
	prefix string
}

// NewStream returns a stream over w. prefix comes before the path of each
// file in its header, such as the name of a member of a list and a slash,
// and is empty for the paths as a workspace stages them. It allocates the
// stream alone.
func NewStream(w io.Writer, prefix string) *Stream {
	return &Stream{w: w, prefix: prefix}
}

// Open returns a new sink that commits into the stream. Its destination
// starts empty, so its Prepare reports each write as the creation of an
// absent file and each removal as a removal that finds nothing, and its
// Commit removes no file. Open returns no error, and its signature lets a
// composition pass it to the builder of a workspace as its output. It is
// safe for concurrent use.
func (s *Stream) Open() (Sink, error) {
	return &streamSink{stream: s}, nil
}

// streamSink stages in memory and commits into a stream. It stages,
// prepares and discards as [Mem] does.
type streamSink struct {
	Mem
	stream *Stream
}

var _ Sink = (*streamSink)(nil)

// Commit writes each staged file to the stream in path order, under the
// lock of the stream, and returns the record of each file as [Mem.Commit]
// returns it.
//
// Error modes: [ErrFinished] after Commit or Discard, and an error that
// joins the error of each write to the writer of the stream that fails.
// Commit continues with the next file after such a write.
func (k *streamSink) Commit() ([]Written, error) {
	records, err := k.Mem.Commit()
	if err != nil {
		return nil, err
	}
	k.stream.mu.Lock()
	defer k.stream.mu.Unlock()
	var errs []error
	for _, r := range records {
		_, werr := fmt.Fprintf(k.stream.w, "%s%s%s%s%s\n", headerOpen, k.stream.prefix, r.Path, headerClose,
			k.committed[r.Path])
		if werr != nil {
			errs = append(errs, fmt.Errorf("output: write %s to the stream: %w", r.Path, werr))
		}
	}
	return records, errors.Join(errs...)
}
