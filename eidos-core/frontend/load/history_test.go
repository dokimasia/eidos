// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"encoding/hex"
	"io/fs"
	"iter"
	"maps"
	"slices"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// editTime is the modification time a file a case edits takes, which
// precedes every load's anchor and differs from every file's own.
var editTime = time.Unix(1, 0)

// lostFile is a file without a package line, whose unit reports the
// scripted language's one finding.
const lostFile = "bad/oops.zz"

// broken is the record of an earlier load whose units, regions or doors
// fail to read.
type broken struct {
	load.Prior
	units, regions, doors bool
}

// Units returns errUnreadable where the units fail, and the record's
// units otherwise.
func (b broken) Units() iter.Seq2[load.UnitRecord, error] {
	if b.units {
		return func(yield func(load.UnitRecord, error) bool) { yield(load.UnitRecord{}, errUnreadable) }
	}
	return b.Prior.Units()
}

// Region returns errUnreadable where the regions fail, and the record's
// region otherwise.
func (b broken) Region(u load.UnitRecord) (*store.Region, error) {
	if b.regions {
		return nil, errUnreadable
	}
	return b.Prior.Region(u)
}

// Doors returns errUnreadable where the doors fail, and the record's
// doors otherwise.
func (b broken) Doors(f plugin.ID) ([]load.DoorRecord, error) {
	if b.doors {
		return nil, errUnreadable
	}
	return b.Prior.Doors(f)
}

// counted is the record of an earlier load that counts each region it
// decodes, by the unit's first member, and fails the region of the unit
// whose first member is fail.
type counted struct {
	load.Prior
	fail    string
	mu      sync.Mutex
	regions map[string]int
}

// countedOf returns a counting record over a prior, failing no region.
func countedOf(p load.Prior) *counted {
	return &counted{Prior: p, regions: map[string]int{}}
}

// Region counts the decode, and returns errUnreadable for the failing
// unit and the record's region for any other.
func (c *counted) Region(u load.UnitRecord) (*store.Region, error) {
	c.mu.Lock()
	c.regions[u.Files[0].Path]++
	c.mu.Unlock()
	if u.Files[0].Path == c.fail {
		return nil, errUnreadable
	}
	return c.Prior.Region(u)
}

// recorder records loads one after another in a memory ledger's sealed
// state, as the runs of one workspace do, each over the generation the
// one before it made live.
type recorder struct {
	l     *ledger.Mem
	gen   *state.Generation
	prior *state.LoadState
}

// newRecorder returns a recorder over an empty ledger.
func newRecorder() *recorder { return &recorder{l: ledger.NewMem()} }

// record commits a load's report over the live generation and returns
// the record as the next load reads it.
func (r *recorder) record(tb testing.TB, report *load.Report) load.Prior {
	tb.Helper()

	ctx := tb.Context()
	c := state.NewCommit(r.gen, nil)
	assert.NoError(tb, state.RecordLoad(ctx, c, r.prior, report), "the load records")
	_, err := c.Write(ctx, r.l, state.Header{Anchor: report.Anchor}, manifest.Manifest{Version: manifest.Version})
	assert.NoError(tb, err, "the record commits")
	r.gen, err = state.Open(ctx, r.l)
	assert.NoError(tb, err, "and opens")
	r.prior = r.gen.Load(ctx)
	return r.prior
}

// loaded is one load's graph, report and sink, which a case compares
// with another load's.
type loaded struct {
	g      *store.Graph
	report *load.Report
	sink   *diag.Sink
}

// The record of a load is what the next load keeps, so how the load
// reads it, and what a record that does not read does, are pinned.
func TestHistory(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps every unit of an unchanged tree", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, stdTree(), stdTree())
			for _, u := range warm.report.Units {
				assert.Equal(t, u.From, load.FromGeneration, "each unit's key is the recorded one")
				assert.Nil(t, u.Region, "and its region is the record's")
			}
			assert.Equal(t, warm.report.Reparsed, 0, "no unit parses")
			assertSameLoad(t, warm, cold)
		})

		t.Run("keeps a unit whose key is the recorded one beside a unit it parses", func(t *testing.T) {
			t.Parallel()

			after := stdTree()
			after[storeFile] = &fstest.MapFile{Data: []byte(
				"package svc/store\nimport api svc/api\ntype Row api.User string\n+gen:table name=users\nconst rowmax\n",
			)}
			warm, cold := warmCold(t, stdTree(), after)
			assert.Equal(t, fromOf(warm.report), map[string]load.From{
				apiFile: load.FromGeneration, depFile: load.FromGeneration, storeFile: load.FromParse,
			}, "the one edited unit parses")
			assertSameLoad(t, warm, cold)
		})

		t.Run("reports a kept unit's findings again", func(t *testing.T) {
			t.Parallel()

			lost := "type Lost string\n"
			warm, cold := warmCold(t, stdTreeWith(lostFile, lost), stdTreeWith(lostFile, lost))
			assert.Equal(t, fromOf(warm.report)[lostFile], load.FromGeneration, "the unit with the finding is kept")
			coretest.AssertReports(t, warm.sink, frontendtest.ScriptedBadFile)
			assertSameLoad(t, warm, cold)
		})

		t.Run("decodes each recorded region once", func(t *testing.T) {
			t.Parallel()

			first := loadOf(t, twoUnitTree("type Twin left\n", "type Other right\n"))
			prior := countedOf(committed(t, first.report))
			after := twoUnitTree("type Twin left\n", "type Other right\ntype Twin right\n")
			after[twoFile].ModTime = editTime
			warm := loadOf(t, after, func(cfg *load.Config) { cfg.Prior = prior })
			assert.NotEmpty(t, prior.regions, "the coupled package's regions decode")
			for _, decodes := range prior.regions {
				assert.Equal(t, decodes, 1, "each region decodes once, whatever needs it")
			}
			assert.Equal(t, warm.report.Decoded(), len(prior.regions), "and the report counts each decode")
		})

		t.Run("returns the error of a record whose units do not read", func(t *testing.T) {
			t.Parallel()

			prior := broken{Prior: committed(t, loadOf(t, stdTree()).report), units: true}
			err := refuse(t, stdTree(), func(cfg *load.Config) { cfg.Prior = prior })
			assert.ErrorIs(t, err, errUnreadable, "the record's own error returns")
		})

		t.Run("returns the error of a record whose doors do not read", func(t *testing.T) {
			t.Parallel()

			prior := broken{Prior: committed(t, loadOf(t, stdTree()).report), doors: true}
			err := refuse(t, stdTree(), func(cfg *load.Config) { cfg.Prior = prior })
			assert.ErrorIs(t, err, errUnreadable, "the record's own error returns")
		})

		t.Run("returns the error of a record whose region does not read", func(t *testing.T) {
			t.Parallel()

			prior := broken{Prior: committed(t, loadOf(t, stdTree()).report), regions: true}
			after := stdTree()
			delete(after, apiFile)
			err := refuse(t, after, func(cfg *load.Config) { cfg.Prior = prior })
			assert.ErrorIs(t, err, errUnreadable, "the removed unit's region does not read")
		})
	})
}

