// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"math/bits"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/internal/grow"
	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// phaseTables are the tables the record of the phases fills, in table
// order.
var phaseTables = [...]Table{
	TableModules, TableValidations, TableClaims, TablePresent,
	TableInvocations, TableReaders, TableAudit, TableChecks,
}

// recordTables are the tables of the record kinds, in [RecordKind] order.
var recordTables = [recordKinds]Table{TableValidations, TableInvocations, TableChecks}

// The buckets [sortByHash] distributes into: about eight elements to a
// bucket, and at most 65,536 buckets, so their counts take at most
// 512 KiB.
const (
	bucketShare   = 8
	maxBucketBits = 16
)

// minChanges is the least capacity a table's list of changes takes on
// its first change.
const minChanges = 64

// keptInvocations are the prior invocations of the plans that do not
// commit: each framed as a row of its entry alone, back to back in buf,
// with the edges each read.
type keptInvocations struct {
	buf   []byte
	rows  []recordRow
	reads []edgeRead
}

// PhaseState is a generation's record of the phases after the load: each
// validation, invocation and check with the edges it read, the records
// that read each edge, the fact store's claims and the facts that read
// present, the audit's findings, and how many packages name each module.
// A warm run reads it to find what an edit makes dirty and what it
// keeps. Its fact tables are the [meta.BagSource] a restored fact store
// reads.
//
// # Concurrency
//
// A PhaseState is safe for concurrent use: it reads through the
// generation, which is, and the present table loads once under a
// [sync.Once].
type PhaseState struct {
	ctx context.Context
	g   *Generation

	presentOnce sync.Once
	present     map[meta.KeyName][]symbol.Identity
	presentErr  error
}

var _ meta.BagSource = (*PhaseState)(nil)

// Phases returns the generation's record of the phases after the load,
// read through ctx.
func (g *Generation) Phases(ctx context.Context) *PhaseState { return &PhaseState{ctx: ctx, g: g} }

// PhaseRun is what [RecordPhases] reads of a run besides its [Recorder]:
// the fact store the run's phases left, and how many packages name each
// module.
type PhaseRun struct {
	Facts   *meta.Facts
	Modules map[plugin.Module]int
}

// PhaseRecord is a run's record of its phases, prepared before any plan
// commits: the prior record of a warm run, read whole, the recorder's
// lanes, and the rows of the tables that no plan's commit decides.
// [PhaseRecord.Commit] completes it into the run's commit once each plan's
// commit has run.
type PhaseRecord struct {
	prior [tableCount][]entry
	rows  [tableCount][]entry
	lanes []*Lane
}

// RecordPhases prepares a run's record of its phases, after the run's
// last record and before any plan commits, so damage it meets in the
// prior record discards the run with nothing written. It reads every
// row of the phase tables of g, the generation the run opened and nil for
// a cold run, and encodes the fact store's claims and the facts that read
// present, the audit's findings and the modules' package counts.
//
// Error modes: an error wrapping [ErrDamaged] for a prior table that does
// not read whole, and the error of a claim whose value is outside the
// fact vocabulary.
func RecordPhases(ctx context.Context, g *Generation, r *Recorder, run PhaseRun) (*PhaseRecord, error) {
	p := &PhaseRecord{lanes: r.lanes}
	if g != nil {
		for _, t := range phaseTables {
			rows, err := g.readers[t].all(ctx)
			if err != nil {
				return nil, fmt.Errorf("state: read the %s table: %w", t, err)
			}
			p.prior[t] = rows
		}
	}
	claims, err := claimRows(run.Facts)
	if err != nil {
		return nil, err
	}
	p.rows[TableClaims] = claims
	p.rows[TablePresent] = presentRows(run.Facts)
	p.rows[TableAudit] = auditRows(r.audits)
	p.rows[TableModules] = moduleRows(run.Modules)
	return p, nil
}

