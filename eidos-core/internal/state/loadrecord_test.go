// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The scripted tree's files: an API package, and a store package whose
// one type names the API's user through an import.
const (
	apiFile   = "svc/api/user.zz"
	storeFile = "svc/store/row.zz"
	extraFile = "svc/extra/extra.zz"
)

// loadBrand is the brand the scripted loads run under.
const loadBrand output.Brand = "own"

// The ceilings of the reads and the record of a load over the scripted
// tree of two units.
const (
	// loadStateAllocs is one load state over a generation: the state.
	loadStateAllocs = 1
	// filesAllocs is a first range over the record's files, which reads
	// the files table whole: the run, its index and blocks, the merged
	// entries, and the two records with their paths and packages.
	filesAllocs = 13
	// unitsAllocs is a first range over the record's units, which reads
	// the units table whole: the run, its index and blocks, the merged
	// entries, and the two records with their members, keys, summaries
	// and imports.
	unitsAllocs = 42
	// regionAllocs is one decode of a unit's region of one package: the
	// run read from the ledger, the string table, the region, and the
	// package's declarations and lists.
	regionAllocs = 20
	// doorsAllocs is one read of the scripted frontend's doors: the doors
	// table read whole, and the one door's reads, units and paths.
	doorsAllocs = 13
	// probedAllocs is one lookup of a candidate a unit named: its key,
	// the run read from the ledger, the block, the units' paths decoded,
	// and the list of numbers.
	probedAllocs = 6
	// followedAllocs is one lookup of a package no reference followed:
	// its key, the run read from the ledger, and the block the row would
	// be in.
	followedAllocs = 3
	// recordLoadAllocs is one record of a cold load of the two units into
	// a new commit: each region's encoding and the segment, each file's
	// and unit's row, the door's row, and the probes.
	recordLoadAllocs = 89
)

