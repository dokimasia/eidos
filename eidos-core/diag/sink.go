// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/position"
)

// Sink collects the findings of one run.
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
// allocates its message. An enumeration allocates its snapshot.
type Sink struct {
	mu     sync.Mutex
	found  []Diag
	failed bool
}

// NewSink returns an empty sink.
func NewSink() *Sink { return &Sink{} }

// Report attaches one finding.
func (s *Sink) Report(d Diag) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.found = append(s.found, d)
	if d.Severity == SeverityError {
		s.failed = true
	}
}

// Errorf reports at [SeverityError], which fails the run.
func (s *Sink) Errorf(c Code, at position.Pos, by Origin, format string, args ...any) {
	s.reportf(c, SeverityError, at, by, format, args...)
}

// Warnf reports at [SeverityWarning], which never fails a run.
func (s *Sink) Warnf(c Code, at position.Pos, by Origin, format string, args ...any) {
	s.reportf(c, SeverityWarning, at, by, format, args...)
}

// Infof reports at [SeverityInfo], which carries provenance and
// progress.
func (s *Sink) Infof(c Code, at position.Pos, by Origin, format string, args ...any) {
	s.reportf(c, SeverityInfo, at, by, format, args...)
}

// Failed reports whether any [SeverityError] was attached, which is
// what decides the run's outcome.
func (s *Sink) Failed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.failed
}

// All returns every finding, in the order the type documents.
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
	s.mu.Unlock()

	slices.SortStableFunc(found, func(a, b Diag) int {
		return cmp.Compare(a.Origin, b.Origin)
	})
	for _, d := range found {
		if !yield(d) {
			return
		}
	}
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
