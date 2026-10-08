// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"cmp"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The plans, plugins and checks the recorded cases name. The replacing
// plan records nothing and runs whole, so its lane replaces its records.
const (
	recordedPlan                  = "mirror"
	failedPlan                    = "stubs"
	replacingPlan                 = "whole"
	annotatorID     plugin.ID     = "shape"
	generatorID     plugin.ID     = "mirror"
	checkID         plugin.ID     = "audit"
	structRule      plugin.RuleID = 2
	recordedFinding               = "a finding the record keeps"
)

// The subjects the recorded cases execute over: the edge cases' struct,
// and a sibling struct of its package.
var (
	recordedSubject = edgeStruct
	siblingSubject  = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: "svc/api", Name: "Account", Kind: symbol.KindStruct,
	}
)

// reported is what the recorded cases' invocations report, so a lane
// records each of them: a lane leaves out an invocation that reads,
// touches and reports nothing.
var reported = []diag.Diag{recordedDiag}

// The ceilings of the record of a run's phases, over the generation
// lookedUp records and a recorder of the same records.
const (
	// phasesAllocs is a generation's record of the phases: the state.
	phasesAllocs = 1
	// recordPhasesAllocs is the record's preparation over the generation:
	// the record, and for each of the five tables that have rows, the run
	// read from the ledger, its index, one list of its entries and the
	// merged entries.
	recordPhasesAllocs = 21
	// phaseCommitAllocs is the ceiling of the record's completion into a
	// commit over the generation, which does not change a row. Commit
	// allocates the list of buffers, one list for the records of each of
	// the four kinds, and one list for the reads. For each of the five
	// tables that have rows, it allocates the sorted records, the keys and
	// the list of rows. The readers table also allocates the bytes of its
	// rows. The bucket counts of so few records fit on the stack.
	phaseCommitAllocs = 22
	// warmRecordPhasesAllocs is the record's preparation over the
	// generation for a warm run that drops and records each record again
	// with the same reads, so no readers row changes:
	//   - the record, the set of the touched records and the shared strings
	//   - for each record table, the list of IDs, the keys, the rows looked
	//     up, and a decoder
	//   - the fields and the reads of each prior entry, decoded once for the
	//     reads before and once for the reads that the commit keeps
	//   - the lists of the reads before and after
	//   - the checks table, which the record reads whole
	warmRecordPhasesAllocs = 58
	// warmPhaseCommitAllocs is the ceiling of that record's completion into
	// a commit, which changes no row: the buffers, the lists of records and
	// reads and the rows of the record tables, a decoder for each record
	// table, and the lists and the reads of each prior entry that the commit
	// keeps. The entries' strings are those that the preparation decoded,
	// and the commit changes no readers row.
	warmPhaseCommitAllocs = 26
)

// phaseKeys are the keys the recorded fact stores claim: one of each
// type of the fact vocabulary, and a key in a group.
type phaseKeys struct {
	role   meta.Key[string]
	count  meta.Key[int64]
	flag   meta.Key[bool]
	tags   meta.Key[[]string]
	target meta.Key[symbol.Identity]
}

// bagSource is a scripted recorded source for a restored fact store: the
// claims a previous run recorded.
type bagSource struct {
	claims map[symbol.Identity][]meta.StoredClaim
}

// Claims returns the subject's recorded claims.
func (s *bagSource) Claims(subject symbol.Identity) ([]meta.StoredClaim, error) {
	return s.claims[subject], nil
}

// Present returns no recorded presence.
func (*bagSource) Present(meta.KeyName) ([]symbol.Identity, error) { return nil, nil }

