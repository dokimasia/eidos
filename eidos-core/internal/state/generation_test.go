// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"io/fs"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
)

// currentBlob is the name of the pointer to the live generation.
const currentBlob = "state/CURRENT"

// The pins of a generation's layout the format case rewrites: the blob
// begins with its format as a uvarint, and ends in the little-endian
// CRC-32C of everything before it.
const (
	formatAt    = 0
	trailerSize = 4
)

// castagnoli is the CRC-32C table a generation's trailer is computed
// with.
var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// A generation is the run's view of the sealed state, so what Open
// accepts and refuses decides whether a run goes warm or cold.
func TestGeneration(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("returns ErrNotExist for a ledger without a live generation", func(t *testing.T) {
			t.Parallel()

			_, err := state.Open(t.Context(), ledger.NewMem())
			assert.ErrorIs(t, err, fs.ErrNotExist, "no run committed a generation")
		})

		t.Run("returns ErrDamaged for a CURRENT that names no generation", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			assert.NoError(t, l.Write(t.Context(), currentBlob, []byte("garbage\n")), "CURRENT writes")
			_, err := state.Open(t.Context(), l)
			assert.ErrorIs(t, err, state.ErrDamaged, "the name is no generation's")
		})

		t.Run("returns ErrDamaged for a CURRENT that names a missing generation", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			g := committed(t, l, past, "alpha")
			assert.NoError(t, l.Remove(t.Context(), g.Name), "the generation is removed")
			_, err := state.Open(t.Context(), l)
			assert.ErrorIs(t, err, state.ErrDamaged, "CURRENT names nothing that exists")
		})

		t.Run("returns ErrDamaged for a generation whose CRC-32C fails", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			g := committed(t, l, past, "alpha")
			b, err := l.Read(t.Context(), g.Name)
			assert.NoError(t, err, "the generation reads")
			b[0] ^= 0xff
			assert.NoError(t, l.Write(t.Context(), g.Name, b), "the damaged generation writes")
			_, err = state.Open(t.Context(), l)
			assert.ErrorIs(t, err, state.ErrDamaged, "the damage is found at the open")
		})

		t.Run("returns ErrDamaged for a generation of no field", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			named(t, l, binary.LittleEndian.AppendUint32(nil, crc32.Checksum(nil, castagnoli)))
			_, err := state.Open(t.Context(), l)
			assert.ErrorIs(t, err, state.ErrDamaged, "a blob of its trailer alone states no format")
		})

		t.Run("returns ErrFormat for a generation of another format", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			b, err := l.Read(t.Context(), committed(t, l, past, "alpha").Name)
			assert.NoError(t, err, "the generation reads")
			body := bytes.Clone(b[:len(b)-trailerSize])
			body[formatAt] = state.Format + 1
			named(t, l, binary.LittleEndian.AppendUint32(body, crc32.Checksum(body, castagnoli)))
			_, err = state.Open(t.Context(), l)
			assert.ErrorIs(t, err, state.ErrFormat, "the kernel reads its own format alone")
		})

		t.Run("returns the error of a ledger that fails to read", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			committed(t, mem, past, "alpha")
			_, err := state.Open(t.Context(), failing{Mem: mem, read: true})
			assert.ErrorIs(t, err, errDevice, "the ledger's own error returns")
		})

		t.Run("returns the generation CURRENT names", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			want := committed(t, l, past, "alpha")
			current, err := l.Read(t.Context(), currentBlob)
			assert.NoError(t, err, "CURRENT reads")
			assert.Equal(t, strings.TrimSpace(string(current)), want.Name, "CURRENT names the generation")
		})
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the row a commit put", func(t *testing.T) {
			t.Parallel()

			g := committed(t, ledger.NewMem(), past, "alpha")
			row, held, err := g.Get(t.Context(), state.TableChecks, []byte("alpha"))
			assert.NoError(t, err, "the row reads")
			assert.True(t, held, "the key has a row")
			assert.Equal(t, string(row), "row of alpha", "the row the commit put")
		})

		t.Run("reports false for a key no commit put", func(t *testing.T) {
			t.Parallel()

			g := committed(t, ledger.NewMem(), past, "alpha")
			_, held, err := g.Get(t.Context(), state.TableChecks, []byte("absent"))
			assert.NoError(t, err, "the lookup reads")
			assert.False(t, held, "the key has no row")
		})

		t.Run("reports false for a table no commit wrote", func(t *testing.T) {
			t.Parallel()

			g := committed(t, ledger.NewMem(), past, "alpha")
			_, held, err := g.Get(t.Context(), state.TableAudit, []byte("alpha"))
			assert.NoError(t, err, "the lookup reads")
			assert.False(t, held, "an empty table has no row")
		})
	})

	t.Run("All", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every live row of a table in key order", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			committed(t, l, past, "gamma", "alpha")
			g := committed(t, l, past, "beta")
			rows, err := g.All(t.Context(), state.TableChecks)
			assert.NoError(t, err, "the table reads")
			var keys []string
			for _, r := range rows {
				keys = append(keys, string(r.Key))
			}
			assert.Equal(t, keys, []string{"alpha", "beta", "gamma"}, "the runs merge in key order")
		})
	})

	t.Run("Live", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero for a segment the generation does not reference", func(t *testing.T) {
			t.Parallel()

			g := committed(t, ledger.NewMem(), past, "alpha")
			assert.Equal(t, g.Live("state/seg/00/absent"), 0, "no region of the segment is live")
		})
	})
}

// named writes a generation blob under the name of its digest, and
// CURRENT naming it.
func named(t *testing.T, l *ledger.Mem, blob []byte) {
	t.Helper()

	sum := sha256.Sum256(blob)
	name := genPrefix + hex.EncodeToString(sum[:])
	assert.NoError(t, l.Write(t.Context(), name, blob), "the generation writes")
	assert.NoError(t, l.Write(t.Context(), currentBlob, []byte(name+"\n")), "and CURRENT names it")
}

// damageRun overwrites one byte of the live generation's only run
// segment, so a read of the run meets the damage.
func damageRun(t *testing.T, l *ledger.Mem, at func(n int) int) {
	t.Helper()

	segs := blobsUnder(t, l, segPrefix)
	assert.Length(t, segs, 1, "the generation has one segment")
	b, err := l.Read(t.Context(), segs[0])
	assert.NoError(t, err, "the segment reads")
	b = bytes.Clone(b)
	b[at(len(b))] ^= 0xff
	assert.NoError(t, l.Write(t.Context(), segs[0], b), "the damaged segment writes")
}
