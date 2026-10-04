// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/ledger"
)

// past is the mtime a fixture sets on a blob, so a touch moves the time.
var past = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// allocRuns is the number of calls [assert.MaxAllocs] makes: one to
// warm the function, and the 100 it counts.
const allocRuns = 101

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

			file := filepath.Join(t.TempDir(), "file")
			assert.NoError(t, os.WriteFile(file, nil, 0o600), "the file is written")
			_, err := ledger.OpenDir(file, brand)
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
			got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(docName)))
			assert.NoError(t, err, "the blob is a file of the directory itself")
			assert.Equal(t, string(got), docBody, "with its bytes")
		})

		t.Run("returns an error for a path that is a file", func(t *testing.T) {
			t.Parallel()

			file := filepath.Join(t.TempDir(), "file")
			assert.NoError(t, os.WriteFile(file, nil, 0o600), "the file is written")
			_, err := ledger.OpenAt(file)
			assert.HasError(t, err, "the path is refused")
		})

		t.Run("returns an error for a path whose parent does not stat", func(t *testing.T) {
			t.Parallel()

			file := filepath.Join(t.TempDir(), "file")
			assert.NoError(t, os.WriteFile(file, nil, 0o600), "the file is written")
			_, err := ledger.OpenAt(filepath.Join(file, "below"))
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
			_, err := d.Read(cancelled(t), docName)
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
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
			got, err := os.ReadFile(onDisk(root, segName))
			assert.NoError(t, err, "the blob is a file of the state directory")
			assert.Equal(t, string(got), docBody, "with its bytes")
		})

		t.Run("leaves the blob alone in its directory", func(t *testing.T) {
			t.Parallel()

			_, root := written(t, docName, docBody)
			listed, err := os.ReadDir(filepath.Dir(onDisk(root, docName)))
			assert.NoError(t, err, "the directory reads")
			assert.Length(t, listed, 1, "no staging file remains")
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
			_, err := os.Stat(onDisk(root, segName))
			assert.NoError(t, err, "the directory's blob remains")
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
			assert.ErrorIs(t, d.Write(cancelled(t), docName, []byte(docBody)), context.Canceled,
				"the cancellation is returned")
			_, err := os.Stat(filepath.Join(root, ledger.StateDir(brand)))
			assert.ErrorIs(t, err, fs.ErrNotExist, "and nothing is created")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.ErrorIs(t, d.Write(t.Context(), "../x", []byte(docBody)), fs.ErrInvalid, "the name is refused")
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
			for _, name := range []string{"state/seg/a.b", "state/gen/0001", "state/seg/0f/9999"} {
				assert.NoError(t, d.Write(t.Context(), name, []byte(otherBody)), "a blob is written")
			}
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
// written before the count. The check runs alone, because AllocsPerRun
// counts every goroutine's allocations and refuses to run beside
// parallel tests.
func TestDirAllocs(t *testing.T) {
	ctx, body, p := t.Context(), []byte(docBody), make([]byte, 5)
	d, root := documented(t)
	assert.MaxAllocs(t, func() {
		if _, err := ledger.OpenDir(root, brand); err != nil {
			t.Fatalf("OpenDir: unexpected error: %v", err)
		}
	}, openDirAllocs, "OpenDir allocates the path and the ledger")
	assert.MaxAllocs(t, func() {
		if _, err := ledger.OpenAt(root); err != nil {
			t.Fatalf("OpenAt: unexpected error: %v", err)
		}
	}, openAtAllocs, "OpenAt allocates the path and the ledger")
	assert.MaxAllocs(t, func() {
		if d.Workspace() == "" {
			t.Fatal("Workspace returned no name")
		}
	}, 0, "Workspace allocates nothing")
	assert.MaxAllocs(t, func() {
		if _, err := d.Read(ctx, docName); err != nil {
			t.Fatalf("Read: unexpected error: %v", err)
		}
	}, dirReadAllocs, "Read allocates the root, the file and the bytes")
	assert.MaxAllocs(t, func() {
		if _, err := d.ReadAt(ctx, docName, p, 4); err != nil {
			t.Fatalf("ReadAt: unexpected error: %v", err)
		}
	}, dirReadAtAllocs, "ReadAt allocates the root and the file")
	assert.MaxAllocs(t, func() {
		if err := d.Write(ctx, docName, body); err != nil {
			t.Fatalf("Write: unexpected error: %v", err)
		}
	}, dirWriteAllocs, "Write allocates the root and the staged, synced file")
	assert.MaxAllocs(t, func() {
		if err := d.Put(ctx, docName, body); err != nil {
			t.Fatalf("Put: unexpected error: %v", err)
		}
	}, dirPutAllocs, "Put allocates the root and the staged file")
	assert.MaxAllocs(t, func() {
		if err := d.Touch(ctx, docName); err != nil {
			t.Fatalf("Touch: unexpected error: %v", err)
		}
	}, dirTouchAllocs, "Touch allocates the root and the times' change")
	assert.MaxAllocs(t, func() {
		if listed, err := d.List(ctx, "manifest"); err != nil || len(listed) != 2 {
			t.Fatalf("List: blobs %d, error %v", len(listed), err)
		}
	}, dirListAllocs, "List allocates the root, the walk and the blobs")
	stored := make([]string, allocRuns)
	for i := range stored {
		stored[i] = "state/seg/rm/" + strconv.Itoa(i)
		assert.NoError(t, d.Put(ctx, stored[i], body), "the blob is put")
	}
	at := 0
	assert.MaxAllocs(t, func() {
		if err := d.Remove(ctx, stored[at]); err != nil {
			t.Fatalf("Remove: unexpected error: %v", err)
		}
		at++
	}, dirRemoveAllocs, "Remove allocates the root and the removal")
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
	d, err := ledger.OpenDir(root, brand)
	assert.NoError(tb, err, "the ledger opens")
	return d, root
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
	for _, name := range []string{"manifest/cd.json", segName} {
		assert.NoError(tb, d.Write(tb.Context(), name, []byte(docBody)), "the blob is written")
	}
	return d, root
}
