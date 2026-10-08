// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

// Pending reports whether the generation records a plan as pending: a
// plan that did not commit in the run that recorded the generation, whose
// records are those of an earlier run. A warm run runs such a plan whole.
// The lookup allocates nothing for a name that fits in 256 bytes.
//
// Error modes: an error wrapping [ErrDamaged] for a table that does not
// read whole.
func (s *PhaseState) Pending(plan string) (bool, error) {
	var key [spellingCap]byte
	_, held, err := s.g.readers[TablePlans].get(s.ctx, append(key[:0], plan...))
	return held, err
}

// planRows returns the plans table's rows of a run, sorted by key: an
// empty row under the name of each plan that does not commit.
func planRows(uncommitted []string) []entry {
	rows := make([]entry, 0, len(uncommitted))
	for _, plan := range uncommitted {
		rows = append(rows, entry{key: []byte(plan), row: []byte{}})
	}
	return sortedRows(rows)
}