// A generation's record of the load is what a warm load keeps, so every
// part of a report returns whole from a commit, and a second commit
// changes only what the second load changed.
func TestLoadState(t *testing.T) {
	t.Parallel()

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every file record the load reported", func(t *testing.T) {
			t.Parallel()

			report := loaded(t, scriptedTree(), nil)
			g := recordedLoad(t, ledger.NewMem(), report)
			assert.Equal(t, filesOf(t, g.Load(t.Context())), report.Files, "the records return whole")
		})

		t.Run("returns a racily clean file's record with no size", func(t *testing.T) {
			t.Parallel()

			tree := scriptedTree()
			tree[apiFile].ModTime = time.Now().Add(time.Hour)
			g := recordedLoad(t, ledger.NewMem(), loaded(t, tree, nil))
			for _, f := range filesOf(t, g.Load(t.Context())) {
				if f.Path == apiFile {
					assert.Equal(t, f.Size, int64(0), "the next gate that meets the file hashes it")
				}
			}
		})

		t.Run("returns the anchor the recording run took", func(t *testing.T) {
			t.Parallel()

			g := recordedLoad(t, ledger.NewMem(), loaded(t, scriptedTree(), nil))
			assert.True(t, g.Load(t.Context()).Anchor().Equal(past), "the header's anchor")
		})
	})

	t.Run("Units", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every unit's record in splice order", func(t *testing.T) {
			t.Parallel()

			report := loaded(t, scriptedTree(), nil)
			units := unitsOf(t, recordedLoad(t, ledger.NewMem(), report).Load(t.Context()))
			assert.Length(t, units, len(report.Units), "one record for each unit")
			for i, u := range units {
				want := report.Units[i]
				assert.Equal(t, u.Number, i, "numbered in splice order")
				assert.Equal(t, u.Files, want.Files, "with the unit's members")
				assert.Equal(t, u.Key, want.Key, "and its key")
				assert.Equal(t, u.Summary, want.Region.Info(), "and its region's summary")
			}
		})

		t.Run("returns the imports a unit's files name", func(t *testing.T) {
			t.Parallel()

			units := unitsOf(t, recordedLoad(t, ledger.NewMem(), loaded(t, scriptedTree(), nil)).Load(t.Context()))
			assert.Equal(t, units[1].Imports, []load.Import{{
				Path: "svc/api", Files: []string{storeFile}, At: position.Pos{File: storeFile, Line: 2},
			}}, "the store package imports the API on its second line")
		})
	})

	t.Run("Region", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the region the load built", func(t *testing.T) {
			t.Parallel()

			report := loaded(t, scriptedTree(), nil)
			s := recordedLoad(t, ledger.NewMem(), report).Load(t.Context())
			for i, u := range unitsOf(t, s) {
				got, err := s.Region(u)
				assert.NoError(t, err, "the region decodes")
				assert.Equal(t, got, report.Units[i].Region, "to the region the load built")
			}
		})

		t.Run("returns an error for a unit the record does not contain", func(t *testing.T) {
			t.Parallel()

			s := recordedLoad(t, ledger.NewMem(), loaded(t, scriptedTree(), nil)).Load(t.Context())
			_, err := s.Region(load.UnitRecord{Number: 9})
			assert.HasError(t, err, "the record has two units")
		})
	})

	t.Run("Doors", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend's doors as the load reported them", func(t *testing.T) {
			t.Parallel()

			report := loaded(t, scriptedTree(), nil)
			doors, err := recordedLoad(t, ledger.NewMem(), report).Load(t.Context()).Doors(frontendtest.ScriptedID)
			assert.NoError(t, err, "the doors read")
			assert.Equal(t, doors, report.Doors[frontendtest.ScriptedID], "the partition's record returns whole")
		})

		t.Run("returns nothing for a frontend the record lacks", func(t *testing.T) {
			t.Parallel()

			s := recordedLoad(t, ledger.NewMem(), loaded(t, scriptedTree(), nil)).Load(t.Context())
			doors, err := s.Doors("absent")
			assert.NoError(t, err, "the doors read")
			assert.Empty(t, doors, "no door is recorded under the name")
		})
	})

	t.Run("Probed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the units whose references named a candidate", func(t *testing.T) {
			t.Parallel()

			s := recordedLoad(t, ledger.NewMem(), loaded(t, scriptedTree(), nil)).Load(t.Context())
			got, err := s.Probed(userID())
			assert.NoError(t, err, "the probes read")
			assert.Equal(t, got, []int{1}, "the store unit named the user")
		})

		t.Run("returns nothing for a candidate no reference named", func(t *testing.T) {
			t.Parallel()

			s := recordedLoad(t, ledger.NewMem(), loaded(t, scriptedTree(), nil)).Load(t.Context())
			got, err := s.Probed(symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/api", Name: "Absent"})
			assert.NoError(t, err, "the probes read")
			assert.Empty(t, got, "no unit named it")
		})

		t.Run("drops a unit whose new region no longer names the candidate", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			recordedLoad(t, l, loaded(t, scriptedTree(), nil))
			tree := scriptedTree()
			tree[storeFile] = &fstest.MapFile{Data: []byte("package svc/store\ntype Row string\n")}
			g := recordedLoad(t, l, loaded(t, tree, nil))
			got, err := g.Load(t.Context()).Probed(userID())
			assert.NoError(t, err, "the probes read")
			assert.Empty(t, got, "the store unit names the user no more")
		})
	})

	t.Run("Followed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a package no reference followed", func(t *testing.T) {
			t.Parallel()

			s := recordedLoad(t, ledger.NewMem(), loaded(t, scriptedTree(), nil)).Load(t.Context())
			api := symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/api", Kind: symbol.KindPackage}
			got, err := s.Followed(api)
			assert.NoError(t, err, "the probes read")
			assert.Empty(t, got, "the scripted language follows no re-export")
		})
	})

	t.Run("RecordLoad", func(t *testing.T) {
		t.Parallel()

		t.Run("adds a unit a second load parsed", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			recordedLoad(t, l, loaded(t, scriptedTree(), nil))
			tree := scriptedTree()
			tree[extraFile] = &fstest.MapFile{Data: []byte("package svc/extra\ntype Extra string\n")}
			units := unitsOf(t, recordedLoad(t, l, loaded(t, tree, nil)).Load(t.Context()))
			assert.Length(t, units, 3, "the new unit has its row")
		})

		t.Run("drops a unit a second load no longer has", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			recordedLoad(t, l, loaded(t, scriptedTree(), nil))
			tree := scriptedTree()
			delete(tree, apiFile)
			g := recordedLoad(t, l, loaded(t, tree, nil))
			units := unitsOf(t, g.Load(t.Context()))
			assert.Length(t, units, 1, "the dropped unit's row is gone")
			files := filesOf(t, g.Load(t.Context()))
			assert.False(t, slices.ContainsFunc(files, func(f load.FileRecord) bool { return f.Path == apiFile }),
				"and so is its file's record")
		})

		t.Run("writes no row for a second load of the same tree", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := recordedLoad(t, l, loaded(t, scriptedTree(), nil))
			report := loaded(t, scriptedTree(), first.Load(t.Context()))
			for i := range report.Units {
				report.Units[i].Region = nil
			}
			writes := l.Writes()
			second := recordedLoad(t, l, report)
			assert.Equal(t, second.Name, first.Name, "the record is unchanged")
			assert.Equal(t, l.Writes(), writes, "so nothing is written")
		})
	})
}

