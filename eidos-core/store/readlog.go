// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/symbol"
)

// ReadLog keeps the edges of many read sets in flat storage, one entry
// for each set it records: what a dispatcher keeps of each invocation's
// reads while it reuses one set across a rule's invocations.
//
// [ReadLog.Append] copies a set's edges into the log, and
// [ReadLog.Load] records an entry's edges into a set again, so a
// consumer reads every entry through one set the caller resets between
// entries. An entry keeps the set's edges and not the order they
// arrived in, which no enumeration of a set returns either.
//
// The zero ReadLog is empty and ready to record.
//
// # Concurrency
//
// A ReadLog is not safe for concurrent use. A dispatcher keeps one for
// each worker.
//
// # Allocation contract
//
// Append allocates only to grow the log's slices, and Load only to grow
// the set's maps past the largest entry the set held since its
// creation.
type ReadLog struct {
	identities []symbol.Identity
	packages   []symbol.Identity
	kinds      []symbol.Kind
	facts      []factRead
	directives []directive.Name
	// ends contains, for each entry, the length of every slice once the
	// entry was appended. The entry before an entry gives its start.
	ends []logEnd
}

// logEnd is the length of each of a log's edge slices after one entry.
type logEnd struct {
	identities, packages, kinds, facts, directives int
}

// Append records the edges of s as the log's next entry and returns the
// entry's index, counted from zero. It cannot fail, and it leaves s
// unchanged.
func (l *ReadLog) Append(s *ReadSet) int {
	for id := range s.identities {
		l.identities = append(l.identities, id)
	}
	for id := range s.packages {
		l.packages = append(l.packages, id)
	}
	for k := range s.kinds {
		l.kinds = append(l.kinds, k)
	}
	for f := range s.facts {
		l.facts = append(l.facts, f)
	}
	for n := range s.directives {
		l.directives = append(l.directives, n)
	}
	l.ends = append(l.ends, logEnd{
		identities: len(l.identities),
		packages:   len(l.packages),
		kinds:      len(l.kinds),
		facts:      len(l.facts),
		directives: len(l.directives),
	})
	return len(l.ends) - 1
}

// Load resets s and records into it the edges of the entry at i, so s
// returns what the recorded set returned. An index outside the log
// panics, as an index outside a slice does.
func (l *ReadLog) Load(i int, s *ReadSet) {
	var start logEnd
	if i > 0 {
		start = l.ends[i-1]
	}
	end := l.ends[i]
	s.Reset()
	for _, id := range l.identities[start.identities:end.identities] {
		s.recordIdentity(id)
	}
	for _, id := range l.packages[start.packages:end.packages] {
		s.recordPackage(id)
	}
	for _, k := range l.kinds[start.kinds:end.kinds] {
		s.recordKind(k)
	}
	for _, f := range l.facts[start.facts:end.facts] {
		s.RecordFact(f.subject, f.key)
	}
	for _, n := range l.directives[start.directives:end.directives] {
		s.recordDirective(n)
	}
}
