// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"context"
	"io"
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
)

// holding returns a memory ledger that contains one blob.
func holding(t *testing.T, name, body string) *ledger.Mem {
	t.Helper()

	l := ledger.NewMem()
	assert.NoError(t, l.Write(t.Context(), name, []byte(body)), "the blob is written")
	return l
}

// The memory ledger keeps blobs without state on disk: copies in, copies
// out, and a logical clock that orders every write and touch.
func TestMem(t *testing.T) {
	t.Parallel()

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

			_, err := holding(t, docName, docBody).Read(cancelled(t), docName)
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
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
			assert.ErrorIs(t, l.Write(cancelled(t), docName, nil), context.Canceled, "the cancellation is returned")
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
			assert.ErrorIs(t, l.Remove(cancelled(t), docName), context.Canceled, "the cancellation is returned")
			_, err := l.Read(t.Context(), docName)
			assert.NoError(t, err, "and the blob remains")
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every blob below the directory sorted by name", func(t *testing.T) {
			t.Parallel()

			l := holding(t, "state/seg/ab/0123", docBody)
			for _, name := range []string{"state/seg/a.b", "state/gen/0001", "state/seg/0f/9999", "state/segment"} {
				assert.NoError(t, l.Write(t.Context(), name, []byte(otherBody)), "a blob is written")
			}
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

			_, err := holding(t, docName, docBody).List(cancelled(t), "manifest")
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
		})
	})

	t.Run("Writes", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero before the first write", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ledger.NewMem().Writes(), 0, "nothing was written")
		})

		t.Run("counts each write and put", func(t *testing.T) {
			t.Parallel()

			l := holding(t, docName, docBody)
			assert.NoError(t, l.Put(t.Context(), docName, []byte(docBody)), "the blob is put")
			assert.NoError(t, l.Touch(t.Context(), docName), "the blob is touched")
			assert.Equal(t, l.Writes(), 2, "a touch is no write")
		})
	})
}