// A run's phases are recorded whole on a cold run, and a warm run's
// commit writes only what changed: a row it added or changed and a
// tombstone for a row it lost, while a plan that does not commit keeps
// the invocations of its previous run. The commit of a warm run also
// keeps each shared record and each bag of claims that the run did not
// drop.
func TestPhases(t *testing.T) {
	t.Parallel()

	t.Run("Generation", func(t *testing.T) {
		t.Parallel()

		t.Run("Phases", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of the generation's phases", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, nil, nil)
				})
				_, held, err := g.Phases(t.Context()).Check(checkID)
				assert.NoError(t, err, "the check's row reads")
				assert.True(t, held, "the record lists the check")
			})
		})
	})

	t.Run("RecordPhases", func(t *testing.T) {
		t.Parallel()

		t.Run("records a row for every validation, invocation, check and edge", func(t *testing.T) {
			t.Parallel()

			fact := meta.FactRef{Subject: recordedSubject, Key: edgeKey}
			g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
				shared := r.Lane("")
				shared.Validation(recordedSubject, nil, nil, nil)
				shared.Invoked(plugin.Invocation{Match: annotated(recordedSubject), Findings: reported})
				shared.Check(checkID, readsOf(t, nil, fact), nil)
				r.Lane(recordedPlan).Invoked(plugin.Invocation{
					Match: generated(recordedSubject), Reads: readsOf(t, nil, fact),
				})
			})
			s := g.Phases(t.Context())
			declared, err := s.Readers(state.DeclarationEdge(recordedSubject))
			assert.NoError(t, err, "the subject's readers read")
			assert.Equal(t, declared, sortedRefs(
				state.ValidationRef(recordedSubject),
				state.InvocationRef("", annotated(recordedSubject)),
				state.InvocationRef(recordedPlan, generated(recordedSubject)),
			), "the validation and both invocations read the subject")
			read, err := s.Readers(state.FactEdge(recordedSubject, edgeKey))
			assert.NoError(t, err, "the fact's readers read")
			assert.Equal(t, read, sortedRefs(
				state.InvocationRef(recordedPlan, generated(recordedSubject)), state.CheckRef(checkID),
			), "the plan's invocation and the check read the fact")
		})

		t.Run("records two entries of one ID in one row", func(t *testing.T) {
			t.Parallel()

			g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
				r.Lane("").Validation(recordedSubject, nil, nil, nil)
				r.Lane("").Validation(recordedSubject, nil, nil, nil)
			})
			rows, err := g.All(t.Context(), state.TableValidations)
			assert.NoError(t, err, "the validations table reads")
			assert.Length(t, rows, 1, "one row keeps both entries")
			assert.Equal(t, rows[0].Value[0], byte(2), "and counts two")
			got, held, err := g.Phases(t.Context()).Validation(recordedSubject)
			assert.NoError(t, err, "the shared row decodes")
			assert.True(t, held, "and the subject has an entry")
			assert.Equal(t, got.Subject, recordedSubject, "and its first entry is the subject's")
			readers, err := g.Phases(t.Context()).Readers(state.DeclarationEdge(recordedSubject))
			assert.NoError(t, err, "the subject's readers read")
			assert.Equal(t, readers, []state.RecordRef{state.ValidationRef(recordedSubject)},
				"which list the validation once")
		})

		t.Run("returns ErrDamaged for a prior table that does not read whole", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			parent := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
				r.Lane("").Check(checkID, nil, nil)
			})
			damageRun(t, l, func(int) int { return 3 })
			_, err := state.RecordPhases(t.Context(), parent, &state.Recorder{}, state.PhaseRun{Facts: emptyFacts()})
			assert.ErrorIs(t, err, state.ErrDamaged, "the damaged block is found before any plan commits")
		})

		t.Run("returns an error for a claim whose value is outside the fact vocabulary", func(t *testing.T) {
			t.Parallel()

			r, keys := phaseRegistry(t)
			facts := meta.Restore(r, &bagSource{claims: map[symbol.Identity][]meta.StoredClaim{
				recordedSubject: {{Key: keys.role.Name(), Claim: claimBy("defaults", 1), Value: 1.5, Drop: true}},
			}})
			assert.NotEmpty(t, slices.Collect(facts.Claims(recordedSubject, keys.role.ID())),
				"the role's claims restore the subject's bag")
			_, err := state.RecordPhases(t.Context(), nil, &state.Recorder{}, state.PhaseRun{Facts: facts})
			assert.HasError(t, err, "a float is outside the vocabulary")
		})

		t.Run("keeps the generation's claims on a subject that a warm run did not touch", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			facts, keys := stampedFacts(t)
			recordedPhases(t, l, facts, nil, func(*state.Recorder) {})
			g := recordedPhases(t, l, siblingCounted(t, facts, keys), nil, func(r *state.Recorder) { r.Keep() })
			got, err := g.Phases(t.Context()).Claims(recordedSubject)
			assert.NoError(t, err, "the claims table reads")
			assert.Equal(t, got, bagOf(facts, recordedSubject), "the subject keeps the generation's bag")
		})

		t.Run("replaces the generation's claims on a subject that a warm run claimed", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			facts, keys := stampedFacts(t)
			recordedPhases(t, l, facts, nil, func(*state.Recorder) {})
			run := siblingCounted(t, facts, keys)
			g := recordedPhases(t, l, run, nil, func(r *state.Recorder) { r.Keep() })
			got, err := g.Phases(t.Context()).Claims(siblingSubject)
			assert.NoError(t, err, "the claims table reads")
			assert.Equal(t, got, bagOf(run, siblingSubject), "the sibling has the run's bag")
		})
	})

	t.Run("PhaseRecord", func(t *testing.T) {
		t.Parallel()

		t.Run("Commit", func(t *testing.T) {
			t.Parallel()

			t.Run("writes no generation for a run that recorded what the prior record keeps", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				build := func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
					r.Lane(recordedPlan).Invoked(plugin.Invocation{
						Match: generated(recordedSubject), Findings: reported,
					})
				}
				first := recordedPhases(t, l, emptyFacts(), nil, build)
				var second *state.Generation
				assert.Pure(t, l.Writes, func() { second = recordedPhases(t, l, emptyFacts(), nil, build) },
					"nothing is written")
				assert.Equal(t, second.Name, first.Name, "and the generation is the prior one")
			})

			t.Run("deletes the row of a record the run no longer has", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, nil, nil)
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				_, held, err := g.Phases(t.Context()).Check(checkID)
				assert.NoError(t, err, "the checks table reads")
				assert.False(t, held, "the check's row is gone")
			})

			t.Run("keeps the prior invocations of a plan that does not commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(failedPlan).Invoked(plugin.Invocation{Match: generated(recordedSubject), Findings: reported})
				})
				g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(r *state.Recorder) {
					r.Lane(failedPlan).Invoked(plugin.Invocation{Match: generated(siblingSubject), Findings: reported})
				})
				s := g.Phases(t.Context())
				_, kept, err := s.Invocation(failedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, kept, "the prior invocation remains")
				_, held, err := s.Invocation(failedPlan, generated(siblingSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.False(t, held, "and the run's own is not recorded")
				readers, err := s.Readers(state.DeclarationEdge(recordedSubject))
				assert.NoError(t, err, "the subject's readers read")
				assert.Equal(t, readers, []state.RecordRef{state.InvocationRef(failedPlan, generated(recordedSubject))},
					"and the kept invocation still reads its subject")
			})

			tests := []struct {
				name string
				give state.Row
			}{
				{
					name: "keeps nothing of a prior invocations row that ends inside its entry",
					give: state.Row{Key: make([]byte, 8), Value: []byte{1, 9}},
				},
				{
					name: "keeps nothing of a prior invocations row that counts more entries than it has",
					give: state.Row{Key: make([]byte, 8), Value: []byte{5}},
				},
				{
					name: "keeps nothing of a prior invocations entry whose plan is cut short",
					give: state.Row{Key: make([]byte, 8), Value: []byte{1, 1, 0x80}},
				},
				{
					name: "keeps nothing of a prior invocations entry whose match is cut short",
					give: state.Row{Key: make([]byte, 8), Value: []byte{1, 6, 5, 's', 't', 'u', 'b', 's'}},
				},
				{
					name: "keeps nothing of a prior invocations row under a key of another length",
					give: state.Row{Key: []byte("id"), Value: []byte{0}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					l := ledger.NewMem()
					parent := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
						r.Lane(failedPlan).Invoked(plugin.Invocation{
							Match: generated(recordedSubject), Findings: reported,
						})
					})
					c := state.NewCommit(parent, nil)
					c.Put(state.TableInvocations, tt.give.Key, tt.give.Value)
					_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
					assert.NoError(t, err, "the damaged row is committed")
					g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(*state.Recorder) {})
					_, kept, err := g.Phases(t.Context()).Invocation(failedPlan, generated(recordedSubject))
					assert.NoError(t, err, "the invocations table reads")
					assert.True(t, kept, "the invocation that decodes remains")
					rows, err := g.All(t.Context(), state.TableInvocations)
					assert.NoError(t, err, "the invocations table reads")
					assert.Length(t, rows, 1, "and the damaged row is gone")
				})
			}

			t.Run("keeps the prior groups of a plan that does not commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(failedPlan).Group(recordedGroup(hostUnit))
				})
				g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(r *state.Recorder) {
					r.Lane(failedPlan).Group(recordedGroup(edgeUnit))
				})
				s := g.Phases(t.Context())
				_, kept, err := s.Group(failedPlan, hostUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.True(t, kept, "the prior group remains")
				_, held, err := s.Group(failedPlan, edgeUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.False(t, held, "the run's own group is not recorded")
			})

			t.Run("keeps the prior artifacts of a plan that does not commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(failedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(r *state.Recorder) {
					r.Lane(failedPlan).Artifact(recordedArtifact(siblingPath, siblingSubject))
				})
				s := g.Phases(t.Context())
				_, kept, err := s.Artifact(artifactPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.True(t, kept, "the prior artifact remains")
				_, held, err := s.Artifact(siblingPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.False(t, held, "the run's own artifact is not recorded")
				clashes, err := s.Clashes(artifactPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.Length(t, clashes, 1, "the prior artifact's folded row remains")
			})

			t.Run("keeps the prior names of a plan that does not commit", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(failedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(*state.Recorder) {})
				_, held := g.Phases(t.Context()).Names(failedPlan, nil).InPackage(edgeDir, recordedSubject.Name)
				assert.True(t, held, "the prior name remains")
			})

			t.Run("replaces the prior artifacts of a plan that commits", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Artifact(recordedArtifact(siblingPath, siblingSubject))
				})
				s := g.Phases(t.Context())
				_, held, err := s.Artifact(artifactPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.False(t, held, "the prior artifact is gone")
				_, held, err = s.Artifact(siblingPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.True(t, held, "the run's own artifact is recorded")
			})

			t.Run("writes the artifact of a plan that commits over a kept artifact of its path", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(failedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(r *state.Recorder) {
					r.Lane(recordedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				got, held, err := g.Phases(t.Context()).Artifact(artifactPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.True(t, held, "the path has an artifact")
				assert.Equal(t, got.Entry.Plan, recordedPlan, "the committing plan's artifact replaces the kept one")
			})

			t.Run("replaces the prior invocations of a plan that commits", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{
						Match: generated(recordedSubject), Findings: reported,
					})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{
						Match: generated(siblingSubject), Findings: reported,
					})
				})
				s := g.Phases(t.Context())
				_, held, err := s.Invocation(recordedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.False(t, held, "the prior invocation is gone")
				_, held, err = s.Invocation(recordedPlan, generated(siblingSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "and the run's own is recorded")
			})

			t.Run("replaces the prior invocations of the annotators", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Invoked(plugin.Invocation{Match: annotated(recordedSubject), Findings: reported})
				})
				g := recordedPhases(t, l, emptyFacts(), []string{failedPlan}, func(r *state.Recorder) {
					r.Lane("").Invoked(plugin.Invocation{Match: annotated(siblingSubject), Findings: reported})
				})
				s := g.Phases(t.Context())
				_, held, err := s.Invocation("", annotated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.False(t, held, "the prior annotator invocation is gone")
				_, held, err = s.Invocation("", annotated(siblingSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "and the run's own is recorded")
			})

			t.Run("records the annotators' invocations whatever a plan's commit", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), []string{failedPlan}, func(r *state.Recorder) {
					r.Lane("").Invoked(plugin.Invocation{Match: annotated(recordedSubject), Findings: reported})
				})
				_, held, err := g.Phases(t.Context()).Invocation("", annotated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "the shared phases advance whatever a plan's commit")
			})

			t.Run("keeps the annotator invocations of a warm run's generation", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Invoked(plugin.Invocation{Match: annotated(recordedSubject), Findings: reported})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.Lane("").Check(checkID, nil, nil)
				})
				_, held, err := g.Phases(t.Context()).Invocation("", annotated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "the commit keeps an invocation that no lane replaced")
			})

			t.Run("keeps no prior invocation of a plan that ran whole on a warm run", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{
						Match: generated(recordedSubject), Findings: reported,
					})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.Lane(recordedPlan)
				})
				_, held, err := g.Phases(t.Context()).Invocation(recordedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.False(t, held, "the lane of a plan that ran whole replaces the plan's prior invocations")
			})

			t.Run("keeps the prior invocations of a plan without a lane on a warm run", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{
						Match: generated(recordedSubject), Findings: reported,
					})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.Lane("").Check(checkID, nil, nil)
				})
				_, held, err := g.Phases(t.Context()).Invocation(recordedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "a plan that recorded nothing keeps its prior invocations")
			})

			t.Run("keeps the readers rows of the records that a warm run kept", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, readsOf(t, []symbol.Identity{siblingSubject}), nil)
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.Lane("").Check(checkID, nil, nil)
				})
				readers, err := g.Phases(t.Context()).Readers(state.DeclarationEdge(siblingSubject))
				assert.NoError(t, err, "the readers table reads")
				assert.Equal(t, readers, []state.RecordRef{state.ValidationRef(recordedSubject)},
					"the kept validation still lists its read of the sibling")
			})

			t.Run("replaces a validation that a warm run dropped with the lane's record", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, []diag.Diag{recordedDiag})
				})
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.DropValidation(recordedSubject)
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				got, err := g.Phases(t.Context()).Validations(state.ValidationRef(recordedSubject))
				assert.NoError(t, err, "the validations table reads")
				assert.Length(t, got, 1, "the row contains one record of the subject")
				assert.Empty(t, got[0].Findings, "the record is the lane's, which reported no finding")
			})

			t.Run("replaces an invocation that a warm run dropped with the lane's record", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Invoked(plugin.Invocation{Match: annotated(recordedSubject), Findings: reported})
				})
				fact := meta.FactRef{Subject: recordedSubject, Key: edgeKey}
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.DropInvocation("", annotated(recordedSubject))
					r.Lane("").Invoked(plugin.Invocation{
						Match: annotated(recordedSubject), Claimed: []meta.FactRef{fact},
					})
				})
				got, err := g.Phases(t.Context()).Invocations(state.InvocationRef("", annotated(recordedSubject)))
				assert.NoError(t, err, "the invocations table reads")
				assert.Length(t, got, 1, "the row contains one record of the match")
				assert.Empty(t, got[0].Findings, "the record is the lane's, which reported no finding")
			})

			t.Run("replaces a prior validations row that does not decode on a warm run", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				parent := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				c := state.NewCommit(parent, nil)
				c.Put(state.TableValidations, idKey(state.ValidationRef(recordedSubject).ID), []byte{1, 1, 0x80})
				_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
				assert.NoError(t, err, "the damaged row is committed")
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Keep()
					r.DropValidation(recordedSubject)
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				got, err := g.Phases(t.Context()).Validations(state.ValidationRef(recordedSubject))
				assert.NoError(t, err, "the subject's row decodes")
				assert.Length(t, got, 1, "the row contains the lane's record alone")
			})

			t.Run("keeps an undecodable prior validations row that a warm run does not touch", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				parent := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				c := state.NewCommit(parent, nil)
				c.Put(state.TableValidations, make([]byte, 8), []byte{1, 1, 0x80})
				_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
				assert.NoError(t, err, "the damaged row is committed")
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) { r.Keep() })
				rows, err := g.All(t.Context(), state.TableValidations)
				assert.NoError(t, err, "the validations table reads")
				assert.Length(t, rows, 2, "the commit reads neither row")
			})

			t.Run("writes the rows that a commit over every prior row writes", func(t *testing.T) {
				t.Parallel()

				touched, whole := warmPhases(t, false), warmPhases(t, true)
				for _, table := range []state.Table{
					state.TableModules, state.TableValidations, state.TableClaims, state.TablePresent,
					state.TableInvocations, state.TableReaders, state.TablePlans, state.TableGroups,
					state.TableArtifacts, state.TableNames, state.TableAudit, state.TableChecks,
				} {
					got, err := touched.All(t.Context(), table)
					assert.NoError(t, err, "the touched commit's table reads")
					want, err := whole.All(t.Context(), table)
					assert.NoError(t, err, "the whole commit's table reads")
					expect.Equal(t, got, want, "the "+table.String()+" table has the rows of the whole commit")
				}
			})
		})
	})
}

