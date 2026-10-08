// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"cmp"
	"encoding/binary"
	"math"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The tag each record's ID spelling opens with, pinned: a byte for each
// kind.
const (
	validationTag = 'v'
	invocationTag = 'i'
	checkTag      = 'c'
	groupTag      = 'g'
)

// The ceilings of a lookup over the generation lookedUp records, each
// after a lookup before it. The key of a lookup is on the stack, and the
// run reader keeps the block that it decoded last, so a lookup reads
// nothing from the ledger. A record's lookup allocates the decoder that
// its match reads the entry through. A string of one byte, such as the
// subject's discriminator, allocates nothing.
const (
	// validationAllocs is one validation's lookup: the decoder, the
	// subject's four longer strings, and the list of reads.
	validationAllocs = 6
	// invocationAllocs is one invocation's lookup: the decoder, the plan,
	// the plugin, the subject's four longer strings, and the list of
	// reads.
	invocationAllocs = 8
	// checkAllocs is one check's lookup: the decoder, the check's name,
	// and the list of reads.
	checkAllocs = 3
	// validationsAllocs is the ceiling of Validations over one row. It
	// adds the list of records to the allocations of one validation's
	// lookup.
	validationsAllocs = 7
	// invocationsAllocs is the ceiling of Invocations over one row. It
	// adds the list of records to the allocations of one invocation's
	// lookup.
	invocationsAllocs = 9
	// readersAllocs is one edge's readers: the list of records.
	readersAllocs = 1
	// groupAllocs is one group's lookup: the decoder, the plan, the four
	// lists of units, files, contributors and reads, the strings of the
	// key unit and of the listed unit, the file's path, and the plugin and
	// the subject strings of the contributor's match. A unit decodes five
	// longer strings, and a subject four.
	groupAllocs = 22
	// groupsAllocs is the ceiling of Groups over one row. It adds the list
	// of records to the allocations of one group's lookup.
	groupsAllocs = 23
)

// The record cases' carrier and finding, and a host unit of the plan.
var (
	recordedPos  = position.Pos{File: "svc/api/user.zz", Line: 2, Col: 4}
	recordedDiag = diag.Diag{
		Code:     diag.Code{Prefix: "EID", Number: 62},
		Severity: diag.SeverityWarning,
		Pos:      recordedPos,
		Msg:      recordedFinding,
		Origin:   generatorID,
		Related:  []position.Pos{{File: "svc/api/other.zz", Line: 1, Col: 1}},
	}
	hostUnit = plugin.UnitRef{Plugin: generatorID, Tag: "gen", Pkg: edgePackage, Key: "svc/api"}
)

