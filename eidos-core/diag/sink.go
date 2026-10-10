// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/position"
)

// Sink collects the findings of one run.
//
// A sink applies two policies, which a run installs in the sinks that
// decide an outcome. [Sink.Suppress] removes the findings that a table of
// suppressions lists, and [Sink.Promote] reports every Warning as an
// Error. The sink keeps each finding at the severity it was reported at,
// so the table decides on that severity whatever order the two policies
// arrive in.
//
// # Concurrency
//
// A Sink is safe for concurrent use: frontends, and annotators under
// the parallelism opt-in, report while running alongside each other.
//
// # Order
//
// [Sink.All] groups the findings by origin and keeps report order
// within one origin, so two schedulings of one parallel run agree
// on it however the origins interleaved. Ordering for output belongs
// to the run, which sorts by position.
//
// # Allocation contract
//
// The sink keeps every finding in one slice, which grows by doubling.
// A report allocates only to grow it, and a formatted report also
// allocates its message. A report that the table removes allocates the
// counts of its position the first time the table removes a finding
// there. An enumeration allocates its snapshot.
type Sink struct {
	mu     sync.Mutex
	found  []Diag
	failed bool
	// promoted reports every Warning as an Error once Promote ran.
	promoted bool
	// table lists, by the position of a declaration, the codes whose
	// findings the sink removes there. It is nil until Suppress.
	table map[position.Pos][]Code
	// removed counts the findings that the table removed, by position
	// and code.
	removed map[position.Pos]map[Code]int
}

// NewSink returns an empty sink.
func NewSink() *Sink { return &Sink{} }

// Report attaches one finding, unless the table that [Sink.Suppress]
// installed removes it.
func (s *Sink) Report(d Diag) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.removes(d) {
		return
	}
	s.found = append(s.found, d)
	s.failed = s.failed || s.fails(d)
}

// Errorf reports at [SeverityError], which fails the run.
func (s *Sink) Errorf(c Code, at position.Pos, by Origin, format string, args ...any) {
	s.reportf(c, SeverityError, at, by, format, args...)
}

// Warnf reports at [SeverityWarning], which fails a run only after
// [Sink.Promote].
func (s *Sink) Warnf(c Code, at position.Pos, by Origin, format string, args ...any) {
	s.reportf(c, SeverityWarning, at, by, format, args...)
}

// Infof reports at [SeverityInfo], which reports provenance and
// progress.
func (s *Sink) Infof(c Code, at position.Pos, by Origin, format string, args ...any) {
	s.reportf(c, SeverityInfo, at, by, format, args...)
}

// Suppress installs a table of suppressions: for each declaration
// position, the codes that its diag directives name. The sink removes
// every finding that it contains, and every later finding, whose position
// and code the table lists, and counts what it removed. It never removes a
// kernel Error, a finding under [KernelPrefix] reported at
// [SeverityError]. [Sink.Failed] then reports on the findings that remain.
//
// A second call replaces the table, and a nil table removes nothing more.
// The caller does not modify the table after the call.
func (s *Sink) Suppress(table map[position.Pos][]Code) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.table = table
	kept := s.found[:0]
	s.failed = false
	for _, d := range s.found {
		if s.removes(d) {
			continue
		}
		kept = append(kept, d)
		s.failed = s.failed || s.fails(d)
	}
	clear(s.found[len(kept):])
	s.found = kept
}

// Removed returns the number of findings that the table of
// [Sink.Suppress] removed, for each position and code, and nil where it
// removed none. The returned map is the caller's own.
func (s *Sink) Removed() map[position.Pos]map[Code]int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.removed == nil {
		return nil
	}
	out := make(map[position.Pos]map[Code]int, len(s.removed))
	for at, counts := range s.removed {
		out[at] = maps.Clone(counts)
	}
	return out
}

// Promote reports every Warning that the sink contains, and every later
// one, as an Error: [Sink.Failed] counts it, and [Sink.All] returns it at
// [SeverityError]. The table of [Sink.Suppress] decides on the severity
// that a finding was reported at, so it removes a promoted Warning as it
// removes any other.
func (s *Sink) Promote() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.promoted = true
	s.failed = s.failed || slices.ContainsFunc(s.found, s.fails)
}

// Failed reports whether the sink contains an Error, or a Warning after
// [Sink.Promote], which is what decides the run's outcome. A finding that
// the table of [Sink.Suppress] removed does not count.
func (s *Sink) Failed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.failed
}

// All returns every finding, in the order the type documents, and each
// Warning at [SeverityError] after [Sink.Promote].
//
// The range takes a snapshot of the findings when it starts, so an
// iteration runs alongside further reports and returns what the sink
// contained when the range started. The snapshot is one allocation, and
// the returned function inlines into the range, which allocates nothing
// else.
func (s *Sink) All() iter.Seq[Diag] {
	return func(yield func(Diag) bool) { s.each(yield) }
}

// each yields a snapshot of the findings in the order [Sink.All]
// documents, and stops when yield returns false.
func (s *Sink) each(yield func(Diag) bool) {
	s.mu.Lock()
	found := slices.Clone(s.found)
	promoted := s.promoted
	s.mu.Unlock()

	slices.SortStableFunc(found, func(a, b Diag) int {
		return cmp.Compare(a.Origin, b.Origin)
	})
	for _, d := range found {
		if promoted && d.Severity == SeverityWarning {
			d.Severity = SeverityError
		}
		if !yield(d) {
			return
		}
	}
}

// removes reports whether the table lists a finding's position and code,
// and counts the finding where it does. It never removes a kernel Error.
// The caller has the lock.
func (s *Sink) removes(d Diag) bool {
	if s.table == nil || d.Code.Prefix == KernelPrefix && d.Severity == SeverityError {
		return false
	}
	if !slices.Contains(s.table[d.Pos], d.Code) {
		return false
	}
	if s.removed == nil {
		s.removed = map[position.Pos]map[Code]int{}
	}
	counts := s.removed[d.Pos]
	if counts == nil {
		counts = map[Code]int{}
		s.removed[d.Pos] = counts
	}
	counts[d.Code]++
	return true
}

// fails reports whether a finding fails the run: an Error, and a Warning
// once Promote ran. The caller has the lock.
func (s *Sink) fails(d Diag) bool {
	return d.Severity == SeverityError || s.promoted && d.Severity == SeverityWarning
}

// reportf assembles one finding at the given severity, so that a
// caller reporting at one severity does not spell a whole [Diag].
func (s *Sink) reportf(
	c Code, sev Severity, at position.Pos, by Origin, format string, args ...any,
) {
	s.Report(Diag{
		Code:     c,
		Severity: sev,
		Pos:      at,
		Msg:      fmt.Sprintf(format, args...),
		Origin:   by,
	})
}
