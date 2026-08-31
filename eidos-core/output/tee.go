// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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

// NewTee fans out to first and every sink after it. The first is
// the one of record: its Commit returns the records the caller
// reads, because the actions taken are one destination's answer
// and not a merged one.
func NewTee(first Sink, rest ...Sink) *Tee {
	return &Tee{sinks: append([]Sink{first}, rest...)}
}

// Write stages into every sink, joining what any of them refused.
func (t *Tee) Write(path string, body []byte) error {
	var faults []error
	for _, s := range t.sinks {
		if err := s.Write(path, body); err != nil {
			faults = append(faults, err)
		}
	}
	return errors.Join(faults...)
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
	var faults []error
	for _, s := range t.sinks {
		if err := s.Discard(); err != nil {
			faults = append(faults, err)
		}
	}
	return errors.Join(faults...)
}