// The reads of a load's record and the record of a load allocate within
// their ceilings in the ordinary run, which runs no benchmark. A first
// range takes a load state made before the count, because the state
// reads each table once, and a record takes a commit made before it. The
// check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestLoadStateAllocs(t *testing.T) {
	report := loaded(t, scriptedTree(), nil)
	g := recordedLoad(t, ledger.NewMem(), report)
	var s *state.LoadState
	assert.MaxAllocs(t, func() { s = g.Load(t.Context()) }, loadStateAllocs, "Load allocates the state")
	assert.MaxAllocs(t, func() {
		if !s.Anchor().Equal(past) {
			t.Fatal("Anchor returned another instant")
		}
	}, 0, "Anchor allocates nothing")

	states, at := loadStates(t, g, allocRuns), 0
	assert.MaxAllocs(t, func() {
		for range states[at].Files() {
		}
		at++
	}, filesAllocs, "a first range over Files reads the files table")
	states, at = loadStates(t, g, allocRuns), 0
	assert.MaxAllocs(t, func() {
		for range states[at].Units() {
		}
		at++
	}, unitsAllocs, "a first range over Units reads the units table")
	assert.MaxAllocs(t, func() {
		for range s.Files() {
		}
		for range s.Units() {
		}
	}, 0, "a range over tables read before allocates nothing")

	u := lastUnit(t, s)
	assert.MaxAllocs(t, func() {
		if _, err := s.Region(u); err != nil {
			t.Fatalf("Region: unexpected error: %v", err)
		}
	}, regionAllocs, "Region allocates the decoded region")
	assert.MaxAllocs(t, func() {
		if _, err := s.Doors(frontendtest.ScriptedID); err != nil {
			t.Fatalf("Doors: unexpected error: %v", err)
		}
	}, doorsAllocs, "Doors allocates the doors table and the doors")
	user := userID()
	assert.MaxAllocs(t, func() {
		if got, err := s.Probed(user); err != nil || len(got) != 1 {
			t.Fatalf("Probed: units %v, error %v", got, err)
		}
	}, probedAllocs, "Probed allocates the key, the row and the numbers")
	api := apiPackage()
	assert.MaxAllocs(t, func() {
		if _, err := s.Followed(api); err != nil {
			t.Fatalf("Followed: unexpected error: %v", err)
		}
	}, followedAllocs, "Followed allocates the key and the block")

	commits, at := newCommits(allocRuns), 0
	assert.MaxAllocs(t, func() {
		if err := state.RecordLoad(t.Context(), commits[at], nil, report); err != nil {
			t.Fatalf("RecordLoad: unexpected error: %v", err)
		}
		at++
	}, recordLoadAllocs, "RecordLoad allocates the regions, the rows and the probes")
}

