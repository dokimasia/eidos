// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"iter"
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

// createdFile is a file that a run wrote into the scripted tree after its
// load.
const createdFile = "svc/store/row.gen.zz"

// The key of a door's row, pinned: the frontend's name, doorKeySep, and
// the door's place as four big-endian bytes.
const doorKeySep = 0

// damagedPlace is the place of the door row that a damaged case adds,
// after every door of the scripted load.
const damagedPlace = 9

// undecodable is a row that every load table rejects: a varint cut short.
var undecodable = []byte{0x80}

// The ceilings of the reads and the record of a load over the scripted
// tree of two units.
const (
	// loadStateAllocs is one load state over a generation: the state.
	loadStateAllocs = 1
	// filesAllocs is a first range over the record's files, which reads
	// the files table whole: the run, its index, one list of the entries
	// of its blocks, the merged entries, the map of the strings the
	// state's decodes share, and the two records with their paths and
	// packages.
	filesAllocs = 12
	// unitsAllocs is a first range over the record's units, which reads
	// the units table whole: the run, its index, one list of the entries
	// of its blocks, the merged entries, the map of shared strings, and the
	// two records with their members, keys, summaries and imports, which
	// make one string of each distinct text.
	unitsAllocs = 30
	// regionAllocs is one decode of a unit's region of one package: the
	// run read from the ledger, the string table, the region, and the
	// package's declarations and lists.
	regionAllocs = 20
	// doorsAllocs is one read of the scripted frontend's doors after the
	// units: the doors table, read whole, and the one door's reads and
	// units. The door's paths reuse the strings of the units' decode.
	doorsAllocs = 9
	// probedAllocs is one lookup of a candidate that a unit named, after a
	// lookup before it: the list of the units' paths, the path, and the
	// list of numbers. The key is on the stack, and the run reader keeps
	// the block that it decoded last.
	probedAllocs = 3
	// followedAllocs is one lookup of a package that no reference
	// followed, after a lookup before it. The key is on the stack, and the
	// block that the row would be in is the one that the run reader kept,
	// so the lookup allocates nothing.
	followedAllocs = 0
	// recordLoadAllocs is one record of a cold load of the two units into
	// a new commit: each region's encoding and the segment, each file's
	// and unit's row, the door's row, and the probes.
	recordLoadAllocs = 84
	// recordOutputAllocs is one record of a created file into a commit
	// that records the file already: the key, the row, and the commit's
	// key of the change.
	recordOutputAllocs = 3
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

		t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
			t.Parallel()

			s := damagedLoad(t, state.TableFiles, []byte(extraFile)).Load(t.Context())
			assert.ErrorIs(t, rangeErr(s.Files()), state.ErrDamaged, "the files table does not read whole")
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

		t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
			t.Parallel()

			s := damagedLoad(t, state.TableUnits, []byte(extraFile)).Load(t.Context())
			assert.ErrorIs(t, rangeErr(s.Units()), state.ErrDamaged, "the units table does not read whole")
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

		t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
			t.Parallel()

			key := binary.BigEndian.AppendUint32(append([]byte(frontendtest.ScriptedID), doorKeySep), damagedPlace)
			_, err := damagedLoad(t, state.TableDoors, key).Load(t.Context()).Doors(frontendtest.ScriptedID)
			assert.ErrorIs(t, err, state.ErrDamaged, "the frontend's doors do not read whole")
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
			recorded := filesOf(t, g.Load(t.Context()))
			paths := make([]string, 0, len(recorded))
			for _, f := range recorded {
				paths = append(paths, f.Path)
			}
			assert.NotContains(t, paths, apiFile, "and so is its file's record")
		})

		t.Run("writes no row for a second load of the same tree", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := recordedLoad(t, l, loaded(t, scriptedTree(), nil))
			report := loaded(t, scriptedTree(), first.Load(t.Context()))
			for i := range report.Units {
				report.Units[i].Region = nil
			}
			var second *state.Generation
			assert.Pure(t, l.Writes, func() { second = recordedLoad(t, l, report) }, "nothing is written")
			assert.Equal(t, second.Name, first.Name, "because the record is unchanged")
		})
	})

	t.Run("RecordOutput", func(t *testing.T) {
		t.Parallel()

		t.Run("records the brand's output with a size of zero", func(t *testing.T) {
			t.Parallel()

			c := state.NewCommit(nil, nil)
			assert.NoError(t, state.RecordLoad(t.Context(), c, nil, loaded(t, scriptedTree(), nil)), "the load records")
			digest := sha256.Sum256([]byte(createdFile))
			state.RecordOutput(c, createdFile, digest)
			l := ledger.NewMem()
			_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and its generation opens")
			assert.Contains(t, filesOf(t, g.Load(t.Context())),
				load.FileRecord{Path: createdFile, Digest: digest, Verdict: load.VerdictOutput},
				"the next gate hashes the file where it meets it")
		})
	})
}

