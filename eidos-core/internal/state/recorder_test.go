// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// laneRecords is how many invocations the lane's ceilings are measured
// over.
const laneRecords = 10_000

// The ceilings of a recorder and its lanes.
const (
	// laneAllocs is a new recorder's lane that records laneRecords
	// invocations of one fact read each: the recorder, the lane, the
	// recorder's list of lanes and the lane's entry buffer, and the
	// doublings of the lane's other buffers from their least capacities,
	// 26 in all.
	laneAllocs = 26
	// recordAllocs is one record of a validation, an invocation or a
	// check of one fact read on such a lane: the doublings of the lane's
	// buffers alone, fewer than one for each record.
	recordAllocs = 1
	// newLaneAllocs is a new recorder's first lane: the recorder, which
	// Lane leaks to the heap as it locks it, the lane and the recorder's
	// list of lanes.
	newLaneAllocs = 3
	// auditAllocs is a new recorder's first finding: the recorder, which
	// Audit leaks to the heap the same way, and its list of findings.
	auditAllocs = 2
)

// A lane records each validation, invocation and check with the edges it
// read, a subject's own declaration edge added, and the recorder hands
// every lane's records to the commit.
func TestRecorder(t *testing.T) {
	t.Parallel()

	t.Run("Lane", func(t *testing.T) {
		t.Parallel()

		t.Run("Validation", func(t *testing.T) {
			t.Parallel()

			t.Run("adds the subject's declaration edge to what the validation read", func(t *testing.T) {
				t.Parallel()

				got := validationOf(t, func(l *state.Lane) {
					l.Validation(recordedSubject, nil, readsOf(t, []symbol.Identity{siblingSubject}), nil)
				})
				assert.Equal(t, got.Reads, sortedEdges(
					state.DeclarationEdge(recordedSubject), state.DeclarationEdge(siblingSubject),
				), "the sibling it resolved through, and the subject itself")
			})

			t.Run("records nothing on a nil lane", func(t *testing.T) {
				t.Parallel()

				var l *state.Lane
				assert.NotPanics(t, func() { l.Validation(recordedSubject, nil, nil, nil) },
					"a run that records no phases validates through a nil lane")
			})

			t.Run("records a subject that validated nothing with its finding", func(t *testing.T) {
				t.Parallel()

				got := validationOf(t, func(l *state.Lane) {
					l.Validation(recordedSubject, nil, nil, []diag.Diag{recordedDiag})
				})
				assert.Equal(t, got, state.Validation{
					Subject:  recordedSubject,
					Reads:    []state.EdgeHash{state.DeclarationEdge(recordedSubject)},
					Findings: []diag.Diag{recordedDiag},
				}, "no directive, the subject's own edge, and the finding")
			})
		})

		t.Run("Invoked", func(t *testing.T) {
			t.Parallel()

			t.Run("adds the subject's declaration edge to what the invocation read", func(t *testing.T) {
				t.Parallel()

				fact := meta.FactRef{Subject: siblingSubject, Key: edgeKey}
				got := invocationOf(t, generated(recordedSubject), func(l *state.Lane) {
					l.Invoked(plugin.Invocation{Match: generated(recordedSubject), Reads: readsOf(t, nil, fact)})
				})
				assert.Equal(t, got.Reads, sortedEdges(
					state.FactEdge(siblingSubject, edgeKey), state.DeclarationEdge(recordedSubject),
				), "the fact it read, and its subject")
			})

			t.Run("records nothing on a nil lane", func(t *testing.T) {
				t.Parallel()

				var l *state.Lane
				assert.NotPanics(t, func() { l.Invoked(plugin.Invocation{Match: generated(recordedSubject)}) },
					"a run that records no phases journals into a nil lane")
			})

			t.Run("adds no edge for a match without a subject", func(t *testing.T) {
				t.Parallel()

				graphWide := plugin.MatchKey{Plugin: generatorID, Rule: structRule}
				got := invocationOf(t, graphWide, func(l *state.Lane) {
					l.Invoked(plugin.Invocation{Match: graphWide, Findings: reported})
				})
				assert.Empty(t, got.Reads, "a graph-wide match reads no subject")
			})

			t.Run("records nothing for a pure invocation", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{Match: generated(recordedSubject)})
				})
				rows, err := g.All(t.Context(), state.TableInvocations)
				assert.NoError(t, err, "the invocations table reads")
				assert.Empty(t, rows, "an invocation that read, touched and reported nothing leaves no row")
				readers, err := g.Phases(t.Context()).Readers(state.DeclarationEdge(recordedSubject))
				assert.NoError(t, err, "the readers table reads")
				assert.Empty(t, readers, "and reads no edge of its subject")
			})

			fact := meta.FactRef{Subject: siblingSubject, Key: edgeKey}
			hosted := generated(recordedSubject)
			hosted.Host = plugin.EmitRef{Unit: hostUnit, Index: 1}
			tests := []struct {
				name string
				give plugin.Invocation
			}{
				{
					name: "records an invocation that read an edge",
					give: plugin.Invocation{Match: generated(recordedSubject), Reads: readsOf(t, nil, fact)},
				},
				{
					name: "records an invocation that read a plan's export",
					give: plugin.Invocation{Match: generated(recordedSubject), Exports: []string{failedPlan}},
				},
				{
					name: "records an invocation that placed declarations into a unit",
					give: plugin.Invocation{Match: generated(recordedSubject), Units: []plugin.UnitRef{hostUnit}},
				},
				{
					name: "records an invocation that appended into an emit value's slots",
					give: plugin.Invocation{Match: generated(recordedSubject), Hosts: []plugin.EmitRef{hosted.Host}},
				},
				{
					name: "records an invocation that claimed a fact",
					give: plugin.Invocation{Match: generated(recordedSubject), Claimed: []meta.FactRef{fact}},
				},
				{
					name: "records an invocation that reported a finding",
					give: plugin.Invocation{Match: generated(recordedSubject), Findings: reported},
				},
				{
					name: "records a match with a host that read, touched and reported nothing",
					give: plugin.Invocation{Match: hosted},
				},
				{
					name: "records a whole call that read, touched and reported nothing",
					give: plugin.Invocation{Match: plugin.MatchKey{Plugin: generatorID, Rule: plugin.WholeCall}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got := invocationOf(t, tt.give.Match, func(l *state.Lane) { l.Invoked(tt.give) })
					assert.Equal(t, got.Match, tt.give.Match, "the lane records the invocation")
				})
			}
		})

		t.Run("Evaluated", func(t *testing.T) {
			t.Parallel()

			t.Run("records nothing", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Evaluated(recordedSubject, []plugin.MatchKey{generated(recordedSubject)})
				})
				rows, err := g.All(t.Context(), state.TableInvocations)
				assert.NoError(t, err, "the invocations table reads")
				assert.Empty(t, rows, "a candidate is not an invocation")
			})
		})

		t.Run("Check", func(t *testing.T) {
			t.Parallel()

			t.Run("records nothing on a nil lane", func(t *testing.T) {
				t.Parallel()

				var l *state.Lane
				assert.NotPanics(t, func() { l.Check(checkID, nil, nil) },
					"a run that records no phases calls its checks over a nil lane")
			})

			t.Run("records what the check's reader read and no subject", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, readsOf(t, []symbol.Identity{siblingSubject}), nil)
				})
				got, held, err := g.Phases(t.Context()).Check(checkID)
				assert.NoError(t, err, "the checks table reads")
				assert.True(t, held, "the check is recorded")
				assert.Equal(t, got.Reads, []state.EdgeHash{state.DeclarationEdge(siblingSubject)},
					"the declaration the check looked up")
			})
		})
	})

	t.Run("Recorder", func(t *testing.T) {
		t.Parallel()

		t.Run("Lane", func(t *testing.T) {
			t.Parallel()

			t.Run("returns lanes whose records the commit writes", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
					r.Lane("").Validation(siblingSubject, nil, nil, nil)
				})
				rows, err := g.All(t.Context(), state.TableValidations)
				assert.NoError(t, err, "the validations table reads")
				assert.Length(t, rows, 2, "one row for each lane's validation")
			})
		})

		t.Run("Audit", func(t *testing.T) {
			t.Parallel()

			t.Run("records a finding under its key and its subject", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Audit(edgeKey, recordedSubject, recordedDiag)
				})
				got, err := g.Phases(t.Context()).Audits()
				assert.NoError(t, err, "the audit table reads")
				assert.Equal(t, got, []state.Audit{{Key: edgeKey, Subject: recordedSubject, Finding: recordedDiag}},
					"the one finding")
			})
		})
	})
}