// BenchmarkLoadState measures the reads a warm load makes of the record
// of the scripted tree, and the record of a cold load of it, each after
// one call.
func BenchmarkLoadState(b *testing.B) {
	report := loaded(b, scriptedTree(), nil)
	g := recordedLoad(b, ledger.NewMem(), report)
	s := g.Load(b.Context())
	u := lastUnit(b, s)

	b.Run("Generation.Load", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(loadStateAllocs)
		defer c.End()
		var got *state.LoadState
		for c.Loop() {
			got = g.Load(b.Context())
		}
		assert.NotNil(b, got, "Load returns the state")
	})

	b.Run("Anchor", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got time.Time
		for c.Loop() {
			got = s.Anchor()
		}
		assert.True(b, got.Equal(past), "Anchor returns the recording run's anchor")
	})

	b.Run("Files/a first range", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(filesAllocs)
		defer c.End()
		var fresh *state.LoadState
		n := 0
		for c.Loop() {
			c.Excluding(func() { fresh, n = g.Load(b.Context()), 0 })
			for range fresh.Files() {
				n++
			}
		}
		assert.Equal(b, n, len(report.Files), "the range yields every file")
	})

	b.Run("Files/a range over a table read before", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		n := 0
		for first := true; first || c.Loop(); first = false {
			n = 0
			for range s.Files() {
				n++
			}
		}
		assert.Equal(b, n, len(report.Files), "the range yields every file")
	})

	b.Run("Units/a first range", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(unitsAllocs)
		defer c.End()
		var fresh *state.LoadState
		n := 0
		for c.Loop() {
			c.Excluding(func() { fresh, n = g.Load(b.Context()), 0 })
			for range fresh.Units() {
				n++
			}
		}
		assert.Equal(b, n, len(report.Units), "the range yields every unit")
	})

	b.Run("Units/a range over a table read before", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		n := 0
		for first := true; first || c.Loop(); first = false {
			n = 0
			for range s.Units() {
				n++
			}
		}
		assert.Equal(b, n, len(report.Units), "the range yields every unit")
	})

	b.Run("Region", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(regionAllocs)
		defer c.End()
		var (
			got *store.Region
			err error
		)
		for c.Loop() {
			got, err = s.Region(u)
		}
		assert.NoError(b, err, "the region decodes")
		assert.Length(b, got.Packages, 1, "to the unit's package")
	})

	b.Run("Doors", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(doorsAllocs)
		defer c.End()
		var (
			got []load.DoorRecord
			err error
		)
		for c.Loop() {
			got, err = s.Doors(frontendtest.ScriptedID)
		}
		assert.NoError(b, err, "the doors read")
		assert.Equal(b, got, report.Doors[frontendtest.ScriptedID], "Doors returns the recorded doors")
	})

	b.Run("Probed", func(b *testing.B) {
		user := userID()
		_, err := s.Probed(user)
		assert.NoError(b, err, "the probes read before the measurement")
		c := bench.Start(b).MaxAllocs(probedAllocs)
		defer c.End()
		var got []int
		for c.Loop() {
			got, err = s.Probed(user)
		}
		assert.NoError(b, err, "the probes read")
		assert.Equal(b, got, []int{1}, "the store unit named the user")
	})

	b.Run("Followed", func(b *testing.B) {
		api := apiPackage()
		_, err := s.Followed(api)
		assert.NoError(b, err, "the probes read before the measurement")
		c := bench.Start(b).MaxAllocs(followedAllocs)
		defer c.End()
		var got []int
		for c.Loop() {
			got, err = s.Followed(api)
		}
		assert.NoError(b, err, "the probes read")
		assert.Empty(b, got, "the scripted language follows no re-export")
	})

	b.Run("RecordLoad/a cold load of two units", func(b *testing.B) {
		// One record before the contract counts pools the region
		// encoding's scratch.
		assert.NoError(b, state.RecordLoad(b.Context(), state.NewCommit(nil, nil), nil, report),
			"the load records before the measurement")
		c := bench.Start(b).MaxAllocs(recordLoadAllocs)
		defer c.End()
		var (
			commit *state.Commit
			err    error
		)
		for c.Loop() {
			c.Excluding(func() { commit = state.NewCommit(nil, nil) })
			err = state.RecordLoad(b.Context(), commit, nil, report)
		}
		assert.NoError(b, err, "the load records")
	})
}