// The record of a run's phases allocates within its ceilings in the
// ordinary run, which runs no benchmark. The count of the records keeps
// their first error, which cmp.Or returns without allocating. The check
// runs alone, because the count includes every goroutine's allocations.
func TestPhasesAllocs(t *testing.T) {
	g := lookedUp(t)
	r := lookedUpRecorder(t)
	var s *state.PhaseState
	assert.MaxAllocs(t, func() { s = g.Phases(t.Context()) }, phasesAllocs, "Phases allocates the state")
	assert.NotNil(t, s, "Phases returns the state")
	run := state.PhaseRun{Facts: emptyFacts()}
	var (
		p   *state.PhaseRecord
		err error
	)
	assert.MaxAllocs(t, func() {
		var rerr error
		p, rerr = state.RecordPhases(t.Context(), g, r, run)
		err = cmp.Or(err, rerr)
	}, recordPhasesAllocs, "RecordPhases allocates the record and the prior tables it reads")
	assert.NoError(t, err, "the phases record")
	assert.NotNil(t, p, "RecordPhases returns the record")
	c := state.NewCommit(g, nil)
	assert.MaxAllocs(t, func() { p.Commit(c, nil) }, phaseCommitAllocs,
		"Commit allocates the sorted records and the rows")

	warm := lookedUpWarmRecorder(t)
	assert.MaxAllocs(t, func() {
		var rerr error
		p, rerr = state.RecordPhases(t.Context(), g, warm, run)
		err = cmp.Or(err, rerr)
	}, warmRecordPhasesAllocs, "RecordPhases allocates the record and the prior rows it looks up")
	assert.NoError(t, err, "the warm phases record")
	assert.MaxAllocs(t, func() { p.Commit(c, nil) }, warmPhaseCommitAllocs,
		"Commit allocates the sorted records and the merged rows")
}

