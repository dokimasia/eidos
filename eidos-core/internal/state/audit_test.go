// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
)

// auditsAllocs is one read of the audit table of one finding: the run
// read from the ledger, its index, one list of its entries, the merged
// entries, the list of findings, the key's name and the strings of the
// subject the key spells, and the strings and the list of related
// positions of the finding the row decodes into.
const auditsAllocs = 16

// An unmet contract's finding records under its key and its subject, and
// reads back whole, sorted by key, then by subject.
func TestAudit(t *testing.T) {
	t.Parallel()

	t.Run("PhaseState", func(t *testing.T) {
		t.Parallel()

		t.Run("Audits", func(t *testing.T) {
			t.Parallel()

			t.Run("returns every finding sorted by key, then by subject", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Audit("shape.role", recordedSubject, recordedDiag)
					r.Audit("shape.role", siblingSubject, recordedDiag)
					r.Audit("shape.flag", recordedSubject, recordedDiag)
				})
				got, err := g.Phases(t.Context()).Audits()
				assert.NoError(t, err, "the audit table reads")
				assert.Equal(t, got, []state.Audit{
					{Key: "shape.flag", Subject: recordedSubject, Finding: recordedDiag},
					{Key: "shape.role", Subject: siblingSubject, Finding: recordedDiag},
					{Key: "shape.role", Subject: recordedSubject, Finding: recordedDiag},
				}, "shape.flag first, and Account before User")
			})

			t.Run("returns ErrDamaged for a key that does not spell a subject", func(t *testing.T) {
				t.Parallel()

				g := putRows(t, state.TableAudit, state.Row{Key: []byte("shape.role"), Value: []byte{0}})
				_, err := g.Phases(t.Context()).Audits()
				assert.ErrorIs(t, err, state.ErrDamaged, "the key has no subject")
			})

			t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
				t.Parallel()

				key := append(append([]byte("shape.role"), keySep), identityKeyOf(recordedSubject)...)
				g := putRows(t, state.TableAudit, state.Row{Key: key, Value: []byte{0x80}})
				_, err := g.Phases(t.Context()).Audits()
				assert.ErrorIs(t, err, state.ErrDamaged, "the finding is cut short")
			})

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Audit("shape.role", recordedSubject, recordedDiag)
				})
				damageRun(t, l, func(int) int { return 3 })
				_, err := g.Phases(t.Context()).Audits()
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged block is found")
			})
		})
	})
}

// A read of the audit's findings allocates within its ceiling in the
// ordinary run, which runs no benchmark. The count keeps the first error
// of its calls, which cmp.Or returns without allocating. The check runs
// alone, because the count includes every goroutine's allocations.
func TestAuditAllocs(t *testing.T) {
	s := audited(t).Phases(t.Context())
	var (
		n   int
		err error
	)
	assert.MaxAllocs(t, func() {
		got, aerr := s.Audits()
		n, err = len(got), cmp.Or(err, aerr)
	}, auditsAllocs, "Audits allocates the table it reads and the findings")
	assert.NoError(t, err, "the findings read")
	assert.Equal(t, n, 1, "the one unmet contract")
}

// BenchmarkAudit measures a read of the findings of a generation that
// records one unmet contract.
func BenchmarkAudit(b *testing.B) {
	s := audited(b).Phases(b.Context())

	b.Run("PhaseState", func(b *testing.B) {
		b.Run("Audits", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(auditsAllocs)
			defer c.End()
			var (
				got []state.Audit
				err error
			)
			for c.Loop() {
				got, err = s.Audits()
			}
			assert.NoError(b, err, "the audit table reads")
			assert.Length(b, got, 1, "Audits returns the one finding")
		})
	})
}

// audited returns a generation that records one unmet contract of the
// recorded subject.
func audited(tb testing.TB) *state.Generation {
	tb.Helper()

	return recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
		r.Audit("shape.role", recordedSubject, recordedDiag)
	})
}
