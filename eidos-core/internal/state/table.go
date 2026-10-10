// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"sync"

	"go.dokimi.dev/eidos/core/ledger"
)

// Table names one of the sealed state's tables. Each maps a key to a
// row, and the row's encoding is the table's own.
type Table uint8

// The tables, in the order a generation lists them.
const (
	// TableFiles maps a path to the gate's record of the file.
	TableFiles Table = 0
	// TableUnits maps a unit's first member to its record.
	TableUnits Table = 1
	// TableDoors maps a frontend and a door's place to the door's record.
	TableDoors Table = 2
	// TableProbes maps a bare identity to the units whose references
	// named it, and a package to the units that followed its re-exports.
	TableProbes Table = 3
	// TableModules maps a language, a root and a module path to the
	// number of packages whose module facts name the module.
	TableModules Table = 4
	// TableValidations maps a subject to its validation's record.
	TableValidations Table = 5
	// TableClaims maps a subject to every claim on its bag.
	TableClaims Table = 6
	// TablePresent maps a key and a subject to nothing, one row for
	// each fact that reads present.
	TablePresent Table = 7
	// TableInvocations maps a phase call and a match to the
	// invocation's record.
	TableInvocations Table = 8
	// TableReaders maps an edge to the records whose read set contains
	// it.
	TableReaders Table = 9
	// TablePlans maps a plan to the work it left pending.
	TablePlans Table = 10
	// TableGroups maps a plan and a group to the group's record.
	TableGroups Table = 11
	// TableArtifacts maps a path to its artifact's record.
	TableArtifacts Table = 12
	// TableNames maps a plan and a name key to the name entry.
	TableNames Table = 13
	// TableAudit maps a key and a subject to an unmet contract's finding.
	TableAudit Table = 14
	// TableChecks maps a check to its record.
	TableChecks Table = 15
)

// tableCount is how many tables a generation lists.
const tableCount = 16

// String spells the table's name for an error.
func (t Table) String() string {
	names := [tableCount]string{
		"files", "units", "doors", "probes", "modules", "validations", "claims", "present",
		"invocations", "readers", "plans", "groups", "artifacts", "names", "audit", "checks",
	}
	if int(t) < len(names) {
		return names[t]
	}
	return "Table(" + strconv.Itoa(int(t)) + ")"
}

// runRef is where one run of a table is stored: its segment, its
// offset and its length in bytes.
type runRef struct {
	segment string
	offset  int64
	length  int64
}

// runReader reads one stored run: its index once, and each block it
// needs through [ledger.Ledger.ReadAt]. It keeps the block that it
// decoded last, so a lookup in the same block as the lookup before it
// reads nothing from the ledger. Lookups in key order, such as a pass
// over the subjects of a graph, read each block once.
//
// # Concurrency
//
// A runReader is safe for concurrent use: the index loads under a
// [sync.Once], and a mutex guards the kept block. A decoded block is
// never written, so a lookup reads its entries after the lock is free.
type runReader struct {
	ref    runRef
	ledger ledger.Ledger
	once   sync.Once
	index  []blockRef
	limit  int
	err    error
	// last is the place in the index of the block that the reader decoded
	// last, and lastEntries lists that block's entries, nil before the
	// first read. mu guards both.
	mu          sync.Mutex
	last        int
	lastEntries []entry
}

// load reads the run's footer and index once.
func (r *runReader) load(ctx context.Context) error {
	r.once.Do(func() {
		if r.ref.length < footerSize {
			r.err = fmt.Errorf("%w: a run of %d bytes has no footer", ErrDamaged, r.ref.length)
			return
		}
		footer, err := r.read(ctx, r.ref.length-footerSize, footerSize)
		if err != nil {
			r.err = err
			return
		}
		length, sum := decodeFooter(footer)
		r.limit = int(r.ref.length) - footerSize - length
		if r.limit < 0 {
			r.err = fmt.Errorf("%w: a run's index of %d bytes does not fit the run", ErrDamaged, length)
			return
		}
		index, err := r.read(ctx, int64(r.limit), length)
		if err != nil {
			r.err = err
			return
		}
		r.index, r.err = decodeIndex(index, sum, r.limit)
	})
	return r.err
}