// A lane allocates only to grow its buffers, which double when they fill,
// and a recorder only its lanes and the lists it keeps, in the ordinary
// run, which runs no benchmark. The check runs alone, because the count
// includes every goroutine's allocations.
func TestRecorderAllocs(t *testing.T) {
	reads := readsOf(t, nil, meta.FactRef{Subject: recordedSubject, Key: edgeKey})
	inv := plugin.Invocation{Match: generated(recordedSubject), Reads: reads}
	var lane *state.Lane
	assert.MaxAllocs(t, func() {
		lane = (&state.Recorder{}).Lane(recordedPlan)
		for range laneRecords {
			lane.Invoked(inv)
		}
	}, laneAllocs, "a lane allocates only to grow its buffers")
	assert.MaxAllocs(t, func() { lane.Validation(recordedSubject, nil, reads, nil) }, recordAllocs,
		"Validation allocates only to grow the lane's buffers")
	assert.MaxAllocs(t, func() { lane.Invoked(inv) }, recordAllocs, "Invoked allocates only to grow the lane's buffers")
	assert.MaxAllocs(t, func() { lane.Check(checkID, reads, nil) }, recordAllocs,
		"Check allocates only to grow the lane's buffers")
	assert.MaxAllocs(t, func() { lane.Evaluated(recordedSubject, nil) }, 0, "Evaluated allocates nothing")
	assert.MaxAllocs(t, func() { lane = (&state.Recorder{}).Lane(recordedPlan) }, newLaneAllocs,
		"Lane allocates the lane and the recorder's list of lanes")
	assert.MaxAllocs(t, func() { (&state.Recorder{}).Audit(edgeKey, recordedSubject, recordedDiag) }, auditAllocs,
		"Audit allocates the recorder's list of findings")
	assert.NotNil(t, lane, "the lane records")
}

