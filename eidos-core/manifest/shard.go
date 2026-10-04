// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
)

// indent is what EncodeShard indents each nesting level by.
const indent = "  "

// buckets spells every bucket, indexed by its byte: two lowercase hex
// digits, built once so naming a bucket allocates nothing.
var buckets = func() (out [256]string) {
	for i := range out {
		out[i] = hex.EncodeToString([]byte{byte(i)})
	}
	return out
}()

// The empty lists EncodeShard writes where an entry's slice is nil,
// shared so a nil slice costs no allocation.
var (
	noPlugins = []diag.Origin{}
	noSources = []string{}
)

// Shard is one of the manifest's documents: the entries whose path's
// SHA-256 begins with the shard's byte, sorted by path. A commit
// rewrites only the documents whose entries changed, so a run's
// rewrite of the record follows its edit and not the record's size.
//
// # Concurrency
//
// The functions of this package read the Shard they are passed and
// never write it or its entries, so goroutines can share a Shard that
// none of them writes.
//
// # Allocation contract
//
// A Shard's storage is its list of entries, which [Split] and
// [DecodeShard] allocate. A copy of a Shard shares that list.
type Shard struct {
	Version   int    `json:"version"`
	Workspace string `json:"workspace"`
	// Bucket is the shard's byte, as two lowercase hex digits.
	Bucket string `json:"bucket"`
	// Files is sorted by path, one entry per path, each in the bucket.
	Files []Entry `json:"files"`
}

// BucketOf returns the bucket a path's entry belongs to: the first byte
// of the SHA-256 of the path, as two lowercase hex digits.
//
// # Allocation contract
//
// BucketOf allocates nothing. The spellings of the 256 buckets are built
// once, when the package initializes.
func BucketOf(path string) string {
	sum := sha256.Sum256([]byte(path))
	return buckets[sum[0]]
}

// Split returns a manifest's documents, one for each bucket that contains
// a file, sorted by bucket. Each document names the manifest's version
// and workspace, and lists its entries in the manifest's order, so a
// manifest sorted by path splits into documents sorted by path.
//
// # Allocation contract
//
// Split allocates the list of each entry's bucket, one list of entries
// per bucket it fills, sized exactly, and the list of documents: 258
// allocations for a manifest that fills all 256 buckets.
func Split(m Manifest) []Shard {
	at := make([]uint8, len(m.Files))
	var counts [256]int
	for i, e := range m.Files {
		sum := sha256.Sum256([]byte(e.Path))
		at[i] = sum[0]
		counts[sum[0]]++
	}
	var lists [256][]Entry
	filled := 0
	for b, n := range counts {
		if n > 0 {
			lists[b] = make([]Entry, 0, n)
			filled++
		}
	}
	for i, e := range m.Files {
		lists[at[i]] = append(lists[at[i]], e)
	}
	out := make([]Shard, 0, filled)
	for b, files := range lists {
		if files != nil {
			out = append(out, Shard{Version: m.Version, Workspace: m.Workspace, Bucket: buckets[b], Files: files})
		}
	}
	return out
}

// Join returns the manifest the documents of one workspace record: every
// document's entries, sorted by path. No documents join to the empty
// manifest of [Version] with no workspace name.
//
// Error modes: each wraps [ErrUnsupported]. Join refuses a document that
// breaks the format's invariants, two documents of one bucket, and two
// documents that name different workspaces.
//
// # Allocation contract
//
// Join allocates the joined list of entries, one allocation, and
// nothing for documents without an entry. A refusal allocates its error.
func Join(shards []Shard) (Manifest, error) {
	m := Manifest{Version: Version}
	var seen [256]bool
	total := 0
	for i, s := range shards {
		if err := checkShard(s); err != nil {
			return Manifest{}, fmt.Errorf("%w: join: %w", ErrUnsupported, err)
		}
		b := hexValue(s.Bucket)
		switch {
		case seen[b]:
			return Manifest{}, fmt.Errorf("%w: join: two documents record bucket %s", ErrUnsupported, s.Bucket)
		case i > 0 && s.Workspace != m.Workspace:
			return Manifest{}, fmt.Errorf("%w: join: bucket %s records workspace %q, and bucket %s records %q",
				ErrUnsupported, s.Bucket, s.Workspace, shards[0].Bucket, m.Workspace)
		}
		seen[b] = true
		m.Workspace = s.Workspace
		total += len(s.Files)
	}
	if total == 0 {
		return m, nil
	}
	m.Files = make([]Entry, 0, total)
	for _, s := range shards {
		m.Files = append(m.Files, s.Files...)
	}
	slices.SortFunc(m.Files, func(a, b Entry) int { return cmp.Compare(a.Path, b.Path) })
	return m, nil
}

