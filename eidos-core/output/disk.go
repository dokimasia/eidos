// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The permissions a commit writes with: generated files are
// ordinary source files, and their directories ordinary
// directories.
const (
	filePerm = 0o644
	dirPerm  = 0o755
)

// Disk stages for a directory tree and commits into it. The root
// is opened once and every staged path resolves inside it, so a
// symlink pointing out of the tree does not escape it either: the
// jail is the operating system's.
//
// A commit overwrites an existing file only when the file is this
// sink's brand's intact output: its trailer names the brand and its
// digest matches its body, which [Contract.Verify] checks the same
// way. A hand-written file, another brand's output and an output
// edited since it was stamped are refused, naming the path, and
// remain as they are, unless [Disk.Overwrite] allowed their verdict. A
// staged removal deletes only the brand's intact output, and leaves any
// other file in place.
//
// # Concurrency
//
// A Disk belongs to one goroutine, as every [Sink] does.
//
// # Allocation contract
//
// A Disk stages as [Mem] does. A read of a path through the root
// allocates seven times: the path's split, its spelling for the system
// call, the open file's name and its two structures, its status and its
// bytes. A read of a path without a file allocates the split, the
// spelling and the error. Each method states what it reads and writes.
type Disk struct {
	staging
	root  *os.Root
	brand Brand
}

var (
	_ Sink       = (*Disk)(nil)
	_ Overwriter = (*Disk)(nil)
)

// NewDisk opens a sink over an existing directory that writes as
// one brand. It refuses a brand outside [Brand.Valid], because the
// sink proves its ownership of a file through the brand, and a root
// it cannot open, because a sink over nothing writes nowhere.
//
// Commit and Discard close the root. The sink serves one staging.
//
// # Allocation contract
//
// NewDisk allocates four times: the sink, the root's two structures,
// and the root path's spelling for the system call.
func NewDisk(root string, brand Brand) (*Disk, error) {
	if !brand.Valid() {
		return nil, fmt.Errorf(
			"output: %q is not a brand: use a lowercase letter followed by lowercase letters, digits and hyphens",
			string(brand),
		)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("output: opening the sink root: %w", err)
	}
	return &Disk{root: r, brand: brand}, nil
}

// Overwrite lets Commit write over the drifted files, the foreign files
// or both, as [Overwriter] states, and allocates nothing.
//
// Error modes are the ones [Overwriter.Overwrite] lists.
func (d *Disk) Overwrite(found ...Found) error { return d.allow(found) }

// Write stages one file. Nothing is written to the tree until Commit.
// It keeps body without copying it, so the caller leaves body unchanged
// until the commit.
//
// Error modes are the ones [Sink.Write] lists.
//
// # Allocation contract
//
// Write allocates what [Mem.Write] does.
func (d *Disk) Write(path string, body []byte) error { return d.stage(path, body) }

// Delete stages the removal of one file. Nothing leaves the tree
// until Commit.
//
// Error modes are the ones [Sink.Delete] lists.
//
// # Allocation contract
//
// Delete allocates what [Mem.Delete] does: twice for the sink's first
// removal.
func (d *Disk) Delete(path string) error { return d.remove(path) }

