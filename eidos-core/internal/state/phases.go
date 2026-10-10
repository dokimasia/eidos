// Copyright Dokimasia B.V. 2026
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
	TableModules, TableValidations, TableClaims, TablePresent, TableInvocations, TableReaders,
	TablePlans, TableGroups, TableArtifacts, TableNames, TableAudit, TableChecks,
}

// recordTables are the tables of the record kinds, in [RecordKind] order.
var recordTables = [recordKinds]Table{TableValidations, TableInvocations, TableChecks, TableGroups}

// wholeTables are the phase tables that a warm run's record reads whole:
// the modules and plans tables, which have a row for each module and for
// each plan, and the audit and checks tables, whose rows every run records
// again.
var wholeTables = [...]Table{TableModules, TablePlans, TableAudit, TableChecks}

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
// validation, invocation, check and group with the edges it read, the
// records that read each edge, each plan's files and the names they
// declare, the plans whose work is pending, the fact store's claims and
// the facts that read present, the audit's findings, and the number of
// packages in each module. A warm run reads it to find what an edit makes
// dirty and what it keeps. Its fact tables are the [meta.BagSource] a
// restored fact store reads.
//
// # Concurrency
//
// A PhaseState is safe for concurrent use. It reads through the
// generation, which is safe for concurrent use. The present rows of each
// key load once under a lock, and the decodes that share strings take
// turns under a lock.
type PhaseState struct {
	ctx context.Context
	g   *Generation

	// present keeps the subjects of each key whose present rows
	// [PhaseState.Present] read, under presentMu.
	presentMu sync.Mutex
	present   map[meta.KeyName][]symbol.Identity

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
// the fact store the run's phases left, and the number of packages in each
// module.
type PhaseRun struct {
	Facts   *meta.Facts
	Modules map[plugin.Module]int
}

// PhaseRecord is a run's record of its phases, which the run prepares
// before any plan commits. It contains the prior rows that the commit may
// change, the lanes of the recorder, the records that a warm run dropped,
// and the rows of the tables that no plan's commit decides.
// [PhaseRecord.Commit] completes the record into the run's commit after
// each plan's commit has run.
type PhaseRecord struct {
	// prior are the prior rows of each phase table that the commit may
	// change, sorted by key: every row of the table, or on a warm run that
	// executed each plan in part, the rows of the keys that the run
	// touched. rows are the run's rows of the same keys.
	prior [tableCount][]entry
	rows  [tableCount][]entry
	// touched lists the records whose prior rows the record read by key,
	// and is nil where it read the record tables whole. edges are the
	// edges whose readers the touched records change, sorted, and the
	// readers rows of those edges keep every other record that the prior
	// rows list.
	touched map[RecordRef]struct{}
	edges   []EdgeHash
	// strings are the strings that the record's decodes of prior entries
	// share, so the texts that many entries repeat, such as a plan, a
	// plugin or a package, decode to one string each.
	strings map[string]string
	lanes   []*Lane
	// keep reports whether the record is a warm run's. Such a record keeps
	// the prior records and files that the run did not drop, of the shared
	// phases and of each plan but those in replaced: the plans with a lane
	// that [Recorder.KeepPlan] did not name, whose lanes replace every
	// prior record and file of the plan. validations lists the dropped
	// validations by subject, invocations lists the dropped annotator
	// invocations by match, and checks the dropped checks by name.
	// planInvocations and groups list the dropped invocations and groups of
	// each plan, and files the dropped files.
	keep            bool
	replaced        map[string]struct{}
	validations     map[symbol.Identity]struct{}
	invocations     map[plugin.MatchKey]struct{}
	checks          map[plugin.ID]struct{}
	planInvocations map[string]map[plugin.MatchKey]struct{}
	groups          map[string]map[plugin.UnitRef]struct{}
	files           map[string]struct{}
}

// RecordPhases prepares a run's record of its phases, after the run's
// last record and before any plan commits, so damage it meets in the
// prior record discards the run with nothing written. g is the generation
// the run opened, and nil for a cold run. RecordPhases encodes the fact
// store's claims, the audit's findings and the modules' package counts.
//
// A warm run whose every plan with a lane executed in part, as
// [Recorder.KeepPlan] names it, reads the prior rows that its commit may
// change, each through a lookup of its key:
//   - the rows of each record that a lane records or that the run dropped
//   - the readers rows of each edge whose readers those records change
//   - the claims rows of each subject whose bag the run touched, and the
//     present rows of each key that such a subject claims or claimed
//   - the artifacts and names rows of each file that the run dropped
//
// It reads the modules, plans, audit and checks tables whole. Any other
// run over a generation reads every row of the phase tables, because the
// lane of a plan that ran whole replaces every record and file of the
// plan, which a lookup by key cannot find.
//
// On a warm run, the row of each bag that the run touched replaces the
// prior row of its subject. A prior row is dropped when the run withdrew
// claims from its subject and left the subject without a claim. The
// present rows follow the claims.
//
// Error modes: an error wrapping [ErrDamaged] for a prior row that does
// not read whole, and the error of a claim whose value is outside the
// fact vocabulary.
func RecordPhases(ctx context.Context, g *Generation, r *Recorder, run PhaseRun) (*PhaseRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := &PhaseRecord{
		lanes: r.lanes, keep: r.keep && g != nil, validations: r.validations, invocations: r.invocations,
		checks: r.checks, planInvocations: r.planInvocations, groups: r.groups, files: r.files,
	}
	for _, l := range r.lanes {
		if _, kept := r.plans[l.plan]; p.keep && l.plan != "" && !kept {
			if p.replaced == nil {
				p.replaced = map[string]struct{}{}
			}
			p.replaced[l.plan] = struct{}{}
		}
	}
	claims, err := claimRows(run.Facts)
	if err != nil {
		return nil, err
	}
	p.rows[TableAudit] = auditRows(r.audits)
	p.rows[TableModules] = moduleRows(run.Modules)
	if g == nil {
		p.rows[TableClaims], p.rows[TablePresent] = claims, presentRows(run.Facts)
		return p, nil
	}
	if p.keep && len(p.replaced) == 0 {
		if err := p.readTouched(ctx, g, claims, r.withdrew, run.Facts); err != nil {
			return nil, err
		}
		return p, nil
	}
	for _, t := range phaseTables {
		rows, err := g.readers[t].all(ctx)
		if err != nil {
			return nil, fmt.Errorf("state: read the %s table: %w", t, err)
		}
		p.prior[t] = rows
	}
	if p.keep {
		claims = overlaidClaims(p.prior[TableClaims], claims, r.withdrew)
	}
	p.rows[TableClaims], p.rows[TablePresent] = claims, presentRows(run.Facts)
	return p, nil
}

// Commit completes the record into the run's commit after each plan's
// commit has run. The commit receives every record and every file that
// the lanes of the recorder collected, except those of the plans in
// uncommitted. Those plans keep their prior records and files instead,
// and the plans table lists each of them, so the next warm run runs it
// whole. On a warm run, the commit also receives the prior records and
// files that the run did not drop, of the shared phases and of each plan
// whose lane does not replace them. The readers table lists the records
// that read each edge.
//
// Commit compares the rows of each table with the prior rows that the
// record read. It writes each row that is new or changed, and a tombstone
// for each prior row that the run no longer has, so the commit of an
// unchanged run is empty. Commit cannot fail. A prior row that the record
// read and that does not decode is not kept, and a later run executes its
// records again.
func (p *PhaseRecord) Commit(c *Commit, uncommitted []string) {
	kept := p.keptRecords(uncommitted)
	reads := recordRows(&p.rows, p.lanes, &kept, uncommitted)
	if p.touched == nil {
		p.rows[TableReaders] = readerRows(reads)
	} else {
		p.rows[TableReaders] = mergedReaderRows(p.prior[TableReaders], reads, p.touched, p.edges)
	}
	for k, t := range plainTables {
		p.rows[t] = plainRows(p.lanes, k, p.keptRows(t, uncommitted), uncommitted)
	}
	p.rows[TablePlans] = planRows(uncommitted)
	for _, t := range phaseTables {
		c.sorted[t] = diff(p.prior[t], p.rows[t])
	}
}

// readTouched reads the prior rows that a warm run's commit may change, as
// [RecordPhases] lists them, and completes the run's claims and present
// rows. fresh are the claims rows of the run's bags, and withdrew the
// subjects whose claims the run withdrew.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not read
// whole.
func (p *PhaseRecord) readTouched(
	ctx context.Context, g *Generation, fresh []entry, withdrew map[symbol.Identity]struct{}, facts *meta.Facts,
) error {
	for _, t := range wholeTables {
		rows, err := g.readers[t].all(ctx)
		if err != nil {
			return fmt.Errorf("state: read the %s table: %w", t, err)
		}
		p.prior[t] = rows
	}
	if err := p.readRecords(ctx, g); err != nil {
		return err
	}
	if err := p.readClaims(ctx, g, fresh, withdrew, facts); err != nil {
		return err
	}
	return p.readFiles(ctx, g)
}

// readRecords reads the rows of each record that a lane records or that
// the run dropped, and lists every record of those rows, of the checks
// table and of the lanes as touched. It then reads the readers row of each
// edge whose readers the touched records change, where every plan commits:
// an edge that a touched record reads now and did not read, or read and
// does not read now. A plan that does not commit keeps its records, which
// leaves the rows of their edges as they are.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not read
// whole.
func (p *PhaseRecord) readRecords(ctx context.Context, g *Generation) error {
	var ids [recordKinds][]uint64
	for _, l := range p.lanes {
		for k, rows := range l.records {
			for _, r := range rows {
				ids[k] = append(ids[k], r.id)
			}
		}
	}
	for subject := range p.validations {
		ids[RecordValidation-1] = append(ids[RecordValidation-1], ValidationRef(subject).ID)
	}
	for m := range p.invocations {
		ids[RecordInvocation-1] = append(ids[RecordInvocation-1], InvocationRef("", m).ID)
	}
	for plan, dropped := range p.planInvocations {
		for m := range dropped {
			ids[RecordInvocation-1] = append(ids[RecordInvocation-1], InvocationRef(plan, m).ID)
		}
	}
	for plan, dropped := range p.groups {
		for key := range dropped {
			ids[RecordGroup-1] = append(ids[RecordGroup-1], GroupRef(plan, key).ID)
		}
	}
	for _, e := range p.prior[TableChecks] {
		if len(e.key) == 8 {
			ids[RecordCheck-1] = append(ids[RecordCheck-1], binary.BigEndian.Uint64(e.key))
		}
	}
	p.touched = map[RecordRef]struct{}{}
	p.strings = map[string]string{}
	var was []edgeRead
	entries := &decoder{interned: p.strings}
	for k, t := range recordTables {
		kind := RecordKind(k + 1)
		slices.Sort(ids[k])
		ids[k] = slices.Compact(ids[k])
		for _, id := range ids[k] {
			p.touched[RecordRef{Kind: kind, ID: id}] = struct{}{}
		}
		if t != TableChecks {
			rows, err := lookupRows(ctx, g, t, hashKeys(ids[k]))
			if err != nil {
				return err
			}
			p.prior[t] = rows
		}
		eachEntry(p.prior[t], entries, func(id uint64, _ []byte, d *decoder) {
			for _, e := range entryReads(kind, d) {
				was = append(was, edgeRead{edge: e, ref: RecordRef{Kind: kind, ID: id}})
			}
		})
	}
	is := p.keptRecords(nil).reads
	for _, l := range p.lanes {
		is = append(is, l.reads...)
	}
	p.edges = changedEdges(was, is)
	rows, err := lookupRows(ctx, g, TableReaders, hashKeys(p.edges))
	p.prior[TableReaders] = rows
	return err
}

// readClaims reads the claims rows of each subject that the run's bags
// list or whose claims the run withdrew, and completes the run's claims
// rows over them. For each such subject whose claims row changed, it reads
// the present row of each key that the subject's prior or new claims
// name, and lists the present row of each of those keys that reads
// present on the subject now.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not read
// whole.
func (p *PhaseRecord) readClaims(
	ctx context.Context, g *Generation, fresh []entry, withdrew map[symbol.Identity]struct{}, facts *meta.Facts,
) error {
	subjects := make([][]byte, 0, len(fresh)+len(withdrew))
	for _, e := range fresh {
		subjects = append(subjects, e.key)
	}
	for id := range withdrew {
		subjects = append(subjects, identityKey(nil, id))
	}
	slices.SortFunc(subjects, bytes.Compare)
	subjects = slices.CompactFunc(subjects, bytes.Equal)
	prior, err := lookupRows(ctx, g, TableClaims, subjects)
	if err != nil {
		return err
	}
	p.prior[TableClaims] = prior
	p.rows[TableClaims] = overlaidClaims(prior, fresh, withdrew)
	var keys [][]byte
	for _, sk := range subjects {
		was, is := rowOf(prior, sk), rowOf(fresh, sk)
		if bytes.Equal(was, is) {
			continue
		}
		subject, parsed := parseIdentityKey(sk, nil)
		if !parsed {
			continue
		}
		for _, name := range claimedKeys(p.strings, was, is) {
			key := append(append([]byte(name), keySep), sk...)
			keys = append(keys, key)
			if presentOn(facts, subject, name) {
				p.rows[TablePresent] = append(p.rows[TablePresent], entry{key: key, row: []byte{}})
			}
		}
	}
	p.rows[TablePresent] = sortedRows(p.rows[TablePresent])
	slices.SortFunc(keys, bytes.Compare)
	p.prior[TablePresent], err = lookupRows(ctx, g, TablePresent, keys)
	return err
}

// readFiles reads the artifacts rows of each file that the run dropped,
// under its path and under its path in lower case, and the names rows of
// each name that such a file's prior artifact lists.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not read
// whole.
func (p *PhaseRecord) readFiles(ctx context.Context, g *Generation) error {
	keys := make([][]byte, 0, 2*len(p.files))
	for at := range p.files {
		keys = append(keys, []byte(at), foldedKey(nil, at))
	}
	slices.SortFunc(keys, bytes.Compare)
	artifacts, err := lookupRows(ctx, g, TableArtifacts, keys)
	if err != nil {
		return err
	}
	p.prior[TableArtifacts] = artifacts
	var names [][]byte
	for _, e := range artifacts {
		if e.key[0] == keySep {
			continue
		}
		d := newDecoder(e.row, nil)
		d.interned = p.strings
		a := decodeArtifact(d, string(e.key))
		if d.Err() != nil {
			continue
		}
		for i := range a.Names {
			n := &a.Names[i]
			names = append(names, scopeRowKey(nil, a.Entry.Plan, n))
			if !n.Origin.IsZero() {
				names = append(names, originRowKey(nil, a.Entry.Plan, n))
			}
		}
	}
	slices.SortFunc(names, bytes.Compare)
	names = slices.CompactFunc(names, bytes.Equal)
	p.prior[TableNames], err = lookupRows(ctx, g, TableNames, names)
	return err
}

// keptRecords returns the prior records that the commit keeps. These are
// the invocations and the groups of the plans in uncommitted, and on a
// warm run each validation, annotator invocation and check that the run
// did not drop, and each invocation and group that the run did not drop
// of a plan whose lane does not replace them. It frames each record as a
// row of its own entry, in a buffer that the commit places after the
// buffers of the lanes. A row that does not decode is not kept, and
// neither is an entry whose key fields or reads do not decode. The plan
// of an invocation or a group decides first, from its bytes, so a record
// of a plan that commits decodes nothing more.
func (p *PhaseRecord) keptRecords(uncommitted []string) keptRecords {
	var out keptRecords
	if !p.keep && len(uncommitted) == 0 {
		return out
	}
	if p.strings == nil {
		p.strings = map[string]string{}
	}
	at := len(p.lanes)
	entries := &decoder{interned: p.strings}
	if p.keep {
		eachEntry(p.prior[TableValidations], entries, func(id uint64, b []byte, d *decoder) {
			subject := d.identity()
			if _, dropped := p.validations[subject]; dropped {
				return
			}
			v := decodeValidation(d, subject)
			if d.Err() == nil {
				out.add(RecordRef{Kind: RecordValidation, ID: id}, b, v.Reads, at)
			}
		})
		eachEntry(p.prior[TableChecks], entries, func(id uint64, b []byte, d *decoder) {
			if _, dropped := p.checks[plugin.ID(d.text())]; dropped {
				return
			}
			if reads := d.edges(); d.Err() == nil {
				out.add(RecordRef{Kind: RecordCheck, ID: id}, b, reads, at)
			}
		})
	}
	eachEntry(p.prior[TableInvocations], entries, func(id uint64, b []byte, d *decoder) {
		plan := d.Bytes()
		failed := slices.ContainsFunc(uncommitted, func(name string) bool { return name == string(plan) })
		if !failed && !p.keeps(plan) {
			return
		}
		m := d.match()
		if !failed && p.droppedInvocation(plan, m) {
			return
		}
		if reads := d.edges(); d.Err() == nil {
			out.add(RecordRef{Kind: RecordInvocation, ID: id}, b, reads, at)
		}
	})
	eachEntry(p.prior[TableGroups], entries, func(id uint64, b []byte, d *decoder) {
		plan := d.Bytes()
		failed := slices.ContainsFunc(uncommitted, func(name string) bool { return name == string(plan) })
		if !failed && !p.keeps(plan) {
			return
		}
		g := decodeGroup(d, "", d.unitRef())
		if _, dropped := p.groups[string(plan)][g.Key]; d.Err() != nil || dropped && !failed {
			return
		}
		out.add(RecordRef{Kind: RecordGroup, ID: id}, b, g.Reads, at)
	})
	return out
}

// keeps reports whether a warm run's commit keeps the prior records of a
// plan that commits, unless the run dropped them. The commit keeps the
// records of the annotators, which are under the empty plan, and the
// records of each plan whose lane does not replace them.
func (p *PhaseRecord) keeps(plan []byte) bool {
	_, replaced := p.replaced[string(plan)]
	return p.keep && !replaced
}

// droppedInvocation reports whether the run dropped the prior record of
// one invocation of a plan, the empty plan for an annotator's.
func (p *PhaseRecord) droppedInvocation(plan []byte, m plugin.MatchKey) bool {
	dropped := p.invocations
	if len(plan) > 0 {
		dropped = p.planInvocations[string(plan)]
	}
	_, found := dropped[m]
	return found
}

// keptRows returns the prior rows of a table without readers that the
// commit keeps: every row of a plan in uncommitted, and on a warm run each
// row whose file the run did not drop of a plan whose lane does not
// replace it. A row whose plan or file does not decode is not kept.
func (p *PhaseRecord) keptRows(t Table, uncommitted []string) []entry {
	if len(uncommitted) == 0 && !p.keep {
		return nil
	}
	var out []entry
	for _, e := range p.prior[t] {
		plan, file, decoded := rowOwner(t, e)
		if !decoded {
			continue
		}
		failed := slices.ContainsFunc(uncommitted, func(name string) bool { return name == string(plan) })
		_, dropped := p.files[string(file)]
		if failed || len(plan) > 0 && p.keeps(plan) && !dropped {
			out = append(out, e)
		}
	}
	return out
}

// rowOwner returns the plan and the file of a prior row of a table
// without readers, and reports false for a row whose plan does not
// decode. A names row states its plan in its key and its file first in
// its row. An artifact's row states its plan first, under the path as its
// key, and its folded row states its plan, under a key that ends with the
// path.
func rowOwner(t Table, e entry) (plan, file []byte, decoded bool) {
	d := wire.NewDecoder(e.row)
	switch {
	case t == TableNames:
		plan, file = rowPlan(e.key), d.Bytes()
	case len(e.key) > 0 && e.key[0] == keySep:
		_, file, decoded = bytes.Cut(e.key[1:], []byte{keySep})
		plan = d.Bytes()
		return plan, file, decoded && d.Err() == nil
	default:
		plan, file = d.Bytes(), e.key
	}
	return plan, file, d.Err() == nil
}

// plainRows returns the rows of the table without readers at a place in
// [plainTables], sorted by key: the rows of the lanes of the plans that
// commit, and the kept rows. A lane's row replaces a kept row under the
// same key, because the file that a committing plan writes at a path
// replaces the record of an earlier file there. It allocates nothing for a
// table without a row.
func plainRows(lanes []*Lane, table int, kept []entry, uncommitted []string) []entry {
	n := len(kept)
	for _, l := range lanes {
		n += len(l.plain[table])
	}
	if n == 0 {
		return nil
	}
	rows := make([]entry, 0, n)
	for _, l := range lanes {
		if l.plan != "" && slices.Contains(uncommitted, l.plan) {
			continue
		}
		for _, r := range l.plain[table] {
			rows = append(rows, entry{key: l.buf[r.start:r.mid:r.mid], row: l.buf[r.mid:r.end:r.end]})
		}
	}
	rows = append(rows, kept...)
	slices.SortStableFunc(rows, compareEntries)
	return slices.CompactFunc(rows, func(a, b entry) bool { return bytes.Equal(a.key, b.key) })
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
// table. visit receives the row's ID, the entry's bytes and d, which
// eachEntry points at those bytes and which is valid until visit returns.
// d keeps the strings that it interns. A row whose key is not an ID
// visits nothing. A row whose framing does not decode visits only the
// entries before the fault. It allocates nothing.
func eachEntry(rows []entry, d *decoder, visit func(id uint64, b []byte, d *decoder)) {
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
			d.Decoder = wire.NewDecoder(b)
			visit(id, b, d)
		}
	}
}