// Each record reads back whole under the reference of its key fields,
// beside a record that shares its row, and each edge lists the records
// that read it.
func TestRecords(t *testing.T) {
	t.Parallel()

	t.Run("ValidationRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the validation tag and the subject", func(t *testing.T) {
			t.Parallel()

			want := uint64(pinnedHash(spelledIdentity([]byte{validationTag}, recordedSubject)))
			assert.Equal(t, state.ValidationRef(recordedSubject),
				state.RecordRef{Kind: state.RecordValidation, ID: want}, "a validation's reference")
		})
	})

	t.Run("InvocationRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the invocation tag, the plan and the match", func(t *testing.T) {
			t.Parallel()

			m := plugin.MatchKey{
				Plugin: generatorID, Rule: structRule, Subject: recordedSubject, Instance: 3,
				Host: plugin.EmitRef{Unit: hostUnit, Index: 5},
			}
			spelling := spelledText([]byte{invocationTag}, recordedPlan)
			spelling = spelledText(spelling, string(m.Plugin))
			spelling = binary.AppendVarint(spelling, int64(m.Rule))
			spelling = spelledIdentity(spelling, m.Subject)
			spelling = binary.AppendVarint(spelling, int64(m.Instance))
			spelling = spelledText(spelling, string(hostUnit.Plugin))
			spelling = spelledText(spelling, hostUnit.Tag)
			spelling = spelledIdentity(spelling, hostUnit.Pkg)
			spelling = spelledText(spelling, hostUnit.Key)
			spelling = binary.AppendVarint(spelling, int64(m.Host.Index))
			assert.Equal(t, state.InvocationRef(recordedPlan, m),
				state.RecordRef{Kind: state.RecordInvocation, ID: uint64(pinnedHash(spelling))},
				"an invocation's reference")
		})

		t.Run("returns another reference for another plan", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, state.InvocationRef(failedPlan, generated(recordedSubject)),
				state.InvocationRef(recordedPlan, generated(recordedSubject)), "the plan is a key field")
		})
	})

	t.Run("CheckRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the check tag and the name", func(t *testing.T) {
			t.Parallel()

			want := uint64(pinnedHash(spelledText([]byte{checkTag}, string(checkID))))
			assert.Equal(t, state.CheckRef(checkID), state.RecordRef{Kind: state.RecordCheck, ID: want},
				"a check's reference")
		})
	})

	t.Run("GroupRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the tagged plan with the key unit", func(t *testing.T) {
			t.Parallel()

			spelling := spelledText([]byte{groupTag}, recordedPlan)
			spelling = spelledText(spelledText(spelling, string(hostUnit.Plugin)), hostUnit.Tag)
			spelling = spelledText(spelledIdentity(spelling, hostUnit.Pkg), hostUnit.Key)
			assert.Equal(t, state.GroupRef(recordedPlan, hostUnit),
				state.RecordRef{Kind: state.RecordGroup, ID: uint64(pinnedHash(spelling))}, "a group's reference")
		})

		t.Run("returns another reference for another plan", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, state.GroupRef(failedPlan, hostUnit), state.GroupRef(recordedPlan, hostUnit),
				"the plan is a key field")
		})
	})

	t.Run("RecordRef", func(t *testing.T) {
		t.Parallel()

		t.Run("Compare", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				a, b state.RecordRef
				want int
			}{
				{
					name: "returns a negative number for an earlier kind",
					a:    state.RecordRef{Kind: state.RecordValidation, ID: 9},
					b:    state.RecordRef{Kind: state.RecordCheck, ID: 1}, want: -1,
				},
				{
					name: "returns a positive number for a larger ID of one kind",
					a:    state.RecordRef{Kind: state.RecordInvocation, ID: 9},
					b:    state.RecordRef{Kind: state.RecordInvocation, ID: 1}, want: 1,
				},
				{
					name: "returns zero for one reference",
					a:    state.RecordRef{Kind: state.RecordCheck, ID: 4},
					b:    state.RecordRef{Kind: state.RecordCheck, ID: 4}, want: 0,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.a.Compare(tt.b), tt.want, "kind first, then ID")
				})
			}
		})
	})

	t.Run("PhaseState", func(t *testing.T) {
		t.Parallel()

		t.Run("Validation", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of a validated subject", func(t *testing.T) {
				t.Parallel()

				ds := validatedDirectives()
				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, ds, readsOf(t, []symbol.Identity{siblingSubject}),
						[]diag.Diag{recordedDiag})
				})
				got, held, err := g.Phases(t.Context()).Validation(recordedSubject)
				assert.NoError(t, err, "the validations table reads")
				assert.True(t, held, "the subject's validation is recorded")
				assert.Equal(t, got, state.Validation{
					Subject:    recordedSubject,
					Directives: ds,
					Reads: sortedEdges(
						state.DeclarationEdge(siblingSubject),
						state.DeclarationEdge(recordedSubject),
						state.FindingsEdge,
					),
					Findings: []diag.Diag{recordedDiag},
				}, "the record contains the directives, the reads, the findings edge and the finding")
			})

			t.Run("returns false for a subject nothing validated", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				_, held, err := g.Phases(t.Context()).Validation(siblingSubject)
				assert.NoError(t, err, "the validations table reads")
				assert.False(t, held, "the sibling validated nothing")
			})

			t.Run("returns the record of a subject whose row another record's entry shares", func(t *testing.T) {
				t.Parallel()

				shared := sharedRow(t, state.TableValidations, state.ValidationRef(recordedSubject).ID,
					func(r *state.Recorder) { r.Lane("").Validation(siblingSubject, nil, nil, nil) },
					func(r *state.Recorder) { r.Lane("").Validation(recordedSubject, nil, nil, nil) })
				s := shared.Phases(t.Context())
				got, held, err := s.Validation(recordedSubject)
				assert.NoError(t, err, "the shared row reads")
				assert.True(t, held, "the subject has an entry")
				assert.Equal(t, got.Subject, recordedSubject, "the subject's own entry is found")
				_, held, err = s.Validation(siblingSubject)
				assert.NoError(t, err, "the sibling's own row reads")
				assert.False(t, held, "and the sibling's entry is found only under its own ID")
			})

			t.Run("returns false for a subject whose row lists another record's entry alone", func(t *testing.T) {
				t.Parallel()

				shared := sharedRow(t, state.TableValidations, state.ValidationRef(recordedSubject).ID,
					func(r *state.Recorder) { r.Lane("").Validation(siblingSubject, nil, nil, nil) },
					func(r *state.Recorder) { r.Lane("").Validation(siblingSubject, nil, nil, nil) })
				_, held, err := shared.Phases(t.Context()).Validation(recordedSubject)
				assert.NoError(t, err, "the row reads")
				assert.False(t, held, "neither entry is the subject's")
			})

			t.Run("returns ErrDamaged for an entry that does not decode", func(t *testing.T) {
				t.Parallel()

				g := putRows(t, state.TableValidations, state.Row{
					Key: idKey(state.ValidationRef(recordedSubject).ID), Value: []byte{1, 1, 0x80},
				})
				_, _, err := g.Phases(t.Context()).Validation(recordedSubject)
				assert.ErrorIs(t, err, state.ErrDamaged, "the entry's subject is cut short")
			})

			t.Run("returns ErrDamaged for a read record that ends inside a hash", func(t *testing.T) {
				t.Parallel()

				entry := spelledIdentity(nil, recordedSubject)
				entry = append(entry, 0, 1, 0xa, 0xb, 0xc)
				row := append(binary.AppendUvarint([]byte{1}, uint64(len(entry))), entry...)
				g := putRows(t, state.TableValidations, state.Row{
					Key: idKey(state.ValidationRef(recordedSubject).ID), Value: row,
				})
				_, _, err := g.Phases(t.Context()).Validation(recordedSubject)
				assert.ErrorIs(t, err, state.ErrDamaged, "one hash takes eight bytes, and three follow")
			})

			t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
				t.Parallel()

				g := putRows(t, state.TableValidations, state.Row{
					Key: idKey(state.ValidationRef(recordedSubject).ID), Value: []byte{2},
				})
				_, _, err := g.Phases(t.Context()).Validation(recordedSubject)
				assert.ErrorIs(t, err, state.ErrDamaged, "the row counts entries it lacks")
			})

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				damageRun(t, l, func(int) int { return 3 })
				_, _, err := g.Phases(t.Context()).Validation(recordedSubject)
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged block is found")
			})
		})

		t.Run("Invocation", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of an invocation with what it touched and reported", func(t *testing.T) {
				t.Parallel()

				m := plugin.MatchKey{
					Plugin: generatorID, Rule: structRule, Subject: recordedSubject, Instance: 1,
					Host: plugin.EmitRef{Unit: hostUnit, Index: 2},
				}
				fact := meta.FactRef{Subject: recordedSubject, Key: edgeKey}
				inv := plugin.Invocation{
					Match:    m,
					Reads:    readsOf(t, nil, fact),
					Exports:  []string{failedPlan},
					Units:    []plugin.UnitRef{hostUnit},
					Hosts:    []plugin.EmitRef{{Unit: hostUnit, Index: 7}},
					Claimed:  []meta.FactRef{fact},
					Findings: []diag.Diag{recordedDiag},
				}
				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(inv)
				})
				got, held, err := g.Phases(t.Context()).Invocation(recordedPlan, m)
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "the invocation is recorded")
				reads := sortedEdges(
					state.FactEdge(recordedSubject, edgeKey),
					state.DeclarationEdge(recordedSubject),
					state.ExportEdge(failedPlan),
					state.FindingsEdge,
				)
				assert.Equal(t, got, state.Invocation{
					Plan:     recordedPlan,
					Match:    m,
					Reads:    reads,
					Exports:  inv.Exports,
					Units:    inv.Units,
					Hosts:    inv.Hosts,
					Claimed:  inv.Claimed,
					Findings: inv.Findings,
				}, "the record adds the edges of the subject, of the export and of the findings to the invocation")
			})

			t.Run("returns the record of a whole call without a subject's edge", func(t *testing.T) {
				t.Parallel()

				whole := plugin.MatchKey{Plugin: generatorID, Rule: plugin.WholeCall}
				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{Match: whole})
				})
				got, held, err := g.Phases(t.Context()).Invocation(recordedPlan, whole)
				assert.NoError(t, err, "the invocations table reads")
				assert.True(t, held, "the whole call is recorded")
				assert.Empty(t, got.Reads, "and reads no edge, because it has no subject")
			})

			t.Run("returns false for a match of another plan", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{
						Match: generated(recordedSubject), Findings: reported,
					})
				})
				_, held, err := g.Phases(t.Context()).Invocation(failedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the invocations table reads")
				assert.False(t, held, "the other plan ran no such invocation")
			})

			t.Run("returns the record of an invocation whose row another record's entry shares", func(t *testing.T) {
				t.Parallel()

				ref := state.InvocationRef(recordedPlan, generated(recordedSubject))
				shared := sharedRow(t, state.TableInvocations, ref.ID,
					func(r *state.Recorder) {
						r.Lane(recordedPlan).Invoked(plugin.Invocation{
							Match: generated(siblingSubject), Findings: reported,
						})
					},
					func(r *state.Recorder) {
						r.Lane(recordedPlan).Invoked(plugin.Invocation{
							Match: generated(recordedSubject), Findings: reported,
						})
					})
				got, held, err := shared.Phases(t.Context()).Invocation(recordedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the shared row reads")
				assert.True(t, held, "the invocation has an entry")
				assert.Equal(t, got.Match.Subject, recordedSubject, "the invocation's own entry is found")
			})
		})

		t.Run("Check", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of a check's call", func(t *testing.T) {
				t.Parallel()

				fact := meta.FactRef{Subject: siblingSubject, Key: edgeKey}
				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, readsOf(t, nil, fact), []diag.Diag{recordedDiag})
				})
				got, held, err := g.Phases(t.Context()).Check(checkID)
				assert.NoError(t, err, "the checks table reads")
				assert.True(t, held, "the check is recorded")
				assert.Equal(t, got, state.Check{
					Name:     checkID,
					Reads:    sortedEdges(state.FactEdge(siblingSubject, edgeKey), state.FindingsEdge),
					Findings: []diag.Diag{recordedDiag},
				}, "the record contains the reader's reads, the findings edge and the finding")
			})

			t.Run("returns false for a check nothing called", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, nil, nil)
				})
				_, held, err := g.Phases(t.Context()).Check("other")
				assert.NoError(t, err, "the checks table reads")
				assert.False(t, held, "no other check was called")
			})

			t.Run("returns the record of a check whose row another check's entry shares", func(t *testing.T) {
				t.Parallel()

				shared := sharedRow(t, state.TableChecks, state.CheckRef(checkID).ID,
					func(r *state.Recorder) { r.Lane("").Check("other", nil, nil) },
					func(r *state.Recorder) { r.Lane("").Check(checkID, nil, nil) })
				got, held, err := shared.Phases(t.Context()).Check(checkID)
				assert.NoError(t, err, "the shared row reads")
				assert.True(t, held, "the check has an entry")
				assert.Equal(t, got.Name, checkID, "the check's own entry is found")
			})
		})

		t.Run("Group", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of a group", func(t *testing.T) {
				t.Parallel()

				want := recordedGroup(hostUnit)
				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Group(want)
				})
				got, held, err := g.Phases(t.Context()).Group(recordedPlan, hostUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.True(t, held, "the group is recorded")
				want.Plan, want.Reads = recordedPlan, sortedEdges(want.Reads...)
				assert.Equal(t, got, want, "the record contains the units, files, contributors and reads")
			})

			t.Run("returns false for a group of another plan", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Group(recordedGroup(hostUnit))
				})
				_, held, err := g.Phases(t.Context()).Group(failedPlan, hostUnit)
				assert.NoError(t, err, "the groups table reads")
				assert.False(t, held, "the other plan recorded no group")
			})

			t.Run("returns ErrDamaged for an entry that does not decode", func(t *testing.T) {
				t.Parallel()

				g := putRows(t, state.TableGroups, state.Row{
					Key: idKey(state.GroupRef(recordedPlan, hostUnit).ID), Value: []byte{1, 1, 0x80},
				})
				_, _, err := g.Phases(t.Context()).Group(recordedPlan, hostUnit)
				assert.ErrorIs(t, err, state.ErrDamaged, "the entry's plan is cut short")
			})
		})

		t.Run("Groups", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of the reference's group", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Group(recordedGroup(hostUnit))
				})
				got, err := g.Phases(t.Context()).Groups(state.GroupRef(recordedPlan, hostUnit))
				assert.NoError(t, err, "the groups table reads")
				assert.Length(t, got, 1, "the row lists the group")
				assert.Equal(t, got[0].Key, hostUnit, "the group of the reference's key")
			})

			t.Run("returns nothing for a reference that the table does not contain", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Group(recordedGroup(hostUnit))
				})
				got, err := g.Phases(t.Context()).Groups(state.GroupRef(failedPlan, hostUnit))
				assert.NoError(t, err, "the groups table reads")
				assert.Empty(t, got, "the other plan recorded no group")
			})
		})

		t.Run("Validations", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of the reference's subject", func(t *testing.T) {
				t.Parallel()

				ds := validatedDirectives()
				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, ds, nil, []diag.Diag{recordedDiag})
				})
				got, err := g.Phases(t.Context()).Validations(state.ValidationRef(recordedSubject))
				assert.NoError(t, err, "the validations table reads")
				assert.Equal(t, got, []state.Validation{{
					Subject:    recordedSubject,
					Directives: ds,
					Reads:      sortedEdges(state.DeclarationEdge(recordedSubject), state.FindingsEdge),
					Findings:   []diag.Diag{recordedDiag},
				}}, "Validations returns the whole record")
			})

			t.Run("returns every entry of a shared row in row order", func(t *testing.T) {
				t.Parallel()

				ref := state.ValidationRef(recordedSubject)
				shared := sharedRow(t, state.TableValidations, ref.ID,
					func(r *state.Recorder) { r.Lane("").Validation(siblingSubject, nil, nil, nil) },
					func(r *state.Recorder) { r.Lane("").Validation(recordedSubject, nil, nil, nil) })
				got, err := shared.Phases(t.Context()).Validations(ref)
				assert.NoError(t, err, "the shared row reads")
				assert.Length(t, got, 2, "the row lists both entries")
				expect.Equal(t, got[0].Subject, siblingSubject, "the sibling's entry comes first")
				expect.Equal(t, got[1].Subject, recordedSubject, "the subject's entry comes second")
			})

			t.Run("returns nothing for a reference that the table does not contain", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				got, err := g.Phases(t.Context()).Validations(state.ValidationRef(siblingSubject))
				assert.NoError(t, err, "the validations table reads")
				assert.Empty(t, got, "the sibling validated nothing")
			})

			t.Run("returns ErrDamaged for an entry that does not decode", func(t *testing.T) {
				t.Parallel()

				ref := state.ValidationRef(recordedSubject)
				g := putRows(t, state.TableValidations, state.Row{Key: idKey(ref.ID), Value: []byte{1, 1, 0x80}})
				_, err := g.Phases(t.Context()).Validations(ref)
				assert.ErrorIs(t, err, state.ErrDamaged, "the entry's subject is cut short")
			})

			t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
				t.Parallel()

				ref := state.ValidationRef(recordedSubject)
				g := putRows(t, state.TableValidations, state.Row{Key: idKey(ref.ID), Value: []byte{2}})
				_, err := g.Phases(t.Context()).Validations(ref)
				assert.ErrorIs(t, err, state.ErrDamaged, "the row counts more entries than it contains")
			})

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Validation(recordedSubject, nil, nil, nil)
				})
				damageRun(t, l, func(int) int { return 3 })
				_, err := g.Phases(t.Context()).Validations(state.ValidationRef(recordedSubject))
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged block is found")
			})
		})

		t.Run("Invocations", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of the reference's match", func(t *testing.T) {
				t.Parallel()

				fact := meta.FactRef{Subject: recordedSubject, Key: edgeKey}
				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Invoked(plugin.Invocation{
						Match: annotated(recordedSubject), Claimed: []meta.FactRef{fact}, Findings: reported,
					})
				})
				got, err := g.Phases(t.Context()).Invocations(state.InvocationRef("", annotated(recordedSubject)))
				assert.NoError(t, err, "the invocations table reads")
				assert.Equal(t, got, []state.Invocation{{
					Match:    annotated(recordedSubject),
					Reads:    sortedEdges(state.DeclarationEdge(recordedSubject), state.FindingsEdge),
					Claimed:  []meta.FactRef{fact},
					Findings: reported,
				}}, "Invocations returns the whole record")
			})

			t.Run("returns every entry of a shared row in row order", func(t *testing.T) {
				t.Parallel()

				ref := state.InvocationRef(recordedPlan, generated(recordedSubject))
				shared := sharedRow(t, state.TableInvocations, ref.ID,
					func(r *state.Recorder) {
						r.Lane(recordedPlan).Invoked(plugin.Invocation{
							Match: generated(siblingSubject), Findings: reported,
						})
					},
					func(r *state.Recorder) {
						r.Lane(recordedPlan).Invoked(plugin.Invocation{
							Match: generated(recordedSubject), Findings: reported,
						})
					})
				got, err := shared.Phases(t.Context()).Invocations(ref)
				assert.NoError(t, err, "the shared row reads")
				assert.Length(t, got, 2, "the row lists both entries")
				expect.Equal(t, got[0].Match, generated(siblingSubject), "the sibling's entry comes first")
				expect.Equal(t, got[1].Match, generated(recordedSubject), "the subject's entry comes second")
				expect.Equal(t, got[1].Plan, recordedPlan, "the subject's entry names the plan")
			})

			t.Run("returns nothing for a reference that the table does not contain", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Invoked(plugin.Invocation{
						Match: generated(recordedSubject), Findings: reported,
					})
				})
				ref := state.InvocationRef(failedPlan, generated(recordedSubject))
				got, err := g.Phases(t.Context()).Invocations(ref)
				assert.NoError(t, err, "the invocations table reads")
				assert.Empty(t, got, "the other plan ran no such invocation")
			})

			t.Run("returns ErrDamaged for an entry that does not decode", func(t *testing.T) {
				t.Parallel()

				ref := state.InvocationRef(recordedPlan, generated(recordedSubject))
				g := putRows(t, state.TableInvocations, state.Row{Key: idKey(ref.ID), Value: []byte{1, 1, 0x80}})
				_, err := g.Phases(t.Context()).Invocations(ref)
				assert.ErrorIs(t, err, state.ErrDamaged, "the entry's plan is cut short")
			})
		})

		t.Run("Readers", func(t *testing.T) {
			t.Parallel()

			t.Run("returns nothing for an edge nothing read", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, nil, nil)
				})
				refs, err := g.Phases(t.Context()).Readers(state.KindEdge(symbol.KindStruct))
				assert.NoError(t, err, "the readers table reads")
				assert.Empty(t, refs, "no record enumerated the kind")
			})

			t.Run("returns every record that enumerated a kind or a directive or took a package", func(t *testing.T) {
				t.Parallel()

				set := store.NewReadSet()
				g := store.New()
				g.Freeze()
				reader, err := g.Reader(set, nil)
				assert.NoError(t, err, "the reader opens")
				for range reader.ByKind(symbol.KindStruct) {
				}
				for range reader.ByDirective(edgeDirective) {
				}
				reader.PackageOf(recordedSubject)
				recorded := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, set, nil)
				})
				s := recorded.Phases(t.Context())
				for _, h := range []state.EdgeHash{
					state.KindEdge(symbol.KindStruct), state.DirectiveEdge(edgeDirective),
					state.PackageEdge(recordedSubject.PackageIdentity()),
				} {
					refs, err := s.Readers(h)
					assert.NoError(t, err, "the readers table reads")
					assert.Equal(t, refs, []state.RecordRef{state.CheckRef(checkID)}, "the check read the edge")
				}
			})

			t.Run("returns every record that reported a finding for the findings edge", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					shared := r.Lane("")
					shared.Validation(recordedSubject, nil, nil, []diag.Diag{recordedDiag})
					shared.Validation(siblingSubject, nil, nil, nil)
					shared.Invoked(plugin.Invocation{Match: annotated(siblingSubject), Findings: reported})
					shared.Check(checkID, nil, []diag.Diag{recordedDiag})
				})
				refs, err := g.Phases(t.Context()).Readers(state.FindingsEdge)
				assert.NoError(t, err, "the readers table reads")
				assert.Equal(t, refs, sortedRefs(
					state.ValidationRef(recordedSubject),
					state.InvocationRef("", annotated(siblingSubject)),
					state.CheckRef(checkID),
				), "the readers are the three records that reported a finding")
			})

			t.Run("returns ErrDamaged for a row whose length does not fit its count", func(t *testing.T) {
				t.Parallel()

				g := putRows(t, state.TableReaders, state.Row{
					Key:   edgeKeyOf(state.KindEdge(symbol.KindStruct)),
					Value: []byte{1, byte(state.RecordCheck)},
				})
				_, err := g.Phases(t.Context()).Readers(state.KindEdge(symbol.KindStruct))
				assert.ErrorIs(t, err, state.ErrDamaged, "one reference takes nine bytes")
			})

			t.Run("returns ErrDamaged for a row that names no record kind", func(t *testing.T) {
				t.Parallel()

				row := append([]byte{1, 9}, make([]byte, 8)...)
				g := putRows(t, state.TableReaders, state.Row{
					Key:   edgeKeyOf(state.KindEdge(symbol.KindStruct)),
					Value: row,
				})
				_, err := g.Phases(t.Context()).Readers(state.KindEdge(symbol.KindStruct))
				assert.ErrorIs(t, err, state.ErrDamaged, "kind 9 is no record kind")
			})

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				g := recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane("").Check(checkID, readsOf(t, []symbol.Identity{recordedSubject}), nil)
				})
				damageRun(t, l, func(int) int { return 3 })
				_, err := g.Phases(t.Context()).Readers(state.DeclarationEdge(recordedSubject))
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged block is found")
			})
		})
	})
}

