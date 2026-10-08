// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The spellings the kept names cases declare: a settled name that a
// second declaration of one emitted name takes, and a method's name.
const (
	respelledName = "User2"
	methodName    = "Save"
)

// manyNames is how many files declare a name in the scope whose rows fill
// several of a run's blocks of 4 KiB.
const manyNames = 200

// The ceilings of the kept names' lookups over the names of the recorded
// artifact, each after a lookup before it, so the run reader keeps the
// block that the lookup reads.
const (
	// inPackageAllocs is one lookup by package: the rows it scans and their
	// entries. The prefix stays on the stack, and each string decodes to
	// the one that an earlier lookup made.
	inPackageAllocs = 2
	// ofOriginAllocs is one lookup by origin, which allocates what a
	// lookup by package does.
	ofOriginAllocs = 2
)

// The kept names of a plan answer each lookup from the rows of one key
// prefix, without the entries of a file that the warm run generates
// again.
func TestNames(t *testing.T) {
	t.Parallel()

	t.Run("KeptNames", func(t *testing.T) {
		t.Parallel()

		t.Run("InPackage", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the entry that a file of the package declares under the emitted name", func(t *testing.T) {
				t.Parallel()

				got, held := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject)).
					InPackage(edgeDir, recordedSubject.Name)
				assert.True(t, held, "the file declares the name")
				assert.Equal(t, got, recordedArtifact(artifactPath, recordedSubject).Names[0],
					"the entry is the file's")
			})

			t.Run("returns false for an emitted name that no file declares", func(t *testing.T) {
				t.Parallel()

				_, held := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject)).
					InPackage(edgeDir, siblingSubject.Name)
				assert.False(t, held, "no file declares the sibling")
			})

			t.Run("reports Ambiguous for entries that settle the name apart", func(t *testing.T) {
				t.Parallel()

				respelled := recordedArtifact(siblingPath, siblingSubject)
				respelled.Names[0].Emitted, respelled.Names[0].Settled = recordedSubject.Name, respelledName
				got, held := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject), respelled).
					InPackage(edgeDir, recordedSubject.Name)
				assert.True(t, held, "two files declare the name")
				assert.True(t, got.Ambiguous, "the two settled names differ")
			})

			t.Run("returns false for a method", func(t *testing.T) {
				t.Parallel()

				a := recordedArtifact(artifactPath, recordedSubject)
				a.Names[0].Kind = symbol.KindMethod
				_, held := keptNames(t, nil, a).InPackage(edgeDir, recordedSubject.Name)
				assert.False(t, held, "no bare reference names a method")
			})

			t.Run("returns false for the name of a file that skip reports", func(t *testing.T) {
				t.Parallel()

				again := func(file string) bool { return file == artifactPath }
				_, held := keptNames(t, again, recordedArtifact(artifactPath, recordedSubject)).
					InPackage(edgeDir, recordedSubject.Name)
				assert.False(t, held, "the run generates the file again")
			})

			t.Run("returns false for a name of another plan", func(t *testing.T) {
				t.Parallel()

				g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(failedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
				})
				_, held := g.Phases(t.Context()).Names(recordedPlan, nil).InPackage(edgeDir, recordedSubject.Name)
				assert.False(t, held, "the name is the other plan's")
			})
		})

		t.Run("OfOrigin", func(t *testing.T) {
			t.Parallel()

			t.Run(
				"returns the entry that a declaration of the origin declares under the emitted name",
				func(t *testing.T) {
					t.Parallel()

					got, held := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject)).
						OfOrigin(recordedSubject, recordedSubject.Name)
					assert.True(t, held, "the file declares a name of the origin")
					assert.Equal(t, got.Settled, recordedSubject.Name, "the entry is the origin's")
				},
			)

			t.Run("returns false for an origin without a declaration under the name", func(t *testing.T) {
				t.Parallel()

				_, held := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject)).
					OfOrigin(siblingSubject, recordedSubject.Name)
				assert.False(t, held, "no declaration derives from the sibling")
			})

			t.Run("returns false for the name of a file that skip reports", func(t *testing.T) {
				t.Parallel()

				again := func(file string) bool { return file == artifactPath }
				_, held := keptNames(t, again, recordedArtifact(artifactPath, recordedSubject)).
					OfOrigin(recordedSubject, recordedSubject.Name)
				assert.False(t, held, "the run generates the file again")
			})
		})

		t.Run("InScope", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the entries of the package sorted by settled name", func(t *testing.T) {
				t.Parallel()

				got := keptNames(t, nil,
					recordedArtifact(artifactPath, recordedSubject), recordedArtifact(siblingPath, siblingSubject),
				).InScope(edgeDir, "")
				assert.Length(t, got, 2, "both files declare a name in the package")
				assert.Equal(t, got[0].Settled, siblingSubject.Name, "the sibling's name sorts first")
			})

			t.Run("returns the methods that attach to the receiver", func(t *testing.T) {
				t.Parallel()

				a := recordedArtifact(artifactPath, recordedSubject)
				a.Names = append(a.Names, plugin.NameEntry{
					Package: edgeDir, Receiver: recordedSubject.Name, Kind: symbol.KindMethod,
					Emitted: methodName, Settled: methodName,
				})
				got := keptNames(t, nil, a).InScope(edgeDir, recordedSubject.Name)
				assert.Length(t, got, 1, "the type has one method")
				assert.Equal(t, got[0].Settled, methodName, "the method is the type's")
			})

			t.Run("returns every entry of a scope whose rows fill several blocks", func(t *testing.T) {
				t.Parallel()

				artifacts := make([]state.Artifact, 0, manyNames)
				for i := range manyNames {
					subject := recordedSubject
					subject.Name = fmt.Sprintf("Type%03d", i)
					artifacts = append(artifacts, recordedArtifact(fmt.Sprintf("svc/api/type%03d_gen.zz", i), subject))
				}
				got := keptNames(t, nil, artifacts...).InScope(edgeDir, "")
				assert.Length(t, got, manyNames, "every file's name is in the scope")
			})

			t.Run("leaves out the entries of a file that a later commit dropped", func(t *testing.T) {
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
				got := g.Phases(t.Context()).Names(recordedPlan, nil).InScope(edgeDir, "")
				assert.Length(t, got, 1, "one file remains")
				assert.Equal(t, got[0].File, siblingPath, "the sibling's name remains")
			})

			t.Run("returns the list of the first call to a second call", func(t *testing.T) {
				t.Parallel()

				names := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject))
				first := names.InScope(edgeDir, "")
				assert.Equal(t, names.InScope(edgeDir, ""), first, "the scope is read once", assert.ByIdentity())
			})
		})

		t.Run("Err", func(t *testing.T) {
			t.Parallel()

			t.Run("returns nil after lookups whose rows decode", func(t *testing.T) {
				t.Parallel()

				names := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject))
				names.InPackage(edgeDir, recordedSubject.Name)
				assert.NoError(t, names.Err(), "every row decodes")
			})

			t.Run("returns ErrDamaged after a lookup whose row does not decode", func(t *testing.T) {
				t.Parallel()

				key := []byte(recordedPlan + "\x00s\x00" + edgeDir + "\x00\x00" + recordedSubject.Name + "\x00x")
				g := putRows(t, state.TableNames, state.Row{Key: key, Value: []byte{5, 'f'}})
				names := g.Phases(t.Context()).Names(recordedPlan, nil)
				_, held := names.InPackage(edgeDir, recordedSubject.Name)
				assert.False(t, held, "the lookup finds nothing")
				assert.ErrorIs(t, names.Err(), state.ErrDamaged, "the file is cut short")
			})
		})
	})
}