// BenchmarkPhases measures a generation's record of the phases, and a
// run's record of the records lookedUp recorded again over the
// generation: its preparation, and its completion into a commit.
func BenchmarkPhases(b *testing.B) {
	g := lookedUp(b)
	r := lookedUpRecorder(b)
	run := state.PhaseRun{Facts: emptyFacts()}

	b.Run("Generation", func(b *testing.B) {
		b.Run("Phases", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(phasesAllocs)
			defer c.End()
			var got *state.PhaseState
			for c.Loop() {
				got = g.Phases(b.Context())
			}
			assert.NotNil(b, got, "Phases returns the state")
		})
	})

	records := []struct {
		name          string
		recorder      *state.Recorder
		record, write uint64
	}{
		{name: "over every prior row", recorder: r, record: recordPhasesAllocs, write: phaseCommitAllocs},
		{
			name: "over the rows a warm run touches", recorder: lookedUpWarmRecorder(b),
			record: warmRecordPhasesAllocs, write: warmPhaseCommitAllocs,
		},
	}

	b.Run("RecordPhases", func(b *testing.B) {
		for _, tt := range records {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.record)
				defer c.End()
				var (
					got *state.PhaseRecord
					err error
				)
				for c.Loop() {
					got, err = state.RecordPhases(b.Context(), g, tt.recorder, run)
				}
				assert.NoError(b, err, "the phases record")
				assert.NotNil(b, got, "RecordPhases returns the record")
			})
		}
	})

	b.Run("PhaseRecord", func(b *testing.B) {
		b.Run("Commit", func(b *testing.B) {
			for _, tt := range records {
				b.Run(tt.name, func(b *testing.B) {
					p, err := state.RecordPhases(b.Context(), g, tt.recorder, run)
					assert.NoError(b, err, "the phases record")
					commit := state.NewCommit(g, nil)
					c := bench.Start(b).MaxAllocs(tt.write)
					defer c.End()
					for c.Loop() {
						p.Commit(commit, nil)
					}
					_, err = commit.Write(b.Context(), ledger.NewMem(), header(past),
						manifest.Manifest{Version: manifest.Version})
					assert.NoError(b, err, "the commit writes")
				})
			}
		})
	})
}

