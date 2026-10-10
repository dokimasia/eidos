// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"

	"go.dokimi.dev/eidos/core/internal/grow"
	"go.dokimi.dev/eidos/core/internal/wire"
)

// blockSize is the size a run's block grows to before the next entry
// opens a block of its own. An entry larger than a block is a block of
// its own.
const blockSize = 4096

// trailerSize is the length of a block's trailer. The trailer contains
// the CRC-32C of the block. footerSize is the length of a run's footer.
// The footer contains the CRC-32C of the run's index, then the length of
// the index, in four bytes each.
const (
	trailerSize = 4
	footerSize  = 8
)

// The byte after an entry's key is the kind of the entry. A row entry
// has its row after the kind byte. A tombstone has no row, and it deletes
// the key in every older run.
const (
	entryTombstone = 0
	entryRow       = 1
)

// minEntries is the least capacity that [decodeBlock] allocates for a
// list of entries.
const minEntries = 64

// castagnoli is the table of the CRC-32C. The package computes the
// checksum of every block, index, generation and region with it.
var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// ErrDamaged is the class of every failure of a stored generation to
// read whole. The package returns an error that wraps it for each of
// these causes:
//
//   - a CRC-32C that does not match
//   - a block, an index or a row that does not decode
//   - a blob that is missing or shorter than its record
//
// A run that receives it discards what it derived and runs cold.
var ErrDamaged = errors.New("state: the sealed state is damaged")

// entry is one key of a table and its row, or a tombstone.
type entry struct {
	key  []byte
	row  []byte
	dead bool
}

// compareEntries orders two entries by key.
func compareEntries(a, b entry) int { return bytes.Compare(a.key, b.key) }

// blockRef is the index entry of one block of a run. It has the first
// key of the block, the offset of the block in the run, and the length of
// the block without its trailer.
type blockRef struct {
	first  []byte
	offset int
	length int
}

// appendRun appends the encoding of a run to dst and returns the extended
// slice. entries must be sorted by key and contain each key once. The
// encoding has three parts:
//
//   - the entries in blocks of about [blockSize] bytes, each block
//     followed by its CRC-32C
//   - the sparse index, with the first key, the offset and the length of
//     each block
//   - the footer, with the CRC-32C of the index and the length of the
//     index
//
// The first block is at offset 0 of the run, and each later block starts
// where the block before it ends.
func appendRun(dst []byte, entries []entry) []byte {
	start := len(dst)
	var refs []blockRef
	for i := 0; i < len(entries); {
		ref := blockRef{first: entries[i].key, offset: len(dst) - start}
		blockStart := len(dst)
		for i < len(entries) && (len(dst) == blockStart || len(dst)-blockStart < blockSize) {
			dst = appendEntry(dst, entries[i])
			i++
		}
		ref.length = len(dst) - blockStart
		dst = binary.LittleEndian.AppendUint32(dst, crc32.Checksum(dst[blockStart:], castagnoli))
		refs = append(refs, ref)
	}
	indexStart := len(dst)
	dst = binary.AppendUvarint(dst, uint64(len(refs)))
	for _, ref := range refs {
		dst = wire.AppendBytes(dst, ref.first)
		dst = binary.AppendUvarint(dst, uint64(ref.offset))
		dst = binary.AppendUvarint(dst, uint64(ref.length))
	}
	index := dst[indexStart:]
	dst = binary.LittleEndian.AppendUint32(dst, crc32.Checksum(index, castagnoli))
	return binary.LittleEndian.AppendUint32(dst, uint32(len(index)))
}

// appendEntry appends the key, the kind and the row of one entry to dst,
// and returns the extended slice. A tombstone has no row.
func appendEntry(dst []byte, e entry) []byte {
	dst = wire.AppendBytes(dst, e.key)
	if e.dead {
		return append(dst, entryTombstone)
	}
	dst = append(dst, entryRow)
	return wire.AppendBytes(dst, e.row)
}

// runSize returns an upper bound of the number of bytes that [appendRun]
// appends for entries. It adds these parts:
//
//   - the encoding of each entry
//   - a trailer and an index entry for each block. An index entry takes at
//     most the length of the longest key and three varints.
//   - the count of the index and the footer
//
// [appendRun] writes at most n/[blockSize]+1 blocks for n bytes of encoded
// entries.
func runSize(entries []entry) int {
	size, longest := 0, 0
	for _, e := range entries {
		size += entrySize(e)
		longest = max(longest, len(e.key))
	}
	blocks := size/blockSize + 1
	return size + blocks*(trailerSize+longest+3*binary.MaxVarintLen64) + binary.MaxVarintLen64 + footerSize
}

