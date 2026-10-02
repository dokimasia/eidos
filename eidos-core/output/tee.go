// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"errors"
	"slices"
)

// Tee stages one set of files into several sinks at once: a disk
// sink beside a memory sink is how a run writes and reports the
// same bytes.
type Tee struct {
	sinks []Sink
}

var _ Sink = (*Tee)(nil)

// NewTee fans out to first and every sink after it. The first is
// the one of record: its Prepare and its Commit return what the
// caller reads, because the verdicts and the actions are one
// destination's answer and not a merged one.
func NewTee(first Sink, rest ...Sink) *Tee {
	return &Tee{sinks: append([]Sink{first}, rest...)}
}

// Write stages into every sink, joining what any of them refused.
func (t *Tee) Write(path string, body []byte) error {
	return t.each(func(s Sink) error { return s.Write(path, body) })
}

// Delete stages the removal into every sink, joining what any of them
// refused.
func (t *Tee) Delete(path string) error {
	return t.each(func(s Sink) error { return s.Delete(path) })
}

// Prepare prepares every sink and returns the first one's changes,
// joining what any of them refused.
func (t *Tee) Prepare() ([]Change, error) {
	var changes []Change
	var faults []error
	for i, s := range t.sinks {
		got, err := s.Prepare()
		if err != nil {
			faults = append(faults, err)
		}
		if i == 0 {
			changes = slices.Clone(got)
		}
	}
	return changes, errors.Join(faults...)
}

// Commit commits every sink and returns the first one's records,
// joining what any of them refused.
func (t *Tee) Commit() ([]Written, error) {
	var records []Written
	var faults []error
	for i, s := range t.sinks {
		got, err := s.Commit()
		if err != nil {
			faults = append(faults, err)
		}
		if i == 0 {
			records = slices.Clone(got)
		}
	}
	return records, errors.Join(faults...)
}

// Discard discards every sink, joining what any of them refused.
func (t *Tee) Discard() error {
	return t.each(Sink.Discard)
}

// each runs one call on every sink, joining what any of them refused.
func (t *Tee) each(call func(Sink) error) error {
	var faults []error
	for _, s := range t.sinks {
		if err := call(s); err != nil {
			faults = append(faults, err)
		}
	}
	return errors.Join(faults...)
}
