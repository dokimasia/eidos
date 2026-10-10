// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// manyRows is the number of rows that the cases spread over blocks. A row
// takes about a hundred bytes, so the run spans many blocks of 4 KiB.
const manyRows = 500

// footerSize is the length of a run's footer, pinned. The footer contains
// the little-endian CRC-32C of the run's index, then the little-endian
// length of the index, in four bytes each.
const footerSize = 8

// indexEntry is an entry of a forged index of a run. It has the first key
// of a block, the offset of the block in the run, and the length of the
// block without its trailer.
type indexEntry struct {
	first          []byte
	offset, length uint64
}

// Each block of a run has its own CRC-32C. Get finds a row in any block of
// a run. A read of a damaged run, or of a run whose index puts a block in
// the wrong place, returns ErrDamaged.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a row from any block of a run of many blocks", func(t *testing.T) {
			t.Parallel()

			g := spread(t, ledger.NewMem())
			for _, i := range []int{0, 1, manyRows / 2, manyRows - 1} {
				row, held, err := g.Get(t.Context(), state.TableChecks, fmt.Appendf(nil, "key-%04d", i))
				assert.NoError(t, err, "the row reads")
				assert.True(t, held, "every key has its row")
				assert.Equal(t, string(row), fmt.Sprintf("%090d", i), "Get returns the row of the key")
			}
		})

		t.Run("reports false for a key before the first block", func(t *testing.T) {
			t.Parallel()

			g := spread(t, ledger.NewMem())
			_, held, err := g.Get(t.Context(), state.TableChecks, []byte("aaa"))
			assert.NoError(t, err, "the lookup reads")
			assert.False(t, held, "no block starts at or before the key")
		})

		t.Run("reports false for a key between two rows of a block", func(t *testing.T) {
			t.Parallel()

			g := spread(t, ledger.NewMem())
			_, held, err := g.Get(t.Context(), state.TableChecks, []byte("key-0001a"))
			assert.NoError(t, err, "the lookup reads")
			assert.False(t, held, "the block contains no such key")
		})
	})

	t.Run("all", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every row of a run of many blocks", func(t *testing.T) {
			t.Parallel()

			rows, err := spread(t, ledger.NewMem()).All(t.Context(), state.TableChecks)
			assert.NoError(t, err, "the table reads")
			assert.Length(t, rows, manyRows, "All returns every row in key order")
		})
	})

	damages := []struct {
		name string
		at   func(n int) int
	}{
		{name: "returns ErrDamaged for a block that fails its CRC-32C", at: func(int) int { return 3 }},
		{name: "returns ErrDamaged for an index that fails its CRC-32C", at: func(n int) int { return n - 12 }},
		{name: "returns ErrDamaged for a footer with a wrong index length", at: func(n int) int { return n - 1 }},
	}
	for _, tt := range damages {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			g := spread(t, l)
			damageRun(t, l, tt.at)
			_, _, err := g.Get(t.Context(), state.TableChecks, []byte("key-0000"))
			assert.ErrorIs(t, err, state.ErrDamaged, "the damage is found at the read")
			_, err = g.All(t.Context(), state.TableChecks)
			assert.ErrorIs(t, err, state.ErrDamaged, "a read of the whole table finds the damage too")
		})
	}

	tests := []struct {
		name string
		give []indexEntry
	}{
		{
			name: "returns ErrDamaged for a block of math.MaxInt bytes",
			give: []indexEntry{{first: append([]byte(flagKey), keySep, 'a'), offset: 0, length: math.MaxInt}},
		},
		{
			name: "returns ErrDamaged for a block that starts inside the block before it",
			give: []indexEntry{
				{first: append([]byte(flagKey), keySep, 'a'), offset: 0, length: 1},
				{first: append([]byte(flagKey), keySep, 'b'), offset: 0, length: 0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := forged(t, tt.give)
			_, _, err := g.Get(t.Context(), state.TablePresent, tt.give[0].first)
			assert.ErrorIs(t, err, state.ErrDamaged, "Get returns ErrDamaged")
			_, err = g.All(t.Context(), state.TablePresent)
			assert.ErrorIs(t, err, state.ErrDamaged, "All returns ErrDamaged")
			_, err = g.Phases(t.Context()).Present(flagKey)
			assert.ErrorIs(t, err, state.ErrDamaged, "the scan of Present returns ErrDamaged")
		})
	}

	t.Run("returns ErrDamaged for a missing segment", func(t *testing.T) {
		t.Parallel()

		l := ledger.NewMem()
		g := spread(t, l)
		for _, s := range blobsUnder(t, l, segPrefix) {
			assert.NoError(t, l.Remove(t.Context(), s), "the segment is removed")
		}
		_, _, err := g.Get(t.Context(), state.TableChecks, []byte("key-0000"))
		assert.ErrorIs(t, err, state.ErrDamaged, "a missing segment is damage")
	})

	t.Run("returns ErrDamaged for a segment shorter than its record", func(t *testing.T) {
		t.Parallel()

		l := ledger.NewMem()
		g := spread(t, l)
		segs := blobsUnder(t, l, segPrefix)
		b, err := l.Read(t.Context(), segs[0])
		assert.NoError(t, err, "the segment reads")
		assert.NoError(t, l.Write(t.Context(), segs[0], b[:len(b)/2]), "the cut segment writes")
		_, err = g.All(t.Context(), state.TableChecks)
		assert.ErrorIs(t, err, state.ErrDamaged, "a short segment is damage")
	})
}

// spread commits a table of many rows in one run and returns the live
// generation.
func spread(t *testing.T, l ledger.Ledger) *state.Generation {
	t.Helper()

	c := state.NewCommit(nil, nil)
	for i := range manyRows {
		c.Put(state.TableChecks, fmt.Appendf(nil, "key-%04d", i), fmt.Appendf(nil, "%090d", i))
	}
	_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(t, err, "the commit writes")
	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "and opens")
	return g
}

// forged commits the rows of spanRows to the present table in one run,
// and replaces the index of the run with give. The new footer has the
// CRC-32C of the new index, so only the placement of the blocks is wrong.
// The run keeps its length, and its blocks end where the new index
// starts. forged returns the live generation.
func forged(t *testing.T, give []indexEntry) *state.Generation {
	t.Helper()

	l := ledger.NewMem()
	rows, _ := spanRows()
	c := state.NewCommit(nil, nil)
	for _, r := range rows {
		c.Put(state.TablePresent, r.Key, r.Value)
	}
	_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(t, err, "the commit writes")
	segs := blobsUnder(t, l, segPrefix)
	assert.Length(t, segs, 1, "the generation has one segment")
	b, err := l.Read(t.Context(), segs[0])
	assert.NoError(t, err, "the segment reads")
	index := binary.AppendUvarint(nil, uint64(len(give)))
	for _, e := range give {
		index = binary.AppendUvarint(index, uint64(len(e.first)))
		index = append(index, e.first...)
		index = binary.AppendUvarint(index, e.offset)
		index = binary.AppendUvarint(index, e.length)
	}
	run := append(bytes.Clone(b[:len(b)-footerSize-len(index)]), index...)
	run = binary.LittleEndian.AppendUint32(run, crc32.Checksum(index, castagnoli))
	run = binary.LittleEndian.AppendUint32(run, uint32(len(index)))
	assert.NoError(t, l.Write(t.Context(), segs[0], run), "the forged segment writes")
	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "the generation opens")
	return g
}
