// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"

	"go.dokimi.dev/eidos/core/internal/wire"
)

// blockSize is the size a run's block grows to before the next entry
// opens a block of its own. An entry larger than a block is a block of
// its own.
const blockSize = 4096

// The lengths of a block's trailer, its CRC-32C, and of a run's footer:
// the CRC-32C of the run's index and the index's length, four bytes
// each.
const (
	trailerSize = 4
	footerSize  = 8
)

// The byte an entry states its kind with: a row, or a tombstone that
// deletes the key in every older run.
const (
	entryTombstone = 0
	entryRow       = 1
)

// castagnoli is the CRC-32C table every block, index, generation and
// region is checked with.
var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// ErrDamaged is the class of every failure of a stored generation to
// read whole: a CRC-32C that does not match, a block, an index or a row
// that does not decode, a blob that is missing or shorter than its
// record. A run that meets it discards what it derived and runs cold.
var ErrDamaged = errors.New("state: the sealed state is damaged")

// entry is one key of a table and its row, or a tombstone.
type entry struct {
	key  []byte
	row  []byte
	dead bool
}

// compareEntries orders two entries by key.
func compareEntries(a, b entry) int { return bytes.Compare(a.key, b.key) }

// blockRef is one block of a run as its index records it: the block's
// first key, and its offset and length within the run, its trailer
// left out.
type blockRef struct {
	first  []byte
	offset int
	length int
}

// appendRun appends the encoding of a run to dst: the entries, sorted
// by key and without repeats, in blocks of about [blockSize] bytes,
// each followed by its CRC-32C, then the sparse index of each block's
// first key, offset and length, the index's CRC-32C and its length.
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

// appendEntry appends one entry: its key, its kind, and its row.
func appendEntry(dst []byte, e entry) []byte {
	dst = wire.AppendBytes(dst, e.key)
	if e.dead {
		return append(dst, entryTombstone)
	}
	dst = append(dst, entryRow)
	return wire.AppendBytes(dst, e.row)
}

// decodeFooter returns the length of the index a run's footer states,
// and the index's CRC-32C.
func decodeFooter(footer []byte) (int, uint32) {
	return int(binary.LittleEndian.Uint32(footer[4:])), binary.LittleEndian.Uint32(footer)
}

// decodeIndex decodes a run's index, checked against its CRC-32C.
// limit is the length of the run's blocks with their trailers, which no
// block may extend past.
//
// Error modes: an error wrapping [ErrDamaged] for an index whose CRC-32C
// does not match, that does not decode, or that places a block outside
// the run.
func decodeIndex(index []byte, sum uint32, limit int) ([]blockRef, error) {
	if crc32.Checksum(index, castagnoli) != sum {
		return nil, fmt.Errorf("%w: a run's index fails its CRC-32C", ErrDamaged)
	}
	d := wire.NewDecoder(index)
	refs := make([]blockRef, d.Count())
	for i := range refs {
		refs[i] = blockRef{first: d.Bytes(), offset: int(d.Uvarint()), length: int(d.Uvarint())}
		if refs[i].offset+refs[i].length+trailerSize > limit {
			d.Fail(fmt.Errorf("%w: a block lies outside its run", ErrDamaged))
		}
	}
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: a run's index does not decode: %w", ErrDamaged, err)
	}
	return refs, nil
}

// decodeBlock decodes one block and its trailer, checked against the
// block's CRC-32C. The keys and rows are windows of b.
//
// Error modes: an error wrapping [ErrDamaged] for a block whose CRC-32C
// does not match or whose entries do not decode.
func decodeBlock(b []byte) ([]entry, error) {
	block := b[:len(b)-trailerSize]
	if crc32.Checksum(block, castagnoli) != binary.LittleEndian.Uint32(b[len(block):]) {
		return nil, fmt.Errorf("%w: a block fails its CRC-32C", ErrDamaged)
	}
	var out []entry
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
		out = append(out, e)
	}
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: a block does not decode: %w", ErrDamaged, err)
	}
	return out, nil
}

// decodeRun decodes every entry of a run held whole in memory, in key
// order.
//
// Error modes: an error wrapping [ErrDamaged] for a run shorter than its
// footer, and the errors of [decodeIndex] and [decodeBlock].
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
		block, err := decodeBlock(run[ref.offset : ref.offset+ref.length+trailerSize])
		if err != nil {
			return nil, err
		}
		out = append(out, block...)
	}
	return out, nil
}
