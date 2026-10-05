// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"encoding/binary"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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
)

// The ceilings of a lookup over the generation lookedUp records, each
// after one lookup read the run's index. Every lookup allocates the
// row's key, the block it reads and the block's list of entries, and a
// record's lookup the decoder its match reads the entry through. A
// string of one byte, such as the subject's discriminator, allocates
// nothing.
const (
	// validationAllocs is one validation's lookup: the key, the block and
	// its entries, the decoder, the subject's four longer strings, and the
	// list of reads.
	validationAllocs = 9
	// invocationAllocs is one invocation's lookup: the key, the block and
	// its entries, the decoder, the plan, the plugin, the subject's four
	// longer strings, and the list of reads.
	invocationAllocs = 11
	// checkAllocs is one check's lookup: the key, the block and its
	// entries, the decoder, the check's name, and the list of reads.
	checkAllocs = 6
	// readersAllocs is one edge's readers: the key, the block and its
	// entries, and the list of records.
	readersAllocs = 4
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
						state.DeclarationEdge(siblingSubject), state.DeclarationEdge(recordedSubject),
					),
					Findings: []diag.Diag{recordedDiag},
				}, "with its directives, its reads and its findings")
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
				assert.True(t, held && got.Subject == recordedSubject, "the subject's own entry is found")
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
				reads := sortedEdges(state.FactEdge(recordedSubject, edgeKey), state.DeclarationEdge(recordedSubject))
				assert.Equal(t, got, state.Invocation{
					Plan:     recordedPlan,
					Match:    m,
					Reads:    reads,
					Exports:  inv.Exports,
					Units:    inv.Units,
					Hosts:    inv.Hosts,
					Claimed:  inv.Claimed,
					Findings: inv.Findings,
				}, "with its reads, its subject's edge added, and everything else as journaled")
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
					r.Lane(recordedPlan).Invoked(plugin.Invocation{Match: generated(recordedSubject)})
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
						r.Lane(recordedPlan).Invoked(plugin.Invocation{Match: generated(siblingSubject)})
					},
					func(r *state.Recorder) {
						r.Lane(recordedPlan).Invoked(plugin.Invocation{Match: generated(recordedSubject)})
					})
				got, held, err := shared.Phases(t.Context()).Invocation(recordedPlan, generated(recordedSubject))
				assert.NoError(t, err, "the shared row reads")
				assert.True(t, held && got.Match.Subject == recordedSubject, "the invocation's own entry is found")
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
					Reads:    []state.EdgeHash{state.FactEdge(siblingSubject, edgeKey)},
					Findings: []diag.Diag{recordedDiag},
				}, "with its reader's reads and its findings")
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
				assert.True(t, held && got.Name == checkID, "the check's own entry is found")
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
// benchmark. The check runs alone, because AllocsPerRun counts every
// goroutine's allocations and refuses to run beside parallel tests.
func TestRecordsAllocs(t *testing.T) {
	var got state.RecordRef
	assert.MaxAllocs(t, func() { got = state.ValidationRef(recordedSubject) }, 0,
		"ValidationRef allocates nothing")
	assert.MaxAllocs(t, func() { got = state.InvocationRef(recordedPlan, generated(recordedSubject)) }, 0,
		"InvocationRef allocates nothing")
	assert.MaxAllocs(t, func() { got = state.CheckRef(checkID) }, 0, "CheckRef allocates nothing")
	assert.Equal(t, got.Kind, state.RecordCheck, "the last reference is a check's")
	other := state.ValidationRef(recordedSubject)
	order := 0
	assert.MaxAllocs(t, func() { order = got.Compare(other) }, 0, "Compare allocates nothing")
	assert.True(t, order > 0, "a check sorts after a validation")

	s := lookedUp(t).Phases(t.Context())
	assert.MaxAllocs(t, func() {
		if _, held, err := s.Validation(recordedSubject); err != nil || !held {
			t.Fatalf("Validation: held %t, error %v", held, err)
		}
	}, validationAllocs, "Validation allocates the key, the block and the decoded record")
	assert.MaxAllocs(t, func() {
		if _, held, err := s.Invocation(recordedPlan, generated(recordedSubject)); err != nil || !held {
			t.Fatalf("Invocation: held %t, error %v", held, err)
		}
	}, invocationAllocs, "Invocation allocates the key, the block and the decoded record")
	assert.MaxAllocs(t, func() {
		if _, held, err := s.Check(checkID); err != nil || !held {
			t.Fatalf("Check: held %t, error %v", held, err)
		}
	}, checkAllocs, "Check allocates the key, the block and the decoded record")
	edge := state.DeclarationEdge(recordedSubject)
	assert.MaxAllocs(t, func() {
		if refs, err := s.Readers(edge); err != nil || len(refs) != 2 {
			t.Fatalf("Readers: %d records, error %v", len(refs), err)
		}
	}, readersAllocs, "Readers allocates the key, the block and the list of records")
}

