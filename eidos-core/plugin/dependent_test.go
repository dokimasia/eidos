// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"io/fs"
	"path"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/plugin"
)

// The last element of a need's path the depending role reports placed
// nowhere, and the reason it reports the need with.
const (
	missingName    = "missing"
	unplacedReason = "the store has no such module"
)

// depending returns one unit per need, the shape of a language whose
// dependency grain is the imported package, and reports a need whose
// path ends in /missing placed nowhere.
type depending struct{}

// Dependencies returns one unit per need it places.
func (depending) Dependencies(
	_ context.Context, round *plugin.DependencyRound, _ plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	out := make([][]plugin.SourceRef, 0, len(round.Needs))
	for _, n := range round.Needs {
		if path.Base(n.Path) == missingName {
			round.Unplace(n.Path, unplacedReason)
			continue
		}
		out = append(out, []plugin.SourceRef{{Path: plugin.StorePath(cacheStore, n.Path+"/a.go")}})
	}
	return out, nil
}

// listing is a store reader over a tree with stores.
type listing struct {
	tree storeTree
}

// Read returns one file's bytes.
func (r listing) Read(path string) ([]byte, error) { return plugin.ReadFile(r.tree, path) }

// ReadDir returns one directory's entries.
func (r listing) ReadDir(path string) ([]fs.DirEntry, error) { return plugin.ReadDir(r.tree, path) }

// The dependency role's data contracts: what a round hands the
// frontend, and what the frontend returns for it.
func TestDependent(t *testing.T) {
	t.Parallel()

	t.Run("Dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns qualified members for the round's needs", func(t *testing.T) {
			t.Parallel()

			var role plugin.Dependent = depending{}
			parts, err := role.Dependencies(t.Context(), &plugin.DependencyRound{
				Number: 1,
				Needs:  []plugin.Need{{Path: "example.test/lib", From: []string{workspaceFile}}},
			}, listing{withCache()})
			assert.NoError(t, err, "the round returns")
			assert.Length(t, parts, 1, "one unit per need")
			_, _, qualified := plugin.CutStorePath(parts[0][0].Path)
			assert.True(t, qualified, "a dependency member is a store path")
		})
	})

	t.Run("Unplace", func(t *testing.T) {
		t.Parallel()

		t.Run("records the need with its reason", func(t *testing.T) {
			t.Parallel()

			round := &plugin.DependencyRound{Number: 1}
			round.Unplace("example.test/"+missingName, unplacedReason)
			assert.Equal(t, round.Unplaced(),
				[]plugin.Unplaced{{Path: "example.test/" + missingName, Reason: unplacedReason}},
				"the report keeps the need and the reason the finding quotes")
		})
	})

	t.Run("Unplaced", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the reports in report order", func(t *testing.T) {
			t.Parallel()

			round := &plugin.DependencyRound{Number: 1, Needs: []plugin.Need{
				{Path: "example.test/b/" + missingName, From: []string{workspaceFile}},
				{Path: "example.test/lib", From: []string{workspaceFile}},
				{Path: "example.test/a/" + missingName, From: []string{workspaceFile}},
			}}
			_, err := depending{}.Dependencies(t.Context(), round, listing{withCache()})
			assert.NoError(t, err, "the round returns")
			assert.Equal(t, round.Unplaced(), []plugin.Unplaced{
				{Path: "example.test/b/" + missingName, Reason: unplacedReason},
				{Path: "example.test/a/" + missingName, Reason: unplacedReason},
			}, "each report with its reason")
		})

		t.Run("returns nothing for a round without a report", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, (&plugin.DependencyRound{Number: 1}).Unplaced(), "the frontend placed every need")
		})
	})

	t.Run("ReadDir", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the entries of a store directory through the reader", func(t *testing.T) {
			t.Parallel()

			var r plugin.StoreReader = listing{withCache()}
			entries, err := r.ReadDir(plugin.StorePath(cacheStore, "example.test/lib@v1.0.0"))
			assert.NoError(t, err, "the directory lists")
			assert.Length(t, entries, 1, "the module's one file")
		})
	})
}

// A round's first report allocates the list of reports, and reading the
// reports allocates nothing, in the ordinary run, which runs no
// benchmark. Each counted report goes to a round of its own, built
// outside the count. The check runs alone, because the count includes
// every goroutine's allocations.
func TestDependentAllocs(t *testing.T) {
	var round *plugin.DependencyRound
	assert.MaxAllocsWithSetup(t, func() *plugin.DependencyRound { return &plugin.DependencyRound{Number: 1} },
		func(r *plugin.DependencyRound) {
			r.Unplace("example.test/"+missingName, unplacedReason)
			round = r
		}, 1, "Unplace allocates the list of reports on a round's first report")

	var got []plugin.Unplaced
	assert.MaxAllocs(t, func() { got = round.Unplaced() }, 0, "Unplaced allocates nothing")
	assert.Length(t, got, 1, "Unplaced returns the round's report")
}

// BenchmarkDependent measures a round's first report of a need placed
// nowhere, each on a round of its own, and the read of the reports.
func BenchmarkDependent(b *testing.B) {
	b.Run("Unplace", func(b *testing.B) {
		b.Run("the first report of a round", func(b *testing.B) {
			var round *plugin.DependencyRound
			fresh := func() { round = &plugin.DependencyRound{Number: 1} }
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			for c.Loop() {
				c.Excluding(fresh)
				round.Unplace("example.test/"+missingName, unplacedReason)
			}
			assert.Length(b, round.Unplaced(), 1, "the round keeps the report")
		})
	})

	b.Run("Unplaced", func(b *testing.B) {
		round := &plugin.DependencyRound{Number: 1}
		round.Unplace("example.test/"+missingName, unplacedReason)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []plugin.Unplaced
		for c.Loop() {
			got = round.Unplaced()
		}
		assert.Length(b, got, 1, "Unplaced returns the round's report")
	})
}