// entrySize returns the number of bytes [appendEntry] appends for e.
func entrySize(e entry) int {
	size := uvarintLen(uint64(len(e.key))) + len(e.key) + 1
	if !e.dead {
		size += uvarintLen(uint64(len(e.row))) + len(e.row)
	}
	return size
}

// decodeFooter returns the length and the CRC-32C of the index from a
// run's footer.
func decodeFooter(footer []byte) (int, uint32) {
	return int(binary.LittleEndian.Uint32(footer[4:])), binary.LittleEndian.Uint32(footer)
}

// decodeIndex decodes a run's index and checks it against its CRC-32C.
// It requires the first block to start at offset 0 and each later block
// to start where the block before it ends. [appendRun] writes the blocks
// in that order. limit is the length of the run's blocks with their
// trailers. No block may extend past limit.
//
// Error modes: an error that wraps [ErrDamaged] for an index that fails
// its CRC-32C or does not decode, and for a block that starts elsewhere
// or extends past limit.
func decodeIndex(index []byte, sum uint32, limit int) ([]blockRef, error) {
	if crc32.Checksum(index, castagnoli) != sum {
		return nil, fmt.Errorf("%w: a run's index fails its CRC-32C", ErrDamaged)
	}
	d := wire.NewDecoder(index)
	refs := make([]blockRef, d.Count())
	end := 0
	for i := range refs {
		refs[i] = blockRef{first: d.Bytes(), offset: d.Int(), length: d.Int()}
		if refs[i].offset != end {
			d.Fail(fmt.Errorf("%w: a run's block %d starts at byte %d instead of byte %d",
				ErrDamaged, i, refs[i].offset, end))
		}
		if refs[i].length > limit-trailerSize-end {
			d.Fail(fmt.Errorf("%w: a run's block %d extends past the start of the index at byte %d",
				ErrDamaged, i, limit))
		}
		end += refs[i].length + trailerSize
	}
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: a run's index does not decode: %w", ErrDamaged, err)
	}
	return refs, nil
}

// decodeBlock appends the entries of the block in b to dst, and returns
// the extended slice. It first checks the block against the CRC-32C in
// the trailer at the end of b. The keys and the rows are windows of b. It
// allocates only when dst is full, and each allocation at least doubles
// the capacity of dst.
//
// Error modes: an error that wraps [ErrDamaged] for a block whose CRC-32C
// does not match or whose entries do not decode.
func decodeBlock(dst []entry, b []byte) ([]entry, error) {
	block := b[:len(b)-trailerSize]
	if crc32.Checksum(block, castagnoli) != binary.LittleEndian.Uint32(b[len(block):]) {
		return nil, fmt.Errorf("%w: a block fails its CRC-32C", ErrDamaged)
	}
	d := wire.NewDecoder(block)
	for d.Len() > 0 && d.Err() == nil {
		e := entry{key: d.Bytes()}
		switch d.Byte() {
		case entryTombstone:
			e.dead = true
		case entryRow:
			e.row = d.Bytes()
		default:
			d.Fail(fmt.Errorf("%w: an entry is neither a row nor a tombstone", ErrDamaged))
		}
		dst = append(grow.Room(dst, 1, minEntries), e)
	}
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: a block does not decode: %w", ErrDamaged, err)
	}
	return dst, nil
}

// decodeRun decodes every entry of a whole run in memory. It returns the
// entries in key order.
//
// Error modes: an error that wraps [ErrDamaged] for a run shorter than
// its footer, and the errors of [decodeIndex] and [decodeBlock].
func decodeRun(run []byte) ([]entry, error) {
	if len(run) < footerSize {
		return nil, fmt.Errorf("%w: a run of %d bytes has no footer", ErrDamaged, len(run))
	}
	length, sum := decodeFooter(run[len(run)-footerSize:])
	limit := len(run) - footerSize - length
	if limit < 0 {
		return nil, fmt.Errorf("%w: a run's index of %d bytes does not fit the run", ErrDamaged, length)
	}
	refs, err := decodeIndex(run[limit:len(run)-footerSize], sum, limit)
	if err != nil {
		return nil, err
	}
	var out []entry
	for _, ref := range refs {
		out, err = decodeBlock(out, run[ref.offset:ref.offset+ref.length+trailerSize])
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
