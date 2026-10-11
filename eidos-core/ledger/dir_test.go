// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/ledger"
)

// past is the mtime a fixture sets on a blob, so a touch moves the time.
var past = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// The ceilings of the disk ledger's calls over a state directory of two
// documents. Every call other than an open opens the root, resolves the
// name inside it and closes it.
const (
	// openDirAllocs is one open: the absolute path, the root's stat, the
	// state directory's name, and the ledger.
	openDirAllocs = 4
	// openAtAllocs is one open of a directory: the absolute path, its
	// stat, and the ledger.
	openAtAllocs = 3
	// dirReadAllocs is one read: the root, the name's path, the file it
	// opens with its stat, and the bytes it reads.
	dirReadAllocs = 15
	// dirReadAtAllocs is one read of a range: the root, the name's path,
	// and the file it opens.
	dirReadAtAllocs = 13
	// dirWriteAllocs is one durable replacement: the root, the name's
	// path, the staging file's name, write, sync and rename, and the sync
	// of the file's directory.
	dirWriteAllocs = 41
	// dirWriteAllAllocs is one durable replacement of two documents in
	// one group: the sorted names, the root, the group and its barrier, the
	// paths, the pool, and what the group allocates for the two writes. For
	// each write that is the stat of its directory and the staging file's
	// name, open and rename, and for the group it is the barrier's open of
	// the root and the open and the sync of the directory. os.Root splits
	// each path that it resolves, and copies each element into the bytes
	// that a system call takes.
	dirWriteAllAllocs = 93
	// dirPutAllocs is one replacement without a sync: the root, the
	// name's path, and the staging file's name, write and rename.
	dirPutAllocs = 34
	// dirTouchAllocs is one touch: the root, the name's path, and the
	// change of the file's times.
	dirTouchAllocs = 13
	// dirRemoveAllocs is one removal of a blob whose name has four
	// elements: the root, the name's path, and the removal, which the
	// root resolves one element at a time.
	dirRemoveAllocs = 13
	// dirListAllocs is one listing of the manifest's directory: the root,
	// the walk of the directory, and a blob for each of its two files.
	dirListAllocs = 34
)