// A reference and its comparison allocate nothing, and a record's lookups
// allocate within their ceilings, in the ordinary run, which runs no
// benchmark. Each count of a lookup keeps the first error of its calls,
// which cmp.Or returns without allocating. The check runs alone, because
// the count includes every goroutine's allocations.
func TestRecordsAllocs(t *testing.T) {
	var got state.RecordRef
	assert.MaxAllocs(t, func() { got = state.ValidationRef(recordedSubject) }, 0,
		"ValidationRef allocates nothing")
	assert.MaxAllocs(t, func() { got = state.InvocationRef(recordedPlan, generated(recordedSubject)) }, 0,
		"InvocationRef allocates nothing")
	assert.MaxAllocs(t, func() { got = state.GroupRef(recordedPlan, hostUnit) }, 0, "GroupRef allocates nothing")
	assert.MaxAllocs(t, func() { got = state.CheckRef(checkID) }, 0, "CheckRef allocates nothing")
	assert.Equal(t, got.Kind, state.RecordCheck, "the last reference is a check's")
	other := state.ValidationRef(recordedSubject)
	order := 0
	assert.MaxAllocs(t, func() { order = got.Compare(other) }, 0, "Compare allocates nothing")
	assert.InRange(t, order, 1, math.Inf(1), "a check sorts after a validation")

	s := lookedUp(t).Phases(t.Context())
	var (
		held bool
		err  error
	)
	assert.MaxAllocs(t, func() {
		var verr error
		_, held, verr = s.Validation(recordedSubject)
		err = cmp.Or(err, verr)
	}, validationAllocs, "Validation allocates the decoder and the decoded record")
	assert.NoError(t, err, "the validation reads")
	assert.True(t, held, "the subject's validation is recorded")
	match := generated(recordedSubject)
	assert.MaxAllocs(t, func() {
		var ierr error
		_, held, ierr = s.Invocation(recordedPlan, match)
		err = cmp.Or(err, ierr)
	}, invocationAllocs, "Invocation allocates the decoder and the decoded record")
	assert.NoError(t, err, "the invocation reads")
	assert.True(t, held, "the invocation is recorded")
	assert.MaxAllocs(t, func() {
		var cerr error
		_, held, cerr = s.Check(checkID)
		err = cmp.Or(err, cerr)
	}, checkAllocs, "Check allocates the decoder and the decoded record")
	assert.NoError(t, err, "the check reads")
	assert.True(t, held, "the check is recorded")
	validated := state.ValidationRef(recordedSubject)
	var validations []state.Validation
	assert.MaxAllocs(t, func() {
		var verr error
		validations, verr = s.Validations(validated)
		err = cmp.Or(err, verr)
	}, validationsAllocs, "Validations allocates the decoder and the decoded records")
	assert.NoError(t, err, "the validations table reads")
	assert.Length(t, validations, 1, "the row lists the subject's validation")
	invoked := state.InvocationRef(recordedPlan, match)
	var invocations []state.Invocation
	assert.MaxAllocs(t, func() {
		var ierr error
		invocations, ierr = s.Invocations(invoked)
		err = cmp.Or(err, ierr)
	}, invocationsAllocs, "Invocations allocates the decoder and the decoded records")
	assert.NoError(t, err, "the invocations table reads")
	assert.Length(t, invocations, 1, "the row lists the plan's invocation")
	edge := state.DeclarationEdge(recordedSubject)
	var readers []state.RecordRef
	assert.MaxAllocs(t, func() {
		var rerr error
		readers, rerr = s.Readers(edge)
		err = cmp.Or(err, rerr)
	}, readersAllocs, "Readers allocates the list of records")
	assert.NoError(t, err, "the readers read")
	assert.Length(t, readers, 2, "both readers of the subject")
	assert.MaxAllocs(t, func() {
		var gerr error
		_, held, gerr = s.Group(recordedPlan, hostUnit)
		err = cmp.Or(err, gerr)
	}, groupAllocs, "Group allocates the decoder and the decoded record")
	assert.NoError(t, err, "the group reads")
	assert.True(t, held, "the group is recorded")
	grouped := state.GroupRef(recordedPlan, hostUnit)
	var groups []state.Group
	assert.MaxAllocs(t, func() {
		var gerr error
		groups, gerr = s.Groups(grouped)
		err = cmp.Or(err, gerr)
	}, groupsAllocs, "Groups allocates the decoder and the decoded records")
	assert.NoError(t, err, "the groups table reads")
	assert.Length(t, groups, 1, "the row lists the plan's group")
}

