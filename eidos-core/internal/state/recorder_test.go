// Copyright Dokimasia B.V. 2026
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

// A lane records each validation, invocation, check and group with the
// edges that it read. The lane adds the declaration edge of the record's
// subject, the export edge of each plan whose export an invocation read,
// and the findings edge for a record that reported a finding.
// The recorder passes the records of every lane to the commit. On a warm
// run, the commit also keeps the records of the generation that the run
// did not drop.
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
					Reads:    sortedEdges(state.DeclarationEdge(recordedSubject), pinnedHash([]byte{findingsTag})),
					Findings: []diag.Diag{recordedDiag},
				}, "the record lists the subject's own edge, the findings edge and the finding")
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

			t.Run("adds the findings edge to the reads of an invocation that reported a finding", func(t *testing.T) {
				t.Parallel()

				got := invocationOf(t, generated(recordedSubject), func(l *state.Lane) {
					l.Invoked(plugin.Invocation{Match: generated(recordedSubject), Findings: reported})
				})
				assert.Equal(t, got.Reads, sortedEdges(state.DeclarationEdge(recordedSubject), state.FindingsEdge),
					"the reads list the subject's edge and the findings edge")
			})

			t.Run("adds no edge for a match without a subject", func(t *testing.T) {
				t.Parallel()

				graphWide := plugin.MatchKey{Plugin: generatorID, Rule: structRule}
				got := invocationOf(t, graphWide, func(l *state.Lane) {
					l.Invoked(plugin.Invocation{Match: graphWide, Exports: []string{failedPlan}})
				})
				assert.Equal(t, got.Reads, []state.EdgeHash{state.ExportEdge(failedPlan)},
					"a graph-wide match reads the export alone")
			})

			t.Run("adds the export edge of each plan whose export the invocation read", func(t *testing.T) {
				t.Parallel()

				exports := []string{failedPlan, recordedPlan}
				got := invocationOf(t, generated(recordedSubject), func(l *state.Lane) {
					l.Invoked(plugin.Invocation{Match: generated(recordedSubject), Exports: exports})
				})
				assert.Equal(t, got.Reads, sortedEdges(
					state.DeclarationEdge(recordedSubject),
					state.ExportEdge(failedPlan),
					state.ExportEdge(recordedPlan),
				), "the reads list the subject's edge and the edge of each export")
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

		t.Run("Group", func(t *testing.T) {
			t.Parallel()

			t.Run("records the group under the lane's plan", func(t *testing.T) {
				t.Parallel()

				g := recordedGroup(hostUnit)
				g.Plan = failedPlan
				got := groupOf(t, func(l *state.Lane) { l.Group(g) })
				assert.Equal(t, got.Plan, recordedPlan, "the lane names the plan")
			})

			t.Run("records what the group read", func(t *testing.T) {
				t.Parallel()

				got := groupOf(t, func(l *state.Lane) { l.Group(recordedGroup(hostUnit)) })
				want := sortedEdges(state.UnitEdge(recordedPlan, hostUnit), state.DirectoryEdge(edgeDir))
				assert.Equal(t, got.Reads, want, "the edge of the group's unit and the directory of its file")
			})

			t.Run("adds the findings edge to the reads of a group that reported a finding", func(t *testing.T) {
				t.Parallel()

				g := recordedGroup(hostUnit)
				g.Findings = []diag.Diag{recordedDiag}
				got := groupOf(t, func(l *state.Lane) { l.Group(g) })
				assert.Equal(t, got.Reads, sortedEdges(
					state.UnitEdge(recordedPlan, hostUnit), state.DirectoryEdge(edgeDir), state.FindingsEdge,
				), "the reads list the findings edge")
			})

			t.Run("records nothing on a nil lane", func(t *testing.T) {
				t.Parallel()

				var l *state.Lane
				assert.NotPanics(t, func() { l.Group(recordedGroup(hostUnit)) },
					"a run that records no phases groups through a nil lane")
			})
		})

		t.Run("Reset", func(t *testing.T) {
			t.Parallel()

			t.Run("drops the lane's records", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					plan := r.Lane(recordedPlan)
					plan.Group(recordedGroup(hostUnit))
					plan.Artifact(recordedArtifact(artifactPath, recordedSubject))
					plan.Reset()
					plan.Group(recordedGroup(edgeUnit))
				})
				s := g.Phases(t.Context())
				_, held, err := s.Group(recordedPlan, hostUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.False(t, held, "the group recorded before the reset is gone")
				_, held, err = s.Artifact(artifactPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.False(t, held, "the artifact recorded before the reset is gone")
				_, held, err = s.Group(recordedPlan, edgeUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.True(t, held, "the group recorded after the reset is recorded")
			})

			t.Run("does nothing on a nil lane", func(t *testing.T) {
				t.Parallel()

				var l *state.Lane
				assert.NotPanics(t, l.Reset, "a run that records no phases resets a nil lane")
			})
		})

		t.Run("Artifact", func(t *testing.T) {
			t.Parallel()

			t.Run("records the artifact under the lane's plan", func(t *testing.T) {
				t.Parallel()

				a := recordedArtifact(artifactPath, recordedSubject)
				a.Entry.Plan = failedPlan
				got := artifactOf(t, func(l *state.Lane) { l.Artifact(a) })
				assert.Equal(t, got.Entry.Plan, recordedPlan, "the lane names the plan")
			})

			t.Run("records the names of the file under the plan", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				got, held := g.Phases(t.Context()).Names(recordedPlan, nil).InPackage(edgeDir, edgeReceiver)
				assert.True(t, held, "the file's name is recorded")
				assert.Equal(t, got.File, artifactPath, "the name states the file")
			})

			t.Run("records nothing on a nil lane", func(t *testing.T) {
				t.Parallel()

				var l *state.Lane
				assert.NotPanics(t, func() { l.Artifact(recordedArtifact(artifactPath, recordedSubject)) },
					"a run that records no phases records its files through a nil lane")
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

			t.Run("adds the findings edge to the reads of a check that reported a finding", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, nil, []diag.Diag{recordedDiag})
				})
				got, held, err := g.Phases(t.Context()).Check(checkID)
				assert.NoError(t, err, "the checks table reads")
				assert.True(t, held, "the check is recorded")
				assert.Equal(t, got.Reads, []state.EdgeHash{state.FindingsEdge}, "the reads list the findings edge")
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

		t.Run("Keep", func(t *testing.T) {
			t.Parallel()

			t.Run("makes the commit keep the generation's validations", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, []diag.Diag{recordedDiag})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.Lane("").Check(checkID, nil, nil)
				})
				_, held, err := g.Phases(t.Context()).Validation(recordedSubject)
				assert.NoError(t, err, "the validations table reads")
				assert.True(t, held, "the commit keeps a validation that no lane replaced")
			})

			t.Run("does nothing on a nil recorder", func(t *testing.T) {
				t.Parallel()

				var r *state.Recorder
				assert.NotPanics(t, r.Keep, "a run that records no phases calls Keep on a nil recorder")
			})
		})

		t.Run("DropValidation", func(t *testing.T) {
			t.Parallel()

			t.Run("leaves the subject's validation out of a kept commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					shared := r.Lane("")
					shared.Validation(recordedSubject, nil, nil, nil)
					shared.Validation(siblingSubject, nil, nil, nil)
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.DropValidation(recordedSubject)
				})
				s := g.Phases(t.Context())
				_, held, err := s.Validation(recordedSubject)
				assert.NoError(t, err, "the validations table reads")
				assert.False(t, held, "the commit leaves out the dropped validation")
				_, held, err = s.Validation(siblingSubject)
				assert.NoError(t, err, "the validations table reads")
				assert.True(t, held, "the commit keeps the sibling's validation")
			})

			t.Run("does nothing on a nil recorder", func(t *testing.T) {
				t.Parallel()

				var r *state.Recorder
				assert.NotPanics(t, func() { r.DropValidation(recordedSubject) },
					"a run that records no phases calls DropValidation on a nil recorder")
			})
		})

		t.Run("DropInvocation", func(t *testing.T) {
			t.Parallel()

			t.Run("leaves the match's invocation out of a kept commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					shared := r.Lane("")
					shared.Invoked(plugin.Invocation{Match: annotated(recordedSubject), Findings: reported})
					shared.Invoked(plugin.Invocation{Match: annotated(siblingSubject), Findings: reported})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.DropInvocation("", annotated(recordedSubject))
				})
				s := g.Phases(t.Context())
				_, held, err := s.Invocation("", annotated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.False(t, held, "the commit leaves out the dropped invocation")
				_, held, err = s.Invocation("", annotated(siblingSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "the commit keeps the sibling's invocation")
			})

			t.Run("leaves a kept plan's invocation out of the commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					plan := r.Lane(recordedPlan)
					plan.Invoked(plugin.Invocation{Match: generated(recordedSubject), Findings: reported})
					plan.Invoked(plugin.Invocation{Match: generated(siblingSubject), Findings: reported})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.KeepPlan(recordedPlan)
					r.DropInvocation(recordedPlan, generated(recordedSubject))
				})
				s := g.Phases(t.Context())
				_, held, err := s.Invocation(recordedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.False(t, held, "the commit leaves out the dropped invocation")
				_, held, err = s.Invocation(recordedPlan, generated(siblingSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "the commit keeps the plan's other invocation")
			})

			t.Run("does nothing on a nil recorder", func(t *testing.T) {
				t.Parallel()

				var r *state.Recorder
				assert.NotPanics(t, func() { r.DropInvocation("", annotated(recordedSubject)) },
					"a run that records no phases calls DropInvocation on a nil recorder")
			})
		})

		t.Run("KeepPlan", func(t *testing.T) {
			t.Parallel()

			t.Run("keeps the plan's records that a lane did not replace", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Group(recordedGroup(hostUnit))
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.KeepPlan(recordedPlan)
				})
				_, held, err := g.Phases(t.Context()).Group(recordedPlan, hostUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.True(t, held, "the commit keeps the plan's group")
			})

			t.Run("does nothing on a nil recorder", func(t *testing.T) {
				t.Parallel()

				var r *state.Recorder
				assert.NotPanics(t, func() { r.KeepPlan(recordedPlan) },
					"a run that records no phases calls KeepPlan on a nil recorder")
			})
		})

		t.Run("DropGroup", func(t *testing.T) {
			t.Parallel()

			t.Run("leaves a kept plan's group out of the commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					plan := r.Lane(recordedPlan)
					plan.Group(recordedGroup(hostUnit))
					plan.Group(recordedGroup(edgeUnit))
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.KeepPlan(recordedPlan)
					r.DropGroup(recordedPlan, hostUnit)
				})
				s := g.Phases(t.Context())
				_, held, err := s.Group(recordedPlan, hostUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.False(t, held, "the commit leaves out the dropped group")
				_, held, err = s.Group(recordedPlan, edgeUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.True(t, held, "the commit keeps the plan's other group")
			})

			t.Run("does nothing on a nil recorder", func(t *testing.T) {
				t.Parallel()

				var r *state.Recorder
				assert.NotPanics(t, func() { r.DropGroup(recordedPlan, hostUnit) },
					"a run that records no phases calls DropGroup on a nil recorder")
			})
		})

		t.Run("DropFile", func(t *testing.T) {
			t.Parallel()

			t.Run("leaves a kept plan's artifact out of the commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					plan := r.Lane(recordedPlan)
					plan.Artifact(recordedArtifact(artifactPath, recordedSubject))
					plan.Artifact(recordedArtifact(siblingPath, siblingSubject))
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.KeepPlan(recordedPlan)
					r.DropFile(artifactPath)
				})
				s := g.Phases(t.Context())
				_, held, err := s.Artifact(artifactPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.False(t, held, "the commit leaves out the dropped artifact")
				_, held, err = s.Artifact(siblingPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.True(t, held, "the commit keeps the plan's other artifact")
			})

			t.Run("leaves the names of a kept plan's file out of the commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.KeepPlan(recordedPlan)
					r.DropFile(artifactPath)
				})
				_, held := g.Phases(t.Context()).Names(recordedPlan, nil).InPackage(edgeDir, edgeReceiver)
				assert.False(t, held, "the commit leaves out the names of the dropped file")
			})

			t.Run("does nothing on a nil recorder", func(t *testing.T) {
				t.Parallel()

				var r *state.Recorder
				assert.NotPanics(t, func() { r.DropFile(artifactPath) },
					"a run that records no phases calls DropFile on a nil recorder")
			})
		})

		t.Run("Withdrew", func(t *testing.T) {
			t.Parallel()

			t.Run("drops the bag of a subject that the run left without a claim", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				facts, _ := stampedFacts(t)
				recordedPhases(t, l, facts, nil, func(*state.Recorder) {})
				g := recordedPhases(t, l, meta.NewFacts(facts.Registry()), nil, func(r *state.Recorder) {
					r.Keep()
					r.Withdrew(recordedSubject)
				})
				s := g.Phases(t.Context())
				got, err := s.Claims(recordedSubject)
				assert.NoError(t, err, "the claims table reads")
				assert.Empty(t, got, "the commit drops the subject's bag")
				got, err = s.Claims(siblingSubject)
				assert.NoError(t, err, "the claims table reads")
				assert.Equal(t, got, bagOf(facts, siblingSubject), "the commit keeps the sibling's bag")
			})

			t.Run("does nothing on a nil recorder", func(t *testing.T) {
				t.Parallel()

				var r *state.Recorder
				assert.NotPanics(t, func() { r.Withdrew(recordedSubject) },
					"a run that records no phases calls Withdrew on a nil recorder")
			})
		})
	})
}