// The disk ledger stores each blob as a file of the brand's state
// directory, resolved inside the workspace root, and replaces it through
// a staging file of its own.
func TestDir(t *testing.T) {
	t.Parallel()

	t.Run("OpenDir", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for an invalid brand", func(t *testing.T) {
			t.Parallel()

			_, err := ledger.OpenDir(t.TempDir(), "Acme")
			assert.HasError(t, err, "the brand is refused")
		})

		t.Run("returns an error for a root that does not exist", func(t *testing.T) {
			t.Parallel()

			_, err := ledger.OpenDir(filepath.Join(t.TempDir(), "missing"), brand)
			assert.HasError(t, err, "the root is refused")
		})

		t.Run("returns an error for a root that is a file", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"file": files.Text("")})
			_, err := ledger.OpenDir(filepath.Join(dir, "file"), brand)
			assert.HasError(t, err, "the root is refused")
		})
	})

	t.Run("OpenAt", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a ledger whose first write creates its directory", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "memo", "shared")
			d, err := ledger.OpenAt(dir)
			assert.NoError(t, err, "a missing directory opens")
			assert.NoError(t, d.Put(t.Context(), docName, []byte(docBody)), "the first write creates it")
			files.HasContent(t, filepath.Join(dir, filepath.FromSlash(docName)), docBody,
				"the blob is a file of the directory itself, with its bytes")
		})

		t.Run("returns an error for a path that is a file", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"file": files.Text("")})
			_, err := ledger.OpenAt(filepath.Join(dir, "file"))
			assert.HasError(t, err, "the path is refused")
		})

		t.Run("returns an error for a path whose parent does not stat", func(t *testing.T) {
			t.Parallel()

			if runtime.GOOS == "windows" {
				t.Skip("Windows reports a path below a file as missing, so only the first write fails")
			}
			dir := files.Workspace(t, files.Tree{"file": files.Text("")})
			_, err := ledger.OpenAt(filepath.Join(dir, "file", "below"))
			assert.HasError(t, err, "a path below a file is refused")
		})

		t.Run("returns a ledger whose write fails where its directory cannot be made", func(t *testing.T) {
			t.Parallel()

			parent := filepath.Join(t.TempDir(), "memo")
			d, err := ledger.OpenAt(filepath.Join(parent, "shared"))
			assert.NoError(t, err, "a missing directory opens")
			assert.NoError(t, os.WriteFile(parent, nil, 0o600), "a file takes the parent's place")
			assert.HasError(t, d.Put(t.Context(), docName, []byte(docBody)), "the directory is not made")
		})
	})

	t.Run("Workspace", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the base name of the workspace root", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.Equal(t, d.Workspace(), filepath.Base(root), "the root's base name")
		})

		t.Run("returns the empty string for a ledger OpenAt returned", func(t *testing.T) {
			t.Parallel()

			d, err := ledger.OpenAt(t.TempDir())
			assert.NoError(t, err, "the directory opens")
			assert.Equal(t, d.Workspace(), "", "no workspace")
		})
	})

	t.Run("Lock", func(t *testing.T) {
		t.Parallel()

		t.Run("records the holder beside the lock file", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			t.Cleanup(func() { assert.NoError(t, release(), "the lock is released") })
			files.IsFile(t, onDisk(root, lockFile), "the lock file is in the state directory")
			b, err := d.Read(t.Context(), holderFile)
			assert.NoError(t, err, "the record reads")
			var got ledger.Holder
			assert.NoError(t, json.Unmarshal(b, &got), "the record decodes")
			assert.Equal(t, got, fixtureHolder, "the record is the holder")
		})

		t.Run("returns a LockedError that names the holder to a second ledger over the directory", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			t.Cleanup(func() { assert.NoError(t, release(), "the lock is released") })
			_, err = reopened(t, root).Lock(t.Context(), ledger.Holder{PID: 1})
			locked := assert.ErrorAs[*ledger.LockedError](t, err, "the second lock is refused")
			assert.Equal(t, locked.Holder, fixtureHolder, "the error names the first holder")
		})

		t.Run("returns ErrLocked to a second lock of the same ledger", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			t.Cleanup(func() { assert.NoError(t, release(), "the lock is released") })
			_, err = d.Lock(t.Context(), fixtureHolder)
			assert.ErrorIs(t, err, ledger.ErrLocked, "one ledger does not take its lock twice")
		})

		t.Run("returns the lock to the next holder after the release", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			assert.NoError(t, release(), "the lock is released")
			again, err := reopened(t, root).Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the next holder takes the lock")
			assert.NoError(t, again(), "and releases it")
		})

		t.Run("removes the record at the release", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			assert.NoError(t, release(), "the lock is released")
			files.Absent(t, onDisk(root, holderFile), "the record is gone")
			files.IsFile(t, onDisk(root, lockFile), "and the lock file remains")
		})

		t.Run("removes the record at the release after the context ends", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			ctx, cancel := context.WithCancel(t.Context())
			release, err := d.Lock(ctx, fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			cancel()
			assert.NoError(t, release(), "the lock is released")
			files.Absent(t, onDisk(root, holderFile), "the record is gone")
		})

		t.Run("returns the error of the record's removal from the release", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			assert.NoError(t, os.Remove(onDisk(root, holderFile)), "the record is removed")
			assert.NoError(t, os.MkdirAll(onDisk(root, path.Join(holderFile, "blocked")), 0o755),
				"a directory with an entry takes the record's place")
			assert.HasError(t, release(), "the release cannot remove the record")
			assert.NoError(t, os.RemoveAll(onDisk(root, holderFile)), "the directory is removed")
			again, err := reopened(t, root).Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the release closed the lock file all the same")
			assert.NoError(t, again(), "and the next holder releases the lock")
		})

		t.Run("returns a release that fails on its second call", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			assert.NoError(t, release(), "the first release closes the lock file")
			assert.HasError(t, release(), "the second release finds the file closed")
		})

		t.Run("returns a LockedError with the zero Holder for a record that does not read", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			t.Cleanup(func() { assert.NoError(t, release(), "the lock is released") })
			assert.NoError(t, d.Remove(t.Context(), holderFile), "the record is removed")
			_, err = reopened(t, root).Lock(t.Context(), fixtureHolder)
			locked := assert.ErrorAs[*ledger.LockedError](t, err, "the second lock is refused")
			assert.Equal(t, locked.Holder, ledger.Holder{}, "the error names no holder")
		})

		t.Run("returns a LockedError with the zero Holder for a record that does not decode", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			t.Cleanup(func() { assert.NoError(t, release(), "the lock is released") })
			assert.NoError(t, d.Put(t.Context(), holderFile, []byte(`{"pid":"four"}`)), "the record is damaged")
			_, err = reopened(t, root).Lock(t.Context(), fixtureHolder)
			locked := assert.ErrorAs[*ledger.LockedError](t, err, "the second lock is refused")
			assert.Equal(t, locked.Holder, ledger.Holder{}, "the error names no holder")
		})

		t.Run("returns the lock to the next holder after the holder's process is killed", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			child := lockedChild(t, root)
			_, err := d.Lock(t.Context(), fixtureHolder)
			locked := assert.ErrorAs[*ledger.LockedError](t, err, "the child has the lock")
			assert.Equal(t, locked.Holder.PID, child.Process.Pid, "the error names the child")
			assert.NoError(t, child.Process.Kill(), "the child is killed")
			assert.HasError(t, child.Wait(), "the child exits on the signal")
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the operating system released the killed child's lock")
			assert.NoError(t, release(), "and the next holder releases it")
		})

		t.Run("locks the directory of a ledger OpenAt returned", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "memo")
			d, err := ledger.OpenAt(dir)
			assert.NoError(t, err, "the directory opens")
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			assert.NoError(t, release(), "the lock is released")
			files.IsFile(t, filepath.Join(dir, lockFile), "the lock file is in the directory itself")
		})

		t.Run("returns an error and keeps no lock for a holder whose record does not encode", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			unencodable := ledger.Holder{PID: 1, Since: time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)}
			_, err := d.Lock(t.Context(), unencodable)
			assert.HasError(t, err, "a year past 9999 does not encode")
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the failed lock released its file")
			assert.NoError(t, release(), "and the lock is released")
		})

		t.Run("returns an error and keeps no lock for a record that cannot be written", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.MkdirAll(onDisk(root, path.Join(holderFile, "blocked")), 0o755),
				"a directory takes the record's place")
			_, err := d.Lock(t.Context(), fixtureHolder)
			assert.That(t, err).
				HasError("the record is not written").
				ErrorIsNot(ledger.ErrLocked, "and the error is not a lock")
			assert.NoError(t, os.RemoveAll(onDisk(root, holderFile)), "the directory is removed")
			release, err := d.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the failed lock released its file")
			assert.NoError(t, release(), "and the lock is released")
		})

		t.Run("returns an error for a lock file that is a directory", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.MkdirAll(onDisk(root, lockFile), 0o755), "a directory takes the lock file's place")
			_, err := d.Lock(t.Context(), fixtureHolder)
			assert.That(t, err).
				HasError("the lock file does not open").
				ErrorIsNot(ledger.ErrLocked, "and the error is not a lock")
		})

		t.Run("returns an error for a state directory that is a file", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.WriteFile(filepath.Join(root, ledger.StateDir(brand)), nil, 0o600),
				"a file takes the state directory's place")
			_, err := d.Lock(t.Context(), fixtureHolder)
			assert.HasError(t, err, "the state directory is not made")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := d.Lock(ctx, fixtureHolder)
				return err
			}, "the cancellation is returned")
			files.Absent(t, filepath.Join(root, ledger.StateDir(brand)), "and nothing is created")
		})
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the bytes the last write stored", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			assert.NoError(t, d.Write(t.Context(), docName, []byte(otherBody)), "the blob is replaced")
			got, err := d.Read(t.Context(), docName)
			assert.NoError(t, err, "the blob reads")
			assert.Equal(t, string(got), otherBody, "the second write's bytes")
		})

		t.Run("returns ErrNotExist for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			_, err := d.Read(t.Context(), segName)
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is stored there")
		})

		t.Run("returns ErrNotExist before the state directory exists", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			_, err := d.Read(t.Context(), docName)
			assert.ErrorIs(t, err, fs.ErrNotExist, "no state directory")
		})

		t.Run("returns ErrNotExist for a ledger OpenAt returned before its first write", func(t *testing.T) {
			t.Parallel()

			d, err := ledger.OpenAt(filepath.Join(t.TempDir(), "memo"))
			assert.NoError(t, err, "the directory opens")
			_, err = d.Read(t.Context(), docName)
			assert.ErrorIs(t, err, fs.ErrNotExist, "no directory")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := d.Read(ctx, docName)
				return err
			}, "the cancellation is returned")
		})

		for _, tt := range invalidNames {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				d, _ := opened(t)
				_, err := d.Read(t.Context(), tt.give)
				assert.ErrorIs(t, err, fs.ErrInvalid, "the name is refused")
			})
		}
	})

	t.Run("ReadAt", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the bytes at an offset", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			p := make([]byte, 5)
			n, err := d.ReadAt(t.Context(), docName, p, 4)
			assert.NoError(t, err, "the range reads")
			assert.Equal(t, string(p[:n]), "first", "the bytes at the offset")
		})

		t.Run("returns EOF beside the count for a range past the end", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			p := make([]byte, 64)
			n, err := d.ReadAt(t.Context(), docName, p, 4)
			assert.ErrorIs(t, err, io.EOF, "the range ends early")
			assert.Equal(t, string(p[:n]), docBody[4:], "beside the bytes it read")
		})

		t.Run("returns ErrNotExist for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			_, err := d.ReadAt(t.Context(), segName, make([]byte, 1), 0)
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is stored there")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			_, err := d.ReadAt(t.Context(), "../x", make([]byte, 1), 0)
			assert.ErrorIs(t, err, fs.ErrInvalid, "the name is refused")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("creates the directories down to the blob", func(t *testing.T) {
			t.Parallel()

			_, root := written(t, segName, docBody)
			files.HasContent(t, onDisk(root, segName), docBody,
				"the blob is a file of the state directory, with its bytes")
		})

		t.Run("leaves the blob alone in its directory", func(t *testing.T) {
			t.Parallel()

			_, root := written(t, docName, docBody)
			files.Equal(t, os.DirFS(filepath.Dir(onDisk(root, docName))),
				files.Tree{path.Base(docName): files.Text(docBody)}, "no staging file remains")
		})

		t.Run("returns an error for a blob whose directory is a file", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, "manifest", docBody)
			assert.HasError(t, d.Write(t.Context(), docName, []byte(docBody)), "the directory is a file")
		})

		t.Run("returns an error for a blob that is a directory", func(t *testing.T) {
			t.Parallel()

			d, root := written(t, segName, docBody)
			assert.HasError(t, d.Write(t.Context(), "state/seg", []byte(docBody)), "the rename is refused")
			files.IsFile(t, onDisk(root, segName), "the directory's blob remains")
		})

		t.Run("returns an error for a root removed after the ledger opened", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.RemoveAll(root), "the root is removed")
			assert.HasError(t, d.Write(t.Context(), docName, []byte(docBody)), "the root does not open")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return d.Write(ctx, docName, []byte(docBody))
			}, "the cancellation is returned")
			files.Absent(t, filepath.Join(root, ledger.StateDir(brand)), "and nothing is created")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.ErrorIs(t, d.Write(t.Context(), "../x", []byte(docBody)), fs.ErrInvalid, "the name is refused")
		})
	})

	t.Run("WriteAll", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each blob with its bytes in the directories it creates", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, d.WriteAll(t.Context(), map[string][]byte{
				docName: []byte(docBody), segName: []byte(otherBody),
			}), "the blobs are written")
			files.Equal(t, os.DirFS(onDisk(root, ".")), files.Tree{
				docName: files.Text(docBody), segName: files.Text(otherBody),
			}, "each blob is a file of the state directory with its bytes, and no staging file remains")
		})

		t.Run("replaces a blob stored before", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			assert.NoError(t, d.WriteAll(t.Context(), map[string][]byte{docName: []byte(otherBody)}),
				"the blob is replaced")
			got, err := d.Read(t.Context(), docName)
			assert.NoError(t, err, "the blob reads")
			assert.Equal(t, string(got), otherBody, "the new bytes")
		})

		t.Run("creates the directory of a ledger OpenAt returned", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "memo", "shared")
			d, err := ledger.OpenAt(dir)
			assert.NoError(t, err, "a missing directory opens")
			assert.NoError(t, d.WriteAll(t.Context(), map[string][]byte{docName: []byte(docBody)}),
				"the first write creates it")
			files.HasContent(t, filepath.Join(dir, filepath.FromSlash(docName)), docBody,
				"the blob is a file of the directory itself, with its bytes")
		})

		t.Run("leaves one ledger's whole blob when two ledgers write one name at once", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			other := reopened(t, root)
			bodies := []string{strings.Repeat("a", 8192), strings.Repeat("b", 8193)}
			outcomes := history.Concurrently(len(bodies), time.Minute, func(writer int) (any, error) {
				l := []*ledger.Dir{d, other}[writer]
				return writer, l.WriteAll(t.Context(), map[string][]byte{docName: []byte(bodies[writer])})
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "each ledger finishes")
				expect.NoError(t, o.Error, "each ledger writes the blob")
			}
			got := files.Read(t, onDisk(root, docName))
			assert.Contains(t, bodies, got, "the blob is one ledger's whole bytes")
			files.Equal(t, os.DirFS(filepath.Dir(onDisk(root, docName))),
				files.Tree{path.Base(docName): files.Text(got)}, "no staging file remains")
		})

		t.Run("returns nil for no blob and creates nothing", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, d.WriteAll(t.Context(), nil), "nothing is written")
			files.Absent(t, filepath.Join(root, ledger.StateDir(brand)), "and no state directory is made")
		})

		t.Run("returns ErrInvalid for an invalid name and writes the other blobs", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			err := d.WriteAll(t.Context(), map[string][]byte{"../x": nil, docName: []byte(docBody)})
			assert.ErrorIs(t, err, fs.ErrInvalid, "the name is refused")
			got, rerr := d.Read(t.Context(), docName)
			assert.NoError(t, rerr, "the valid blob reads")
			assert.Equal(t, string(got), docBody, "with its bytes")
		})

		t.Run("returns ErrInvalid for invalid names alone and creates nothing", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.ErrorIs(t, d.WriteAll(t.Context(), map[string][]byte{"": nil}), fs.ErrInvalid, "the name is refused")
			files.Absent(t, filepath.Join(root, ledger.StateDir(brand)), "and no state directory is made")
		})

		t.Run("returns an error for a blob whose directory is a file and writes the other blobs", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, "manifest", docBody)
			err := d.WriteAll(t.Context(), map[string][]byte{docName: []byte(docBody), segName: []byte(otherBody)})
			assert.HasError(t, err, "the directory is a file")
			assert.Contains(t, err.Error(), docName, "the error names the blob")
			got, rerr := d.Read(t.Context(), segName)
			assert.NoError(t, rerr, "the other blob reads")
			assert.Equal(t, string(got), otherBody, "with its bytes")
		})

		t.Run("returns an error for a root removed after the ledger opened", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.RemoveAll(root), "the root is removed")
			assert.HasError(t, d.WriteAll(t.Context(), map[string][]byte{docName: []byte(docBody)}),
				"the root does not open")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return d.WriteAll(ctx, map[string][]byte{docName: []byte(docBody)})
			}, "the cancellation is returned")
			files.Absent(t, filepath.Join(root, ledger.StateDir(brand)), "and nothing is created")
		})
	})

	t.Run("Put", func(t *testing.T) {
		t.Parallel()

		t.Run("stores the bytes a read returns", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.NoError(t, d.Put(t.Context(), segName, []byte(docBody)), "the blob is put")
			got, err := d.Read(t.Context(), segName)
			assert.NoError(t, err, "the blob reads")
			assert.Equal(t, string(got), docBody, "the bytes put")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.ErrorIs(t, d.Put(t.Context(), "", []byte(docBody)), fs.ErrInvalid, "the name is refused")
		})
	})

	t.Run("Touch", func(t *testing.T) {
		t.Parallel()

		t.Run("moves the blob's modification time to the present", func(t *testing.T) {
			t.Parallel()

			d, root := written(t, docName, docBody)
			assert.NoError(t, os.Chtimes(onDisk(root, docName), past, past), "the blob ages")
			assert.NoError(t, d.Touch(t.Context(), docName), "the blob is touched")
			listed, err := d.List(t.Context(), "manifest")
			assert.NoError(t, err, "the blobs list")
			assert.Length(t, listed, 1, "the touched blob")
			assert.True(t, listed[0].ModTime.After(past), "the time moved")
		})

		t.Run("returns ErrNotExist for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			assert.ErrorIs(t, d.Touch(t.Context(), segName), fs.ErrNotExist, "nothing is stored there")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.ErrorIs(t, d.Touch(t.Context(), "a//b"), fs.ErrInvalid, "the name is refused")
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		t.Run("deletes the blob", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			assert.NoError(t, d.Remove(t.Context(), docName), "the blob is removed")
			_, err := d.Read(t.Context(), docName)
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is stored there")
		})

		t.Run("returns nil for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			assert.NoError(t, d.Remove(t.Context(), segName), "a missing blob is no error")
		})

		t.Run("returns nil for a ledger OpenAt returned before its first write", func(t *testing.T) {
			t.Parallel()

			d, err := ledger.OpenAt(filepath.Join(t.TempDir(), "memo"))
			assert.NoError(t, err, "the directory opens")
			assert.NoError(t, d.Remove(t.Context(), docName), "a missing directory is no error")
		})

		t.Run("returns an error for a name that is a directory with blobs", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, segName, docBody)
			assert.HasError(t, d.Remove(t.Context(), "state/seg"), "the directory is not removed")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.ErrorIs(t, d.Remove(t.Context(), "/x"), fs.ErrInvalid, "the name is refused")
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every blob below the directory sorted by name", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, "state/seg/ab/0123", docBody)
			assert.Total(t, func(name string) error { return d.Write(t.Context(), name, []byte(otherBody)) },
				[]string{"state/seg/a.b", "state/gen/0001", "state/seg/0f/9999"}, "a blob is written")
			listed, err := d.List(t.Context(), "state/seg")
			assert.NoError(t, err, "the blobs list")
			names := make([]string, 0, len(listed))
			for _, b := range listed {
				names = append(names, b.Name)
			}
			assert.Equal(t, names, []string{"state/seg/0f/9999", "state/seg/a.b", "state/seg/ab/0123"},
				"the directory's blobs alone, in name order")
			assert.Equal(t, listed[2].Size, int64(len(docBody)), "with their sizes")
		})

		t.Run("returns nothing for a directory nothing wrote under", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			listed, err := d.List(t.Context(), "memo")
			assert.NoError(t, err, "a missing directory is no error")
			assert.Empty(t, listed, "and lists nothing")
		})

		t.Run("returns nothing for a name that is a blob", func(t *testing.T) {
			t.Parallel()

			d, _ := written(t, docName, docBody)
			listed, err := d.List(t.Context(), docName)
			assert.NoError(t, err, "a blob's name is no error")
			assert.Empty(t, listed, "and lists nothing below it")
		})

		t.Run("returns nothing for a ledger OpenAt returned before its first write", func(t *testing.T) {
			t.Parallel()

			d, err := ledger.OpenAt(filepath.Join(t.TempDir(), "memo"))
			assert.NoError(t, err, "the directory opens")
			listed, err := d.List(t.Context(), "ab")
			assert.NoError(t, err, "a missing directory is no error")
			assert.Empty(t, listed, "and lists nothing")
		})

		t.Run("returns an error for a directory that does not walk", func(t *testing.T) {
			t.Parallel()

			d, root := written(t, segName, docBody)
			blocked := filepath.Dir(onDisk(root, segName))
			assert.NoError(t, os.Chmod(blocked, 0), "the directory loses its permissions")
			t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
			if _, err := os.ReadDir(blocked); err == nil {
				t.Skip("the process reads a directory without permissions, as root does")
			}
			_, err := d.List(t.Context(), "state")
			assert.HasError(t, err, "the walk is refused")
		})

		t.Run("returns ErrInvalid for an invalid directory", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			_, err := d.List(t.Context(), ".")
			assert.ErrorIs(t, err, fs.ErrInvalid, "the name is refused")
		})
	})
}

