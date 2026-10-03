// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"context"
	"crypto/sha256"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
)

// The memo's own blobs the cases read and write: its size in bytes, and
// when it was last listed whole.
const (
	memoTotal   = "memo/total"
	memoTrimmed = "memo/trimmed"
)

// The executables the memo cases key their entries under, and a cap
// above every case's total.
var (
	thisBuild  = sha256.Sum256([]byte("this build"))
	otherBuild = sha256.Sum256([]byte("another build"))
	ample      = int64(1 << 20)
)

// memoAt returns a memo of this build over a ledger, at an instant.
func memoAt(l ledger.Ledger, limit int64, now time.Time) *state.Memo {
	return state.NewMemo(context.Background(), l, thisBuild, limit, now)
}

// putAll writes one entry for each key into the ledger's memo, the
// region of each its encoding of fullRegion, at an instant.
func putAll(t *testing.T, l ledger.Ledger, limit int64, now time.Time, keys ...string) {
	t.Helper()

	m := memoAt(l, limit, now)
	for _, k := range keys {
		m.Put([]byte(k), fullRegion())
	}
	_, err := m.Write(t.Context())
	assert.NoError(t, err, "the memo writes")
}

// entriesOf returns the names of the memo's entries, without its own
// blobs.
func entriesOf(t *testing.T, l ledger.Ledger) []string {
	t.Helper()

	blobs, err := l.List(t.Context(), "memo")
	assert.NoError(t, err, "the memo lists")
	var out []string
	for _, b := range blobs {
		if b.Name != memoTotal && b.Name != memoTrimmed {
			out = append(out, b.Name)
		}
	}
	return out
}

// numberIn returns the number one of the memo's own blobs states.
func numberIn(t *testing.T, l ledger.Ledger, name string) int64 {
	t.Helper()

	b, err := l.Read(t.Context(), name)
	assert.NoError(t, err, "the blob reads")
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	assert.NoError(t, err, "and states a number")
	return n
}

// The memo restores a unit some earlier run parsed, so its keys, its
// entries' damage and its cap are each pinned.
func TestMemo(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("Get", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the region a write put under the key", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			r, hit := memoAt(l, ample, now).Get([]byte("alpha"))
			assert.True(t, hit, "the key has an entry")
			assert.Equal(t, r, fullRegion(), "the region returns whole")
		})

		t.Run("reports false for a key nothing put", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			_, hit := memoAt(l, ample, now).Get([]byte("beta"))
			assert.False(t, hit, "the key has no entry")
		})

		t.Run("reports false for an entry another build wrote", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			_, hit := state.NewMemo(t.Context(), l, otherBuild, ample, now).Get([]byte("alpha"))
			assert.False(t, hit, "an entry serves the build that wrote it")
		})

		t.Run("reports false for an entry that does not decode", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			name := entriesOf(t, l)[0]
			assert.NoError(t, l.Put(t.Context(), name, []byte("torn")), "the entry tears")
			_, hit := memoAt(l, ample, now).Get([]byte("alpha"))
			assert.False(t, hit, "a torn entry is a miss")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing for a memo nothing was put into or hit in", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			writes := l.Writes()
			_, err := memoAt(l, ample, now).Write(t.Context())
			assert.NoError(t, err, "the memo writes")
			assert.Equal(t, l.Writes(), writes, "without a write")
		})

		t.Run("adds the bytes it wrote to the memo's total", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			first := numberIn(t, l, memoTotal)
			m := memoAt(l, ample, now)
			m.Put([]byte("beta"), fullRegion())
			written, err := m.Write(t.Context())
			assert.NoError(t, err, "the memo writes")
			assert.Equal(t, numberIn(t, l, memoTotal), first+written, "the total grows by the entry")
		})

		t.Run("touches an entry a load hit", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha", "beta")
			m := memoAt(l, ample, now)
			m.Get([]byte("alpha"))
			_, err := m.Write(t.Context())
			assert.NoError(t, err, "the memo writes")
			blobs, err := l.List(t.Context(), "memo")
			assert.NoError(t, err, "the memo lists")
			var newest ledger.Blob
			for _, b := range blobs {
				if b.Name != memoTotal && b.Name != memoTrimmed && b.ModTime.After(newest.ModTime) {
					newest = b
				}
			}
			assert.Equal(t, newest.Name, entryOf(t, l, "alpha"), "the hit entry is the most recent")
		})

		t.Run("removes an entry that does not decode", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			name := entriesOf(t, l)[0]
			assert.NoError(t, l.Put(t.Context(), name, []byte("torn")), "the entry tears")
			m := memoAt(l, ample, now)
			m.Get([]byte("alpha"))
			_, err := m.Write(t.Context())
			assert.NoError(t, err, "the memo writes")
			assert.Empty(t, entriesOf(t, l), "the torn entry is gone")
		})

		t.Run("removes the oldest entries until the memo is under nine tenths of its cap", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			for _, k := range []string{"a", "b", "c", "d"} {
				putAll(t, l, ample, now, k)
			}
			size := numberIn(t, l, memoTotal) / 4
			limit := 3*size + size/2
			putAll(t, l, limit, now, "e")
			assert.Equal(t, entriesOf(t, l), sortedEntries(t, l, "e", "d", "c"),
				"the two oldest entries leave a memo of three entries, under nine tenths of a cap of three and a half")
		})

		t.Run("lists the memo a day after its last listing", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			putAll(t, l, ample, now, "alpha")
			truth := numberIn(t, l, memoTotal)
			assert.NoError(t, l.Put(t.Context(), memoTotal, []byte("1\n")), "the total drifts")
			_, err := memoAt(l, ample, now.Add(25*time.Hour)).Write(t.Context())
			assert.NoError(t, err, "the memo writes")
			assert.Equal(t, numberIn(t, l, memoTotal), truth, "the listing corrects the total")
		})

		t.Run("returns the error of a ledger that fails to write an entry", func(t *testing.T) {
			t.Parallel()

			m := memoAt(failing{Mem: ledger.NewMem(), put: true}, ample, now)
			m.Put([]byte("alpha"), fullRegion())
			_, err := m.Write(t.Context())
			assert.ErrorIs(t, err, errDevice, "the ledger's own error returns")
		})

		t.Run("returns the error of a read a load met", func(t *testing.T) {
			t.Parallel()

			m := memoAt(failing{Mem: ledger.NewMem(), read: true}, ample, now)
			_, hit := m.Get([]byte("alpha"))
			assert.False(t, hit, "the read misses")
			_, err := m.Write(t.Context())
			assert.ErrorIs(t, err, errDevice, "and its error returns at the commit")
		})
	})
}

// entryOf returns the ledger name the memo stores a key's entry under,
// found as the one entry a fresh ledger contains after a write of the
// key.
func entryOf(t *testing.T, l ledger.Ledger, key string) string {
	t.Helper()

	probe := ledger.NewMem()
	putAll(t, probe, ample, time.Now(), key)
	name := entriesOf(t, probe)[0]
	_, err := l.Read(t.Context(), name)
	assert.NoError(t, err, "the key's entry is in the ledger")
	return name
}

// sortedEntries returns the names of the keys' entries, sorted as a
// listing sorts them.
func sortedEntries(t *testing.T, l ledger.Ledger, keys ...string) []string {
	t.Helper()

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, entryOf(t, l, k))
	}
	slices.Sort(out)
	return out
}
