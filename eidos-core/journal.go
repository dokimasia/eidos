// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"cmp"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// span is a range of one of the buffers a record indexes.
type span struct{ from, to int32 }

// record is one journaled invocation: the match it ran, the entry of
// its reads in the lane's read log, the ranges of the lane's buffers its
// handler filled, and the ranges of the call's buffers its effects
// filled as they applied. A record keeps the match's parts and not its
// key, whose host the delivery resolves, so a call of many invocations
// keeps a small record for each.
type record struct {
	seq     int
	fr      *flatRule
	subject symbol.Identity
	// value is the matched declaration or emit value, nil for a
	// graph-wide rule, and gate the gating instance, nil without one.
	value symbol.Symbol
	gate  *directive.Directive
	// reads is the record's entry in the lane's read log, and -1 for an
	// invocation that read nothing. host is the index of the key's host
	// in the lane's matched values, and -1 for a rule that is not
	// emit-phase.
	reads, host int32
	// exports and claimed index the lane's readExports and stamped.
	exports, claimed span
	// units, hosts and findings index the call's buffers of the same
	// names.
	units, hosts, findings span
}

// entry is one record the delivery orders, with the lane whose buffers
// it indexes.
type entry struct {
	ln *runState
	r  *record
}

// compareEntries orders the records of one rule in canonical match
// order: by subject, then gating instance, then host, then sequence.
func compareEntries(a, b entry) int {
	if by := a.r.subject.Compare(b.r.subject); by != 0 {
		return by
	}
	if by := cmp.Compare(instanceOf(a.r), instanceOf(b.r)); by != 0 {
		return by
	}
	if by := a.ln.hostOf(a.r).Compare(b.ln.hostOf(b.r)); by != 0 {
		return by
	}
	return cmp.Compare(a.r.seq, b.r.seq)
}

// instanceOf returns a record's gating instance, and zero without one.
func instanceOf(r *record) int {
	if r.gate == nil {
		return 0
	}
	return r.gate.Instance
}

// spanOf returns the part of buf a span indexes, capped at its end, so
// an append to it copies.
func spanOf[T any](buf []T, s span) []T { return buf[s.from:s.to:s.to] }

// sortedSet sorts s in place by compare and drops its repeats.
func sortedSet[T any](s []T, compare func(a, b T) int) []T {
	slices.SortFunc(s, compare)
	return slices.CompactFunc(s, func(a, b T) bool { return compare(a, b) == 0 })
}

// open appends the record of the invocation inv binds, before the
// handler runs.
func (rs *runState) open(fr *flatRule, inv *invocation) {
	rs.records = append(rs.records, record{
		seq:     inv.seq,
		fr:      fr,
		subject: inv.subject,
		value:   inv.value,
		gate:    inv.gate,
		reads:   -1,
		host:    -1,
		exports: span{int32(len(rs.readExports)), int32(len(rs.readExports))},
		claimed: span{int32(len(rs.stamped)), int32(len(rs.stamped))},
	})
	rs.reading = nil
}

// close keeps the reads of the invocation the lane ran last in the
// lane's read log: the set the match recorded into, where the handler
// read anything.
func (rs *runState) close() {
	if rs.reading == nil || rs.reading.Len() == 0 {
		return
	}
	rs.running().reads = int32(rs.log.Append(rs.reading))
}

// running returns the record of the invocation the lane runs: the last
// one it opened.
func (rs *runState) running() *record { return &rs.records[len(rs.records)-1] }

// noteExport records that the running invocation read a plan's export.
func (rs *runState) noteExport(plan string) {
	rs.readExports = append(rs.readExports, plan)
	rs.running().exports.to = int32(len(rs.readExports))
}

// noteClaim records that the running invocation stamped a fact.
func (rs *runState) noteClaim(f meta.FactRef) {
	rs.stamped = append(rs.stamped, f)
	rs.running().claimed.to = int32(len(rs.stamped))
}

// keyOf returns a record's match key: the lane's plugin, the rule's
// ordinal, the subject, the gating instance and the resolved host.
func (rs *runState) keyOf(r *record) plugin.MatchKey {
	return plugin.MatchKey{
		Plugin: rs.plugin, Rule: r.fr.ordinal, Subject: r.subject, Instance: instanceOf(r), Host: rs.hostOf(r),
	}
}

// hostOf returns a record's resolved host, and the zero reference for a
// rule that is not emit-phase.
func (rs *runState) hostOf(r *record) plugin.EmitRef {
	if r.host < 0 {
		return plugin.EmitRef{}
	}
	return rs.matched[r.host]
}