// loadOf loads a tree under the standard configuration, mutated per
// case.
func loadOf(tb testing.TB, tree fs.FS, mutate ...func(*load.Config)) loaded {
	tb.Helper()

	g, report, sink := loadTree(tb, tree, mutate...)
	return loaded{g: g, report: report, sink: sink}
}

// committed returns a load's record as the next load reads it: the
// report recorded into a fresh memory ledger's sealed state, and opened
// again.
func committed(tb testing.TB, report *load.Report) load.Prior {
	tb.Helper()

	return newRecorder().record(tb, report)
}

// warmCold loads before cold, then after warm over the record of that
// load, and after cold beside it, each under the standard configuration
// mutated per case. Every file of after whose bytes differ from
// before's takes [editTime], as an editor's write moves a file's
// modification time.
func warmCold(tb testing.TB, before, after fstest.MapFS, mutate ...func(*load.Config)) (warm, cold loaded) {
	tb.Helper()

	for path, f := range after {
		if was, held := before[path]; !held || string(was.Data) != string(f.Data) {
			f.ModTime = editTime
		}
	}
	prior := committed(tb, loadOf(tb, before, mutate...).report)
	warm = loadOf(tb, after, append(slices.Clip(mutate), func(cfg *load.Config) { cfg.Prior = prior })...)
	cold = loadOf(tb, after, mutate...)
	return warm, cold
}

// assertSameLoad checks that a warm load left what a cold load of the
// same tree leaves: the same packages, byte for byte in the binary
// encoding, the same directives and stamps, the same units under the
// same keys, the same doors, and the same findings.
func assertSameLoad(tb testing.TB, warm, cold loaded) {
	tb.Helper()

	assert.Equal(tb, packagesOf(tb, warm.g), packagesOf(tb, cold.g), "the graphs encode the same packages")
	assert.Equal(tb, maps.Collect(warm.g.Directives()), maps.Collect(cold.g.Directives()),
		"and attach the same directives")
	assert.Equal(tb, maps.Collect(warm.g.Stamps()), maps.Collect(cold.g.Stamps()), "and the same stamps")
	assert.Equal(tb, keysIn(warm.report), keysIn(cold.report), "the loads have the same units under the same keys")
	assert.Equal(tb, warm.report.Doors, cold.report.Doors, "and record the same doors")
	assert.Permutation(tb, slices.Collect(warm.sink.All()), slices.Collect(cold.sink.All()),
		"and report the same findings")
}

// packagesOf returns the binary encoding of each package of a graph, by
// identity.
func packagesOf(tb testing.TB, g *store.Graph) map[symbol.Identity]string {
	tb.Helper()

	out := map[symbol.Identity]string{}
	for p := range g.Packages() {
		b, err := node.AppendBinary(nil, p, nil)
		assert.NoError(tb, err, "the package encodes")
		out[p.ID] = string(b)
	}
	return out
}

// keysIn returns each unit's first member and its key in hex, in splice
// order.
func keysIn(r *load.Report) []string {
	out := make([]string, len(r.Units))
	for i, u := range r.Units {
		out[i] = u.Files[0].Path + " " + hex.EncodeToString(u.Key)
	}
	return out
}

// fromOf returns where the load took each unit's region from, by its
// first member.
func fromOf(r *load.Report) map[string]load.From {
	out := make(map[string]load.From, len(r.Units))
	for _, u := range r.Units {
		out[u.Files[0].Path] = u.From
	}
	return out
}

// stdTreeWith returns the standard tree with one more file.
func stdTreeWith(path, body string) fstest.MapFS {
	tree := stdTree()
	tree[path] = &fstest.MapFile{Data: []byte(body)}
	return tree
}

// twoUnitTree returns one package the files of two directories declare,
// which the scripted language makes two units: the first unit's file
// with the first body, and the second's with the second.
func twoUnitTree(first, second string) fstest.MapFS {
	return fstest.MapFS{
		oneFile: {Data: []byte("package " + sharedPath + "\n" + first)},
		twoFile: {Data: []byte("package " + sharedPath + "\n" + second)},
	}
}
