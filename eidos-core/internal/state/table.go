// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
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
// needs through [ledger.Ledger.ReadAt].
//
// # Concurrency
//
// A runReader is safe for concurrent use: the index loads under a
// [sync.Once], and each read of a block is its own.
type runReader struct {
	ref    runRef
	ledger ledger.Ledger
	once   sync.Once
	index  []blockRef
	limit  int
	err    error
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
// before the key.
func (r *runReader) get(ctx context.Context, key []byte) (entry, bool, error) {
	if err := r.load(ctx); err != nil {
		return entry{}, false, err
	}
	at, exact := slices.BinarySearchFunc(r.index, key, func(b blockRef, k []byte) int {
		return bytes.Compare(b.first, k)
	})
	if !exact {
		at--
	}
	if at < 0 {
		return entry{}, false, nil
	}
	block, err := r.block(ctx, r.index[at])
	if err != nil {
		return entry{}, false, err
	}
	i, found := slices.BinarySearchFunc(block, key, func(e entry, k []byte) int {
		return bytes.Compare(e.key, k)
	})
	if !found {
		return entry{}, false, nil
	}
	return block[i], true, nil
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
	return decodeBlock(b)
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
// entry deciding a key both contain. Both are sorted by key.
func mergeEntries(older, newer []entry) []entry {
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