// Commit completes the record into the run's commit, once each plan's
// commit has run: every record the recorder's lanes collected, except the
// invocations of the plans in uncommitted, which do not commit and keep
// the prior record's, and the records that read each edge those records
// read. It compares each table's rows with the prior record's, and
// records the rows that are new or changed and a tombstone for each prior
// row the run no longer has, so a run that changed nothing changes no
// row. Commit cannot fail: a prior invocations row that does not decode
// keeps nothing, so a later run executes its invocations again.
func (p *PhaseRecord) Commit(c *Commit, uncommitted []string) {
	kept := planInvocations(p.prior[TableInvocations], uncommitted, len(p.lanes))
	recordRows(&p.rows, p.lanes, &kept, uncommitted)
	for _, t := range phaseTables {
		c.sorted[t] = diff(p.prior[t], p.rows[t])
	}
}

// planInvocations returns the prior invocations of the plans in
// uncommitted, each framed as a row of its entry alone into a buffer the
// commit gathers at index at, and nothing where every plan commits. A
// row that does not decode, and an entry whose key fields or reads do not
// decode, keep nothing.
func planInvocations(rows []entry, uncommitted []string, at int) keptInvocations {
	var out keptInvocations
	if len(uncommitted) == 0 {
		return out
	}
	for _, e := range rows {
		if len(e.key) != 8 {
			continue
		}
		ref := RecordRef{Kind: RecordInvocation, ID: binary.BigEndian.Uint64(e.key)}
		row := newDecoder(e.row, nil)
		for range row.Count() {
			b := row.Bytes()
			d := newDecoder(b, nil)
			if plan := d.text(); row.Err() != nil || !slices.Contains(uncommitted, plan) {
				continue
			}
			d.match()
			reads := d.edges()
			if d.Err() != nil {
				continue
			}
			start := len(out.buf)
			out.buf = wire.AppendBytes(append(out.buf, singleEntry), b)
			out.rows = append(out.rows, recordRow{id: ref.ID, buf: at, start: start, end: len(out.buf)})
			for _, h := range reads {
				out.reads = append(out.reads, edgeRead{edge: h, ref: ref})
			}
		}
	}
	return out
}

// recordRows fills the rows of the four record tables, each sorted by
// key: each validation, invocation and check under its ID, where the
// records that share an ID share a row, and each edge's readers under the
// edge's hash. The lanes of the plans in uncommitted are left out, and
// kept contains those plans' prior invocations.
func recordRows(rows *[tableCount][]entry, lanes []*Lane, kept *keptInvocations, uncommitted []string) {
	bufs := make([][]byte, len(lanes)+1)
	var (
		records [recordKinds][][]recordRow
		reads   [][]edgeRead
	)
	for i, l := range lanes {
		bufs[i] = l.buf
		if l.plan != "" && slices.Contains(uncommitted, l.plan) {
			continue
		}
		for k := range records {
			records[k] = append(records[k], l.records[k])
		}
		reads = append(reads, l.reads)
	}
	bufs[len(lanes)] = kept.buf
	records[RecordInvocation-1] = append(records[RecordInvocation-1], kept.rows)
	reads = append(reads, kept.reads)
	compare := func(a, b recordRow) int {
		if a.id != b.id {
			return cmp.Compare(a.id, b.id)
		}
		return bytes.Compare(bufs[a.buf][a.start:a.end], bufs[b.buf][b.start:b.end])
	}
	for k, t := range recordTables {
		rows[t] = groupedRows(sortByHash(records[k], func(r recordRow) uint64 { return r.id }, compare), bufs)
	}
	rows[TableReaders] = readerRows(sortByHash(reads, func(r edgeRead) uint64 { return uint64(r.edge) }, compareReads))
}

// groupedRows returns the rows of one record table, sorted by key, of its
// records sorted by ID: each record's row as its lane framed it, and one
// row for the records that share an ID, which lists each one's entry.
func groupedRows(sorted []recordRow, bufs [][]byte) []entry {
	keys := make([]byte, 0, 8*len(sorted))
	out := make([]entry, 0, len(sorted))
	var shared []byte
	for i := 0; i < len(sorted); {
		j := i + 1
		for j < len(sorted) && sorted[j].id == sorted[i].id {
			j++
		}
		from := len(keys)
		keys = binary.BigEndian.AppendUint64(keys, sorted[i].id)
		key := keys[from:len(keys):len(keys)]
		first := sorted[i]
		row := bufs[first.buf][first.start:first.end:first.end]
		if j-i > 1 {
			start := len(shared)
			shared = binary.AppendUvarint(shared, uint64(j-i))
			for _, r := range sorted[i:j] {
				shared = append(shared, bufs[r.buf][r.start+1:r.end]...)
			}
			row = shared[start:len(shared):len(shared)]
		}
		out = append(out, entry{key: key, row: row})
		i = j
	}
	return out
}