// fill makes the record of the lane's invocation at seq the one the
// applying effects fill. The merge applies one invocation's effects
// together and a lane's effects in increasing sequence, so the lane's
// cursor only moves forward, and a record's ranges open at the buffers'
// lengths the first time one of its effects applies.
func (c *phaseCall) fill(ln *runState, seq int) {
	for ln.records[ln.cursor].seq != seq {
		ln.cursor++
	}
	r := &ln.records[ln.cursor]
	if r == c.filling {
		return
	}
	r.units = span{int32(len(c.units)), int32(len(c.units))}
	r.hosts = span{int32(len(c.hosts)), int32(len(c.hosts))}
	r.findings = span{int32(len(c.findings)), int32(len(c.findings))}
	c.filling = r
}

// keyHosts resolves the host of every emit-phase record through the
// plan's store into the lane's matched values, before the flush adds
// the call's own units, so the store walks none of them. A call that
// journals nothing, and an annotator's call, has nothing to resolve.
func (c *phaseCall) keyHosts() {
	if c.journal == nil || c.emit == nil {
		return
	}
	for _, ln := range c.lanes {
		for i := range ln.records {
			r := &ln.records[i]
			if r.fr.phase != plugin.PhaseEmit {
				continue
			}
			ref, _ := c.emit.Ref(r.value)
			r.host = int32(len(ln.matched))
			ln.matched = append(ln.matched, ref)
		}
	}
}

// deliver hands the call's journal every record in canonical match
// order, then each candidate's matches in identity order, and does
// nothing for a call that journals nothing. Every record's reads load
// into one set the delivery resets between records. Each record's
// exports, units, hosts and claimed facts are sorted without repeats in
// place, and its findings remain in the order they applied.
//
// The call numbers its matches from zero without a gap, rule after rule,
// so the records fall into sequence order by their numbers, and each
// rule's records are one run of it. A run is in canonical match order
// already where the rule's index enumerates in identity order, and the
// delivery sorts the other runs alone.
//
// # Allocation contract
//
// The order of the records, the read set each record's reads load into
// and the buffer a candidate's matches pass through are the call's own
// buffers. Each allocates only to grow past the largest delivery of an
// earlier call that used the same state.
func (c *phaseCall) deliver() {
	if c.journal == nil {
		return
	}
	c.order = slices.Grow(c.order[:0], c.seq)[:c.seq]
	all := c.order
	for _, ln := range c.lanes {
		for i := range ln.records {
			r := &ln.records[i]
			all[r.seq] = entry{ln: ln, r: r}
		}
	}
	for start := 0; start < len(all); {
		end := start + 1
		for end < len(all) && all[end].r.fr == all[start].r.fr {
			end++
		}
		if run := all[start:end]; !slices.IsSortedFunc(run, compareEntries) {
			slices.SortFunc(run, compareEntries)
		}
		start = end
	}
	if c.delivered == nil {
		c.delivered = store.NewReadSet()
	}
	reads := c.delivered
	for _, e := range all {
		inv := plugin.Invocation{
			Match:    e.ln.keyOf(e.r),
			Exports:  sortedSet(spanOf(e.ln.readExports, e.r.exports), strings.Compare),
			Units:    sortedSet(spanOf(c.units, e.r.units), plugin.UnitRef.Compare),
			Hosts:    sortedSet(spanOf(c.hosts, e.r.hosts), plugin.EmitRef.Compare),
			Claimed:  sortedSet(spanOf(e.ln.stamped, e.r.claimed), meta.FactRef.Compare),
			Findings: spanOf(c.findings, e.r.findings),
		}
		if e.r.reads >= 0 {
			e.ln.log.Load(int(e.r.reads), reads)
			inv.Reads = reads
		}
		c.journal.Invoked(inv)
	}
	if !c.selects {
		return
	}
	s := &c.selection
	slices.SortStableFunc(s.evaluated, func(a, b evaluatedMatch) int { return cmp.Compare(a.candidate, b.candidate) })
	keys := c.keys
	from := 0
	for j, id := range s.candidates {
		if j > 0 && id == s.candidates[j-1] {
			continue
		}
		keys, from = s.evaluatedOf(j, from, keys)
		if len(keys) == 0 {
			c.journal.Evaluated(id, nil)
			continue
		}
		c.journal.Evaluated(id, keys)
	}
	c.keys = keys
}
