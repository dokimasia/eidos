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

// keptRecords lists the prior records that a commit keeps. These are the
// invocations of the plans that do not commit, and on a warm run the
// shared records that the run did not drop. buf contains each record as a
// row of its own entry. rows lists the records by kind, and reads lists
// the edges that each record read.
type keptRecords struct {
	buf   []byte
	rows  [recordKinds][]recordRow
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
// A PhaseState is safe for concurrent use. It reads through the
// generation, which is safe for concurrent use, and the present table
// loads once under a [sync.Once]. The decodes that share strings take
// turns under a lock.
type PhaseState struct {
	ctx context.Context
	g   *Generation

	presentOnce sync.Once
	present     map[meta.KeyName][]symbol.Identity
	presentErr  error

	// strings are the strings that the decodes of the claims and of the
	// present table share, so the texts that many claims repeat, such as
	// a key, a plugin or a file, decode to one string each.
	strings interner
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

// PhaseRecord is a run's record of its phases, which the run prepares
// before any plan commits. It contains the prior record of a warm run,
// read whole, the lanes of the recorder, the shared records that a warm
// run dropped, and the rows of the tables that no plan's commit decides.
// [PhaseRecord.Commit] completes the record into the run's commit after
// each plan's commit has run.
type PhaseRecord struct {
	prior [tableCount][]entry
	rows  [tableCount][]entry
	lanes []*Lane
	// keep reports whether the record is a warm run's. Such a record keeps
	// the prior shared records that the run did not drop. validations
	// lists the dropped validations by subject, and invocations lists the
	// dropped annotator invocations by match.
	keep        bool
	validations map[symbol.Identity]struct{}
	invocations map[plugin.MatchKey]struct{}
}

// RecordPhases prepares a run's record of its phases, after the run's
// last record and before any plan commits, so damage it meets in the
// prior record discards the run with nothing written. It reads every
// row of the phase tables of g, the generation the run opened and nil for
// a cold run, and encodes the fact store's claims and the facts that read
// present, the audit's findings and the modules' package counts. On a
// warm run, the claims rows start from the prior rows. The row of each
// bag that the run touched replaces the prior row of its subject. A prior
// row is dropped when the run withdrew claims from its subject and left
// the subject without a claim.
//
// Error modes: an error wrapping [ErrDamaged] for a prior table that does
// not read whole, and the error of a claim whose value is outside the
// fact vocabulary.
func RecordPhases(ctx context.Context, g *Generation, r *Recorder, run PhaseRun) (*PhaseRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := &PhaseRecord{lanes: r.lanes, keep: r.keep && g != nil, validations: r.validations, invocations: r.invocations}
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
	if p.keep {
		claims = overlaidClaims(p.prior[TableClaims], claims, r.withdrew)
	}
	p.rows[TableClaims] = claims
	p.rows[TablePresent] = presentRows(run.Facts)
	p.rows[TableAudit] = auditRows(r.audits)
	p.rows[TableModules] = moduleRows(run.Modules)
	return p, nil
}

// Commit completes the record into the run's commit after each plan's
// commit has run. The commit receives every record that the lanes of the
// recorder collected, except the invocations of the plans in
// uncommitted. Those plans keep their prior invocations instead. On a
// warm run, the commit also receives the prior shared records that the
// run kept. The readers table lists the records that read each edge.
//
// Commit compares the rows of each table with the prior rows. It writes
// each row that is new or changed, and a tombstone for each prior row
// that the run no longer has, so the commit of an unchanged run is empty.
// Commit cannot fail. A prior row that does not decode is not kept, and a
// later run executes its records again.
func (p *PhaseRecord) Commit(c *Commit, uncommitted []string) {
	kept := p.keptRecords(uncommitted)
	recordRows(&p.rows, p.lanes, &kept, uncommitted)
	for _, t := range phaseTables {
		c.sorted[t] = diff(p.prior[t], p.rows[t])
	}
}

// keptRecords returns the prior records that the commit keeps. These are
// the invocations of the plans in uncommitted, and on a warm run each
// validation and annotator invocation that the run did not drop. It
// frames each record as a row of its own entry, in a buffer that the
// commit places after the buffers of the lanes. A row that does not
// decode is not kept, and neither is an entry whose key fields or reads
// do not decode. The plan of an invocation decides first, from its bytes,
// so an invocation of a plan that commits decodes nothing more.
func (p *PhaseRecord) keptRecords(uncommitted []string) keptRecords {
	var out keptRecords
	at := len(p.lanes)
	if p.keep {
		eachEntry(p.prior[TableValidations], func(id uint64, b []byte, d *decoder) {
			subject := d.identity()
			if _, dropped := p.validations[subject]; dropped {
				return
			}
			v := decodeValidation(d, subject)
			if d.Err() == nil {
				out.add(RecordRef{Kind: RecordValidation, ID: id}, b, v.Reads, at)
			}
		})
	}
	if !p.keep && len(uncommitted) == 0 {
		return out
	}
	eachEntry(p.prior[TableInvocations], func(id uint64, b []byte, d *decoder) {
		plan := d.Bytes()
		failed := slices.ContainsFunc(uncommitted, func(name string) bool { return name == string(plan) })
		if !failed && (len(plan) > 0 || !p.keep) {
			return
		}
		if _, dropped := p.invocations[d.match()]; dropped && !failed {
			return
		}
		if reads := d.edges(); d.Err() == nil {
			out.add(RecordRef{Kind: RecordInvocation, ID: id}, b, reads, at)
		}
	})
	return out
}

// add frames one prior entry as a row of its own in the kept buffer,
// which the commit gathers at index at. It lists the record under ref,
// and lists each edge that the record read.
func (k *keptRecords) add(ref RecordRef, entry []byte, reads []EdgeHash, at int) {
	start := len(k.buf)
	k.buf = wire.AppendBytes(append(k.buf, singleEntry), entry)
	k.rows[ref.Kind-1] = append(k.rows[ref.Kind-1], recordRow{id: ref.ID, buf: at, start: start, end: len(k.buf)})
	for _, h := range reads {
		k.reads = append(k.reads, edgeRead{edge: h, ref: ref})
	}
}

// eachEntry calls visit once for each entry in the rows of a record
// table. visit receives the row's ID, the entry's bytes and a decoder
// over those bytes, which is valid until visit returns. A row whose key
// is not an ID visits nothing. A row whose framing does not decode visits
// only the entries before the fault. It allocates one decoder, which
// every visit reuses.
func eachEntry(rows []entry, visit func(id uint64, b []byte, d *decoder)) {
	d := &decoder{}
	for _, e := range rows {
		if len(e.key) != 8 {
			continue
		}
		id := binary.BigEndian.Uint64(e.key)
		row := decoder{Decoder: wire.NewDecoder(e.row)}
		for range row.Count() {
			b := row.Bytes()
			if row.Err() != nil {
				break
			}
			*d = decoder{Decoder: wire.NewDecoder(b)}
			visit(id, b, d)
		}
	}
}

// overlaidClaims returns the claims rows of a warm run, sorted by key. It
// starts from the prior rows. The run's row of a subject replaces the
// prior row of that subject, and the run's rows of other subjects are
// added. A prior row is dropped when its subject is in withdrew and the
// run has no row for the subject. Both inputs are sorted by key.
func overlaidClaims(prior, fresh []entry, withdrew map[symbol.Identity]struct{}) []entry {
	emptied := make(map[string]struct{}, len(withdrew))
	for id := range withdrew {
		emptied[string(identityKey(nil, id))] = struct{}{}
	}
	out := make([]entry, 0, len(prior)+len(fresh))
	i, j := 0, 0
	for i < len(prior) || j < len(fresh) {
		order := 1
		switch {
		case j == len(fresh):
			order = -1
		case i < len(prior):
			order = bytes.Compare(prior[i].key, fresh[j].key)
		}
		if order < 0 {
			if _, gone := emptied[string(prior[i].key)]; !gone {
				out = append(out, prior[i])
			}
			i++
			continue
		}
		out = append(out, fresh[j])
		if order == 0 {
			i++
		}
		j++
	}
	return out
}

// recordRows fills the rows of the four record tables, each sorted by
// key. Each validation, invocation and check is a row under its ID, and
// records that share an ID share a row. The readers of each edge are a
// row under the edge's hash. recordRows leaves out the lanes of the plans
// in uncommitted, and kept contains the prior records that the commit
// keeps. It makes each list of records and reads once, at the size that
// the lanes and kept need.
func recordRows(rows *[tableCount][]entry, lanes []*Lane, kept *keptRecords, uncommitted []string) {
	bufs := make([][]byte, len(lanes)+1)
	var records [recordKinds][][]recordRow
	for k := range records {
		records[k] = make([][]recordRow, 0, len(lanes)+1)
	}
	reads := make([][]edgeRead, 0, len(lanes)+1)
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
	for k := range records {
		records[k] = append(records[k], kept.rows[k])
	}
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
