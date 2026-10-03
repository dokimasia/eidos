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

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
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

// scriptedTree returns the two packages the load cases load.
func scriptedTree() fstest.MapFS {
	return fstest.MapFS{
		apiFile:   {Data: []byte("package svc/api\ntype User string\n")},
		storeFile: {Data: []byte("package svc/store\nimport api svc/api\ntype Row api.User\n")},
	}
}

// loaded loads a tree through the scripted frontend, over a prior
// record where one is set.
func loaded(t *testing.T, tree fstest.MapFS, prior load.Prior) *load.Report {
	t.Helper()

	_, report, err := load.Load(context.Background(), load.Config{
		FS:        tree,
		Frontends: []plugin.Frontend{frontendtest.NewScripted()},
		Sink:      diag.NewSink(),
		Brand:     loadBrand,
		Prior:     prior,
	})
	assert.NoError(t, err, "the tree loads")
	return report
}

// recordedLoad commits a load's record over the live generation of a
// ledger, nil for a ledger without one, and returns the generation the
// commit made live.
func recordedLoad(t *testing.T, l ledger.Ledger, r *load.Report) *state.Generation {
	t.Helper()

	parent, err := state.Open(t.Context(), l)
	var prior *state.LoadState
	if err == nil {
		prior = parent.Load(t.Context())
	} else {
		parent = nil
	}
	c := state.NewCommit(parent, nil)
	assert.NoError(t, state.RecordLoad(t.Context(), c, prior, r), "the load records")
	_, err = c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(t, err, "the commit writes")
	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "and its generation opens")
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