// get returns one key's entry, and false where the run does not contain
// the key. It reads the one block whose first key is the last at or
// before the key, from the ledger unless the reader decoded that block
// last.
func (r *runReader) get(ctx context.Context, key []byte) (entry, bool, error) {
	if err := r.load(ctx); err != nil {
		return entry{}, false, err
	}
	at := r.blockOf(key)
	if at < 0 {
		return entry{}, false, nil
	}
	block, err := r.entries(ctx, at)
	if err != nil {
		return entry{}, false, err
	}
	i := sort.Search(len(block), func(i int) bool { return bytes.Compare(block[i].key, key) >= 0 })
	if i == len(block) || !bytes.Equal(block[i].key, key) {
		return entry{}, false, nil
	}
	return block[i], true, nil
}

// scan returns every entry of the run whose key begins with prefix, in key
// order, in a list of its own. The entries are in the block that get would
// read for prefix and in each block after it whose first key begins with
// prefix. scan reads one such block through the block that the reader
// keeps. It reads two or more of them from the ledger in one read, and
// decodes them into one list, which allocates once for the read and once
// for each doubling of the list.
func (r *runReader) scan(ctx context.Context, prefix []byte) ([]entry, error) {
	if err := r.load(ctx); err != nil {
		return nil, err
	}
	at := r.blockOf(prefix)
	lo, hi := max(at, 0), at+1
	for hi < len(r.index) && bytes.HasPrefix(r.index[hi].first, prefix) {
		hi++
	}
	if hi <= lo {
		return nil, nil
	}
	if hi == lo+1 {
		block, err := r.entries(ctx, lo)
		if err != nil {
			return nil, err
		}
		var out []entry
		for _, e := range block {
			if bytes.HasPrefix(e.key, prefix) {
				out = append(out, e)
			}
		}
		return out, nil
	}
	first, last := r.index[lo], r.index[hi-1]
	b, err := r.read(ctx, int64(first.offset), last.offset+last.length+trailerSize-first.offset)
	if err != nil {
		return nil, err
	}
	var decoded []entry
	for _, ref := range r.index[lo:hi] {
		from := ref.offset - first.offset
		if decoded, err = decodeBlock(decoded, b[from:from+ref.length+trailerSize]); err != nil {
			return nil, err
		}
	}
	i := sort.Search(len(decoded), func(i int) bool { return bytes.Compare(decoded[i].key, prefix) >= 0 })
	j := i
	for j < len(decoded) && bytes.HasPrefix(decoded[j].key, prefix) {
		j++
	}
	return decoded[i:j], nil
}

// blockOf returns the place in the index of the block whose first key is
// the last at or before key, and -1 where every block starts after key.
func (r *runReader) blockOf(key []byte) int {
	// slices.BinarySearchFunc moves key to the heap. sort.Search keeps a
	// caller's key on the stack.
	return sort.Search(len(r.index), func(i int) bool { return bytes.Compare(r.index[i].first, key) > 0 }) - 1
}

// entries returns the entries of the block at a place in the index: the
// block that the reader decoded last where it is that block, and the
// block read from the ledger otherwise, which the reader then keeps.
func (r *runReader) entries(ctx context.Context, at int) ([]entry, error) {
	r.mu.Lock()
	block, kept := r.lastEntries, r.lastEntries != nil && r.last == at
	r.mu.Unlock()
	if kept {
		return block, nil
	}
	block, err := r.block(ctx, r.index[at])
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.last, r.lastEntries = at, block
	r.mu.Unlock()
	return block, nil
}