// lookupRows returns the live rows of a table under keys, which are sorted
// without repeats, in key order. A key that the table does not contain
// has no row.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not read
// whole.
func lookupRows(ctx context.Context, g *Generation, t Table, keys [][]byte) ([]entry, error) {
	var out []entry
	for _, k := range keys {
		row, held, err := g.readers[t].get(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("state: read the %s table: %w", t, err)
		}
		if held {
			out = append(grow.Room(out, 1, minChanges), entry{key: k, row: row})
		}
	}
	return out, nil
}

// hashKeys returns the key of each record ID or edge hash, its eight bytes
// big-endian, in one buffer.
func hashKeys[T ~uint64](hs []T) [][]byte {
	buf := make([]byte, 8*len(hs))
	keys := make([][]byte, len(hs))
	for i, h := range hs {
		keys[i] = buf[8*i : 8*i+8 : 8*i+8]
		binary.BigEndian.PutUint64(keys[i], uint64(h))
	}
	return keys
}

// entryReads returns the edges that one entry of a record table lists. It
// decodes the entry's fields up to its read record, and a fault returns
// the edges decoded before it.
func entryReads(kind RecordKind, d *decoder) []EdgeHash {
	switch kind {
	case RecordValidation:
		return decodeValidation(d, d.identity()).Reads
	case RecordInvocation:
		d.text()
		d.match()
		return d.edges()
	case RecordCheck:
		d.text()
		return d.edges()
	default:
		return decodeGroup(d, d.text(), d.unitRef()).Reads
	}
}