// BenchmarkRecords measures each record's reference and their
// comparison, and each lookup over the generation lookedUp records,
// after one lookup read the run's index.
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
			assert.True(b, order > 0, "a check sorts after a validation")
		})
	})

	s := lookedUp(b).Phases(b.Context())
	b.Run("PhaseState", func(b *testing.B) {
		b.Run("Validation", func(b *testing.B) {
			_, _, err := s.Validation(recordedSubject)
			assert.NoError(b, err, "the validation reads before the measurement")
			c := bench.Start(b).MaxAllocs(validationAllocs)
			defer c.End()
			var got state.Validation
			for c.Loop() {
				got, _, err = s.Validation(recordedSubject)
			}
			assert.NoError(b, err, "the validation reads")
			assert.Equal(b, got.Subject, recordedSubject, "Validation returns the subject's record")
		})

		b.Run("Invocation", func(b *testing.B) {
			_, _, err := s.Invocation(recordedPlan, generated(recordedSubject))
			assert.NoError(b, err, "the invocation reads before the measurement")
			c := bench.Start(b).MaxAllocs(invocationAllocs)
			defer c.End()
			var got state.Invocation
			for c.Loop() {
				got, _, err = s.Invocation(recordedPlan, generated(recordedSubject))
			}
			assert.NoError(b, err, "the invocation reads")
			assert.Equal(b, got.Plan, recordedPlan, "Invocation returns the plan's record")
		})

		b.Run("Check", func(b *testing.B) {
			_, _, err := s.Check(checkID)
			assert.NoError(b, err, "the check reads before the measurement")
			c := bench.Start(b).MaxAllocs(checkAllocs)
			defer c.End()
			var got state.Check
			for c.Loop() {
				got, _, err = s.Check(checkID)
			}
			assert.NoError(b, err, "the check reads")
			assert.Equal(b, got.Name, checkID, "Check returns the check's record")
		})

		b.Run("Readers", func(b *testing.B) {
			edge := state.DeclarationEdge(recordedSubject)
			_, err := s.Readers(edge)
			assert.NoError(b, err, "the readers read before the measurement")
			c := bench.Start(b).MaxAllocs(readersAllocs)
			defer c.End()
			var got []state.RecordRef
			for c.Loop() {
				got, err = s.Readers(edge)
			}
			assert.NoError(b, err, "the readers read")
			assert.Length(b, got, 2, "the validation and the invocation read the subject")
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
// which read one fact.
func recordLookedUp(tb testing.TB, r *state.Recorder) {
	tb.Helper()

	reads := readsOf(tb, nil, meta.FactRef{Subject: recordedSubject, Key: edgeKey})
	shared := r.Lane("")
	shared.Validation(recordedSubject, nil, reads, nil)
	shared.Check(checkID, reads, nil)
	r.Lane(recordedPlan).Invoked(plugin.Invocation{Match: generated(recordedSubject), Reads: reads})
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