// all returns every entry of the run, in key order, through one read
// of the whole run.
func (r *runReader) all(ctx context.Context) ([]entry, error) {
	run, err := r.read(ctx, 0, int(r.ref.length))
	if err != nil {
		return nil, err
	}
	return decodeRun(run)
}

// block reads and decodes one block.
func (r *runReader) block(ctx context.Context, ref blockRef) ([]entry, error) {
	b, err := r.read(ctx, int64(ref.offset), ref.length+trailerSize)
	if err != nil {
		return nil, err
	}
	return decodeBlock(nil, b)
}

// read returns n bytes of the run from offset off within it.
//
// Error modes: an error wrapping [ErrDamaged] for a segment that is
// missing or ends before the bytes the run's record states.
func (r *runReader) read(ctx context.Context, off int64, n int) ([]byte, error) {
	b := make([]byte, n)
	got, err := r.ledger.ReadAt(ctx, r.ref.segment, b, r.ref.offset+off)
	if got == n {
		return b, nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return nil, fmt.Errorf("%w: read segment %s: %w", ErrDamaged, r.ref.segment, err)
}

// tableReader reads one table of a generation over its runs, oldest
// first. A newer run's entry decides its key: a row replaces an older
// row, and a tombstone deletes the key.
type tableReader struct {
	runs []*runReader
}

// get returns one key's row, and false where no run contains the key or
// the newest entry of it is a tombstone.
//
// Error modes: an error wrapping [ErrDamaged] for a run that does not
// read whole.
func (t *tableReader) get(ctx context.Context, key []byte) ([]byte, bool, error) {
	for _, r := range slices.Backward(t.runs) {
		e, found, err := r.get(ctx, key)
		if err != nil {
			return nil, false, err
		}
		if found {
			return e.row, !e.dead, nil
		}
	}
	return nil, false, nil
}

// all returns every live row of the table, in key order: the runs
// merged, the newest entry of each key deciding, and tombstones left
// out.
//
// Error modes: an error wrapping [ErrDamaged] for a run that does not
// read whole.
func (t *tableReader) all(ctx context.Context) ([]entry, error) {
	merged, err := t.merged(ctx)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(merged, func(e entry) bool { return e.dead }), nil
}

// scan returns every live row of the table whose key begins with prefix,
// in key order: the runs' entries under the prefix merged, the newest
// entry of each key deciding, and tombstones left out. The entries of the
// first run with any are the merge's start, which no copy precedes.
//
// Error modes: an error wrapping [ErrDamaged] for a run that does not
// read whole.
func (t *tableReader) scan(ctx context.Context, prefix []byte) ([]entry, error) {
	var out []entry
	for _, r := range t.runs {
		entries, err := r.scan(ctx, prefix)
		if err != nil {
			return nil, err
		}
		if len(out) == 0 {
			out = entries
			continue
		}
		out = mergeEntries(out, entries)
	}
	return slices.DeleteFunc(out, func(e entry) bool { return e.dead }), nil
}

// merged returns every key of the table with its newest entry, in key
// order, tombstones included.
func (t *tableReader) merged(ctx context.Context) ([]entry, error) {
	var out []entry
	for _, r := range t.runs {
		entries, err := r.all(ctx)
		if err != nil {
			return nil, err
		}
		out = mergeEntries(out, entries)
	}
	return out, nil
}

// mergeEntries returns older and newer merged in key order, newer's
// entry deciding a key both contain. Both are sorted by key. An empty
// newer returns older itself.
func mergeEntries(older, newer []entry) []entry {
	if len(newer) == 0 {
		return older
	}
	out := make([]entry, 0, len(older)+len(newer))
	i, j := 0, 0
	for i < len(older) && j < len(newer) {
		switch c := bytes.Compare(older[i].key, newer[j].key); {
		case c < 0:
			out = append(out, older[i])
			i++
		case c > 0:
			out = append(out, newer[j])
			j++
		default:
			out = append(out, newer[j])
			i++
			j++
		}
	}
	out = append(out, older[i:]...)
	return append(out, newer[j:]...)
}