// EncodeShard returns a document's bytes: JSON indented by two spaces,
// with a final newline, and [] wherever a list is nil. Two equal
// documents encode to equal bytes. It copies the file list once, so the
// caller's document is not changed.
//
// Error modes: EncodeShard refuses a document that breaks the format's
// invariants, naming the first entry that breaks one.
//
// # Allocation contract
//
// EncodeShard allocates the copy of the file list, the document it
// hands the JSON encoder by pointer, the options that leave HTML
// characters unescaped, and the output buffer with its growths: six
// allocations for a document of 40 entries, whose buffer grows twice.
// The JSON encoder takes its state from a pool, which a collection
// empties, so the first call after a collection allocates that state
// again.
func EncodeShard(s Shard) ([]byte, error) {
	if err := checkShard(s); err != nil {
		return nil, fmt.Errorf("manifest: encode: %w", err)
	}
	out := s
	out.Files = make([]Entry, len(s.Files))
	for i, e := range s.Files {
		if e.Plugins == nil {
			e.Plugins = noPlugins
		}
		if e.Sources == nil {
			e.Sources = noSources
		}
		out.Files[i] = e
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(&out); err != nil {
		return nil, fmt.Errorf("manifest: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// DecodeShard reads what EncodeShard writes. A key the format does not
// know is skipped.
//
// Error modes: each wraps [ErrUnsupported]: another version, bytes that
// are not a JSON object of the format, and a document that breaks the
// format's invariants.
//
// # Allocation contract
//
// DecodeShard allocates each entry's path and the growths of its lists
// of plugins and sources: four allocations for an entry of two plugins
// and one source. Per document it allocates the document, its bucket and
// the growths of the list of entries, about nine allocations for a
// document of 40 entries. The JSON decoder returns a repeated string,
// such as a plan's name, from its string cache without an allocation.
// It takes its state, the cache included, from a pool, which a
// collection empties, so the first call after a collection allocates
// that state again.
func DecodeShard(b []byte) (Shard, error) {
	var s Shard
	if err := json.Unmarshal(b, &s); err != nil {
		return Shard{}, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	if err := checkShard(s); err != nil {
		return Shard{}, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	return s, nil
}

// checkShard returns the first invariant a document breaks, nil where it
// breaks none: the version, a bucket spelled as two lowercase hex
// digits, the invariants every list of entries keeps, and every entry's
// path in the document's bucket.
func checkShard(s Shard) error {
	if s.Version != Version {
		return fmt.Errorf("version %d is not version %d", s.Version, Version)
	}
	if len(s.Bucket) != 2 || buckets[hexValue(s.Bucket)] != s.Bucket {
		return fmt.Errorf("bucket %q is not two lowercase hex digits", s.Bucket)
	}
	if err := checkFiles(s.Files); err != nil {
		return err
	}
	for _, e := range s.Files {
		if b := BucketOf(e.Path); b != s.Bucket {
			return fmt.Errorf("file %q belongs to bucket %s, not to bucket %s", e.Path, b, s.Bucket)
		}
	}
	return nil
}

// hexValue returns the byte two hex digits spell, and 0 for digits that
// are not lowercase hex, which the caller tells apart by spelling the
// result back.
func hexValue(digits string) uint8 {
	var v uint8
	for i := range 2 {
		c := digits[i]
		switch {
		case c >= '0' && c <= '9':
			v = v<<4 | (c - '0')
		case c >= 'a' && c <= 'f':
			v = v<<4 | (c - 'a' + 10)
		default:
			return 0
		}
	}
	return v
}
