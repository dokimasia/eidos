// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strconv"
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
		return "Action(" + strconv.Itoa(int(a)) + ")"
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
	// or was staged before. It refuses a path that cannot exist on
	// a filesystem beside the staged ones: a file where a staged path
	// needs a directory, a directory where a file is staged, and a
	// name that differs from a staged one only in case. It refuses
	// every call after Commit or Discard with [ErrFinished].
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
	files map[string][]byte
	// folded maps the lower case of each staged path to the path.
	// dirs contains every directory a staged path needs. fits checks
	// a new path against both.
	folded   map[string]string
	dirs     map[string]struct{}
	finished bool
}

// stage records one file, refusing what no sink may take.
func (s *staging) stage(p string, body []byte) error {
	if s.finished {
		return ErrFinished
	}
	if err := stageable(p); err != nil {
		return err
	}
	if _, held := s.files[p]; held {
		return fmt.Errorf(
			"output: %q is staged twice: one sink writes each path once", p,
		)
	}
	if err := s.fits(p); err != nil {
		return err
	}
	if s.files == nil {
		s.files = map[string][]byte{}
		s.folded = map[string]string{}
		s.dirs = map[string]struct{}{}
	}
	s.files[p] = body
	s.folded[strings.ToLower(p)] = p
	for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
		s.dirs[dir] = struct{}{}
	}
	return nil
}

// fits refuses a path that cannot exist beside the staged ones on
// every filesystem: a path that differs from a staged one only in
// case, which a case-insensitive filesystem stores as one file, a
// path that a staged path needs as a directory, and a path under a
// path staged as a file.
func (s *staging) fits(p string) error {
	if other, held := s.folded[strings.ToLower(p)]; held {
		return fmt.Errorf(
			"output: %q and %q differ only in case, and a case-insensitive filesystem "+
				"stores them as one file", other, p,
		)
	}
	if _, held := s.dirs[p]; held {
		return fmt.Errorf("output: %q is a directory of staged files, so it cannot be a file", p)
	}
	for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
		if _, held := s.files[dir]; held {
			return fmt.Errorf("output: %q needs %q as a directory, and it is staged as a file", p, dir)
		}
	}
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
func stageable(p string) error {
	switch {
	case !fs.ValidPath(p) || p == ".":
		return fmt.Errorf(
			"output: %q is not a workspace-relative, slash-separated file path", p,
		)
	case strings.ContainsRune(p, '\\'):
		return fmt.Errorf(
			"output: %q separates with a backslash, and paths are slash-separated "+
				"on every platform", p,
		)
	case strings.HasSuffix(p, stageSuffix):
		return fmt.Errorf(
			"output: %q ends in %s, which a commit stages through", p, stageSuffix,
		)
	}
	return nil
}
