// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"time"

	"go.dokimi.dev/eidos/core/internal/stagefile"
	"go.dokimi.dev/eidos/core/output"
)

// The permissions a write creates files and directories with, the ones
// the output sinks write generated files and their directories with.
const (
	filePerm = 0o644
	dirPerm  = 0o755
)

// Dir is the ledger of a directory on disk. Each name maps to a file
// below the directory, resolved through an [os.Root], so a symlink does
// not lead a read or a write outside it. Each call opens the root,
// resolves the name inside it and closes it, so a Dir keeps no file
// open between calls.
//
// # Concurrency
//
// A Dir is safe for concurrent use, and so are two Dirs over one
// directory in two processes. [Dir.Write] and [Dir.Put] stage each blob
// in a file of their own and rename it over the name, so a reader sees
// one writer's whole bytes. [Dir.Lock] admits one holder of the
// directory's lock at a time across every Dir over the directory.
//
// # Allocation contract
//
// Every call allocates for the root it opens and the path it resolves.
// [Dir.Read] allocates the blob's bytes, [Dir.List] one [Blob] per file
// it lists, and [Dir.Lock] the lock file, the holder's record and the
// release.
type Dir struct {
	// root is the absolute directory the jail opens: the workspace root
	// for OpenDir, and the ledger's own directory for OpenAt.
	root string
	// base is the ledger's directory inside root: .<brand> for OpenDir,
	// and the root itself, ".", for OpenAt.
	base string
	// creates reports whether a write creates root itself, which OpenAt's
	// directory may not yet exist for.
	creates bool
	// workspace is the base name of the workspace root, and empty for
	// OpenAt.
	workspace string
}

var (
	_ Ledger = (*Dir)(nil)
	_ Locker = (*Dir)(nil)
)

// OpenDir returns the ledger of the state directory .<brand>/ under
// root, which the first write creates.
//
// Error modes: a brand outside [output.Brand.Valid], and a root that
// does not exist or is not a directory.
func OpenDir(root string, brand output.Brand) (*Dir, error) {
	if !brand.Valid() {
		return nil, fmt.Errorf(
			"ledger: %q is not a brand: use a lowercase letter followed by lowercase letters, digits and hyphens",
			string(brand),
		)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("ledger: resolving the workspace root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("ledger: opening the workspace root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("ledger: the workspace root %s is not a directory", root)
	}
	return &Dir{root: abs, base: StateDir(brand), workspace: filepath.Base(abs)}, nil
}

// OpenAt returns the ledger of the directory dir itself, which the first
// write creates: the location of a parse memo that more than one
// workspace shares.
//
// Error modes: a dir that exists and is not a directory, and a dir that
// does not resolve to an absolute path.
func OpenAt(dir string) (*Dir, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("ledger: resolving the ledger directory: %w", err)
	}
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("ledger: opening the ledger directory: %w", err)
	case !info.IsDir():
		return nil, fmt.Errorf("ledger: the ledger directory %s is not a directory", dir)
	}
	return &Dir{root: abs, base: ".", creates: true}, nil
}

// Workspace returns the base name of the workspace root, which a
// manifest that names no workspace is recorded under, and the empty
// string for a ledger [OpenAt] returned.
func (d *Dir) Workspace() string { return d.workspace }

