// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// filePerm is the mode a commit writes with: generated files are
// ordinary source files. The group creates their directories as
// ordinary directories.
const filePerm = 0o644

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
// other file in place. A file without a frame is the brand's output only
// at the recorded digest that [Disk.Own] declared for its path.
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
	_ Sink  = (*Disk)(nil)
	_ Owner = (*Disk)(nil)
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
// or both, as [Sink.Overwrite] states, and allocates nothing.
//
// Error modes are the ones [Sink.Overwrite] lists.
func (d *Disk) Overwrite(found ...Found) error { return d.allow(found) }

// Own declares the recorded digest of a file without a frame, as
// [Owner.Own] states. It reads nothing.
//
// # Allocation contract
//
// Own allocates the map of the declared paths on its first call, and what
// the map allocates as it grows.
func (d *Disk) Own(path, hash string) { d.own(path, hash) }

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
		f, _, err := d.found(p, body, write)
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

// Commit writes and removes every staged path in three steps:
//
//  1. A pool of [stagefile.Workers] goroutines reads each staged path and
//     classifies it as Prepare does: identical bytes leave the file and
//     its mtime untouched, and a file that is not the brand's intact
//     output is refused unless [Disk.Overwrite] allowed its verdict.
//     Commit reads each path again, because a file can change after
//     Prepare.
//  2. One [stagefile.Group] with reserved staging names takes every
//     write and every removal of the brand's intact output, and commits
//     them through the barrier of the running system. Each file goes to
//     a staging file that the group renames over its target, so a reader
//     sees the old file or the new one and never half of either. After a
//     crash of the machine, each target contains its old bytes or its new
//     bytes, and once Commit returns, the commit is on stable storage.
//  3. Commit builds the records in path order.
//
// A path that fails is one error, and the rest still commit. A removal
// leaves any file other than the brand's intact output, without a record.
//
// Error modes: a file the brand cannot prove it wrote whose verdict
// Overwrite did not allow, a directory at a staged path, a path it cannot
// read, write or remove, and a barrier of the group that fails, each
// joined into the returned error, and [ErrFinished] after Commit or
// Discard. A barrier that fails before the renames leaves every path as
// it was.
//
// # Allocation contract
//
// Commit allocates the sorted path list, the verdicts, the errors, the
// list of records, one digest per staged file, and the read of every
// path. A file it writes or removes adds the record [Read] allocates for
// its frame, and what the group allocates for it: the staging name, the
// open and the rename of a write, and the removal of a removal. The group
// itself, its barrier and the pool allocate once for each commit.
func (d *Disk) Commit() ([]Written, error) {
	if err := d.finish(); err != nil {
		return nil, err
	}
	defer d.root.Close()

	paths := d.paths()
	verdicts := make([]Found, len(paths))
	faults := make([]error, len(paths), len(paths)+1)
	stagefile.Each(len(paths), func(i int) { verdicts[i], faults[i] = d.verdict(paths[i]) })
	g := stagefile.NewGroup(d.root, stagefile.Reserved, stagefile.System())
	for i, p := range paths {
		body, write := d.files[p]
		switch {
		case faults[i] != nil:
		case write && verdicts[i] != FoundSame:
			g.Write(p, body, filePerm)
		case !write && verdicts[i] == FoundIntact:
			g.Remove(p)
		}
	}
	failed, err := g.Commit()
	records := make([]Written, 0, len(paths))
	for i, p := range paths {
		body, write := d.files[p]
		f := verdicts[i]
		ferr, unapplied := failed[p]
		switch {
		case faults[i] != nil:
		case write && f == FoundSame:
			records = append(records, Written{Path: p, Action: ActionUnchanged, Hash: digest(body)})
		case !write && f != FoundIntact:
		case unapplied && errors.Is(ferr, stagefile.ErrNotApplied):
		case unapplied && write:
			faults[i] = fmt.Errorf("output: writing %q through its staging file: %w", p, ferr)
		case unapplied:
			faults[i] = fmt.Errorf("output: removing %q: %w", p, ferr)
		case write && f == FoundNothing:
			records = append(records, Written{Path: p, Action: ActionCreated, Hash: digest(body)})
		case write:
			records = append(records, Written{Path: p, Action: ActionUpdated, Hash: digest(body)})
		default:
			records = append(records, Written{Path: p, Action: ActionDeleted})
		}
	}
	var commitErr error
	if err != nil {
		commitErr = fmt.Errorf("output: committing the staged files: %w", err)
	}
	return records, errors.Join(append(faults, commitErr)...)
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

// verdict classifies one staged path for the commit: what it contains,
// as Prepare reports it, and the refusal of a write over a directory or
// over a file that is not the brand's intact output, where Overwrite did
// not allow its verdict. It does not keep the bytes of the file that it
// read.
func (d *Disk) verdict(at string) (Found, error) {
	body, write := d.files[at]
	f, existing, err := d.found(at, body, write)
	switch {
	case err != nil || !write || f == FoundNothing || f == FoundSame || f == FoundIntact:
		return f, err
	case existing == nil:
		return f, fmt.Errorf("output: refusing to write over the directory %q", at)
	case d.overwrite&(1<<f) == 0 && f == FoundForeign && d.recorded(at, existing):
		return f, fmt.Errorf("output: refusing to overwrite %q: %w", at, ErrRecordedDigest)
	case d.overwrite&(1<<f) == 0:
		_, refused := verify(existing, d.brand)
		return f, fmt.Errorf("output: refusing to overwrite %q: %w", at, refused)
	default:
		return f, nil
	}
}

// found reads what a path contains and classifies it against the
// staged bytes, for a path staged for writing, and the sink's brand. A
// path that is a directory is foreign. It returns the bytes of a file,
// and nil for a path without a file or with a directory. A read of a file
// returns bytes that are not nil, also for an empty file.
func (d *Disk) found(at string, body []byte, write bool) (Found, []byte, error) {
	existing, err := d.root.ReadFile(at)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return FoundNothing, nil, nil
	case err != nil:
		if info, statErr := d.root.Lstat(at); statErr == nil && info.IsDir() {
			return FoundForeign, nil, nil
		}
		return 0, nil, fmt.Errorf("output: reading %q: %w", at, err)
	case write && bytes.Equal(existing, body):
		return FoundSame, existing, nil
	default:
		return d.ownership(at, existing), existing, nil
	}
}

// ownership classifies a file that exists at a path against the brand of
// the sink: the brand's intact output, the brand's frame over a body
// edited since its stamp, or anything else, which is foreign. A file
// without a frame is intact at the digest that [Disk.Own] declared for
// its path, and foreign at any other.
func (d *Disk) ownership(at string, existing []byte) Found {
	f, held := parse(existing)
	switch hash, owned := d.owned[at]; {
	case !held && owned && attests(existing, hash):
		return FoundIntact
	case !held || f.record.Brand != d.brand:
		return FoundForeign
	case !f.intact():
		return FoundDrifted
	default:
		return FoundIntact
	}
}

// recorded reports whether a file at a path is classified through the
// digest that [Disk.Own] declared: the path has a declared digest, and the
// file has no frame.
func (d *Disk) recorded(at string, existing []byte) bool {
	if _, owned := d.owned[at]; !owned {
		return false
	}
	_, held := parse(existing)
	return !held
}
