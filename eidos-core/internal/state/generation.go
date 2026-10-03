// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"time"

	"go.dokimi.dev/eidos/core/ledger"
)

// Format is the version of the kernel's encoding of every generation,
// table, run and region. A generation of another format is unusable,
// and the run that meets it goes cold.
const Format = 1

// ErrFormat reports a generation written in another format than
// [Format]: its tables and regions do not read under the kernel's
// encoding. A run that meets one runs cold.
var ErrFormat = errors.New("state: the generation is in another format")

// The ledger names of the sealed state: the pointer to the live
// generation, the directory of generations, and the directory of
// segments, each segment under the first byte of its name.
const (
	currentName = "state/CURRENT"
	genDir      = "state/gen"
	segDir      = "state/seg"
)

// Executable is the record of the executable that wrote a generation:
// the SHA-256 of its bytes, and the path, size and modification time
// the digest was taken under.
type Executable struct {
	Digest  [sha256.Size]byte
	Path    string
	Size    int64
	ModTime time.Time
}

// Header is what a generation states about the run that wrote it.
type Header struct {
	// Format is the encoding the generation is written in.
	Format uint64
	// Composition folds everything the composition reads from outside
	// the executable.
	Composition [sha256.Size]byte
	// Executable is the executable that wrote the generation.
	Executable Executable
	// Anchor is the instant the recording run's sweep began, less two
	// seconds.
	Anchor time.Time
	// Sequence is one more than the parent's sequence, and one for a
	// cold run's generation.
	Sequence uint64
	// Parent names the generation the run opened, and is empty for a
	// cold run.
	Parent string
}

// RegionRef is where one region's blob is stored: its segment, and its
// offset and length within the segment.
type RegionRef struct {
	Segment string
	Offset  int64
	Length  int64
}

// Generation is one sealed generation as a run reads it: its name, its
// header, the digest of each manifest document it records, the segments
// it references with the live regions each contains, and the runs of
// each table.
//
// # Concurrency
//
// A Generation is safe for concurrent use: its tables read through the
// ledger, which is.
type Generation struct {
	Name     string
	Header   Header
	Manifest Digests
	segments map[string]int
	tables   [tableCount][]runRef
	readers  [tableCount]tableReader
	ledger   ledger.Ledger
}

// Open reads the live generation: the name the ledger's CURRENT states,
// and the generation blob of that name.
//
// Error modes: an error wrapping [fs.ErrNotExist] for a ledger without a
// CURRENT, which a run reads as no state at all. An error wrapping
// [ErrDamaged] for a CURRENT that names no generation, and for a
// generation that does not read, fails its CRC-32C or does not decode.
// An error wrapping [ErrFormat] for a generation of another format. A
// ledger that fails otherwise returns its own error.
func Open(ctx context.Context, l ledger.Ledger) (*Generation, error) {
	current, err := l.Read(ctx, currentName)
	if err != nil {
		return nil, fmt.Errorf("state: read the live generation's name: %w", err)
	}
	name := strings.TrimSpace(string(current))
	if !validName(name, genDir) {
		return nil, fmt.Errorf("%w: CURRENT names no generation: %q", ErrDamaged, name)
	}
	b, err := l.Read(ctx, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: CURRENT names %s, which does not exist", ErrDamaged, name)
	}
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", name, err)
	}
	g, err := decodeGeneration(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	g.Name = name
	g.ledger = l
	g.readers = readersOf(l, g.tables)
	return g, nil
}

// Row is one live row of a table: its key and its encoding.
type Row struct {
	Key   []byte
	Value []byte
}

// Get returns one key's row of a table, and false where the generation
// has none: no run contains the key, or the newest entry of it is a
// tombstone. It reads one block of each run it searches, newest first.
//
// Error modes: an error wrapping [ErrDamaged] for a run that does not
// read whole.
func (g *Generation) Get(ctx context.Context, t Table, key []byte) ([]byte, bool, error) {
	return g.readers[t].get(ctx, key)
}

// All returns every live row of a table, in key order: its runs merged,
// the newest entry of each key deciding.
//
// Error modes: an error wrapping [ErrDamaged] for a run that does not
// read whole.
func (g *Generation) All(ctx context.Context, t Table) ([]Row, error) {
	entries, err := g.readers[t].all(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(entries))
	for i, e := range entries {
		out[i] = Row{Key: e.key, Value: e.row}
	}
	return out, nil
}

// Live reports how many of a segment's regions the generation's units
// reference, zero for a segment it references through runs alone or not
// at all.
func (g *Generation) Live(segment string) int { return g.segments[segment] }

// region reads one region's blob.
//
// Error modes: an error wrapping [ErrDamaged] for a segment that is
// missing or ends before the region.
func (g *Generation) region(ctx context.Context, ref RegionRef) ([]byte, error) {
	r := runReader{ref: runRef{segment: ref.Segment, offset: ref.Offset, length: ref.Length}, ledger: g.ledger}
	return r.read(ctx, 0, int(ref.Length))
}

