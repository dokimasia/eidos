// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"crypto/sha256"
	"errors"
	"iter"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The paths the gate's cases add: a file no frontend claims, and a
// file in the own brand's state directory.
const (
	readmeFile = "README.md"
	stateFile  = ".own/manifest/ea.json"
)

// errUnreadable is the failure a damaged record returns.
var errUnreadable = errors.New("load_test: the record does not read")

// filed is a [load.Prior] over the file records of one earlier load and
// the anchor it took. It records no unit, no door and no probe, and a
// set err fails the file records.
type filed struct {
	anchor time.Time
	files  []load.FileRecord
	err    error
}

// Anchor returns the earlier load's anchor.
func (p filed) Anchor() time.Time { return p.anchor }

// Files returns the earlier load's file records, or the record's
// failure.
func (p filed) Files() iter.Seq2[load.FileRecord, error] {
	return func(yield func(load.FileRecord, error) bool) {
		if p.err != nil {
			yield(load.FileRecord{}, p.err)
			return
		}
		for _, f := range p.files {
			if !yield(f, nil) {
				return
			}
		}
	}
}

// Units returns no unit.
func (filed) Units() iter.Seq2[load.UnitRecord, error] {
	return func(func(load.UnitRecord, error) bool) {}
}

// Region returns the record's failure, because it records no unit.
func (filed) Region(load.UnitRecord) (*store.Region, error) { return nil, errUnreadable }

// Doors returns no door.
func (filed) Doors(plugin.ID) ([]load.DoorRecord, error) { return nil, nil }

// Probed returns no unit.
func (filed) Probed(symbol.Identity) ([]int, error) { return nil, nil }

// Followed returns no unit.
func (filed) Followed(symbol.Identity) ([]int, error) { return nil, nil }

// priorOf returns a record of one load's files under its anchor.
func priorOf(report *load.Report) filed {
	return filed{anchor: report.Anchor, files: report.Files}
}