// A lane allocates only to grow its buffers, which double when they fill.
// A recorder allocates only its lanes, lists and sets, so a repeated drop
// or withdrawal does not allocate. The ordinary run checks these
// ceilings, because it does not run benchmarks. The check runs alone,
// because the count includes every goroutine's allocations.
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
	group := recordedGroup(hostUnit)
	assert.MaxAllocs(t, func() { lane.Group(group) }, recordAllocs, "Group allocates only to grow the lane's buffers")
	artifact := recordedArtifact(artifactPath, recordedSubject)
	assert.MaxAllocs(t, func() { lane.Artifact(artifact) }, recordAllocs,
		"Artifact allocates only to grow the lane's buffers")
	assert.MaxAllocs(t, func() { lane.Evaluated(recordedSubject, nil) }, 0, "Evaluated allocates nothing")
	assert.MaxAllocs(t, lane.Reset, 0, "Reset allocates nothing")
	assert.MaxAllocs(t, func() { lane = (&state.Recorder{}).Lane(recordedPlan) }, newLaneAllocs,
		"Lane allocates the lane and the recorder's list of lanes")
	assert.MaxAllocs(t, func() { (&state.Recorder{}).Audit(edgeKey, recordedSubject, recordedDiag) }, auditAllocs,
		"Audit allocates the recorder's list of findings")
	assert.NotNil(t, lane, "the lane records")
	r := droppedRecorder()
	match := generated(recordedSubject)
	assert.MaxAllocs(t, r.Keep, 0, "Keep allocates nothing")
	assert.MaxAllocs(t, func() { r.DropValidation(recordedSubject) }, 0,
		"DropValidation allocates nothing for a subject that it dropped before")
	assert.MaxAllocs(t, func() { r.DropInvocation("", match) }, 0,
		"DropInvocation allocates nothing for an annotator's match that it dropped before")
	assert.MaxAllocs(t, func() { r.DropInvocation(recordedPlan, match) }, 0,
		"DropInvocation allocates nothing for a plan's match that it dropped before")
	assert.MaxAllocs(t, func() { r.KeepPlan(recordedPlan) }, 0, "KeepPlan allocates nothing for a plan it kept before")
	assert.MaxAllocs(t, func() { r.DropGroup(recordedPlan, hostUnit) }, 0,
		"DropGroup allocates nothing for a group that it dropped before")
	assert.MaxAllocs(t, func() { r.DropFile(artifactPath) }, 0,
		"DropFile allocates nothing for a file that it dropped before")
	assert.MaxAllocs(t, func() { r.Withdrew(recordedSubject) }, 0,
		"Withdrew allocates nothing for a subject that it recorded before")
}