// The disk ledger's calls allocate within their ceilings in the ordinary
// run, which runs no benchmark. Each removal removes a blob of its own,
// put outside the count, and each count keeps the first error of its
// calls, which cmp.Or returns without allocating. The check runs alone,
// because the count includes every goroutine's allocations. Darwin and
// Windows allocate more in their file system calls, so the test runs on
// Linux alone.
func TestDirAllocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the ceilings count the allocations of Linux's file system calls")
	}
	ctx, body, p := t.Context(), []byte(docBody), make([]byte, 5)
	d, root := documented(t)
	var err error
	assert.MaxAllocs(t, func() {
		_, oerr := ledger.OpenDir(root, brand)
		err = cmp.Or(err, oerr)
	}, openDirAllocs, "OpenDir allocates the path and the ledger")
	assert.NoError(t, err, "the ledger opens")
	assert.MaxAllocs(t, func() {
		_, oerr := ledger.OpenAt(root)
		err = cmp.Or(err, oerr)
	}, openAtAllocs, "OpenAt allocates the path and the ledger")
	assert.NoError(t, err, "the directory opens")
	var workspace string
	assert.MaxAllocs(t, func() { workspace = d.Workspace() }, 0, "Workspace allocates nothing")
	assert.Equal(t, workspace, filepath.Base(root), "Workspace returns the root's base name")
	var read []byte
	assert.MaxAllocs(t, func() {
		var rerr error
		read, rerr = d.Read(ctx, docName)
		err = cmp.Or(err, rerr)
	}, dirReadAllocs, "Read allocates the root, the file and the bytes")
	assert.NoError(t, err, "the blob reads")
	assert.Equal(t, string(read), docBody, "Read returns the blob")
	var n int
	assert.MaxAllocs(t, func() {
		var rerr error
		n, rerr = d.ReadAt(ctx, docName, p, 4)
		err = cmp.Or(err, rerr)
	}, dirReadAtAllocs, "ReadAt allocates the root and the file")
	assert.NoError(t, err, "the range reads")
	assert.Equal(t, string(p[:n]), "first", "ReadAt copies the bytes at the offset")
	assert.MaxAllocs(t, func() { err = cmp.Or(err, d.Write(ctx, docName, body)) }, dirWriteAllocs,
		"Write allocates the root and the staged, synced file")
	assert.NoError(t, err, "the blob is written")
	two := map[string][]byte{docName: body, "manifest/cd.json": body}
	assert.MaxAllocs(t, func() { err = cmp.Or(err, d.WriteAll(ctx, two)) }, dirWriteAllAllocs,
		"WriteAll allocates the root, the group and the staged files")
	assert.NoError(t, err, "the blobs are written")
	assert.MaxAllocs(t, func() { err = cmp.Or(err, d.Put(ctx, docName, body)) }, dirPutAllocs,
		"Put allocates the root and the staged file")
	assert.NoError(t, err, "the blob is put")
	assert.MaxAllocs(t, func() { err = cmp.Or(err, d.Touch(ctx, docName)) }, dirTouchAllocs,
		"Touch allocates the root and the times' change")
	assert.NoError(t, err, "the blob is touched")
	var listed []ledger.Blob
	assert.MaxAllocs(t, func() {
		var lerr error
		listed, lerr = d.List(ctx, "manifest")
		err = cmp.Or(err, lerr)
	}, dirListAllocs, "List allocates the root, the walk and the blobs")
	assert.NoError(t, err, "the blobs list")
	assert.Length(t, listed, 2, "both documents")

	next := 0
	put := func() string {
		name := "state/seg/rm/" + strconv.Itoa(next)
		next++
		assert.NoError(t, d.Put(ctx, name, body), "the blob is put")
		return name
	}
	assert.MaxAllocsWithSetup(t, put, func(name string) { err = cmp.Or(err, d.Remove(ctx, name)) },
		dirRemoveAllocs, "Remove allocates the root and the removal")
	assert.NoError(t, err, "every blob is removed")
}

