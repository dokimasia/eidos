// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/store"
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

		t.Run("selects no kept reference again for an edit that keeps every declaration and name", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, stdTree(), withAPI("package svc/api\n// User is the user.\ntype User string\n"))
			assert.Equal(t, unitOf(t, warm.report, storeFile).Region, (*store.Region)(nil),
				"the store's reference names the same user")
			assert.Equal(t, warm.report.Decoded(), 1, "the load decodes the recorded region of the edited unit alone")
			assertSameLoad(t, warm, cold)
		})

		t.Run("keeps a kept unit's region whose followed re-export keeps its target", func(t *testing.T) {
			t.Parallel()

			before := twoRefBarrel()
			after := twoRefBarrel()
			after[barrelFile] = &fstest.MapFile{Data: append(publishing(barrelPath, leftPath), "type Spare int\n"...)}
			warm, cold := warmCold(t, before, after, with(frontendtest.NewScriptedExporter()))
			assert.Equal(t, unitOf(t, warm.report, holdFile).Region, (*store.Region)(nil),
				"each reference of the holder selects the target that it had")
			assertSameLoad(t, warm, cold)
		})

		t.Run("selects a kept reference again where the package of a candidate is renamed", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, namedDual(leftPath), namedDual(rightPath),
				with(frontendtest.NewScriptedQualifier()))
			assert.Equal(t, holderRef(t, warm.g, holdPath).Target, thingIn(rightPath),
				"the right package declares the alias after the edit")
			assert.NotNil(t, unitOf(t, warm.report, holdFile).Region, "the holder's region changed")
			assertSameLoad(t, warm, cold)
		})

		t.Run("admits a declaration of a kept package by the name that it declares", func(t *testing.T) {
			t.Parallel()

			after := namedDual(leftPath)
			after[holdFile] = &fstest.MapFile{Data: append(slices.Clone(after[holdFile].Data), "type Extra int\n"...)}
			warm, cold := warmCold(t, namedDual(leftPath), after, with(frontendtest.NewScriptedQualifier()))
			assert.Equal(t, unitOf(t, warm.report, "a/left/l.zz").From, load.FromGeneration, "the left package is kept")
			assert.Equal(t, holderRef(t, warm.g, holdPath).Target, thingIn(leftPath),
				"the kept left package declares the alias")
			assertSameLoad(t, warm, cold)
		})

		t.Run("admits a declaration of a kept package by the first name that its units declare", func(t *testing.T) {
			t.Parallel()

			before := fstest.MapFS{
				oneFile: {Data: []byte("package " + sharedPath + "\nname first\ntype " + thingName + " string\n")},
				twoFile: {Data: []byte("package " + sharedPath + "\nname second\n")},
				holdFile: {Data: []byte("package " + holdPath + "\nimport first " + sharedPath +
					"\ntype Holder first.Thing\n")},
			}
			after := maps.Clone(before)
			after[holdFile] = &fstest.MapFile{Data: append(slices.Clone(before[holdFile].Data), "type Extra int\n"...)}
			warm, cold := warmCold(t, before, after, with(frontendtest.NewScriptedQualifier()))
			assert.Equal(t, unitOf(t, warm.report, twoFile).From, load.FromGeneration, "the second unit is kept")
			assert.Equal(t, holderRef(t, warm.g, holdPath).Target, thingIn(sharedPath),
				"the first unit names the package, and the second unit's name is reported")
			assertSameLoad(t, warm, cold)
		})

		t.Run("passes the empty name of a parsed package without a name in a warm load", func(t *testing.T) {
			t.Parallel()

			after := dualTree(leftPath, rightPath)
			after[holdFile] = &fstest.MapFile{Data: append(slices.Clone(after[holdFile].Data), "type Extra int\n"...)}
			after["a/right/r.zz"] = &fstest.MapFile{Data: []byte("package a/right\ntype Thing string\ntype More int\n")}
			warm, cold := warmCold(t, dualTree(leftPath, rightPath), after,
				with(unnamed{frontendtest.NewScriptedQualifier()}))
			_, ambiguous := findingOf(warm.sink, load.AmbiguousReference)
			assert.True(t, ambiguous, "the qualifier admits both packages, which declare no name")
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

// twoRefBarrel returns the barrel tree of the left package with a holder
// of two references: the first follows the re-export of the barrel, and
// the second names a type of the holder's own package.
func twoRefBarrel() fstest.MapFS {
	tree := barrelTree(leftPath)
	tree[holdFile] = &fstest.MapFile{Data: []byte("package " + holdPath + "\nimport dual " + barrelPath +
		"\ntype Holder dual.Thing Local\ntype Local string\n")}
	return tree
}
