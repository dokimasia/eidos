// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/binary"
	"sync"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/grow"
	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The least capacities a lane's buffers take on their first record: the
// bytes of its rows, its lists of records and of reads, and the room a
// record's entry takes in most runs, which the lane makes before it
// encodes an entry, so the encoding does not grow its buffer.
const (
	laneBytes   = 4096
	laneRecords = 256
	entryRoom   = 512
)

// The framing of a row of one entry: the count it opens with, one byte,
// and the most bytes the count and the entry's length take together.
const (
	singleEntry = 1
	frameRoom   = 1 + binary.MaxVarintLen64
)

// recordKinds is how many kinds of record a lane keeps: validations,
// invocations and checks, in [RecordKind] order.
const recordKinds = 3

// recordRow is one record that a commit writes into its table: its ID,
// and its row, a row of the record's entry alone, from start to end in
// one of the buffers the commit gathers.
type recordRow struct {
	id         uint64
	buf        int
	start, end int
}

// edgeRead is one edge a record read: the edge's hash and the record's
// reference.
type edgeRead struct {
	edge EdgeHash
	ref  RecordRef
}

// Lane is one goroutine's share of a [Recorder]. It encodes each record
// as the record arrives, and frames the entry into a buffer of its own as
// a row of that entry alone, which the commit writes as it is where no
// other record shares the ID. It keeps the record's ID and the edges the
// record read beside it. Its buffers contain no pointers, so the collector
// does not scan what a run records. A lane is the [plugin.Journal] a
// recording run hands each phase call of the lane's plan. A nil Lane
// records nothing, so a run that records no phases hands its phases one.
//
// # Concurrency
//
// A Lane is not safe for concurrent use: one goroutine records into it,
// and [Recorder.Lane] hands each goroutine a lane of its own.
//
// # Allocation contract
//
// A record allocates only to grow the lane's buffers, which double when
// they fill, and the sorted list of a grain that a read set keeps in
// maps. A record that spells an identity or a key longer than 255 bytes
// also allocates its spelling.
type Lane struct {
	plan string
	// at is the lane's place among its recorder's lanes, which each of its
	// records names its buffer by.
	at      int
	buf     []byte
	records [recordKinds][]recordRow
	reads   []edgeRead
	// edges is one record's read record, and entry its encoding before the
	// lane frames it, both reused from record to record.
	edges []EdgeHash
	entry []byte
}

var _ plugin.Journal = (*Lane)(nil)

// Validation records the validation of one subject's directives: the
// directives that passed, the edges the validation read, with the
// subject's own declaration edge added, and the findings it reported. A
// validation that reported a finding also lists [FindingsEdge].
func (l *Lane) Validation(
	subject symbol.Identity, ds []directive.Directive, reads *store.ReadSet, findings []diag.Diag,
) {
	if l == nil {
		return
	}
	l.edges = withFindings(append(appendEdges(l.edges[:0], reads), DeclarationEdge(subject)), findings)
	v := Validation{Subject: subject, Directives: ds, Reads: readRecord(l.edges), Findings: findings}
	l.entry = appendValidation(grow.Room(l.entry[:0], entryRoom, entryRoom), &v)
	l.add(ValidationRef(subject), v.Reads)
}

// Invoked records one invocation of a phase call of the lane's plan: its
// match, the edges it read, with its subject's declaration edge added
// where it has a subject, and what it touched and reported. An
// invocation that reported a finding also lists [FindingsEdge]. The run
// records a phase call that journaled no invocation of its own the same
// way, as one invocation under [plugin.WholeCall].
//
// It records nothing for a pure invocation: one that read nothing,
// touched nothing and reported nothing, of a match without a host and
// of a rule other than [plugin.WholeCall]. Such an invocation produced
// nothing. A change to its subject is the one change that can make it
// produce something.
func (l *Lane) Invoked(inv plugin.Invocation) {
	if l == nil {
		return
	}
	if inv.Match.Rule != plugin.WholeCall && inv.Match.Host == (plugin.EmitRef{}) &&
		(inv.Reads == nil || inv.Reads.Len() == 0) && len(inv.Exports) == 0 && len(inv.Units) == 0 &&
		len(inv.Hosts) == 0 && len(inv.Claimed) == 0 && len(inv.Findings) == 0 {
		return
	}
	l.edges = appendEdges(l.edges[:0], inv.Reads)
	if !inv.Match.Subject.IsZero() {
		l.edges = append(l.edges, DeclarationEdge(inv.Match.Subject))
	}
	l.edges = withFindings(l.edges, inv.Findings)
	rec := Invocation{
		Plan:     l.plan,
		Match:    inv.Match,
		Reads:    readRecord(l.edges),
		Exports:  inv.Exports,
		Units:    inv.Units,
		Hosts:    inv.Hosts,
		Claimed:  inv.Claimed,
		Findings: inv.Findings,
	}
	l.entry = appendInvocation(grow.Room(l.entry[:0], entryRoom, entryRoom), &rec)
	l.add(InvocationRef(l.plan, inv.Match), rec.Reads)
}

// Evaluated keeps nothing: a recording run hands no phase call a
// selection, so no call evaluates a candidate.
func (*Lane) Evaluated(symbol.Identity, []plugin.MatchKey) {}

