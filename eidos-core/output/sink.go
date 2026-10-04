// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/internal/pathset"
	"go.dokimi.dev/eidos/core/internal/stagefile"
)

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
	// ActionUnchanged reports a path the commit left as it was:
	// identical bytes, whose file and mtime were not touched, or a
	// staged removal that found nothing of the brand's to remove.
	ActionUnchanged
	// ActionDeleted reports a file the commit removed.
	ActionDeleted
)

// String spells the action for a diagnostic or a dry run, and the
// number of an action nothing declares. It allocates nothing for a
// declared action.
func (a Action) String() string {
	switch a {
	case ActionCreated:
		return "created"
	case ActionUpdated:
		return "updated"
	case ActionUnchanged:
		return "unchanged"
	case ActionDeleted:
		return "deleted"
	default:
		return "Action(" + strconv.Itoa(int(a)) + ")"
	}
}

// Found is what a destination path contains before a commit, as the
// brand's trailer proves it. The zero value names no verdict.
type Found uint8

const (
	// FoundNothing reports a path with no file.
	FoundNothing Found = 1
	// FoundSame reports a file whose bytes equal the staged bytes.
	FoundSame Found = 2
	// FoundIntact reports the brand's intact output: a frame under the
	// brand and a body that hashes to its trailer.
	FoundIntact Found = 3
	// FoundDrifted reports the brand's frame over a body edited since
	// its stamp.
	FoundDrifted Found = 4
	// FoundForeign reports a path without the brand's frame: a
	// hand-written file, another tool's output, a generated file whose
	// trailer was deleted, or a directory.
	FoundForeign Found = 5
)

// String spells the verdict for a diagnostic or a dry run, and the
// number of a verdict nothing declares. It allocates nothing for a
// declared verdict.
func (f Found) String() string {
	switch f {
	case FoundNothing:
		return "nothing"
	case FoundSame:
		return "same"
	case FoundIntact:
		return "intact"
	case FoundDrifted:
		return "drifted"
	case FoundForeign:
		return "foreign"
	default:
		return "Found(" + strconv.Itoa(int(f)) + ")"
	}
}

// Written is one committed file's record.
type Written struct {
	// Path is the staged path, workspace-relative.
	Path string
	// Action is what the commit did.
	Action Action
	// Hash is "sha256:" and the hex digest of the file's bytes as
	// written, frame included, and empty for a removed file. The
	// trailer's own digest covers the body alone. This one covers what
	// was written to the destination, which is the value a record of
	// the run keeps.
	Hash string
}

// Change is one staged path before the commit.
type Change struct {
	// Path is the staged path, workspace-relative.
	Path string
	// Action is what Commit does to the path when it proceeds.
	Action Action
	// Found is what the path contains before the commit.
	Found Found
	// Hash is "sha256:" and the hex digest of the staged bytes, empty
	// for a removal.
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
	// climbs out of the root, ends in the reserved staging suffix,
	// was staged before or is staged for removal. It refuses a path
	// that cannot exist on a filesystem beside the staged ones: a file
	// where a staged path needs a directory, a directory where a file
	// is staged, and a name that differs from a staged one only in
	// case. It refuses a call after Prepare, and every call after
	// Commit or Discard with [ErrFinished].
	Write(path string, body []byte) error
	// Delete stages the removal of one file. It refuses an invalid
	// path, a path staged for writing and a path staged before. It
	// refuses a call after Prepare, and every call after Commit or
	// Discard with [ErrFinished].
	Delete(path string) error
	// Prepare reads the destination and returns, per staged path in
	// path order, the action Commit takes and what the path contains
	// now. It writes nothing. A sink prepares once, before Commit or
	// Discard, and refuses a second call.
	Prepare() ([]Change, error)
	// Commit makes the staged files real, one atomic rename per
	// file, write-if-changed: identical bytes leave the file and
	// its mtime untouched. A sink over a destination that has files
	// of its own, such as [Disk], refuses to overwrite a file its
	// brand did not write. It removes a staged removal's file only
	// where the file is the brand's intact output, and leaves any
	// other file in place without a record. Commit returns one record
	// per file it wrote or removed, sorted by path, and keeps going
	// past a file that fails, joining the errors.
	Commit() ([]Written, error)
	// Discard drops the staged files without touching the
	// destination.
	Discard() error
}

