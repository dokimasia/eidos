// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/symbol"
)

// The identities the change cases name in the standard tree's packages.
var (
	apiPackage   = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: apiPath, Kind: symbol.KindPackage}
	storePackage = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: storePath, Kind: symbol.KindPackage}
	accountID    = declIn(apiPath, "Account")
	userID       = declIn(apiPath, userName)
	apiFileID    = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: apiPath, Name: apiFile, Kind: symbol.KindFile,
	}
	rowFieldID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: storePath, Owner: rowName, Name: "f0", Kind: symbol.KindField,
	}
)

// A load states what it changed by identity, which the warm run's dirty
// set starts from, so each kind of change is pinned.
func TestChanges(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("reports nothing for a cold load", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, loadOf(t, stdTree()).report.Changes, "a cold load compares nothing")
		})

		t.Run("reports no change for an unchanged tree", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTree())
			assert.Equal(t, *warm.report.Changes, load.Changes{}, "nothing appeared, disappeared or changed")
		})

		t.Run("reports a declaration an edit adds as appeared", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), withAPI(apiWithUser))
			assert.Contains(t, warm.report.Changes.Appeared, accountID, "the new type appeared")
		})

		t.Run("reports a declaration an edit removes as disappeared", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), withAPI(apiWithoutUser))
			assert.Contains(t, warm.report.Changes.Disappeared, userID, "the removed type disappeared")
		})

		t.Run("reports the file that contains a changed declaration as changed", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), withAPI(apiWithUser))
			assert.Contains(t, warm.report.Changes.Changed, apiFileID, "the file's subtree changed")
		})

		t.Run("reports the package of a changed declaration", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), withAPI(apiWithUser))
			assert.Equal(t, warm.report.Changes.Packages, []symbol.Identity{apiPackage}, "the one package changed")
		})

		t.Run("reports a kept reference whose target moved as changed", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), withAPI(apiWithoutUser))
			assert.Contains(t, warm.report.Changes.Changed, rowFieldID, "the field's reference targets nothing now")
			assert.Contains(t, warm.report.Changes.Packages, storePackage, "and its package changed")
		})

		t.Run("reports the declarations of a removed unit as disappeared", func(t *testing.T) {
			t.Parallel()

			after := stdTree()
			delete(after, apiFile)
			warm, _ := warmCold(t, stdTree(), after)
			assert.Contains(t, warm.report.Changes.Disappeared, userID, "the removed unit's type disappeared")
			assert.Contains(t, warm.report.Changes.Disappeared, apiFileID, "and so did its file")
		})

		t.Run("reports a package whose own name changed alone", func(t *testing.T) {
			t.Parallel()

			shared := symbol.Identity{Lang: frontendtest.ScriptedLang, Package: sharedPath, Kind: symbol.KindPackage}
			named := func(name string) fstest.MapFS {
				body := namePrefix + name + "\npackage " + sharedPath + "\ntype A int\n"
				return fstest.MapFS{oneFile: {Data: []byte(body)}}
			}
			warm, _ := warmCold(t, named("left"), named("rite"), with(headed{frontendtest.NewScripted()}))
			assert.Empty(t, warm.report.Changes.Changed, "no declaration moved")
			assert.Equal(t, warm.report.Changes.Packages, []symbol.Identity{shared}, "and the package's name changed")
		})
	})
}
