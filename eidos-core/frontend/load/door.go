// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
)

// listingMark ends the path of a listing in a door's record. No file's
// path ends in a slash, so a read and a listing never share a path.
const listingMark = "/"

// Digested is one path a door read, with the digest of what it read: a
// file's bytes, or a directory's listing, whose path ends in a slash. A
// path nothing is at has the zero digest, so a file that appears there
// changes the record.
type Digested struct {
	Path   string
	Digest [sha256.Size]byte
}

// DoorRecord is what a partition or a dependency round read and
// returned: each path it read or listed with the digest of the bytes or
// the listing, a round's needs, the units it returned, and the findings
// it reported.
type DoorRecord struct {
	// Round is zero for the partition, and the round's number for a
	// dependency round.
	Round int
	Reads []Digested
	Needs []plugin.Need
	Units [][]plugin.SourceRef
	// Findings are the needs a round reported it placed nowhere, each a
	// warning under [UnplacedNeed]. A partition reports none.
	Findings []diag.Diag
}

// fold returns the door's reads folded into one digest, in read order:
// each path and digest behind its length, so no path can pass for the
// boundary of the next. Every unit the door returned keys on it,
// because the door decided the unit's shape.
func (d DoorRecord) fold() []byte {
	h := sha256.New()
	for _, r := range d.Reads {
		h.Write(binary.AppendUvarint(nil, uint64(len(r.Path))))
		h.Write([]byte(r.Path))
		h.Write(r.Digest[:])
	}
	return h.Sum(nil)
}

// recordingReader is a recorded door over the workspace tree and the
// load's stores: a partition's, or a dependency round's. It records
// each read and each listing in order with its digest, and a path
// nothing is at with the zero digest. It notes each file it read in the
// gate, so the next load's gate proves the file unchanged without a
// read.
type recordingReader struct {
	fsys  fs.FS
	gate  *gate
	reads []Digested
}

// Read returns one file's bytes and records the read.
//
// Error modes: the error of a file that does not read, naming the path.
// A path nothing is at is recorded before its error returns.
func (r *recordingReader) Read(path string) ([]byte, error) {
	b, err := plugin.ReadFile(r.fsys, path)
	if err != nil {
		if absent(err) {
			r.reads = append(r.reads, Digested{Path: path})
		}
		return nil, fmt.Errorf("load: read %s: %w", path, err)
	}
	digest := sha256.Sum256(b)
	r.reads = append(r.reads, Digested{Path: path, Digest: digest})
	r.gate.note(path, digest)
	return b, nil
}

// ReadDir returns one directory's entries sorted by name and records
// the listing under the directory's path and the listing mark.
//
// Error modes: the error of a directory that does not list, naming the
// path. A path nothing is at is recorded before its error returns.
func (r *recordingReader) ReadDir(path string) ([]fs.DirEntry, error) {
	entries, err := plugin.ReadDir(r.fsys, path)
	if err != nil {
		if absent(err) {
			r.reads = append(r.reads, Digested{Path: path + listingMark})
		}
		return nil, fmt.Errorf("load: list %s: %w", path, err)
	}
	r.reads = append(r.reads, Digested{Path: path + listingMark, Digest: listingDigest(entries)})
	return entries, nil
}

// listingDigest returns the digest of a listing: each entry's name, a
// directory's behind a trailing slash, each followed by a zero byte.
func listingDigest(entries []fs.DirEntry) [sha256.Size]byte {
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.Name()))
		if e.IsDir() {
			h.Write([]byte(listingMark))
		}
		h.Write([]byte{0})
	}
	var digest [sha256.Size]byte
	h.Sum(digest[:0])
	return digest
}

// unchangedRead reports whether what a door's read found is still
// there: a file whose digest the gate states equal, a listing whose
// entries list the same, and nothing still at a path nothing was at.
//
// Error modes: the error of a file or a directory that fails to read
// for a cause other than its absence.
func unchangedRead(g *gate, r Digested) (bool, error) {
	if dir, listed := strings.CutSuffix(r.Path, listingMark); listed {
		entries, err := plugin.ReadDir(g.tree, dir)
		if absent(err) {
			return r.Digest == [sha256.Size]byte{}, nil
		}
		if err != nil {
			return false, fmt.Errorf("load: list %s: %w", dir, err)
		}
		return listingDigest(entries) == r.Digest, nil
	}
	digest, present, err := g.digest(r.Path)
	if err != nil {
		return false, err
	}
	if !present {
		return r.Digest == [sha256.Size]byte{}, nil
	}
	return digest == r.Digest, nil
}

// absent reports whether an error states that nothing is at a path: no
// file or directory, or a store the load does not provide.
func absent(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, plugin.ErrStoreAbsent)
}
