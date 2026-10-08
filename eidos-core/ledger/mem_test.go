// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"cmp"
	"context"
	"io"
	"io/fs"
	"math"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/ledger"
)

// listedDocs are the documents the listing fixtures write under the
// manifest directory.
var listedDocs = []string{docName, "manifest/cd.json", "manifest/ef.json"}

// The memory ledger keeps blobs without state on disk: copies in, copies
// out, and a logical clock that orders every write and touch.
func TestMem(t *testing.T) {
	t.Parallel()

	t.Run("Lock", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a LockedError that names the holder to a second lock", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			release, err := l.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			t.Cleanup(func() { assert.NoError(t, release(), "the lock is released") })
			_, err = l.Lock(t.Context(), ledger.Holder{PID: 1})
			locked := assert.ErrorAs[*ledger.LockedError](t, err, "the second lock is refused")
			assert.Equal(t, locked.Holder, fixtureHolder, "the error names the first holder")
		})

		t.Run("returns the lock to the next holder after the release", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			release, err := l.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the lock is taken")
			assert.NoError(t, release(), "the lock is released")
			again, err := l.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the next holder takes the lock")
			assert.NoError(t, again(), "and releases it")
		})

		t.Run("returns a release whose second call keeps the next holder's lock", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first, err := l.Lock(t.Context(), ledger.Holder{PID: 1})
			assert.NoError(t, err, "the first holder takes the lock")
			assert.NoError(t, first(), "and releases it")
			second, err := l.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "the second holder takes the lock")
			t.Cleanup(func() { assert.NoError(t, second(), "the lock is released") })
			assert.NoError(t, first(), "the first release runs again")
			_, err = l.Lock(t.Context(), ledger.Holder{PID: 1})
			locked := assert.ErrorAs[*ledger.LockedError](t, err, "the second holder keeps the lock")
			assert.Equal(t, locked.Holder, fixtureHolder, "the error names the second holder")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := l.Lock(ctx, fixtureHolder)
				return err
			}, "the cancellation is returned")
			release, err := l.Lock(t.Context(), fixtureHolder)
			assert.NoError(t, err, "and the lock remains free")
			assert.NoError(t, release(), "and is released")
		})
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the bytes the last write stored", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			assert.NoError(t, l.Put(t.Context(), docName, []byte(otherBody)), "the blob is replaced")
			got, err := l.Read(t.Context(), docName)
			assert.NoError(t, err, "the blob reads")
			assert.Equal(t, string(got), otherBody, "the second write's bytes")
		})

		t.Run("returns a copy the caller can change", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			got, err := l.Read(t.Context(), docName)
			assert.NoError(t, err, "the blob reads")
			got[0] = 'X'
			again, err := l.Read(t.Context(), docName)
			assert.NoError(t, err, "the blob reads again")
			assert.Equal(t, string(again), docBody, "the stored bytes are unchanged")
		})

		t.Run("returns ErrNotExist for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			_, err := ledger.NewMem().Read(t.Context(), docName)
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is stored there")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := l.Read(ctx, docName)
				return err
			}, "the cancellation is returned")
		})

		for _, tt := range invalidNames {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := ledger.NewMem().Read(t.Context(), tt.give)
				assert.ErrorIs(t, err, fs.ErrInvalid, "the name is refused")
			})
		}
	})

	t.Run("ReadAt", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the bytes at an offset", func(t *testing.T) {
			t.Parallel()

			p := make([]byte, 5)
			n, err := holding(t, docName, docBody).ReadAt(t.Context(), docName, p, 4)
			assert.NoError(t, err, "the range reads")
			assert.Equal(t, string(p[:n]), "first", "the bytes at the offset")
		})

		t.Run("returns EOF beside the count for a range past the end", func(t *testing.T) {
			t.Parallel()

			p := make([]byte, 64)
			n, err := holding(t, docName, docBody).ReadAt(t.Context(), docName, p, 4)
			assert.ErrorIs(t, err, io.EOF, "the range ends early")
			assert.Equal(t, string(p[:n]), docBody[4:], "beside the bytes it read")
		})

		t.Run("returns EOF for an offset at the end", func(t *testing.T) {
			t.Parallel()

			n, err := holding(t, docName, docBody).ReadAt(t.Context(), docName, make([]byte, 1), int64(len(docBody)))
			assert.ErrorIs(t, err, io.EOF, "nothing follows the end")
			assert.Equal(t, n, 0, "and nothing is read")
		})

		t.Run("returns an error for a negative offset", func(t *testing.T) {
			t.Parallel()

			_, err := holding(t, docName, docBody).ReadAt(t.Context(), docName, make([]byte, 1), -1)
			assert.HasError(t, err, "the offset is refused")
		})

		t.Run("returns ErrNotExist for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			_, err := ledger.NewMem().ReadAt(t.Context(), docName, make([]byte, 1), 0)
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is stored there")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("stores a copy the caller can change", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			b := []byte(docBody)
			assert.NoError(t, l.Write(t.Context(), docName, b), "the blob is written")
			b[0] = 'X'
			got, err := l.Read(t.Context(), docName)
			assert.NoError(t, err, "the blob reads")
			assert.Equal(t, string(got), docBody, "the stored bytes are unchanged")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, ledger.NewMem().Write(t.Context(), "../x", nil), fs.ErrInvalid, "the name is refused")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			assert.HonoursCancellation(t, func(ctx context.Context) error { return l.Write(ctx, docName, nil) },
				"the cancellation is returned")
			assert.Equal(t, l.Writes(), 0, "and nothing is stored")
		})
	})

	t.Run("Put", func(t *testing.T) {
		t.Parallel()

		t.Run("stores the bytes a read returns", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			assert.NoError(t, l.Put(t.Context(), segName, []byte(docBody)), "the blob is put")
			got, err := l.Read(t.Context(), segName)
			assert.NoError(t, err, "the blob reads")
			assert.Equal(t, string(got), docBody, "the bytes put")
		})
	})

	t.Run("Touch", func(t *testing.T) {
		t.Parallel()

		t.Run("moves the blob past every blob written before", func(t *testing.T) {
			t.Parallel()

			l := holding(t, "memo/a", docBody)
			assert.NoError(t, l.Write(t.Context(), "memo/b", []byte(docBody)), "a second blob is written")
			assert.NoError(t, l.Touch(t.Context(), "memo/a"), "the first blob is touched")
			listed, err := l.List(t.Context(), "memo")
			assert.NoError(t, err, "the blobs list")
			assert.Length(t, listed, 2, "both blobs")
			assert.True(t, listed[0].ModTime.After(listed[1].ModTime), "the touched blob is the newest")
		})

		t.Run("returns ErrNotExist for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, ledger.NewMem().Touch(t.Context(), docName), fs.ErrNotExist, "nothing is stored there")
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		t.Run("deletes the blob", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			assert.NoError(t, l.Remove(t.Context(), docName), "the blob is removed")
			_, err := l.Read(t.Context(), docName)
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is stored there")
		})

		t.Run("returns nil for a name nothing wrote", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, ledger.NewMem().Remove(t.Context(), docName), "a missing blob is no error")
		})

		t.Run("returns ErrInvalid for an invalid name", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, ledger.NewMem().Remove(t.Context(), "."), fs.ErrInvalid, "the name is refused")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			assert.HonoursCancellation(t, func(ctx context.Context) error { return l.Remove(ctx, docName) },
				"the cancellation is returned")
			_, err := l.Read(t.Context(), docName)
			assert.NoError(t, err, "and the blob remains")
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every blob below the directory sorted by name", func(t *testing.T) {
			t.Parallel()

			l := holding(t, "state/seg/ab/0123", docBody)
			assert.Total(t, func(name string) error { return l.Write(t.Context(), name, []byte(otherBody)) },
				[]string{"state/seg/a.b", "state/gen/0001", "state/seg/0f/9999", "state/segment"}, "a blob is written")
			listed, err := l.List(t.Context(), "state/seg")
			assert.NoError(t, err, "the blobs list")
			names := make([]string, 0, len(listed))
			for _, b := range listed {
				names = append(names, b.Name)
			}
			assert.Equal(t, names, []string{"state/seg/0f/9999", "state/seg/a.b", "state/seg/ab/0123"},
				"the directory's blobs alone, in name order")
			assert.Equal(t, listed[2].Size, int64(len(docBody)), "with their sizes")
		})

		t.Run("returns ErrInvalid for an invalid directory", func(t *testing.T) {
			t.Parallel()

			_, err := ledger.NewMem().List(t.Context(), "")
			assert.ErrorIs(t, err, fs.ErrInvalid, "the name is refused")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := l.List(ctx, "manifest")
				return err
			}, "the cancellation is returned")
		})
	})

	t.Run("Writes", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero before the first write", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ledger.NewMem().Writes(), 0, "nothing was written")
		})

		t.Run("counts no write for a touch", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			assert.NoError(t, l.Put(t.Context(), docName, []byte(docBody)), "the blob is put")
			assert.NoError(t, l.Touch(t.Context(), docName), "the blob is touched")
			assert.Equal(t, l.Writes(), 2, "a touch is no write")
		})
	})
}