// Lock takes the lock of the ledger's directory for h. It opens the
// directory's lock file, which the first call creates with the directory,
// and takes the operating system's exclusive lock on it without waiting:
// flock(2) on Linux, the BSDs, Darwin and illumos, and an open with a
// share mode of zero on Windows. It then records h in lock.json beside
// the lock file, which a contender reads to name the holder.
//
// The release removes lock.json, ends the lock and closes the lock file,
// so the record exists while its holder has the lock, and the lock file
// remains. It ends the lock before the close, because a child process
// that a fork created holds a copy of the file's descriptor until its
// exec, and that copy would keep the lock past a close alone. The release
// ignores the cancellation of the context that Lock received.
// The operating system also releases the lock when the process exits, so
// a crash leaves no stale lock, and the next holder replaces the record
// that the crash left. Another open file has the lock while it is open,
// in this process or in another one, so two Dirs over one directory
// exclude each other as two processes do.
//
// Error modes: a *[LockedError], which wraps [ErrLocked], where another
// open file has the lock, with the zero [Holder] where lock.json does not
// read; the context's error; an error wrapping [errors.ErrUnsupported] on
// a platform without file locks; and the operating system's error where
// the lock file cannot be created, opened or locked, or the record cannot
// be written. The release returns the errors of the record's removal, of
// the lock's end and of the lock file's close.
func (d *Dir) Lock(ctx context.Context, h Holder) (func() error, error) {
	r, at, err := d.resolve(ctx, lockName, true)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	if err = ensure(r, path.Dir(at), stagefile.Unsynced); err != nil {
		return nil, fmt.Errorf("ledger: lock: %w", err)
	}
	f, err := lockFile(r, at, filepath.Join(d.root, filepath.FromSlash(at)))
	if errors.Is(err, errBusy) {
		return nil, &LockedError{Holder: d.holder(ctx)}
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: lock: %w", err)
	}
	// drop ends the lock and closes the lock file, on a failure below and
	// at the release.
	drop := func() error {
		if unlocked := unlockFile(f); unlocked != nil {
			return errors.Join(fmt.Errorf("ledger: unlock: %w", unlocked), f.Close())
		}
		if closed := f.Close(); closed != nil {
			return fmt.Errorf("ledger: unlock: %w", closed)
		}
		return nil
	}
	record, err := json.Marshal(h)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("ledger: lock: %w", err), drop())
	}
	if err = d.Put(ctx, holderName, record); err != nil {
		return nil, errors.Join(err, drop())
	}
	unlock := context.WithoutCancel(ctx)
	return func() error { return errors.Join(d.Remove(unlock, holderName), drop()) }, nil
}

// Read returns the file a name maps to, whole.
//
// Error modes: an invalid name wraps [fs.ErrInvalid], a name nothing
// wrote wraps [fs.ErrNotExist], and a file that does not read returns
// the operating system's error.
func (d *Dir) Read(ctx context.Context, name string) ([]byte, error) {
	r, at, err := d.resolve(ctx, name, false)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := r.ReadFile(at)
	if err != nil {
		return nil, fmt.Errorf("ledger: read %s: %w", name, err)
	}
	return b, nil
}

// ReadAt reads len(p) bytes of the file a name maps to from offset off,
// under the contract of [io.ReaderAt]: fewer bytes than len(p) return
// [io.EOF] or another error beside the count.
//
// Error modes: an invalid name wraps [fs.ErrInvalid], a name nothing
// wrote wraps [fs.ErrNotExist], and a read past the end returns
// [io.EOF] unwrapped, as io.ReaderAt states it.
func (d *Dir) ReadAt(ctx context.Context, name string, p []byte, off int64) (int, error) {
	r, at, err := d.resolve(ctx, name, false)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	f, err := r.Open(at)
	if err != nil {
		return 0, fmt.Errorf("ledger: read %s: %w", name, err)
	}
	defer f.Close()
	n, err := f.ReadAt(p, off)
	if errors.Is(err, io.EOF) {
		return n, io.EOF
	}
	if err != nil {
		return n, fmt.Errorf("ledger: read %s: %w", name, err)
	}
	return n, nil
}

// Write replaces the file a name maps to through a staging file of its
// own, synced, then syncs the file's directory. A directory the write
// creates is synced into its parent, so once Write returns, a crash of
// the machine keeps the new bytes.
//
// Error modes: an invalid name wraps [fs.ErrInvalid], and a directory or
// a file that cannot be created, written, synced or renamed returns the
// operating system's error, with the old file remaining in place.
func (d *Dir) Write(ctx context.Context, name string, b []byte) error {
	return d.replace(ctx, name, b, stagefile.Synced)
}

// Put replaces the file a name maps to through a staging file of its
// own, without a sync of the file or of a directory.
//
// Error modes: those of [Dir.Write].
func (d *Dir) Put(ctx context.Context, name string, b []byte) error {
	return d.replace(ctx, name, b, stagefile.Unsynced)
}

// Touch sets the modification and access times of a name's file to the
// current time.
//
// Error modes: an invalid name wraps [fs.ErrInvalid], and a name nothing
// wrote wraps [fs.ErrNotExist].
func (d *Dir) Touch(ctx context.Context, name string) error {
	r, at, err := d.resolve(ctx, name, false)
	if err != nil {
		return err
	}
	defer r.Close()
	now := time.Now()
	if err := r.Chtimes(at, now, now); err != nil {
		return fmt.Errorf("ledger: touch %s: %w", name, err)
	}
	return nil
}

