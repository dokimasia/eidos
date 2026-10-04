// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// manyRows is how many rows the cases spread over blocks: each row is
// about a hundred bytes, so the run spans many 4 KiB blocks.
const manyRows = 500

// A run is sorted rows in blocks, each checked by its CRC-32C, so a
// lookup finds a row in any block and a damaged block refuses to read.
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
				assert.Equal(t, string(row), fmt.Sprintf("%090d", i), "the key's own row")
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
			assert.Length(t, rows, manyRows, "every row in key order")
		})
	})

	damages := []struct {
		name string
		at   func(n int) int
	}{
		{name: "returns ErrDamaged for a block that fails its CRC-32C", at: func(int) int { return 3 }},
		{name: "returns ErrDamaged for an index that fails its CRC-32C", at: func(n int) int { return n - 12 }},
		{name: "returns ErrDamaged for a footer that misstates the index", at: func(n int) int { return n - 1 }},
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
			assert.ErrorIs(t, err, state.ErrDamaged, "and at a whole read")
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