// Check records one call of a workspace check: the edges its reader read
// and the findings it reported. A call that reported a finding also lists
// [FindingsEdge].
func (l *Lane) Check(name plugin.ID, reads *store.ReadSet, findings []diag.Diag) {
	if l == nil {
		return
	}
	l.edges = withFindings(appendEdges(l.edges[:0], reads), findings)
	c := Check{Name: name, Reads: readRecord(l.edges), Findings: findings}
	l.entry = appendCheck(grow.Room(l.entry[:0], entryRoom, entryRoom), &c)
	l.add(CheckRef(name), c.Reads)
}

// withFindings appends [FindingsEdge] to the edges of a record that
// reported a finding. It returns the edges of any other record unchanged.
func withFindings(edges []EdgeHash, findings []diag.Diag) []EdgeHash {
	if len(findings) == 0 {
		return edges
	}
	return append(edges, FindingsEdge)
}

// add frames the entry the lane encoded last into its buffer, as a row of
// that entry alone, and keeps the record under ref beside the edges it
// read.
func (l *Lane) add(ref RecordRef, reads []EdgeHash) {
	start := len(l.buf)
	l.buf = grow.Room(l.buf, len(l.entry)+frameRoom, laneBytes)
	l.buf = wire.AppendBytes(append(l.buf, singleEntry), l.entry)
	kind := &l.records[ref.Kind-1]
	*kind = append(grow.Room(*kind, 1, laneRecords), recordRow{id: ref.ID, buf: l.at, start: start, end: len(l.buf)})
	l.reads = grow.Room(l.reads, len(reads), laneRecords)
	for _, h := range reads {
		l.reads = append(l.reads, edgeRead{edge: h, ref: ref})
	}
}

// Recorder collects what one run's phases after the load executed, for
// [RecordPhases] to write into the run's commit: each validation, each
// invocation but a pure one, and each check with the edges it read, and
// the audit's findings. Each goroutine that records takes a [Lane] of its
// own.
//
// A warm run executes only part of the shared phases. It calls
// [Recorder.Keep], and it drops the record of each validation and
// annotator invocation that it executes again or removes. It also names
// each subject whose claims it withdrew. The commit then keeps the other
// shared records and bags of the generation, beside the records of the
// lanes.
//
// The zero Recorder is ready to record.
//
// # Concurrency
//
// A Recorder is safe for concurrent use: every method takes its lock.
//
// # Allocation contract
//
// Lane allocates the lane, and grows the list of lanes, which doubles
// when it fills. Audit allocates only to grow the list of findings. A
// drop or a withdrawal allocates only to grow its set.
type Recorder struct {
	mu     sync.Mutex
	lanes  []*Lane
	audits []Audit
	// keep reports whether the record is a warm run's. The commit of such
	// a record keeps the shared records and bags that the run did not
	// drop.
	keep bool
	// validations lists the dropped validations by subject, and
	// invocations lists the dropped invocations by match. withdrew lists
	// the subjects whose claims the run withdrew.
	validations map[symbol.Identity]struct{}
	invocations map[plugin.MatchKey]struct{}
	withdrew    map[symbol.Identity]struct{}
}

// Lane returns a new lane for one goroutine's records: a plan's generator
// calls under the plan's name, and every other record under the empty
// name. A plan's lane is the plan's alone, because a plan that does not
// commit keeps the invocations of its previous run.
func (r *Recorder) Lane(plan string) *Lane {
	r.mu.Lock()
	defer r.mu.Unlock()
	l := &Lane{plan: plan, at: len(r.lanes)}
	r.lanes = append(r.lanes, l)
	return l
}

// Audit records one unmet contract's finding: the contract's key, the
// declaration that lacks it, and the finding the audit reported.
func (r *Recorder) Audit(key meta.KeyName, subject symbol.Identity, finding diag.Diag) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audits = append(r.audits, Audit{Key: key, Subject: subject, Finding: finding})
}

// Keep marks the record as a warm run's record. The commit then keeps
// each validation and annotator invocation of the opened generation that
// the run did not drop, with the edges that each record read. It also
// keeps each recorded bag of claims that the run did not empty. Without
// Keep, the records of the lanes replace the shared records of the
// generation. Keep on a nil Recorder does nothing.
func (r *Recorder) Keep() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keep = true
}

// DropValidation drops the generation's record of a subject's
// validation. A run calls it for each subject that it validates again or
// removes. A lane's record of the subject replaces the dropped record.
// DropValidation on a nil Recorder does nothing.
func (r *Recorder) DropValidation(subject symbol.Identity) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.validations == nil {
		r.validations = map[symbol.Identity]struct{}{}
	}
	r.validations[subject] = struct{}{}
}

// DropInvocation drops the generation's record of one annotator
// invocation. A run calls it for each invocation that it runs again or
// removes. A lane's record of the match replaces the dropped record when
// the invocation ran and was not pure. DropInvocation on a nil Recorder
// does nothing.
func (r *Recorder) DropInvocation(m plugin.MatchKey) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.invocations == nil {
		r.invocations = map[plugin.MatchKey]struct{}{}
	}
	r.invocations[m] = struct{}{}
}

// Withdrew records that the run withdrew a claim on a subject. When the
// run leaves the subject without a claim, the commit drops the recorded
// bag of the subject. Withdrew on a nil Recorder does nothing.
func (r *Recorder) Withdrew(subject symbol.Identity) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.withdrew == nil {
		r.withdrew = map[symbol.Identity]struct{}{}
	}
	r.withdrew[subject] = struct{}{}
}