// BenchmarkRecords measures each record's reference and their
// comparison, and each lookup over the generation that lookedUp records,
// after one lookup that the warm-up runs.
func BenchmarkRecords(b *testing.B) {
	benches := []struct {
		name string
		ref  func() state.RecordRef
	}{
		{name: "ValidationRef", ref: func() state.RecordRef { return state.ValidationRef(recordedSubject) }},
		{name: "InvocationRef", ref: func() state.RecordRef {
			return state.InvocationRef(recordedPlan, generated(recordedSubject))
		}},
		{name: "CheckRef", ref: func() state.RecordRef { return state.CheckRef(checkID) }},
		{name: "GroupRef", ref: func() state.RecordRef { return state.GroupRef(recordedPlan, hostUnit) }},
	}
	for _, bb := range benches {
		b.Run(bb.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got state.RecordRef
			for c.Loop() {
				got = bb.ref()
			}
			assert.NotEqual(b, got.ID, 0, "the reference names a record")
		})
	}

	b.Run("RecordRef", func(b *testing.B) {
		b.Run("Compare", func(b *testing.B) {
			check, validation := state.CheckRef(checkID), state.ValidationRef(recordedSubject)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			order := 0
			for c.Loop() {
				order = check.Compare(validation)
			}
			assert.InRange(b, order, 1, math.Inf(1), "a check sorts after a validation")
		})
	})

	s := lookedUp(b).Phases(b.Context())
	b.Run("PhaseState", func(b *testing.B) {
		b.Run("Validation", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(validationAllocs)
			defer c.End()
			var (
				got state.Validation
				err error
			)
			for c.Loop() {
				got, _, err = s.Validation(recordedSubject)
			}
			assert.NoError(b, err, "the validation reads")
			assert.Equal(b, got.Subject, recordedSubject, "Validation returns the subject's record")
		})

		b.Run("Invocation", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(invocationAllocs)
			defer c.End()
			var (
				got state.Invocation
				err error
			)
			for c.Loop() {
				got, _, err = s.Invocation(recordedPlan, generated(recordedSubject))
			}
			assert.NoError(b, err, "the invocation reads")
			assert.Equal(b, got.Plan, recordedPlan, "Invocation returns the plan's record")
		})

		b.Run("Check", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(checkAllocs)
			defer c.End()
			var (
				got state.Check
				err error
			)
			for c.Loop() {
				got, _, err = s.Check(checkID)
			}
			assert.NoError(b, err, "the check reads")
			assert.Equal(b, got.Name, checkID, "Check returns the check's record")
		})

		b.Run("Validations", func(b *testing.B) {
			ref := state.ValidationRef(recordedSubject)
			c := bench.Start(b).Warmup(1).MaxAllocs(validationsAllocs)
			defer c.End()
			var (
				got []state.Validation
				err error
			)
			for c.Loop() {
				got, err = s.Validations(ref)
			}
			assert.NoError(b, err, "the validations table reads")
			assert.Length(b, got, 1, "Validations returns the subject's record")
		})

		b.Run("Invocations", func(b *testing.B) {
			ref := state.InvocationRef(recordedPlan, generated(recordedSubject))
			c := bench.Start(b).Warmup(1).MaxAllocs(invocationsAllocs)
			defer c.End()
			var (
				got []state.Invocation
				err error
			)
			for c.Loop() {
				got, err = s.Invocations(ref)
			}
			assert.NoError(b, err, "the invocations table reads")
			assert.Length(b, got, 1, "Invocations returns the plan's record")
		})

		b.Run("Readers", func(b *testing.B) {
			edge := state.DeclarationEdge(recordedSubject)
			c := bench.Start(b).Warmup(1).MaxAllocs(readersAllocs)
			defer c.End()
			var (
				got []state.RecordRef
				err error
			)
			for c.Loop() {
				got, err = s.Readers(edge)
			}
			assert.NoError(b, err, "the readers read")
			assert.Length(b, got, 2, "the validation and the invocation read the subject")
		})

		b.Run("Group", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(groupAllocs)
			defer c.End()
			var (
				got state.Group
				err error
			)
			for c.Loop() {
				got, _, err = s.Group(recordedPlan, hostUnit)
			}
			assert.NoError(b, err, "the group reads")
			assert.Equal(b, got.Key, hostUnit, "Group returns the group of the key")
		})

		b.Run("Groups", func(b *testing.B) {
			ref := state.GroupRef(recordedPlan, hostUnit)
			c := bench.Start(b).Warmup(1).MaxAllocs(groupsAllocs)
			defer c.End()
			var (
				got []state.Group
				err error
			)
			for c.Loop() {
				got, err = s.Groups(ref)
			}
			assert.NoError(b, err, "the groups table reads")
			assert.Length(b, got, 1, "Groups returns the plan's group")
		})
	})
}