// BenchmarkRecorder measures a lane's record of each kind, of one fact
// read, with the lane's buffers grown before the measurement. It also
// measures the first lane and the first finding of a new recorder, and
// the calls that a warm run makes on a recorder that received each call
// before.
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

		b.Run("Group", func(b *testing.B) {
			group := recordedGroup(hostUnit)
			c := bench.Start(b).MaxAllocs(recordAllocs)
			defer c.End()
			for c.Loop() {
				lane.Group(group)
			}
			assert.NotNil(b, lane, "the lane records")
		})

		b.Run("Artifact", func(b *testing.B) {
			artifact := recordedArtifact(artifactPath, recordedSubject)
			c := bench.Start(b).MaxAllocs(recordAllocs)
			defer c.End()
			for c.Loop() {
				lane.Artifact(artifact)
			}
			assert.NotNil(b, lane, "the lane records")
		})

		b.Run("Reset", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			for c.Loop() {
				lane.Reset()
			}
			assert.NotNil(b, lane, "the lane keeps its buffers")
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

		r := droppedRecorder()
		match := generated(recordedSubject)
		benches := []struct {
			name string
			call func()
		}{
			{name: "Keep", call: r.Keep},
			{name: "DropValidation", call: func() { r.DropValidation(recordedSubject) }},
			{name: "DropInvocation", call: func() { r.DropInvocation(recordedPlan, match) }},
			{name: "KeepPlan", call: func() { r.KeepPlan(recordedPlan) }},
			{name: "DropGroup", call: func() { r.DropGroup(recordedPlan, hostUnit) }},
			{name: "DropFile", call: func() { r.DropFile(artifactPath) }},
			{name: "Withdrew", call: func() { r.Withdrew(recordedSubject) }},
		}
		for _, bb := range benches {
			b.Run(bb.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(0)
				defer c.End()
				for c.Loop() {
					bb.call()
				}
				assert.NotNil(b, r, "the recorder keeps its sets")
			})
		}
	})
}