// BenchmarkRecorder measures a lane's record of each kind, of one fact
// read, its buffers grown before the measurement, and a new recorder's
// first lane and first finding.
func BenchmarkRecorder(b *testing.B) {
	reads := readsOf(b, nil, meta.FactRef{Subject: recordedSubject, Key: edgeKey})
	inv := plugin.Invocation{Match: generated(recordedSubject), Reads: reads}
	lane := (&state.Recorder{}).Lane(recordedPlan)
	for range laneRecords {
		lane.Validation(recordedSubject, nil, reads, nil)
		lane.Invoked(inv)
		lane.Check(checkID, reads, nil)
	}

	b.Run("Lane", func(b *testing.B) {
		b.Run("Validation", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(recordAllocs)
			defer c.End()
			for c.Loop() {
				lane.Validation(recordedSubject, nil, reads, nil)
			}
			assert.NotNil(b, lane, "the lane records")
		})

		b.Run("Invoked", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(recordAllocs)
			defer c.End()
			for c.Loop() {
				lane.Invoked(inv)
			}
			assert.NotNil(b, lane, "the lane records")
		})

		b.Run("Evaluated", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			for c.Loop() {
				lane.Evaluated(recordedSubject, nil)
			}
			assert.NotNil(b, lane, "the lane keeps nothing")
		})

		b.Run("Check", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(recordAllocs)
			defer c.End()
			for c.Loop() {
				lane.Check(checkID, reads, nil)
			}
			assert.NotNil(b, lane, "the lane records")
		})
	})

	b.Run("Recorder", func(b *testing.B) {
		b.Run("Lane", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(newLaneAllocs)
			defer c.End()
			var got *state.Lane
			for c.Loop() {
				got = (&state.Recorder{}).Lane(recordedPlan)
			}
			assert.NotNil(b, got, "Lane returns a lane")
		})

		b.Run("Audit", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(auditAllocs)
			defer c.End()
			r := &state.Recorder{}
			for c.Loop() {
				r = &state.Recorder{}
				r.Audit(edgeKey, recordedSubject, recordedDiag)
			}
			assert.NotNil(b, r, "the recorder keeps the finding")
		})
	})
}

// validationOf records the validation build makes into a shared lane, and
// returns the recorded subject's record.
func validationOf(tb testing.TB, build func(*state.Lane)) state.Validation {
	tb.Helper()

	g := recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) { build(r.Lane("")) })
	got, held, err := g.Phases(tb.Context()).Validation(recordedSubject)
	assert.NoError(tb, err, "the validations table reads")
	assert.True(tb, held, "the subject's validation is recorded")
	return got
}

// invocationOf records the invocation build makes into the plan's lane,
// and returns the record of the match.
func invocationOf(tb testing.TB, m plugin.MatchKey, build func(*state.Lane)) state.Invocation {
	tb.Helper()

	g := recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) { build(r.Lane(recordedPlan)) })
	got, held, err := g.Phases(tb.Context()).Invocation(recordedPlan, m)
	assert.NoError(tb, err, "the invocations table reads")
	assert.True(tb, held, "the invocation is recorded")
	return got
}