// lookedUp returns a generation of the records recordLookedUp records.
func lookedUp(tb testing.TB) *state.Generation {
	tb.Helper()

	return recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) { recordLookedUp(tb, r) })
}

// recordLookedUp records into r one validation of the recorded subject,
// one invocation of the plan's generator on it and one check, each of
// which read one fact, and the plan's group of the host unit.
func recordLookedUp(tb testing.TB, r *state.Recorder) {
	tb.Helper()

	reads := readsOf(tb, nil, meta.FactRef{Subject: recordedSubject, Key: edgeKey})
	shared := r.Lane("")
	shared.Validation(recordedSubject, nil, reads, nil)
	shared.Check(checkID, reads, nil)
	plan := r.Lane(recordedPlan)
	plan.Invoked(plugin.Invocation{Match: generated(recordedSubject), Reads: reads})
	plan.Group(recordedGroup(hostUnit))
}

// validatedDirectives returns two validated directives whose values take
// every param type, a keyed list among them.
func validatedDirectives() []directive.Directive {
	return []directive.Directive{
		{
			Name: "shape:mirror",
			Args: []directive.Value{{Kind: directive.TypeString, Str: "User"}},
			Params: map[directive.ParamKey]directive.Value{
				"count": {Kind: directive.TypeInt, Int: -3},
				"flag":  {Kind: directive.TypeBool, Bool: true},
				"tags": {Kind: directive.TypeList, List: []directive.Value{
					{Kind: directive.TypeString, Str: "a"}, {Kind: directive.TypeString, Str: "b"},
				}},
				"to": {Kind: directive.TypeReference, Ref: "Account", Target: siblingSubject},
			},
			Role:     "reader",
			Pos:      recordedPos,
			Instance: 1,
		},
		{Name: "stub", Params: map[directive.ParamKey]directive.Value{}, Pos: recordedPos, Negated: true},
	}
}