// BenchmarkDir measures each call of the disk ledger over a state
// directory of two documents.
func BenchmarkDir(b *testing.B) {
	body := []byte(docBody)
	d, root := documented(b)

	b.Run("OpenDir", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(openDirAllocs)
		defer c.End()
		var (
			got *ledger.Dir
			err error
		)
		for c.Loop() {
			got, err = ledger.OpenDir(root, brand)
		}
		assert.NoError(b, err, "the ledger opens")
		assert.Equal(b, got.Workspace(), filepath.Base(root), "over the workspace root")
	})

	b.Run("OpenAt", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(openAtAllocs)
		defer c.End()
		var (
			got *ledger.Dir
			err error
		)
		for c.Loop() {
			got, err = ledger.OpenAt(root)
		}
		assert.NoError(b, err, "the directory opens")
		assert.Equal(b, got.Workspace(), "", "as no workspace")
	})

	b.Run("Workspace", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = d.Workspace()
		}
		assert.Equal(b, got, filepath.Base(root), "the root's base name")
	})

	b.Run("Read", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(dirReadAllocs)
		defer c.End()
		var (
			got []byte
			err error
		)
		for c.Loop() {
			got, err = d.Read(b.Context(), docName)
		}
		assert.NoError(b, err, "the blob reads")
		assert.Equal(b, string(got), docBody, "Read returns the blob")
	})

	b.Run("ReadAt", func(b *testing.B) {
		p := make([]byte, 5)
		c := bench.Start(b).MaxAllocs(dirReadAtAllocs)
		defer c.End()
		var (
			n   int
			err error
		)
		for c.Loop() {
			n, err = d.ReadAt(b.Context(), docName, p, 4)
		}
		assert.NoError(b, err, "the range reads")
		assert.Equal(b, string(p[:n]), "first", "ReadAt copies the bytes at the offset")
	})

	b.Run("Write", func(b *testing.B) {
		b.Run("a blob stored before", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(dirWriteAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				err = d.Write(b.Context(), docName, body)
			}
			assert.NoError(b, err, "the blob is written")
		})
	})

	b.Run("WriteAll", func(b *testing.B) {
		b.Run("two blobs stored before", func(b *testing.B) {
			two := map[string][]byte{docName: body, "manifest/cd.json": body}
			c := bench.Start(b).Warmup(1).MaxAllocs(dirWriteAllAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				err = d.WriteAll(b.Context(), two)
			}
			assert.NoError(b, err, "the blobs are written")
		})
	})

	b.Run("Put", func(b *testing.B) {
		b.Run("a blob stored before", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(dirPutAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				err = d.Put(b.Context(), docName, body)
			}
			assert.NoError(b, err, "the blob is put")
		})
	})

	b.Run("Touch", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(dirTouchAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			err = d.Touch(b.Context(), docName)
		}
		assert.NoError(b, err, "the blob is touched")
	})

	b.Run("Remove", func(b *testing.B) {
		b.Run("a stored blob", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(dirRemoveAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(func() { err = d.Put(b.Context(), segName, body) })
				err = d.Remove(b.Context(), segName)
			}
			assert.NoError(b, err, "the blob is removed")
		})
	})

	b.Run("List", func(b *testing.B) {
		b.Run("a directory of two blobs", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(dirListAllocs)
			defer c.End()
			var (
				got []ledger.Blob
				err error
			)
			for c.Loop() {
				got, err = d.List(b.Context(), "manifest")
			}
			assert.NoError(b, err, "the blobs list")
			assert.Length(b, got, 2, "both documents")
		})
	})
}