// The gate decides what a warm load reads again, so each verdict and
// each comparison of a stat with a record is pinned.
func TestGate(t *testing.T) {
	t.Parallel()

	t.Run("walk", func(t *testing.T) {
		t.Parallel()

		t.Run("records every file of the tree in path order", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			var paths []string
			for _, f := range report.Files {
				paths = append(paths, f.Path)
			}
			assert.Equal(t, paths, []string{modFile, apiFile, depFile, storeFile}, "the records sort by path")
			assert.Equal(t, report.Statted, len(paths), "the gate statted each file once")
		})

		t.Run("skips the brand's state directory", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[stateFile] = &fstest.MapFile{Data: []byte("{}\n")}
			_, report, _ := loadTree(t, tree)
			for _, f := range report.Files {
				assert.False(t, strings.HasPrefix(f.Path, ".own/"), "the state directory is not the workspace's source")
			}
		})

		t.Run("returns the error of a record whose files do not read", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), func(cfg *load.Config) { cfg.Prior = filed{err: errUnreadable} })
			assert.ErrorIs(t, err, errUnreadable, "the error wraps the record's own")
		})
	})

	t.Run("judge", func(t *testing.T) {
		t.Parallel()

		t.Run("judges a claimed file an input", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			rec := recordOf(t, report, apiFile)
			assert.Equal(t, rec.Verdict, load.VerdictInput, "a claimed file is an input")
			assert.Equal(t, rec.Digest, sha256.Sum256(stdTree()[apiFile].Data), "with the digest of its bytes")
		})

		t.Run("judges the brand's own output an output", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[bulkFile] = &fstest.MapFile{Data: stamped(t, ownBrand, "package svc/bulk\ntype Bulk string\n")}
			_, report, _ := loadTree(t, tree)
			assert.Equal(t, recordOf(t, report, bulkFile).Verdict, load.VerdictOutput, "the trailer proves the output")
		})

		t.Run("judges a file no frontend claims unclaimed", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[readmeFile] = &fstest.MapFile{Data: []byte("# readme\n")}
			_, report, _ := loadTree(t, tree)
			rec := recordOf(t, report, readmeFile)
			assert.Equal(t, rec.Verdict, load.VerdictUnclaimed, "no selection claims the file")
			assert.Equal(t, rec.Digest, [sha256.Size]byte{}, "and the gate does not read it")
		})
	})

	t.Run("declare", func(t *testing.T) {
		t.Parallel()

		t.Run("records the package a loaded file is declared in", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			assert.Equal(t, recordOf(t, report, storeFile).Pkg, symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath, Kind: symbol.KindPackage,
			}, "the file's unit declared it in its package")
			assert.Equal(t, recordOf(t, report, modFile).Pkg, symbol.Identity{}, "no unit declared the module file")
		})
	})

	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()

		t.Run("hashes no file whose stat matches its record", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			_, warm, _ := loadTree(t, stdTree(), func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Hashed, 0, "every file's stat proves it unchanged")
			assert.Equal(t, warm.Files, cold.Files, "and keeps its record")
		})

		t.Run("hashes a file whose size moved", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			tree := stdTree()
			tree[apiFile] = &fstest.MapFile{Data: []byte("package svc/api\ntype User int\n")}
			_, warm, _ := loadTree(t, tree, func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Hashed, 1, "the one file whose size moved is hashed")
		})

		t.Run("hashes a file whose modification time moved", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			tree := stdTree()
			tree[apiFile].ModTime = time.Unix(1, 0)
			_, warm, _ := loadTree(t, tree, func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Hashed, 1, "the one file whose modification time moved is hashed")
			assert.Equal(t, recordOf(t, warm, apiFile).Digest, recordOf(t, cold, apiFile).Digest,
				"and its digest is the recorded one, because its bytes did not move")
		})

		t.Run("hashes a racily clean file", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			prior := priorOf(cold)
			prior.anchor = time.Time{}
			_, warm, _ := loadTree(t, stdTree(), func(cfg *load.Config) { cfg.Prior = prior })
			assert.Equal(t, warm.Hashed, 4,
				"no modification time precedes the zero anchor, so each file the load reads is hashed")
		})
	})

	t.Run("changed", func(t *testing.T) {
		t.Parallel()

		t.Run("reports no moved file for a cold load", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			assert.Nil(t, cold.Moved, "a cold load compares no record")
		})

		t.Run("reports no moved file for a tree whose stats match the record", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			_, warm, _ := loadTree(t, stdTree(), func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Empty(t, warm.Moved, "every record proves its file unchanged")
		})

		t.Run("reports a file whose modification time moved", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			tree := stdTree()
			tree[apiFile].ModTime = time.Unix(1, 0)
			_, warm, _ := loadTree(t, tree, func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Moved, []string{apiFile}, "the file's stat moved")
		})

		t.Run("reports an unclaimed file that the record lacks", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			tree := stdTree()
			tree[readmeFile] = &fstest.MapFile{Data: []byte("# readme\n")}
			_, warm, _ := loadTree(t, tree, func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Moved, []string{readmeFile}, "the file is new")
		})

		t.Run("lists the record of a file whose stat moved", func(t *testing.T) {
			t.Parallel()

			_, cold, _ := loadTree(t, stdTree())
			tree := stdTree()
			tree[apiFile].ModTime = time.Unix(1, 0)
			_, warm, _ := loadTree(t, tree, func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Was, []load.FileRecord{recordOf(t, cold, apiFile)}, "the record before the edit")
		})

		t.Run("lists the record of a recorded file that the walk did not find", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[readmeFile] = &fstest.MapFile{Data: []byte("# readme\n")}
			_, cold, _ := loadTree(t, tree)
			_, warm, _ := loadTree(t, stdTree(), func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Was, []load.FileRecord{recordOf(t, cold, readmeFile)}, "the record of the gone file")
		})

		t.Run("reports a recorded file that the walk did not find", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[readmeFile] = &fstest.MapFile{Data: []byte("# readme\n")}
			_, cold, _ := loadTree(t, tree)
			_, warm, _ := loadTree(t, stdTree(), func(cfg *load.Config) { cfg.Prior = priorOf(cold) })
			assert.Equal(t, warm.Vanished, []string{readmeFile}, "the file is gone")
		})
	})
}

// recordOf returns the gate's record of one path.
func recordOf(tb testing.TB, report *load.Report, path string) load.FileRecord {
	tb.Helper()

	at := slices.IndexFunc(report.Files, func(f load.FileRecord) bool { return f.Path == path })
	assert.NotEqual(tb, at, -1, "the gate records "+path)
	return report.Files[at]
}
