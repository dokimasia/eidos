// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"io/fs"
	"path"
	"testing"

	"go.dokimi.dev/assert"

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
			parts, err := role.Dependencies(context.Background(), &plugin.DependencyRound{
				Number: 1,
				Needs:  []plugin.Need{{Path: "example.test/lib", From: []string{workspaceFile}}},
			}, listing{withCache()})
			assert.NoError(t, err, "the round returns")
			assert.Length(t, parts, 1, "one unit per need")
			_, _, qualified := plugin.CutStorePath(parts[0][0].Path)
			assert.True(t, qualified, "a dependency member is a store path")
		})
	})

	t.Run("Unplaced", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the needs Unplace reported in report order", func(t *testing.T) {
			t.Parallel()

			round := &plugin.DependencyRound{Number: 1, Needs: []plugin.Need{
				{Path: "example.test/b/" + missingName, From: []string{workspaceFile}},
				{Path: "example.test/lib", From: []string{workspaceFile}},
				{Path: "example.test/a/" + missingName, From: []string{workspaceFile}},
			}}
			_, err := depending{}.Dependencies(context.Background(), round, listing{withCache()})
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