// sharedRow records two records whose IDs differ, then commits a row
// under id that lists the first record's entry before the second's, as
// two records sharing an ID would, and returns the generation the commit
// made live.
func sharedRow(tb testing.TB, t state.Table, id uint64, first, second func(*state.Recorder)) *state.Generation {
	tb.Helper()

	entry := func(build func(*state.Recorder)) []byte {
		g := recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, build)
		rows, err := g.All(tb.Context(), t)
		assert.NoError(tb, err, "the table reads")
		assert.Length(tb, rows, 1, "one record, one row")
		n, read := binary.Uvarint(rows[0].Value)
		assert.Equal(tb, n, uint64(1), "the row lists one entry")
		size, sized := binary.Uvarint(rows[0].Value[read:])
		return rows[0].Value[read+sized : read+sized+int(size)]
	}
	a, b := entry(first), entry(second)
	row := binary.AppendUvarint(nil, 2)
	for _, e := range [][]byte{a, b} {
		row = append(binary.AppendUvarint(row, uint64(len(e))), e...)
	}
	return putRows(tb, t, state.Row{Key: idKey(id), Value: row})
}

// idKey returns the key of a record's row: its ID's eight bytes,
// big-endian, pinned.
func idKey(id uint64) []byte {
	key := make([]byte, 0, 8)
	return binary.BigEndian.AppendUint64(key, id)
}

// edgeKeyOf returns the key of an edge's readers row: its hash's eight
// bytes, big-endian, pinned.
func edgeKeyOf(h state.EdgeHash) []byte {
	key := make([]byte, 0, 8)
	return binary.BigEndian.AppendUint64(key, uint64(h))
}

// sortedEdges returns the hashes in the order a read record lists them.
func sortedEdges(hs ...state.EdgeHash) []state.EdgeHash {
	slices.Sort(hs)
	return hs
}