// staging is the bookkeeping every sink shares: the staged bytes and
// removals, and the one-staging rule. Each sink decides how it reads
// and writes its destination.
type staging struct {
	files map[string][]byte
	// removals are the paths staged for deletion.
	removals map[string]struct{}
	// tree contains the staged file paths, which a new path must fit
	// beside.
	tree     pathset.Set
	prepared bool
	finished bool
}

// stage records one file, refusing what no sink may take.
func (s *staging) stage(p string, body []byte) error {
	if err := s.open(p); err != nil {
		return err
	}
	if _, held := s.files[p]; held {
		return fmt.Errorf(
			"output: %q is staged twice: one sink writes each path once", p,
		)
	}
	if _, removed := s.removals[p]; removed {
		return fmt.Errorf("output: %q is staged for removal, so it cannot be written", p)
	}
	if err := s.fits(p); err != nil {
		return err
	}
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[p] = body
	s.tree.Add(p)
	return nil
}

// remove records one removal, refusing a path staged before.
func (s *staging) remove(p string) error {
	if err := s.open(p); err != nil {
		return err
	}
	if _, held := s.files[p]; held {
		return fmt.Errorf("output: %q is staged for writing, so it cannot be removed", p)
	}
	if _, removed := s.removals[p]; removed {
		return fmt.Errorf("output: %q is staged for removal twice", p)
	}
	if s.removals == nil {
		s.removals = map[string]struct{}{}
	}
	s.removals[p] = struct{}{}
	return nil
}

// open refuses a path no staging takes, and every path once the
// staging is prepared or finished.
func (s *staging) open(p string) error {
	switch {
	case s.finished:
		return ErrFinished
	case s.prepared:
		return fmt.Errorf("output: %q arrives after the staging was prepared, and a prepared staging is closed", p)
	}
	return stageable(p)
}

// fits refuses a path that cannot exist beside the staged ones on
// every filesystem: a path that differs from a staged one only in
// case, which a case-insensitive filesystem stores as one file, a
// path that a staged path needs as a directory, and a path under a
// path staged as a file. Every comparison folds case.
func (s *staging) fits(p string) error {
	switch clash, other := s.tree.Clash(p); clash {
	case pathset.ClashCase:
		return fmt.Errorf(
			"output: %q and %q differ only in case, and a case-insensitive filesystem "+
				"stores them as one file", other, p,
		)
	case pathset.ClashDirectory:
		return fmt.Errorf("output: %q is a directory %q needs, so it cannot be a file", p, other)
	case pathset.ClashFile:
		return fmt.Errorf("output: %q needs %q as a directory, and it is staged as a file", p, other)
	default:
		return nil
	}
}

// paths returns every staged path, written and removed, in commit
// order. It allocates the returned list alone, and nothing for a
// staging without a path.
func (s *staging) paths() []string {
	all := make([]string, 0, len(s.files)+len(s.removals))
	for p := range s.files {
		all = append(all, p)
	}
	for p := range s.removals {
		all = append(all, p)
	}
	slices.Sort(all)
	return all
}

// prepare marks the staging prepared, refusing a second preparation and
// one after Commit or Discard.
func (s *staging) prepare() error {
	switch {
	case s.finished:
		return ErrFinished
	case s.prepared:
		return errors.New("output: the staging is prepared once")
	}
	s.prepared = true
	return nil
}

// finish closes the staging, refusing a second Commit or Discard.
func (s *staging) finish() error {
	if s.finished {
		return ErrFinished
	}
	s.finished = true
	return nil
}

// stageable reports what refuses path, nil where a sink may take
// it. The string check is the first refusal and the cheap one. A
// sink writing to a filesystem enforces the jail again at the
// commit, where symlinks resolve.
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
	case strings.HasSuffix(p, stagefile.Suffix):
		return fmt.Errorf(
			"output: %q ends in %s, which a commit stages through", p, stagefile.Suffix,
		)
	}
	return nil
}

// planned returns the action Commit takes on a path staged for writing
// or for removal, from what the path contains: a write creates, leaves
// equal bytes alone or updates, and a removal deletes the brand's
// intact output and leaves anything else.
func planned(write bool, f Found) Action {
	switch {
	case write && f == FoundNothing:
		return ActionCreated
	case write && f == FoundSame:
		return ActionUnchanged
	case write:
		return ActionUpdated
	case f == FoundIntact:
		return ActionDeleted
	default:
		return ActionUnchanged
	}
}