// The memory ledger copies what it stores and what it returns, and its
// other calls allocate nothing, in the ordinary run, which runs no
// benchmark. A write replaces a blob stored before, and each removal
// removes a blob written outside the count. Each count keeps the first
// error of its calls, which cmp.Or returns without allocating. The check
// runs alone, because the count includes every goroutine's allocations.
func TestMemAllocs(t *testing.T) {
	ctx, body, p := t.Context(), []byte(docBody), make([]byte, 5)
	var l *ledger.Mem
	assert.MaxAllocs(t, func() { l = ledger.NewMem() }, 2, "NewMem allocates the ledger and its map")
	assert.Equal(t, l.Writes(), 0, "NewMem returns a ledger that stored nothing")

	l = listing(t)
	var (
		read []byte
		err  error
	)
	assert.MaxAllocs(t, func() {
		var rerr error
		read, rerr = l.Read(ctx, docName)
		err = cmp.Or(err, rerr)
	}, 1, "Read allocates the copy it returns")
	assert.NoError(t, err, "the blob reads")
	assert.Equal(t, string(read), docBody, "Read returns the blob")
	var n int
	assert.MaxAllocs(t, func() {
		var rerr error
		n, rerr = l.ReadAt(ctx, docName, p, 4)
		err = cmp.Or(err, rerr)
	}, 0, "ReadAt allocates nothing")
	assert.NoError(t, err, "the range reads")
	assert.Equal(t, string(p[:n]), "first", "ReadAt copies the bytes at the offset")
	assert.MaxAllocs(t, func() { err = cmp.Or(err, l.Write(ctx, docName, body)) }, 1,
		"Write allocates the copy it stores")
	assert.NoError(t, err, "the blob is written")
	assert.MaxAllocs(t, func() { err = cmp.Or(err, l.Put(ctx, docName, body)) }, 1,
		"Put allocates the copy it stores")
	assert.NoError(t, err, "the blob is put")
	assert.MaxAllocs(t, func() { err = cmp.Or(err, l.Touch(ctx, docName)) }, 0, "Touch allocates nothing")
	assert.NoError(t, err, "the blob is touched")
	stored := func() string {
		assert.NoError(t, l.Write(ctx, segName, body), "the blob is written")
		return segName
	}
	assert.MaxAllocsWithSetup(t, stored, func(name string) { err = cmp.Or(err, l.Remove(ctx, name)) }, 0,
		"Remove allocates nothing")
	assert.NoError(t, err, "every blob is removed")
	var listed []ledger.Blob
	assert.MaxAllocs(t, func() {
		var lerr error
		listed, lerr = l.List(ctx, "manifest")
		err = cmp.Or(err, lerr)
	}, 3, "List allocates the growth of its list")
	assert.NoError(t, err, "the blobs list")
	assert.Length(t, listed, len(listedDocs), "every document")
	var writes int
	assert.MaxAllocs(t, func() { writes = l.Writes() }, 0, "Writes allocates nothing")
	assert.InRange(t, writes, 1, math.Inf(1), "Writes counts the stores")
}

