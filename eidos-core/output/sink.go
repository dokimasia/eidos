// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
)

// stageSuffix names the file a commit writes before renaming it
// over the target. It is reserved: a staged path ending in it is
// refused, so a plan's own file can never collide with a commit
// in progress.
const stageSuffix = ".stage"

// ErrFinished reports a sink used after Commit or Discard. A sink
// serves one staging and is not reused.
var ErrFinished = errors.New("output: the sink already committed or discarded")

// Action is what one commit did to one path.
type Action uint8

const (
	// ActionCreated reports a path that did not exist.
	ActionCreated Action = iota
	// ActionUpdated reports a path that existed with different
	// bytes.
	ActionUpdated
	// ActionUnchanged reports identical bytes, so the file and its
	// mtime were not touched.
	ActionUnchanged
)

// String spells the action for a diagnostic or a dry run.
func (a Action) String() string {
	switch a {
	case ActionCreated:
		return "created"
	case ActionUpdated:
		return "updated"
	case ActionUnchanged:
		return "unchanged"
	default:
		return "Action(" + string(rune('0'+byte(a))) + ")"
	}
}

// Written is one committed file's record.
type Written struct {
	// Path is the staged path, workspace-relative.
	Path string
	// Action is what the commit did.
	Action Action
	// Hash is "sha256:" and the hex digest of the file's bytes as
	// written, frame included. The trailer's own digest covers the
	// body alone; this one covers what reached the destination,
	// which is the value a record of the run keeps.
	Hash string
}

// Sink takes stamped files to their destination in two steps:
// stage, then commit. Staging is invisible, so a failed or
// abandoned plan leaves the previous generation of files exactly
// in place, and a dry run is a sink that never commits.
//
// A Sink belongs to one goroutine and serves one staging.
type Sink interface {
	// Write stages one file under a workspace-relative,
	// slash-separated path. It refuses a path that is invalid,
	// climbs out of the root, ends in the reserved staging suffix
	// or was staged before, and it refuses every call after Commit
	// or Discard with [ErrFinished].
	Write(path string, body []byte) error
	// Commit makes the staged files real, one atomic rename per
	// file, write-if-changed: identical bytes leave the file and
	// its mtime untouched. It returns one record per file it
	// committed, sorted by path, and keeps going past a file that
	// fails, joining the errors.
	Commit() ([]Written, error)
	// Discard drops the staged files without touching the
	// destination. A discarded sink held nothing and leaves
	// nothing.
	Discard() error
}

// staging is the bookkeeping every sink shares: the bytes held
// back, and the one-staging rule. The destination is the sink's
// own business.
type staging struct {
	files    map[string][]byte
	finished bool
}

// stage records one file, refusing what no sink may take.
func (s *staging) stage(path string, body []byte) error {
	if s.finished {
		return ErrFinished
	}
	if err := stageable(path); err != nil {
		return err
	}
	if _, held := s.files[path]; held {
		return fmt.Errorf(
			"output: %q is staged twice: one sink writes each path once", path,
		)
	}
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[path] = body
	return nil
}

// paths returns the staged paths in commit order.
func (s *staging) paths() []string { return slices.Sorted(maps.Keys(s.files)) }

// finish closes the staging, refusing a second Commit or Discard.
func (s *staging) finish() error {
	if s.finished {
		return ErrFinished
	}
	s.finished = true
	return nil
}

// stageable reports what refuses path, nil where a sink may take
// it. The string check is the first refusal and the cheap one; a
// sink writing to a filesystem holds the jail again where symlinks
// live.
func stageable(path string) error {
	switch {
	case !fs.ValidPath(path) || path == ".":
		return fmt.Errorf(
			"output: %q is not a workspace-relative, slash-separated file path", path,
		)
	case strings.ContainsRune(path, '\\'):
		return fmt.Errorf(
			"output: %q separates with a backslash, and paths are slash-separated "+
				"on every platform", path,
		)
	case strings.HasSuffix(path, stageSuffix):
		return fmt.Errorf(
			"output: %q ends in %s, which a commit stages through", path, stageSuffix,
		)
	}
	return nil
}