// Prepare reads every staged path once, in path order, and reports
// what it contains and the action Commit takes on it. A path that is a
// directory is foreign. It writes nothing, and it returns an error for
// a path it cannot read, such as one a symlink leads out of the root.
//
// Error modes: a path it cannot read, a second preparation, and
// [ErrFinished] after Commit or Discard.
//
// # Allocation contract
//
// Prepare allocates the sorted path list, the list of changes, one
// digest per staged file, and the read of every path. A file whose bytes
// differ from the staged ones adds the record [Read] allocates for its
// frame.
func (d *Disk) Prepare() ([]Change, error) {
	if err := d.prepare(); err != nil {
		return nil, err
	}
	paths := d.paths()
	changes := make([]Change, 0, len(paths))
	for _, p := range paths {
		body, write := d.files[p]
		f, err := d.found(p, body, write)
		if err != nil {
			return nil, err
		}
		c := Change{Path: p, Action: planned(write, f), Found: f}
		if write {
			c.Hash = digest(body)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// Commit writes and removes every staged path, in path order:
// identical bytes leave the file and its mtime untouched, a file the
// brand cannot prove it wrote is refused unless [Disk.Overwrite]
// allowed its verdict, and anything else is written to a staging file,
// synced, and renamed over the target, so a reader sees the old file or
// the new one and never half of either. A removal deletes the brand's
// intact output and leaves any other file. A path that fails is one
// error and the rest still commit.
//
// Error modes: a file the brand cannot prove it wrote whose verdict
// Overwrite did not allow, a path it cannot read, write or remove, such
// as a directory, each joined into the returned error, and
// [ErrFinished] after Commit or Discard.
//
// # Allocation contract
//
// Commit allocates the sorted path list, the list of records, one
// digest per staged file, and the read of every path. A file it
// overwrites adds the record [Read] allocates for its frame, and twelve
// allocations for its staging file: the name, the open and the rename.
func (d *Disk) Commit() ([]Written, error) {
	if err := d.finish(); err != nil {
		return nil, err
	}
	defer d.root.Close()

	paths := d.paths()
	records := make([]Written, 0, len(paths))
	var faults []error
	for _, p := range paths {
		var record Written
		var err error
		if body, write := d.files[p]; write {
			record, err = d.commit(p, body)
		} else {
			record, err = d.delete(p)
		}
		switch {
		case err != nil:
			faults = append(faults, err)
		case record.Path != "":
			records = append(records, record)
		}
	}
	return records, errors.Join(faults...)
}

// Discard drops the staging and closes the root. It allocates nothing.
//
// Error modes: a root that fails to close, and [ErrFinished] after
// Commit or Discard.
func (d *Disk) Discard() error {
	if err := d.finish(); err != nil {
		return err
	}
	clear(d.files)
	clear(d.removals)
	if err := d.root.Close(); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	return nil
}

// commit writes one staged file.
func (d *Disk) commit(at string, body []byte) (Written, error) {
	action := ActionCreated
	switch existing, err := d.root.ReadFile(at); {
	case err == nil && bytes.Equal(existing, body):
		return Written{Path: at, Action: ActionUnchanged, Hash: digest(body)}, nil
	case err == nil:
		_, refused := verify(existing, d.brand)
		if refused != nil && d.overwrite&(1<<ownership(existing, d.brand)) == 0 {
			return Written{}, fmt.Errorf("output: refusing to overwrite %q: %w", at, refused)
		}
		action = ActionUpdated
	case !errors.Is(err, fs.ErrNotExist):
		return Written{}, fmt.Errorf("output: reading %q before writing it: %w", at, err)
	}

	if dir := path.Dir(at); dir != "." {
		if err := d.root.MkdirAll(dir, dirPerm); err != nil {
			return Written{}, fmt.Errorf("output: making the directory for %q: %w", at, err)
		}
	}
	if err := stagefile.Replace(d.root, at, body, filePerm); err != nil {
		return Written{}, fmt.Errorf("output: writing %q through its staging file: %w", at, err)
	}
	return Written{Path: at, Action: action, Hash: digest(body)}, nil
}

// delete removes one staged removal's file where it is the brand's
// intact output, and returns the zero record where the path contains
// nothing or a file the brand cannot prove it wrote intact.
func (d *Disk) delete(at string) (Written, error) {
	f, err := d.found(at, nil, false)
	if err != nil || f != FoundIntact {
		return Written{}, err
	}
	if err := d.root.Remove(at); err != nil {
		return Written{}, fmt.Errorf("output: removing %q: %w", at, err)
	}
	return Written{Path: at, Action: ActionDeleted}, nil
}

// found reads what a path contains and classifies it against the
// staged bytes, for a path staged for writing, and the sink's brand. A
// path that is a directory is foreign.
func (d *Disk) found(at string, body []byte, write bool) (Found, error) {
	existing, err := d.root.ReadFile(at)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return FoundNothing, nil
	case err != nil:
		if info, statErr := d.root.Lstat(at); statErr == nil && info.IsDir() {
			return FoundForeign, nil
		}
		return 0, fmt.Errorf("output: reading %q: %w", at, err)
	case write && bytes.Equal(existing, body):
		return FoundSame, nil
	default:
		return ownership(existing, d.brand), nil
	}
}

// ownership classifies a file that exists against one brand: the
// brand's intact output, the brand's frame over a body edited since
// its stamp, or anything else, which is foreign.
func ownership(existing []byte, brand Brand) Found {
	f, held := parse(existing)
	switch {
	case !held || f.record.Brand != brand:
		return FoundForeign
	case !attests(f.body, f.record.Hash):
		return FoundDrifted
	default:
		return FoundIntact
	}
}
