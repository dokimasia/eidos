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
// invocations, checks and groups, in [RecordKind] order.
const recordKinds = 4

// The lists of rows a lane keeps for the tables without readers, in the
// order of [plainTables].
const (
	plainArtifacts = 0
	plainNames     = 1
	plainKinds     = 2
)

// plainTables are the tables without readers that a lane writes rows for,
// in the order of the lane's lists.
var plainTables = [plainKinds]Table{TableArtifacts, TableNames}

// recordRow is one record that a commit writes into its table: its ID,
// and its row, a row of the record's entry alone, from start to end in
// one of the buffers the commit gathers.
type recordRow struct {
	id         uint64
	buf        int
	start, end int
}

// plainRow is one row that a lane wrote for a table without readers: its
// key from start to mid in the lane's buffer, and its row from mid to end.
type plainRow struct {
	start, mid, end int
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
// also allocates its spelling, and an artifact whose path has an
// upper-case letter allocates the path in lower case.
type Lane struct {
	plan string
	// at is the lane's place among its recorder's lanes, which each of its
	// records names its buffer by.
	at      int
	buf     []byte
	records [recordKinds][]recordRow
	plain   [plainKinds][]plainRow
	reads   []edgeRead
	// edges is one record's read record, entry its encoding before the
	// lane frames it, and key the key of a row of a table without readers,
	// each reused from record to record.
	edges []EdgeHash
	entry []byte
	key   []byte
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
// where it has a subject and the export edge of each plan whose export
// it read, and what it touched and reported. An invocation that reported
// a finding also lists [FindingsEdge]. The run records a phase call that
// journaled no invocation of its own the same way, as one invocation
// under [plugin.WholeCall].
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
	for _, plan := range inv.Exports {
		l.edges = append(l.edges, ExportEdge(plan))
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

// Group records one group of the lane's plan: its key unit, its units,
// files and contributors, the edges it read beyond the reads of its
// invocations, and the findings that the render reported for its files.
// A group that reported a finding also lists [FindingsEdge]. The record
// names the lane's plan, whatever plan g names.
func (l *Lane) Group(g Group) {
	if l == nil {
		return
	}
	l.edges = withFindings(append(l.edges[:0], g.Reads...), g.Findings)
	g.Plan, g.Reads = l.plan, readRecord(l.edges)
	l.entry = appendGroup(grow.Room(l.entry[:0], entryRoom, entryRoom), &g)
	l.add(GroupRef(l.plan, g.Key), g.Reads)
}

// Artifact records one file that the lane's plan generated: its row under
// its path, its row under its path in lower case, and the row of each
// name the file declares under the name's collision scope and, for a name
// with an origin, under its origin. The rows name the lane's plan,
// whatever plan a names. A name's row states the file's path and package.
func (l *Lane) Artifact(a Artifact) {
	if l == nil {
		return
	}
	a.Entry.Plan = l.plan
	l.entry = appendArtifact(grow.Room(l.entry[:0], entryRoom, entryRoom), &a)
	l.put(plainArtifacts, append(l.key[:0], a.Entry.Path...))
	l.entry = wire.AppendText(l.entry[:0], l.plan)
	l.put(plainArtifacts, foldedKey(l.key[:0], a.Entry.Path))
	for i := range a.Names {
		n := a.Names[i]
		n.File, n.FilePkg, n.Ambiguous = a.Entry.Path, a.Pkg, false
		l.entry = appendNameRow(l.entry[:0], &n)
		l.put(plainNames, scopeRowKey(l.key[:0], l.plan, &n))
		if !n.Origin.IsZero() {
			l.put(plainNames, originRowKey(l.key[:0], l.plan, &n))
		}
	}
}

// Reset drops every record and row of the lane and keeps its buffers,
// for a plan that starts its generation again from a fresh store. Reset
// on a nil Lane does nothing.
func (l *Lane) Reset() {
	if l == nil {
		return
	}
	l.buf = l.buf[:0]
	for k := range l.records {
		l.records[k] = l.records[k][:0]
	}
	for k := range l.plain {
		l.plain[k] = l.plain[k][:0]
	}
	l.reads = l.reads[:0]
}

// put copies a key and the entry the lane encoded last into the lane's
// buffer, as one row of the table at a place in [plainTables].
func (l *Lane) put(table int, key []byte) {
	l.key = key
	l.buf = grow.Room(l.buf, len(key)+len(l.entry), laneBytes)
	start := len(l.buf)
	l.buf = append(l.buf, key...)
	mid := len(l.buf)
	l.buf = append(l.buf, l.entry...)
	rows := &l.plain[table]
	*rows = append(grow.Room(*rows, 1, laneRecords), plainRow{start: start, mid: mid, end: len(l.buf)})
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
// invocation but a pure one, each check and each group with the edges it
// read, each file that a plan generated, and the audit's findings. Each
// goroutine that records takes a [Lane] of its own.
//
// A warm run executes only part of the shared phases. It calls
// [Recorder.Keep], and it drops the record of each validation and
// annotator invocation that it executes again or removes, and of each
// check that it calls again or does not call because a plan the check
// reads failed. It also names each subject whose claims it withdrew. The
// commit then keeps the other shared records and bags of the generation,
// beside the records of the lanes. A warm run that executes part of a
// plan also calls [Recorder.KeepPlan] for the plan, and drops each of the
// plan's records that it executes again or removes: an invocation, a
// group, or a file with its names.
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
// drop, a withdrawal or a kept plan allocates only to grow its set, and
// the first drop of a plan's record allocates the plan's set.
type Recorder struct {
	mu     sync.Mutex
	lanes  []*Lane
	audits []Audit
	// keep reports whether the record is a warm run's. The commit of such
	// a record keeps the shared records and bags that the run did not
	// drop.
	keep bool
	// validations lists the dropped validations by subject, invocations
	// lists the dropped annotator invocations by match, and checks the
	// dropped checks by name. withdrew lists the subjects whose claims the
	// run withdrew.
	validations map[symbol.Identity]struct{}
	invocations map[plugin.MatchKey]struct{}
	checks      map[plugin.ID]struct{}
	withdrew    map[symbol.Identity]struct{}
	// plans lists the plans whose records the commit keeps where the run
	// did not drop them. planInvocations and groups list the dropped
	// invocations and groups of each plan, and files the dropped files.
	plans           map[string]struct{}
	planInvocations map[string]map[plugin.MatchKey]struct{}
	groups          map[string]map[plugin.UnitRef]struct{}
	files           map[string]struct{}
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

// DropInvocation drops the generation's record of one invocation of a
// plan, the empty plan for an annotator's. A run calls it for each
// invocation that it runs again or removes. A lane's record of the match
// replaces the dropped record when the invocation ran and was not pure.
// The commit keeps the records of a plan that does not commit, whatever
// the run dropped. DropInvocation on a nil Recorder does nothing.
func (r *Recorder) DropInvocation(plan string, m plugin.MatchKey) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if plan == "" {
		if r.invocations == nil {
			r.invocations = map[plugin.MatchKey]struct{}{}
		}
		r.invocations[m] = struct{}{}
		return
	}
	if r.planInvocations == nil {
		r.planInvocations = map[string]map[plugin.MatchKey]struct{}{}
	}
	dropped := r.planInvocations[plan]
	if dropped == nil {
		dropped = map[plugin.MatchKey]struct{}{}
		r.planInvocations[plan] = dropped
	}
	dropped[m] = struct{}{}
}

// DropCheck drops the generation's record of a workspace check. A run
// calls it for each check that it calls again, and for each check that it
// does not call because a plan the check reads failed. A lane's record of
// the check replaces the dropped record when the run called the check.
// DropCheck on a nil Recorder does nothing.
func (r *Recorder) DropCheck(name plugin.ID) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checks == nil {
		r.checks = map[plugin.ID]struct{}{}
	}
	r.checks[name] = struct{}{}
}

// KeepPlan makes the commit of a warm run keep a plan's records of the
// generation that the run did not drop: its invocations, its groups, and
// its files with their names. A run calls it for each plan that it
// executes in part. Without KeepPlan, the lane of a plan that commits
// replaces every record of the plan. KeepPlan on a nil Recorder does
// nothing.
func (r *Recorder) KeepPlan(plan string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plans == nil {
		r.plans = map[string]struct{}{}
	}
	r.plans[plan] = struct{}{}
}

// DropGroup drops the generation's record of a plan's group under its key
// unit. A run calls it for each group that it executes again. A lane's
// record of the group replaces the dropped record. DropGroup on a nil
// Recorder does nothing.
func (r *Recorder) DropGroup(plan string, key plugin.UnitRef) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.groups == nil {
		r.groups = map[string]map[plugin.UnitRef]struct{}{}
	}
	dropped := r.groups[plan]
	if dropped == nil {
		dropped = map[plugin.UnitRef]struct{}{}
		r.groups[plan] = dropped
	}
	dropped[key] = struct{}{}
}

// DropFile drops the generation's record of the file at a path: its
// artifact, and the names it declares. A run calls it for each file of a
// group that it executes again. A lane's record of the file replaces the
// dropped record where the group still generates the file. DropFile on a
// nil Recorder does nothing.
func (r *Recorder) DropFile(path string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.files == nil {
		r.files = map[string]struct{}{}
	}
	r.files[path] = struct{}{}
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