// droppedRecorder returns a recorder that already dropped the validation
// of the recorded subject, the invocation of its match under the empty
// plan and under the recorded plan, the plan's group of the host unit and
// the recorded artifact's file, that kept the plan, and that recorded a
// withdrawal from the subject. A repeated call of each method adds
// nothing to its set.
func droppedRecorder() *state.Recorder {
	r := &state.Recorder{}
	r.DropValidation(recordedSubject)
	r.DropInvocation("", generated(recordedSubject))
	r.DropInvocation(recordedPlan, generated(recordedSubject))
	r.KeepPlan(recordedPlan)
	r.DropGroup(recordedPlan, hostUnit)
	r.DropFile(artifactPath)
	r.Withdrew(recordedSubject)
	return r
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

// recordedGroup returns a group of the recorded plan of one unit and one
// file, which the unit's invocation contributed to and which read the
// unit's edge and the directory of the file.
func recordedGroup(unit plugin.UnitRef) state.Group {
	return state.Group{
		Key:          unit,
		Units:        []plugin.UnitRef{unit},
		Files:        []string{edgeDir + "/user_gen.zz"},
		Contributors: []plugin.MatchKey{generated(recordedSubject)},
		Reads:        []state.EdgeHash{state.UnitEdge(recordedPlan, unit), state.DirectoryEdge(edgeDir)},
	}
}

// groupOf records the group build makes into the plan's lane, and returns
// the record of the group of hostUnit.
func groupOf(tb testing.TB, build func(*state.Lane)) state.Group {
	tb.Helper()

	g := recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) { build(r.Lane(recordedPlan)) })
	got, held, err := g.Phases(tb.Context()).Group(recordedPlan, hostUnit)
	assert.NoError(tb, err, "the groups table reads")
	assert.True(tb, held, "the group is recorded")
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