// opened returns a ledger over a fresh workspace root and the root.
func opened(tb testing.TB) (*ledger.Dir, string) {
	tb.Helper()

	root := tb.TempDir()
	return reopened(tb, root), root
}

// reopened returns a second ledger over the state directory of a
// workspace root, as another process opens it.
func reopened(tb testing.TB, root string) *ledger.Dir {
	tb.Helper()

	d, err := ledger.OpenDir(root, brand)
	assert.NoError(tb, err, "the ledger opens")
	return d
}

// lockedChild starts the test binary as a child that takes the lock of
// the state directory under root, and returns it once the child reports
// that it has the lock. The cleanup kills a child that the case left
// running, under a deadline of its own.
func lockedChild(t *testing.T, root string) *exec.Cmd {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), childDeadline)
	t.Cleanup(cancel)
	exe, err := os.Executable()
	assert.NoError(t, err, "the test binary is found")
	child := exec.CommandContext(ctx, exe)
	child.Env = append(os.Environ(), lockChildEnv+"="+root)
	stdin, err := child.StdinPipe()
	assert.NoError(t, err, "the child's input is a pipe")
	stdout, err := child.StdoutPipe()
	assert.NoError(t, err, "the child's output is a pipe")
	assert.NoError(t, child.Start(), "the child starts")
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	assert.NoError(t, err, "the child reports")
	assert.Equal(t, line, lockedLine+"\n", "the child has the lock")
	return child
}

// onDisk returns the absolute path of a blob of a workspace root's
// state directory.
func onDisk(root, name string) string {
	return filepath.Join(root, ledger.StateDir(brand), filepath.FromSlash(name))
}

// written returns a ledger over a fresh root that contains one blob.
func written(tb testing.TB, name, body string) (*ledger.Dir, string) {
	tb.Helper()

	d, root := opened(tb)
	assert.NoError(tb, d.Write(tb.Context(), name, []byte(body)), "the blob is written")
	return d, root
}

// documented returns a ledger over a fresh root of two documents and a
// segment, and the root.
func documented(tb testing.TB) (*ledger.Dir, string) {
	tb.Helper()

	d, root := written(tb, docName, docBody)
	assert.Total(tb, func(name string) error { return d.Write(tb.Context(), name, []byte(docBody)) },
		[]string{"manifest/cd.json", segName}, "the blob is written")
	return d, root
}
