// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The paths of the recorded artifacts: the recorded subject's file and
// the sibling's.
const (
	artifactPath = "svc/api/user_gen.zz"
	siblingPath  = "svc/api/account_gen.zz"
)

// recordedHash is the digest the recorded artifacts state.
const recordedHash = "sha256:00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// The ceilings of the artifacts table's lookups over the generation that
// lookedUpArtifact records, each after a lookup before it, so the run
// reader keeps the block that the lookup reads.
const (
	// artifactAllocs is one artifact's lookup: the strings and the lists of
	// the decoded record, whose decoder stays on the stack.
	artifactAllocs = 31
	// clashesAllocs is one path's clashes: the rows of the scan that finds
	// the path, and the list of clashes with the path and the plan of its
	// row. The prefixes of the four scans stay on the stack, and the scans
	// that find nothing allocate nothing.
	clashesAllocs = 4
)

// An artifact reads back whole under its path, and its folded row finds
// each recorded path that one tree cannot contain beside a path.
func TestArtifacts(t *testing.T) {
	t.Parallel()

	t.Run("PhaseState", func(t *testing.T) {
		t.Parallel()

		t.Run("Artifact", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the record of a file", func(t *testing.T) {
				t.Parallel()

				want := recordedArtifact(artifactPath, recordedSubject)
				got := artifactOf(t, func(l *state.Lane) { l.Artifact(want) })
				assert.Equal(t, got, want, "the record contains the entry, the export rows and the names")
			})

			t.Run("returns false for a path that no plan wrote", func(t *testing.T) {
				t.Parallel()

				g := lookedUpArtifact(t)
				_, held, err := g.Phases(t.Context()).Artifact(siblingPath)
				assert.NoError(t, err, "the artifacts table reads")
				assert.False(t, held, "no plan wrote the sibling's file")
			})

			t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
				t.Parallel()

				g := putRows(t, state.TableArtifacts, state.Row{Key: []byte(artifactPath), Value: []byte{1, 'm', 0x80}})
				_, _, err := g.Phases(t.Context()).Artifact(artifactPath)
				assert.ErrorIs(t, err, state.ErrDamaged, "the digest is cut short")
			})
		})

		t.Run("Recorded", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give string
				want bool
			}{
				{name: "reports true for a path that a plan wrote", give: artifactPath, want: true},
				{name: "reports false for a path that no plan wrote", give: siblingPath},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := lookedUpArtifact(t).Phases(t.Context()).Recorded(tt.give)
					assert.NoError(t, err, "the artifacts table reads")
					assert.Equal(t, got, tt.want, "Recorded reports whether the generation records the file")
				})
			}

			t.Run("reports true for a row that does not decode", func(t *testing.T) {
				t.Parallel()

				g := putRows(t, state.TableArtifacts, state.Row{Key: []byte(artifactPath), Value: []byte{1, 'm', 0x80}})
				got, err := g.Phases(t.Context()).Recorded(artifactPath)
				assert.NoError(t, err, "Recorded decodes nothing")
				assert.True(t, got, "the row is there")
			})

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				unnamed := recordedArtifact(artifactPath, recordedSubject)
				unnamed.Names = nil
				recordedPhases(t, l, emptyFacts(), nil, func(r *state.Recorder) {
					r.Lane(recordedPlan).Artifact(unnamed)
				})
				damageRun(t, l, func(n int) int { return n - 1 })
				g, err := state.Open(t.Context(), l)
				assert.NoError(t, err, "the generation opens")
				_, err = g.Phases(t.Context()).Recorded(artifactPath)
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged footer is found")
			})
		})

		t.Run("Clashes", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name     string
				recorded string
				give     string
				want     []state.Clash
			}{
				{
					name:     "returns the recorded path that the path names",
					recorded: artifactPath, give: artifactPath,
					want: []state.Clash{{Path: artifactPath, Plan: recordedPlan}},
				},
				{
					name:     "returns a recorded path that differs from the path only in case",
					recorded: "svc/api/User_gen.zz", give: artifactPath,
					want: []state.Clash{{Path: "svc/api/User_gen.zz", Plan: recordedPlan}},
				},
				{
					name:     "returns a recorded path that the path needs as a directory",
					recorded: "svc/api", give: artifactPath,
					want: []state.Clash{{Path: "svc/api", Plan: recordedPlan}},
				},
				{
					name:     "returns a recorded path that needs the path as a directory",
					recorded: artifactPath, give: "svc/API",
					want: []state.Clash{{Path: artifactPath, Plan: recordedPlan}},
				},
				{
					name:     "returns nothing for a path that fits beside the recorded path",
					recorded: artifactPath, give: siblingPath,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					g := recordedPhases(t, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
						r.Lane(recordedPlan).Artifact(recordedArtifact(tt.recorded, recordedSubject))
					})
					got, err := g.Phases(t.Context()).Clashes(tt.give)
					assert.NoError(t, err, "the artifacts table reads")
					assert.Equal(t, got, tt.want,
						"the clashes are the recorded paths one tree cannot contain beside it")
				})
			}

			t.Run("returns ErrDamaged for a folded row whose plan does not decode", func(t *testing.T) {
				t.Parallel()

				key := []byte("\x00" + artifactPath + "\x00" + artifactPath)
				g := putRows(t, state.TableArtifacts, state.Row{Key: key, Value: []byte{5, 'm'}})
				_, err := g.Phases(t.Context()).Clashes(artifactPath)
				assert.ErrorIs(t, err, state.ErrDamaged, "the plan is cut short")
			})
		})
	})
}