// BenchmarkMem measures each call of the memory ledger over the three
// documents of [listing].
func BenchmarkMem(b *testing.B) {
	body, l := []byte(docBody), listing(b)

	b.Run("NewMem", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(2)
		defer c.End()
		var got *ledger.Mem
		for c.Loop() {
			got = ledger.NewMem()
		}
		assert.Equal(b, got.Writes(), 0, "a new ledger stored nothing")
	})

	b.Run("Read", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var (
			got []byte
			err error
		)
		for c.Loop() {
			got, err = l.Read(b.Context(), docName)
		}
		assert.NoError(b, err, "the blob reads")
		assert.Equal(b, string(got), docBody, "Read returns the blob")
	})

	b.Run("ReadAt", func(b *testing.B) {
		p := make([]byte, 5)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			n   int
			err error
		)
		for c.Loop() {
			n, err = l.ReadAt(b.Context(), docName, p, 4)
		}
		assert.NoError(b, err, "the range reads")
		assert.Equal(b, string(p[:n]), "first", "ReadAt copies the bytes at the offset")
	})

	b.Run("Write", func(b *testing.B) {
		b.Run("a blob stored before", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var err error
			for c.Loop() {
				err = l.Write(b.Context(), docName, body)
			}
			assert.NoError(b, err, "the blob is written")
		})
	})

	b.Run("Put", func(b *testing.B) {
		b.Run("a blob stored before", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var err error
			for c.Loop() {
				err = l.Put(b.Context(), docName, body)
			}
			assert.NoError(b, err, "the blob is put")
		})
	})

	b.Run("Touch", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = l.Touch(b.Context(), docName)
		}
		assert.NoError(b, err, "the blob is touched")
	})

	b.Run("Remove", func(b *testing.B) {
		b.Run("a stored blob", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(func() { err = l.Write(b.Context(), segName, body) })
				err = l.Remove(b.Context(), segName)
			}
			assert.NoError(b, err, "the blob is removed")
		})
	})

	b.Run("List", func(b *testing.B) {
		b.Run("a directory of three blobs", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(3)
			defer c.End()
			var (
				got []ledger.Blob
				err error
			)
			for c.Loop() {
				got, err = l.List(b.Context(), "manifest")
			}
			assert.NoError(b, err, "the blobs list")
			assert.Length(b, got, len(listedDocs), "every document")
		})
	})

	b.Run("Writes", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = l.Writes()
		}
		assert.InRange(b, got, 1, math.Inf(1), "Writes counts the stores")
	})
}

// holding returns a memory ledger that contains one blob.
func holding(tb testing.TB, name, body string) *ledger.Mem {
	tb.Helper()

	l := ledger.NewMem()
	assert.NoError(tb, l.Write(tb.Context(), name, []byte(body)), "the blob is written")
	return l
}

// listing returns a memory ledger of the three documents of
// [listedDocs] and the segment blob.
func listing(tb testing.TB) *ledger.Mem {
	tb.Helper()

	l := holding(tb, segName, docBody)
	assert.Total(tb, func(name string) error { return l.Write(tb.Context(), name, []byte(docBody)) }, listedDocs,
		"the document is written")
	return l
}