// The kept names' lookups allocate within their ceilings in the ordinary
// run, which runs no benchmark, and a scope read before allocates
// nothing. The check runs alone, because the count includes every
// goroutine's allocations.
func TestNamesAllocs(t *testing.T) {
	names := keptNames(t, nil, recordedArtifact(artifactPath, recordedSubject))
	held := false
	assert.MaxAllocs(t, func() { _, held = names.InPackage(edgeDir, recordedSubject.Name) }, inPackageAllocs,
		"InPackage allocates the prefix, the rows and the entries")
	assert.True(t, held, "the package declares the name")
	assert.MaxAllocs(t, func() { _, held = names.OfOrigin(recordedSubject, recordedSubject.Name) }, ofOriginAllocs,
		"OfOrigin allocates the prefix, the rows and the entries")
	assert.True(t, held, "the origin declares the name")
	var scope []plugin.NameEntry
	assert.MaxAllocs(t, func() { scope = names.InScope(edgeDir, "") }, 0,
		"InScope allocates nothing for a scope it read")
	assert.Length(t, scope, 1, "the package declares one name")
	assert.NoError(t, names.Err(), "every row decodes")
}

// BenchmarkNames measures each lookup of the kept names over the names of
// the recorded artifact, after one lookup that the warm-up runs.
func BenchmarkNames(b *testing.B) {
	names := keptNames(b, nil, recordedArtifact(artifactPath, recordedSubject))

	b.Run("KeptNames", func(b *testing.B) {
		b.Run("InPackage", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(inPackageAllocs)
			defer c.End()
			held := false
			for c.Loop() {
				_, held = names.InPackage(edgeDir, recordedSubject.Name)
			}
			assert.True(b, held, "InPackage returns the file's name")
		})

		b.Run("OfOrigin", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(ofOriginAllocs)
			defer c.End()
			held := false
			for c.Loop() {
				_, held = names.OfOrigin(recordedSubject, recordedSubject.Name)
			}
			assert.True(b, held, "OfOrigin returns the file's name")
		})

		b.Run("InScope", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			var got []plugin.NameEntry
			for c.Loop() {
				got = names.InScope(edgeDir, "")
			}
			assert.Length(b, got, 1, "InScope returns the package's name")
		})
	})
}

// keptNames records the artifacts into the recorded plan's lane and
// returns the plan's kept names, without the names of each file that skip
// reports.
func keptNames(tb testing.TB, skip func(string) bool, artifacts ...state.Artifact) *state.KeptNames {
	tb.Helper()

	g := recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
		plan := r.Lane(recordedPlan)
		for _, a := range artifacts {
			plan.Artifact(a)
		}
	})
	return g.Phases(tb.Context()).Names(recordedPlan, skip)
}