// The artifacts table's lookups allocate within their ceilings in the
// ordinary run, which runs no benchmark. Each count keeps the first error
// of its calls, which cmp.Or returns without allocating. The check runs
// alone, because the count includes every goroutine's allocations.
func TestArtifactsAllocs(t *testing.T) {
	s := lookedUpArtifact(t).Phases(t.Context())
	var (
		held bool
		err  error
	)
	assert.MaxAllocs(t, func() {
		var aerr error
		_, held, aerr = s.Artifact(artifactPath)
		err = cmp.Or(err, aerr)
	}, artifactAllocs, "Artifact allocates the decoder and the decoded record")
	assert.NoError(t, err, "the artifact reads")
	assert.True(t, held, "the artifact is recorded")
	assert.MaxAllocs(t, func() {
		var rerr error
		held, rerr = s.Recorded(artifactPath)
		err = cmp.Or(err, rerr)
	}, 0, "Recorded allocates nothing")
	assert.NoError(t, err, "the artifacts table reads")
	assert.True(t, held, "the file is recorded")
	var clashes []state.Clash
	assert.MaxAllocs(t, func() {
		var cerr error
		clashes, cerr = s.Clashes(artifactPath)
		err = cmp.Or(err, cerr)
	}, clashesAllocs, "Clashes allocates its prefixes, the rows it finds and their clashes")
	assert.NoError(t, err, "the clashes read")
	assert.Length(t, clashes, 1, "the path clashes with itself")
}

// BenchmarkArtifacts measures each lookup of the artifacts table over the
// generation that lookedUpArtifact records, after one lookup that the
// warm-up runs.
func BenchmarkArtifacts(b *testing.B) {
	s := lookedUpArtifact(b).Phases(b.Context())

	b.Run("PhaseState", func(b *testing.B) {
		b.Run("Artifact", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(artifactAllocs)
			defer c.End()
			var (
				got state.Artifact
				err error
			)
			for c.Loop() {
				got, _, err = s.Artifact(artifactPath)
			}
			assert.NoError(b, err, "the artifact reads")
			assert.Equal(b, got.Entry.Path, artifactPath, "Artifact returns the file's record")
		})

		b.Run("Recorded", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			var (
				got bool
				err error
			)
			for c.Loop() {
				got, err = s.Recorded(artifactPath)
			}
			assert.NoError(b, err, "the artifacts table reads")
			assert.True(b, got, "the file is recorded")
		})

		b.Run("Clashes", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(clashesAllocs)
			defer c.End()
			var (
				got []state.Clash
				err error
			)
			for c.Loop() {
				got, err = s.Clashes(artifactPath)
			}
			assert.NoError(b, err, "the clashes read")
			assert.Length(b, got, 1, "the path clashes with itself")
		})
	})
}

// lookedUpArtifact returns a generation that records the artifact of the
// recorded subject's file.
func lookedUpArtifact(tb testing.TB) *state.Generation {
	tb.Helper()

	return recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) {
		r.Lane(recordedPlan).Artifact(recordedArtifact(artifactPath, recordedSubject))
	})
}

// recordedArtifact returns the recorded plan's artifact of one file at a
// path, generated from a subject by the generator into the host unit: one
// export row and one name, each the subject's name.
func recordedArtifact(at string, subject symbol.Identity) state.Artifact {
	return state.Artifact{
		Entry: manifest.Entry{
			Path: at, Plan: recordedPlan, Hash: recordedHash,
			Plugins: []diag.Origin{generatorID}, Sources: []string{recordedPos.File},
		},
		Pkg:   edgePackage,
		Group: hostUnit,
		At:    recordedPos,
		First: "struct " + subject.Name,
		Export: []plugin.ExportedSymbol{{
			Origin: subject, Plugin: generatorID, Name: subject.Name,
			Kind: symbol.KindStruct, Spelling: subject.Name, Package: edgePackage, File: at,
		}},
		Names: []plugin.NameEntry{{
			Package: edgeDir, Origin: subject, Kind: symbol.KindStruct, Emitted: subject.Name, Settled: subject.Name,
			File: at, FilePkg: edgePackage,
		}},
	}
}

// artifactOf records the artifact build makes into the plan's lane, and
// returns the record of the file at artifactPath.
func artifactOf(tb testing.TB, build func(*state.Lane)) state.Artifact {
	tb.Helper()

	g := recordedPhases(tb, ledger.NewMem(), emptyFacts(), nil, func(r *state.Recorder) { build(r.Lane(recordedPlan)) })
	got, held, err := g.Phases(tb.Context()).Artifact(artifactPath)
	assert.NoError(tb, err, "the artifacts table reads")
	assert.True(tb, held, "the file is recorded")
	return got
}