// scriptedTree returns the two packages the load cases load.
func scriptedTree() fstest.MapFS {
	return fstest.MapFS{
		apiFile:   {Data: []byte("package svc/api\ntype User string\n")},
		storeFile: {Data: []byte("package svc/store\nimport api svc/api\ntype Row api.User\n")},
	}
}

// loaded loads a tree through the scripted frontend, over a prior
// record where one is set.
func loaded(tb testing.TB, tree fstest.MapFS, prior load.Prior) *load.Report {
	tb.Helper()

	_, report, err := load.Load(context.Background(), load.Config{
		FS:        tree,
		Frontends: []plugin.Frontend{frontendtest.NewScripted()},
		Sink:      diag.NewSink(),
		Brand:     loadBrand,
		Prior:     prior,
	})
	assert.NoError(tb, err, "the tree loads")
	return report
}

// recordedLoad commits a load's record over the live generation of a
// ledger, nil for a ledger without one, and returns the generation the
// commit made live.
func recordedLoad(tb testing.TB, l ledger.Ledger, r *load.Report) *state.Generation {
	tb.Helper()

	parent, err := state.Open(tb.Context(), l)
	var prior *state.LoadState
	if err == nil {
		prior = parent.Load(tb.Context())
	} else {
		parent = nil
	}
	c := state.NewCommit(parent, nil)
	assert.NoError(tb, state.RecordLoad(tb.Context(), c, prior, r), "the load records")
	_, err = c.Write(tb.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(tb, err, "the commit writes")
	g, err := state.Open(tb.Context(), l)
	assert.NoError(tb, err, "and its generation opens")
	return g
}

// unitsOf returns a generation's unit records.
func unitsOf(t *testing.T, s *state.LoadState) []load.UnitRecord {
	t.Helper()

	var out []load.UnitRecord
	for u, err := range s.Units() {
		assert.NoError(t, err, "the units read")
		out = append(out, u)
	}
	return out
}

// filesOf returns a generation's file records.
func filesOf(t *testing.T, s *state.LoadState) []load.FileRecord {
	t.Helper()

	var out []load.FileRecord
	for f, err := range s.Files() {
		assert.NoError(t, err, "the files read")
		out = append(out, f)
	}
	return out
}

// userID is the bare identity the store package's reference names.
func userID() symbol.Identity {
	return symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/api", Name: "User"}
}

// loadStates returns n load states over a generation, none of which
// has read a table.
func loadStates(tb testing.TB, g *state.Generation, n int) []*state.LoadState {
	tb.Helper()

	out := make([]*state.LoadState, n)
	for i := range out {
		out[i] = g.Load(tb.Context())
	}
	return out
}

// lastUnit returns the record of a state's last unit, the store unit.
func lastUnit(tb testing.TB, s *state.LoadState) load.UnitRecord {
	tb.Helper()

	var last load.UnitRecord
	for u, err := range s.Units() {
		assert.NoError(tb, err, "the units read")
		last = u
	}
	return last
}

// apiPackage is the package identity of the API package, whose
// re-exports no reference of the scripted tree follows.
func apiPackage() symbol.Identity {
	return symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/api", Kind: symbol.KindPackage}
}