// rowOf returns the row of a key among rows sorted by key, and nil for a
// key that they do not contain.
func rowOf(rows []entry, key []byte) []byte {
	at, found := slices.BinarySearchFunc(rows, key, func(e entry, k []byte) int { return bytes.Compare(e.key, k) })
	if !found {
		return nil
	}
	return rows[at].row
}

// claimedKeys returns the keys that the claims of claims rows name, sorted
// without repeats: the key of each claim, and none for a group's drop. It
// reads the rows' strings through strings. A row that does not decode
// names the keys decoded before the fault.
func claimedKeys(strings map[string]string, rows ...[]byte) []meta.KeyName {
	var out []meta.KeyName
	for _, row := range rows {
		d := newDecoder(row, nil)
		d.interned = strings
		for range d.Count() {
			if c := d.storedClaim(symbol.Identity{}); d.Err() == nil && c.Key != "" {
				out = append(out, c.Key)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// presentOn reports whether a key reads present on a subject: the claim
// that ranks first among the subject's claims under the key and the drops
// of its group claims a value.
func presentOn(facts *meta.Facts, subject symbol.Identity, name meta.KeyName) bool {
	k, registered := facts.Registry().Resolve(name)
	if !registered {
		return false
	}
	for v := range facts.Claims(subject, k) {
		return v.Value != nil
	}
	return false
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

// recordRows fills the rows of the record tables, each sorted by key, and
// returns the edges that the records read, sorted by edge, then by record.
// Each validation, invocation, check and group is a row under its ID, and
// records that share an ID share a row. recordRows leaves out the lanes of
// the plans in uncommitted, and kept contains the prior records that the
// commit keeps. It makes each list of records and reads once, at the size
// that the lanes and kept need.
func recordRows(rows *[tableCount][]entry, lanes []*Lane, kept *keptRecords, uncommitted []string) []edgeRead {
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
	return sortByHash(reads, func(r edgeRead) uint64 { return uint64(r.edge) }, compareReads)
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

// mergedReaderRows returns the readers rows of a warm run's commit for the
// edges whose readers the touched records change, sorted by key. prior are
// the prior readers rows of those edges, and sorted the reads of the
// touched records that the commit keeps or the lanes record, sorted by
// edge, then by record. Each edge's row lists the records of its prior row
// that are not touched, and each touched record that reads the edge now,
// each record once. A prior row that does not decode keeps none of its
// records. An edge without a reader has no row, and the rows of every
// other edge are as they were.
func mergedReaderRows(prior []entry, sorted []edgeRead, touched map[RecordRef]struct{}, edges []EdgeHash) []entry {
	var (
		out        []entry
		refs       []edgeRead
		keys, rows []byte
	)
	i, j := 0, 0
	for _, edge := range edges {
		refs = refs[:0]
		if i < len(prior) && EdgeHash(binary.BigEndian.Uint64(prior[i].key)) == edge {
			was, err := decodeReaders(prior[i].row)
			for _, ref := range was {
				if _, again := touched[ref]; err == nil && !again {
					refs = append(refs, edgeRead{edge: edge, ref: ref})
				}
			}
			i++
		}
		for j < len(sorted) && sorted[j].edge < edge {
			j++
		}
		for ; j < len(sorted) && sorted[j].edge == edge; j++ {
			refs = append(refs, sorted[j])
		}
		if len(refs) == 0 {
			continue
		}
		slices.SortFunc(refs, compareReads)
		refs = slices.Compact(refs)
		from, start := len(keys), len(rows)
		keys = binary.BigEndian.AppendUint64(keys, uint64(edge))
		rows = appendReaders(rows, refs)
		out = append(out, entry{key: keys[from:len(keys):len(keys)], row: rows[start:len(rows):len(rows)]})
	}
	return out
}

// changedEdges returns the edges of the reads that one list has and the
// other lacks, sorted without repeats. It sorts both lists in place by
// edge, then by record.
func changedEdges(was, is []edgeRead) []EdgeHash {
	slices.SortFunc(was, compareReads)
	slices.SortFunc(is, compareReads)
	was, is = slices.Compact(was), slices.Compact(is)
	var out []EdgeHash
	for i, j := 0, 0; i < len(was) || j < len(is); {
		var order int
		switch {
		case j == len(is):
			order = -1
		case i == len(was):
			order = 1
		default:
			order = compareReads(was[i], is[j])
		}
		switch {
		case order < 0:
			out = append(out, was[i].edge)
			i++
		case order > 0:
			out = append(out, is[j].edge)
			j++
		default:
			i++
			j++
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
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