// The reads of a load's record and the record of a load allocate within
// their ceilings in the ordinary run, which runs no benchmark. A first
// range takes a load state made outside the count, because the state
// reads each table once, and a record takes a commit made outside it.
// Each count keeps the first error of its calls, which cmp.Or returns
// without allocating. The check runs alone, because the count includes
// every goroutine's allocations.
func TestLoadStateAllocs(t *testing.T) {
	report := loaded(t, scriptedTree(), nil)
	g := recordedLoad(t, ledger.NewMem(), report)
	var s *state.LoadState
	assert.MaxAllocs(t, func() { s = g.Load(t.Context()) }, loadStateAllocs, "Load allocates the state")
	var anchor time.Time
	assert.MaxAllocs(t, func() { anchor = s.Anchor() }, 0, "Anchor allocates nothing")
	assert.True(t, anchor.Equal(past), "Anchor returns the recording run's anchor")

	fresh := func() *state.LoadState { return g.Load(t.Context()) }
	var n int
	assert.MaxAllocsWithSetup(t, fresh, func(ls *state.LoadState) {
		n = 0
		for range ls.Files() {
			n++
		}
	}, filesAllocs, "a first range over Files reads the files table")
	assert.Equal(t, n, len(report.Files), "the range yields every file")
	assert.MaxAllocsWithSetup(t, fresh, func(ls *state.LoadState) {
		n = 0
		for range ls.Units() {
			n++
		}
	}, unitsAllocs, "a first range over Units reads the units table")
	assert.Equal(t, n, len(report.Units), "the range yields every unit")
	assert.MaxAllocs(t, func() {
		n = 0
		for range s.Files() {
			n++
		}
		for range s.Units() {
			n++
		}
	}, 0, "a range over tables read before allocates nothing")
	assert.Equal(t, n, len(report.Files)+len(report.Units), "the ranges yield every file and every unit")

	u := lastUnit(t, s)
	var (
		region *store.Region
		err    error
	)
	assert.MaxAllocs(t, func() {
		var rerr error
		region, rerr = s.Region(u)
		err = cmp.Or(err, rerr)
	}, regionAllocs, "Region allocates the decoded region")
	assert.NoError(t, err, "the region decodes")
	assert.Length(t, region.Packages, 1, "to the unit's package")
	var doors []load.DoorRecord
	assert.MaxAllocs(t, func() {
		var derr error
		doors, derr = s.Doors(frontendtest.ScriptedID)
		err = cmp.Or(err, derr)
	}, doorsAllocs, "Doors allocates the doors table and the doors")
	assert.NoError(t, err, "the doors read")
	assert.Equal(t, doors, report.Doors[frontendtest.ScriptedID], "the partition's record returns whole")
	user := userID()
	var probed []int
	assert.MaxAllocs(t, func() {
		var perr error
		probed, perr = s.Probed(user)
		err = cmp.Or(err, perr)
	}, probedAllocs, "Probed allocates the decoded paths and numbers")
	assert.NoError(t, err, "the probes read")
	assert.Equal(t, probed, []int{1}, "the store unit named the user")
	api := apiPackage()
	assert.MaxAllocs(t, func() {
		_, ferr := s.Followed(api)
		err = cmp.Or(err, ferr)
	}, followedAllocs, "a later Followed allocates nothing")
	assert.NoError(t, err, "the follows read")

	assert.MaxAllocsWithSetup(t, func() *state.Commit { return state.NewCommit(nil, nil) },
		func(c *state.Commit) { err = cmp.Or(err, state.RecordLoad(t.Context(), c, nil, report)) },
		recordLoadAllocs, "RecordLoad allocates the regions, the rows and the probes")
	assert.NoError(t, err, "every load records")

	digest := sha256.Sum256([]byte(createdFile))
	commit := state.NewCommit(nil, nil)
	state.RecordOutput(commit, createdFile, digest)
	assert.MaxAllocs(t, func() { state.RecordOutput(commit, createdFile, digest) }, recordOutputAllocs,
		"RecordOutput allocates the key, the row and the commit's key of the change")
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

	b.Run("Files", func(b *testing.B) {
		b.Run("a first range", func(b *testing.B) {
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

		b.Run("a range over a table read before", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			n := 0
			for c.Loop() {
				n = 0
				for range s.Files() {
					n++
				}
			}
			assert.Equal(b, n, len(report.Files), "the range yields every file")
		})
	})

	b.Run("Units", func(b *testing.B) {
		b.Run("a first range", func(b *testing.B) {
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

		b.Run("a range over a table read before", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			n := 0
			for c.Loop() {
				n = 0
				for range s.Units() {
					n++
				}
			}
			assert.Equal(b, n, len(report.Units), "the range yields every unit")
		})
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
		c := bench.Start(b).Warmup(1).MaxAllocs(probedAllocs)
		defer c.End()
		var (
			got []int
			err error
		)
		for c.Loop() {
			got, err = s.Probed(user)
		}
		assert.NoError(b, err, "the probes read")
		assert.Equal(b, got, []int{1}, "the store unit named the user")
	})

	b.Run("Followed", func(b *testing.B) {
		api := apiPackage()
		c := bench.Start(b).Warmup(1).MaxAllocs(followedAllocs)
		defer c.End()
		var (
			got []int
			err error
		)
		for c.Loop() {
			got, err = s.Followed(api)
		}
		assert.NoError(b, err, "the probes read")
		assert.Empty(b, got, "the scripted language follows no re-export")
	})

	b.Run("RecordLoad", func(b *testing.B) {
		b.Run("a cold load of two units", func(b *testing.B) {
			// The warm-up record pools the region encoding's scratch.
			c := bench.Start(b).Warmup(1).MaxAllocs(recordLoadAllocs)
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
	})

	b.Run("RecordOutput", func(b *testing.B) {
		digest := sha256.Sum256([]byte(createdFile))
		commit := state.NewCommit(nil, nil)
		state.RecordOutput(commit, createdFile, digest)
		c := bench.Start(b).MaxAllocs(recordOutputAllocs)
		defer c.End()
		for c.Loop() {
			state.RecordOutput(commit, createdFile, digest)
		}
		assert.NotNil(b, commit, "the commit receives the record")
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

	_, report, err := load.Load(tb.Context(), load.Config{
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

// damagedLoad commits the record of the scripted tree's load and one
// more row under key in a table, a row that does not decode, and returns
// the generation the commit made live.
func damagedLoad(t *testing.T, table state.Table, key []byte) *state.Generation {
	t.Helper()

	c := state.NewCommit(nil, nil)
	assert.NoError(t, state.RecordLoad(t.Context(), c, nil, loaded(t, scriptedTree(), nil)), "the load records")
	c.Put(table, key, undecodable)
	l := ledger.NewMem()
	_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(t, err, "the commit writes")
	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "and its generation opens")
	return g
}

// rangeErr ranges over seq to its end and returns the last error it
// yielded, nil for a range that yielded none.
func rangeErr[T any](seq iter.Seq2[T, error]) error {
	var last error
	for _, err := range seq {
		if err != nil {
			last = err
		}
	}
	return last
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