// Remove deletes the file a name maps to, without a sync of its
// directory: a crash of the machine can undo the removal. A name nothing
// wrote, and a ledger directory that does not exist, are not errors.
//
// Error modes: an invalid name wraps [fs.ErrInvalid], and a file the
// operating system refuses to remove returns its error.
func (d *Dir) Remove(ctx context.Context, name string) error {
	r, at, err := d.resolve(ctx, name, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.Remove(at); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("ledger: remove %s: %w", name, err)
	}
	return nil
}

// List walks the directory a name maps to and returns every regular
// file below it, at any depth, sorted by name. A directory that does not
// exist lists nothing.
//
// Error modes: an invalid dir wraps [fs.ErrInvalid], and a directory
// that does not walk returns the operating system's error.
func (d *Dir) List(ctx context.Context, dir string) ([]Blob, error) {
	r, at, err := d.resolve(ctx, dir, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out []Blob
	err = fs.WalkDir(r.FS(), at, func(p string, e fs.DirEntry, err error) error {
		switch {
		case p == at && errors.Is(err, fs.ErrNotExist):
			return fs.SkipAll
		case err != nil:
			return err
		case p == at || !e.Type().IsRegular():
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		out = append(out, Blob{Name: path.Join(dir, p[len(at)+1:]), Size: info.Size(), ModTime: info.ModTime()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ledger: list %s: %w", dir, err)
	}
	slices.SortFunc(out, func(a, b Blob) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

// holder returns the record of the lock's holder in lock.json, and the
// zero [Holder] where the record does not read or decode, such as before
// the holder wrote it.
func (d *Dir) holder(ctx context.Context) Holder {
	var h Holder
	b, err := d.Read(ctx, holderName)
	if err != nil {
		return h
	}
	if err := json.Unmarshal(b, &h); err != nil {
		return Holder{}
	}
	return h
}

// replace creates the directories a name's file needs and replaces the
// file through a staging file of its own, at the durability d states: a
// synced write syncs every directory it creates into its parent, and the
// file's directory after the rename.
func (d *Dir) replace(ctx context.Context, name string, b []byte, durability stagefile.Durability) error {
	r, at, err := d.resolve(ctx, name, true)
	if err != nil {
		return err
	}
	defer r.Close()
	dir := path.Dir(at)
	if err := ensure(r, dir, durability); err != nil {
		return fmt.Errorf("ledger: write %s: %w", name, err)
	}
	if err := stagefile.ReplaceShared(r, at, b, filePerm, durability); err != nil {
		return fmt.Errorf("ledger: write %s: %w", name, err)
	}
	if durability == stagefile.Synced {
		if err := syncDir(r, dir); err != nil {
			return fmt.Errorf("ledger: write %s: %w", name, err)
		}
	}
	return nil
}

// resolve checks a name and opens the jail, and returns the root and the
// name's path inside it. A write creates the root of an [OpenAt] ledger
// where it does not exist. The caller closes the root.
func (d *Dir) resolve(ctx context.Context, name string, write bool) (*os.Root, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if err := checkName(name); err != nil {
		return nil, "", err
	}
	if write && d.creates {
		if err := os.MkdirAll(d.root, dirPerm); err != nil {
			return nil, "", fmt.Errorf("ledger: making the ledger directory: %w", err)
		}
	}
	r, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, "", fmt.Errorf("ledger: opening %s: %w", d.root, err)
	}
	return r, path.Join(d.base, name), nil
}

// ensure creates dir and every directory above it that is missing,
// inside the root, one level at a time. Where durability is
// [stagefile.Synced] it syncs the parent of each directory it creates,
// so the new directory survives a crash of the machine.
func ensure(r *os.Root, dir string, durability stagefile.Durability) error {
	if dir == "." {
		return nil
	}
	info, err := r.Stat(dir)
	switch {
	case err == nil && info.IsDir():
		return nil
	case err == nil:
		return fmt.Errorf("%s is not a directory", dir)
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("checking %s: %w", dir, err)
	}
	parent := path.Dir(dir)
	if err := ensure(r, parent, durability); err != nil {
		return err
	}
	if err := r.Mkdir(dir, dirPerm); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if durability == stagefile.Synced {
		return syncDir(r, parent)
	}
	return nil
}

// syncDir syncs a directory inside the root, so an entry created or
// renamed into it is on disk before the call returns.
func syncDir(r *os.Root, dir string) error {
	f, err := r.Open(dir)
	if err != nil {
		return fmt.Errorf("opening %s to sync it: %w", dir, err)
	}
	err = f.Sync()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("syncing %s: %w", dir, err)
	}
	return nil
}
