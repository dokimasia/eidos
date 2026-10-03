// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
)

// The holder file of the dual and barrel trees, which the kept-index
// cases edit.
const holdFile = "svc/hold/h.zz"

// editedStore returns the standard tree with the store package's file
// edited, so a warm load parses the store and keeps the rest.
func editedStore() fstest.MapFS {
	tree := stdTree()
	tree[storeFile] = &fstest.MapFile{Data: []byte(
		"package svc/store\nimport api svc/api\ntype Row api.User string\n+gen:table name=users\nconst rowmax\n",
	)}
	return tree
}

// The resolution step assigns the units a load parses, and looks the
// declarations of the units it keeps up through their regions, so a
// parsed reference resolves against kept declarations as against
// parsed ones.
func TestKept(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a parsed unit's reference to a kept unit's declaration", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, stdTree(), editedStore())
			assert.Equal(t, fromOf(warm.report)[apiFile], load.FromGeneration, "the declaring unit is kept")
			assertSameLoad(t, warm, cold)
		})

		t.Run("decodes the regions of the packages a candidate names and no other", func(t *testing.T) {
			t.Parallel()

			prior := countedOf(committed(t, loadOf(t, stdTree()).report))
			after := editedStore()
			after[storeFile].ModTime = editTime
			loadOf(t, after, func(cfg *load.Config) { cfg.Prior = prior })
			assert.Equal(t, prior.regions, map[string]int{apiFile: 1, storeFile: 1},
				"the store's own record and the package its reference names")
		})

		t.Run("follows a re-export through a kept unit's files", func(t *testing.T) {
			t.Parallel()

			before := barrelTree(leftPath)
			after := barrelTree(leftPath)
			after[holdFile] = &fstest.MapFile{Data: []byte(
				"package svc/hold\nimport dual " + barrelPath + "\ntype Holder dual.Thing string\n",
			)}
			warm, cold := warmCold(t, before, after, with(frontendtest.NewScriptedExporter()))
			assert.Equal(t, fromOf(warm.report)[barrelFile], load.FromGeneration, "the barrel is kept")
			assert.Equal(t, warm.report.Reparsed, 1, "and parses for its bindings alone")
			assertSameLoad(t, warm, cold)
		})

		t.Run("returns the error of a kept region a candidate needs that does not read", func(t *testing.T) {
			t.Parallel()

			prior := countedOf(committed(t, loadOf(t, stdTree()).report))
			prior.fail = apiFile
			after := editedStore()
			after[storeFile].ModTime = editTime
			err := refuse(t, after, func(cfg *load.Config) { cfg.Prior = prior })
			assert.ErrorIs(t, err, errUnreadable, "the lookup's failure returns after the link")
		})
	})
}