// readersOf returns a reader over each table's runs.
func readersOf(l ledger.Ledger, tables [tableCount][]runRef) [tableCount]tableReader {
	var out [tableCount]tableReader
	for t, runs := range tables {
		for _, ref := range runs {
			out[t].runs = append(out[t].runs, &runReader{ref: ref, ledger: l})
		}
	}
	return out
}

// appendGeneration appends a generation's blob: the header, the
// manifest's digests by bucket, the segments with their live regions,
// each table's runs, and a CRC-32C over all of it.
func appendGeneration(dst []byte, g *Generation) []byte {
	start := len(dst)
	e := &encoder{buf: dst}
	e.uvarint(g.Header.Format)
	e.digest(g.Header.Composition)
	e.digest(g.Header.Executable.Digest)
	e.text(g.Header.Executable.Path)
	e.varint(g.Header.Executable.Size)
	e.instant(g.Header.Executable.ModTime)
	e.instant(g.Header.Anchor)
	e.uvarint(g.Header.Sequence)
	e.text(g.Header.Parent)
	buckets := slices.Sorted(maps.Keys(g.Manifest))
	e.uvarint(uint64(len(buckets)))
	for _, bucket := range buckets {
		e.text(bucket)
		e.digest(g.Manifest[bucket])
	}
	segments := slices.Sorted(maps.Keys(g.segments))
	e.uvarint(uint64(len(segments)))
	for _, s := range segments {
		e.text(s)
		e.uvarint(uint64(g.segments[s]))
	}
	for _, runs := range g.tables {
		e.uvarint(uint64(len(runs)))
		for _, r := range runs {
			e.text(r.segment)
			e.uvarint(uint64(r.offset))
			e.uvarint(uint64(r.length))
		}
	}
	return binary.LittleEndian.AppendUint32(e.buf, crc32.Checksum(e.buf[start:], castagnoli))
}

// decodeGeneration decodes a generation's blob. The format is the
// blob's first field, so a generation of another format refuses before
// anything the format decides is read.
//
// Error modes: an error wrapping [ErrDamaged] for a blob shorter than
// its trailer, whose CRC-32C does not match, or that does not decode
// whole, and an error wrapping [ErrFormat] for a blob of another format.
func decodeGeneration(b []byte) (*Generation, error) {
	if len(b) < trailerSize {
		return nil, fmt.Errorf("%w: a generation of %d bytes has no trailer", ErrDamaged, len(b))
	}
	body := b[:len(b)-trailerSize]
	if crc32.Checksum(body, castagnoli) != binary.LittleEndian.Uint32(b[len(body):]) {
		return nil, fmt.Errorf("%w: a generation fails its CRC-32C", ErrDamaged)
	}
	d := newDecoder(body, nil)
	g := &Generation{Manifest: Digests{}, segments: map[string]int{}}
	g.Header.Format = d.Uvarint()
	if d.Err() == nil && g.Header.Format != Format {
		return nil, fmt.Errorf("%w: format %d, and the kernel reads format %d", ErrFormat, g.Header.Format, Format)
	}
	g.Header.Composition = d.Digest()
	g.Header.Executable.Digest = d.Digest()
	g.Header.Executable.Path = d.text()
	g.Header.Executable.Size = d.Varint()
	g.Header.Executable.ModTime = d.instant()
	g.Header.Anchor = d.instant()
	g.Header.Sequence = d.Uvarint()
	g.Header.Parent = d.text()
	for range d.Count() {
		bucket := d.text()
		g.Manifest[bucket] = d.Digest()
	}
	for range d.Count() {
		s := d.text()
		g.segments[s] = int(d.Uvarint())
	}
	for t := range g.tables {
		for range d.Count() {
			g.tables[t] = append(g.tables[t], runRef{
				segment: d.text(), offset: int64(d.Uvarint()), length: int64(d.Uvarint()),
			})
		}
	}
	if d.Err() == nil && d.Len() > 0 {
		d.Fail(fmt.Errorf("%d bytes follow the generation's tables", d.Len()))
	}
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: a generation does not decode: %w", ErrDamaged, err)
	}
	return g, nil
}

// blobName returns the ledger name of a generation or a segment: the
// directory and the hex SHA-256 of the blob's bytes, a segment under
// the digest's first byte.
func blobName(dir string, b []byte) string {
	sum := sha256.Sum256(b)
	digest := hex.EncodeToString(sum[:])
	if dir == segDir {
		return dir + "/" + digest[:2] + "/" + digest
	}
	return dir + "/" + digest
}

// validName reports whether a name is a blob name [blobName] gives
// under dir.
func validName(name, dir string) bool {
	rest, under := strings.CutPrefix(name, dir+"/")
	if !under {
		return false
	}
	if dir == segDir {
		bucket, digest, nested := strings.Cut(rest, "/")
		if !nested || len(bucket) != 2 || !strings.HasPrefix(digest, bucket) {
			return false
		}
		rest = digest
	}
	if len(rest) != 2*sha256.Size {
		return false
	}
	_, err := hex.DecodeString(rest)
	return err == nil
}
