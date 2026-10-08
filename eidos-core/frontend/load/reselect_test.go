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

// The API file's bodies the re-selection cases switch between: one
// without the user the store's reference names, and one with it.
const (
	apiWithoutUser = "package svc/api\ntype Account string\n"
	apiWithUser    = "package svc/api\ntype Account string\ntype User string\n"
)

// A kept unit's references select their targets again from the
// candidates their record names, so a declaration that appears or
// disappears moves a kept target as it moves a parsed one.
func TestReselect(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("clears a kept reference's target whose declaration disappears", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, stdTree(), withAPI(apiWithoutUser))
			store := unitOf(t, warm.report, storeFile)
			assert.Equal(t, store.From, load.FromGeneration, "the store's key is the recorded one")
			assert.NotNil(t, store.Region, "and its region changed")
			assertSameLoad(t, warm, cold)
		})

		t.Run("sets a kept reference's target whose declaration appears", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, withAPI(apiWithoutUser), withAPI(apiWithUser))
			assert.NotNil(t, unitOf(t, warm.report, storeFile).Region, "the store's region changed")
			assertSameLoad(t, warm, cold)
		})

		t.Run("keeps a kept unit's region whose candidates did not move", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, withAPI(apiWithUser), withAPI(apiWithUser+"type Extra int\n"))
			assert.Nil(t, unitOf(t, warm.report, storeFile).Region, "the store's target did not move")
			assertSameLoad(t, warm, cold)
		})

		t.Run("reports an ambiguity where a second candidate appears", func(t *testing.T) {
			t.Parallel()

			before := dualTree(leftPath, rightPath)
			before["a/right/r.zz"] = &fstest.MapFile{Data: []byte("package a/right\ntype Other string\n")}
			warm, cold := warmCold(t, before, dualTree(leftPath, rightPath))
			_, ambiguous := findingOf(warm.sink, load.AmbiguousReference)
			assert.True(t, ambiguous, "the holder's reference is ambiguous")
			assertSameLoad(t, warm, cold)
		})

		t.Run("clears a kept target a re-export reached when its declaration disappears", func(t *testing.T) {
			t.Parallel()

			after := barrelTree(leftPath)
			after["a/left/l.zz"] = &fstest.MapFile{Data: []byte("package a/left\ntype Other string\n")}
			warm, cold := warmCold(t, barrelTree(leftPath), after, with(frontendtest.NewScriptedExporter()))
			assert.NotNil(t, unitOf(t, warm.report, holdFile).Region, "the holder's region changed")
			assert.Equal(t, warm.report.Reparsed, 1, "one kept unit parses again for its bindings")
			assertSameLoad(t, warm, cold)
		})

		t.Run("follows a re-export again where the exporting file changes", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, barrelTree(leftPath), barrelTree(rightPath),
				with(frontendtest.NewScriptedExporter()))
			assert.NotNil(t, unitOf(t, warm.report, holdFile).Region, "the holder's region changed")
			assertSameLoad(t, warm, cold)
		})

		t.Run("selects a restored unit's references against the graph it joins", func(t *testing.T) {
			t.Parallel()

			memo := newRemembered()
			remember := func(cfg *load.Config) { cfg.Memo = memo }
			chain := newRecorder()
			first := loadOf(t, stdTree(), remember)
			second := withAPI(apiWithoutUser)
			second[apiFile].ModTime = editTime
			prior := chain.record(t, first.report)
			edited := loadOf(t, second, remember, func(cfg *load.Config) { cfg.Prior = prior })
			reverted := stdTree()
			reverted[apiFile].ModTime = editTime.Add(1)
			prior = chain.record(t, edited.report)
			warm := loadOf(t, reverted, remember, func(cfg *load.Config) { cfg.Prior = prior })
			assert.Equal(t, unitOf(t, warm.report, apiFile).From, load.FromMemo, "the reverted unit restores")
			assertSameLoad(t, warm, loadOf(t, reverted))
		})
	})
}

// withAPI returns the standard tree with the API file's body replaced.
func withAPI(body string) fstest.MapFS {
	tree := stdTree()
	tree[apiFile] = &fstest.MapFile{Data: []byte(body)}
	return tree
}