// lookedUpRecorder returns a recorder of the records recordLookedUp
// records.
func lookedUpRecorder(tb testing.TB) *state.Recorder {
	tb.Helper()

	r := &state.Recorder{}
	recordLookedUp(tb, r)
	return r
}

// lookedUpWarmRecorder returns a warm run's recorder of the records that
// recordLookedUp records, which first drops the prior record of each of
// them, so the record of the phases looks their prior rows up by key.
func lookedUpWarmRecorder(tb testing.TB) *state.Recorder {
	tb.Helper()

	r := &state.Recorder{}
	r.Keep()
	r.KeepPlan(recordedPlan)
	r.DropValidation(recordedSubject)
	r.DropInvocation(recordedPlan, generated(recordedSubject))
	r.DropGroup(recordedPlan, hostUnit)
	recordLookedUp(tb, r)
	return r
}

// recordedPhases records the phases build collects into a commit over the
// ledger's live generation where it has one, with every plan committing
// but the failed ones, writes the commit, and returns the generation it
// made live.
func recordedPhases(
	tb testing.TB, l ledger.Ledger, facts *meta.Facts, failed []string, build func(*state.Recorder),
) *state.Generation {
	tb.Helper()

	parent, err := state.Open(tb.Context(), l)
	if err != nil {
		parent = nil
	}
	r := &state.Recorder{}
	build(r)
	p, err := state.RecordPhases(tb.Context(), parent, r, state.PhaseRun{Facts: facts})
	assert.NoError(tb, err, "the phases record")
	c := state.NewCommit(parent, nil)
	p.Commit(c, failed)
	_, err = c.Write(tb.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(tb, err, "the commit writes")
	g, err := state.Open(tb.Context(), l)
	assert.NoError(tb, err, "and its generation opens")
	return g
}

// warmPhases records a cold run's phases into a new ledger, then the
// phases of a warm run over them, and returns the generation that the warm
// run made live. The warm run changes each phase table:
//   - it records a validation, an invocation, a group and a file again
//   - it drops records and files, and withdraws a claim
//   - one of its plans does not commit, and keeps its records
//
// A whole warm run adds the empty lane of the replacing plan, so its commit
// reads every prior row.
func warmPhases(tb testing.TB, whole bool) *state.Generation {
	tb.Helper()

	l := ledger.NewMem()
	facts, keys := stampedFacts(tb)
	recordedPhases(tb, l, facts, nil, func(r *state.Recorder) {
		shared := r.Lane("")
		shared.Validation(recordedSubject, nil, readsOf(tb, []symbol.Identity{siblingSubject}), nil)
		shared.Validation(siblingSubject, nil, nil, reported)
		shared.Invoked(plugin.Invocation{Match: annotated(recordedSubject), Findings: reported})
		shared.Invoked(plugin.Invocation{Match: annotated(siblingSubject), Findings: reported})
		shared.Check(checkID, readsOf(tb, nil, meta.FactRef{Subject: recordedSubject, Key: edgeKey}), nil)
		plan := r.Lane(recordedPlan)
		plan.Invoked(plugin.Invocation{Match: generated(recordedSubject), Findings: reported})
		plan.Group(recordedGroup(hostUnit))
		plan.Artifact(recordedArtifact(artifactPath, recordedSubject))
		failed := r.Lane(failedPlan)
		failed.Invoked(plugin.Invocation{Match: generated(recordedSubject), Findings: reported})
		failed.Group(recordedGroup(edgeUnit))
	})
	parent, err := state.Open(tb.Context(), l)
	assert.NoError(tb, err, "the cold run's generation opens")
	run := meta.Restore(facts.Registry(), parent.Phases(tb.Context()))
	counted := claimBy("alpha", 1)
	counted.Subject = siblingSubject
	assert.NoError(tb, meta.Stamp(run, keys.count, int64(3), counted), "the sibling's count is stamped")
	assert.NoError(tb, run.Withdraw(keys.flag.ID(), claimBy("alpha", 1)), "the subject's flag is withdrawn")
	return recordedPhases(tb, l, run, []string{failedPlan}, func(r *state.Recorder) {
		r.Keep()
		r.KeepPlan(recordedPlan)
		r.KeepPlan(failedPlan)
		r.Withdrew(recordedSubject)
		r.DropValidation(recordedSubject)
		r.DropInvocation("", annotated(siblingSubject))
		r.DropInvocation(recordedPlan, generated(recordedSubject))
		r.DropGroup(recordedPlan, hostUnit)
		r.DropGroup(failedPlan, edgeUnit)
		r.DropFile(artifactPath)
		r.DropFile(siblingPath)
		shared := r.Lane("")
		shared.Validation(recordedSubject, nil, nil, reported)
		shared.Check(checkID, nil, nil)
		plan := r.Lane(recordedPlan)
		plan.Invoked(plugin.Invocation{Match: generated(siblingSubject), Findings: reported})
		plan.Group(recordedGroup(edgeUnit))
		plan.Artifact(recordedArtifact(siblingPath, siblingSubject))
		r.Lane(failedPlan).Invoked(plugin.Invocation{Match: generated(siblingSubject), Findings: reported})
		if whole {
			r.Lane(replacingPlan)
		}
	})
}

// putRows commits rows into one table of a new ledger and returns the
// generation the commit made live.
func putRows(tb testing.TB, t state.Table, rows ...state.Row) *state.Generation {
	tb.Helper()

	l := ledger.NewMem()
	c := state.NewCommit(nil, nil)
	for _, r := range rows {
		c.Put(t, r.Key, r.Value)
	}
	_, err := c.Write(tb.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(tb, err, "the commit writes")
	g, err := state.Open(tb.Context(), l)
	assert.NoError(tb, err, "and its generation opens")
	return g
}

// readsOf returns a read set that read each identity as a declaration
// and each fact, through a reader over an empty frozen graph: a read that
// found nothing records its edge too.
func readsOf(tb assert.TB, ids []symbol.Identity, facts ...meta.FactRef) *store.ReadSet {
	tb.Helper()

	set := store.NewReadSet()
	g := store.New()
	g.Freeze()
	r, err := g.Reader(set, nil)
	assert.NoError(tb, err, "the reader opens")
	for _, id := range ids {
		r.Lookup(id)
	}
	for _, f := range facts {
		set.RecordFact(f.Subject, f.Key)
	}
	return set
}

// annotated returns the match of the annotator's struct rule on a
// subject.
func annotated(subject symbol.Identity) plugin.MatchKey {
	return plugin.MatchKey{Plugin: annotatorID, Rule: structRule, Subject: subject}
}

// generated returns the match of the generator's struct rule on a
// subject.
func generated(subject symbol.Identity) plugin.MatchKey {
	return plugin.MatchKey{Plugin: generatorID, Rule: structRule, Subject: subject}
}

// sortedRefs returns the references in the order a readers row lists
// them.
func sortedRefs(refs ...state.RecordRef) []state.RecordRef {
	slices.SortFunc(refs, state.RecordRef.Compare)
	return refs
}

// emptyFacts returns a fact store over a registry with no key.
func emptyFacts() *meta.Facts {
	return meta.NewFacts(meta.NewRegistry())
}

// siblingCounted returns a fact store over the registry of facts with one
// claim, the count of the sibling. The store models a warm run that
// touched only the sibling's bag.
func siblingCounted(tb testing.TB, facts *meta.Facts, keys phaseKeys) *meta.Facts {
	tb.Helper()

	run := meta.NewFacts(facts.Registry())
	claim := claimBy("alpha", 1)
	claim.Subject = siblingSubject
	assert.NoError(tb, meta.Stamp(run, keys.count, int64(3), claim), "the sibling's count is stamped")
	return run
}

// phaseRegistry returns a registry of the recorded keys and the keys.
func phaseRegistry(tb assert.TB) (*meta.Registry, phaseKeys) {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace("shape"), "the namespace is claimed")
	var keys phaseKeys
	var err error
	keys.role, err = meta.Register[string](r, meta.KeySpec{Name: "shape.role", Group: "shape.writer", Doc: "a role"})
	assert.NoError(tb, err, "the role key registers")
	keys.count, err = meta.Register[int64](r, meta.KeySpec{Name: "shape.count", Doc: "a count"})
	assert.NoError(tb, err, "the count key registers")
	keys.flag, err = meta.Register[bool](r, meta.KeySpec{Name: "shape.flag", Doc: "a flag"})
	assert.NoError(tb, err, "the flag key registers")
	keys.tags, err = meta.Register[[]string](r, meta.KeySpec{Name: "shape.tags", Doc: "the tags"})
	assert.NoError(tb, err, "the tags key registers")
	keys.target, err = meta.Register[symbol.Identity](r, meta.KeySpec{Name: "shape.target", Doc: "a target"})
	assert.NoError(tb, err, "the target key registers")
	return r, keys
}

// claimBy returns a plugin's claim on the recorded subject at one
// instance, at the carrier of the recorded finding.
func claimBy(p diag.Origin, instance int) meta.Claim {
	return meta.Claim{
		Subject: recordedSubject,
		Plugin:  p,
		Order:   meta.Order{Rule: int(structRule), Subject: recordedSubject, Instance: instance},
		Pos:     position.Pos{File: "svc/api/user.zz", Line: 3, Col: 1},
	}
}