// readerRows returns the readers table's rows of a run's reads sorted by
// edge, then by record, sorted by key: each edge's hash with the records
// that read it, each once. It drops repeated reads in place, and writes
// the keys and the rows into one buffer each, sized before it fills them.
func readerRows(sorted []edgeRead) []entry {
	sorted = slices.Compact(sorted)
	groups, size := 0, 0
	for i := 0; i < len(sorted); {
		j := i + 1
		for j < len(sorted) && sorted[j].edge == sorted[i].edge {
			j++
		}
		groups++
		size += uvarintLen(uint64(j-i)) + (j-i)*refSize
		i = j
	}
	keys := make([]byte, 0, 8*groups)
	rows := make([]byte, 0, size)
	out := make([]entry, 0, groups)
	for i := 0; i < len(sorted); {
		j := i + 1
		for j < len(sorted) && sorted[j].edge == sorted[i].edge {
			j++
		}
		from, start := len(keys), len(rows)
		keys = binary.BigEndian.AppendUint64(keys, uint64(sorted[i].edge))
		rows = appendReaders(rows, sorted[i:j])
		out = append(out, entry{key: keys[from:len(keys):len(keys)], row: rows[start:len(rows):len(rows)]})
		i = j
	}
	return out
}

// sortedRows sorts a table's rows by key, in place, and returns them.
func sortedRows(rows []entry) []entry {
	slices.SortFunc(rows, compareEntries)
	return rows
}

// compareReads orders two reads by edge, then by record.
func compareReads(a, b edgeRead) int {
	if a.edge != b.edge {
		return cmp.Compare(a.edge, b.edge)
	}
	return a.ref.Compare(b.ref)
}

// sortByHash returns every element of parts in one new slice sorted by
// compare. compare orders elements by hash first. hash is uniform over
// its 64 bits, as the leading bytes of a SHA-256 are. A counting pass
// and a scatter pass place the elements into buckets by the top bits of
// their hashes, about [bucketShare] to a bucket. Each bucket then sorts
// by compare, so the comparisons grow linearly with the number of
// elements. It allocates the slice and the buckets' counts. For no
// element it allocates nothing.
func sortByHash[T any](parts [][]T, hash func(T) uint64, compare func(a, b T) int) []T {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	if n == 0 {
		return nil
	}
	shift := 64 - min(maxBucketBits, bits.Len(uint(n/bucketShare)))
	// next counts each bucket's elements, then contains where the scatter
	// places the bucket's next element, and the bucket's end once it has.
	next := make([]int, 1<<(64-shift))
	for _, p := range parts {
		for _, v := range p {
			next[hash(v)>>shift]++
		}
	}
	start := 0
	for b, count := range next {
		next[b] = start
		start += count
	}
	out := make([]T, n)
	for _, p := range parts {
		for _, v := range p {
			b := hash(v) >> shift
			out[next[b]] = v
			next[b]++
		}
	}
	start = 0
	for _, end := range next {
		slices.SortFunc(out[start:end], compare)
		start = end
	}
	return out
}

// diff returns the changes that turn a table's prior rows into its new
// rows: each new row the prior rows lack or keep with other bytes, and a
// tombstone for each prior key the new rows lack. Both lists are sorted by
// key without repeats, and so are the changes. A table without prior rows
// changes by its new rows, which diff returns as they are.
func diff(prior, rows []entry) []entry {
	if len(prior) == 0 {
		return rows
	}
	var out []entry
	i, j := 0, 0
	for i < len(prior) || j < len(rows) {
		var order int
		if j == len(rows) {
			order = -1
		} else if i == len(prior) {
			order = 1
		} else {
			order = bytes.Compare(prior[i].key, rows[j].key)
		}
		if order < 0 {
			out = append(grow.Room(out, 1, minChanges), entry{key: prior[i].key, dead: true})
			i++
			continue
		}
		if order > 0 || !bytes.Equal(prior[i].row, rows[j].row) {
			out = append(grow.Room(out, 1, minChanges), rows[j])
		}
		if order == 0 {
			i++
		}
		j++
	}
	return out
}
