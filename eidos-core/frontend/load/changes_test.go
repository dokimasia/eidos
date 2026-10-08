// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
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

// The directives that the change cases attach to the row. usersTable is
// the row's directive in the standard tree, rowsTable has its spelling
// with another value, and viewMark has a second spelling.
const (
	usersTable = "+gen:table name=users"
	rowsTable  = "+gen:table name=rows"
	viewMark   = "+gen:view"
)

// The spellings of the row's two directives.
const (
	tableSpelling directive.Name = "gen:table"
	viewSpelling  directive.Name = "gen:view"
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

		t.Run("reports a subject whose directives an edit changed as directed", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTreeWith(storeFile, storeBody(rowsTable)))
			assert.Equal(t, warm.report.Changes.Directed, []symbol.Identity{rowID()}, "the row's directive changed")
		})

		t.Run("reports no spelling for an edit that keeps every spelling", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTreeWith(storeFile, storeBody(rowsTable)))
			assert.Empty(t, warm.report.Changes.Spellings, "the row still has one gen:table instance")
		})

		t.Run("reports a spelling that a subject gained", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTreeWith(storeFile, storeBody(usersTable, viewMark)))
			assert.Equal(t, warm.report.Changes.Spellings, []directive.Name{viewSpelling},
				"the row gained a gen:view instance")
		})

		t.Run("reports a spelling that a subject lost", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTreeWith(storeFile, storeBody()))
			assert.Equal(t, warm.report.Changes.Spellings, []directive.Name{tableSpelling},
				"the row lost its gen:table instance")
		})

		t.Run("reports a file whose stamps an edit changed as restamped", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), withAPI("package svc/api\ntype User string\nstamp fake.testFile yes\n"))
			assert.Equal(t, warm.report.Changes.Restamped, []symbol.Identity{apiFileID}, "the API file has a new stamp")
		})
	})
}

// storeBody returns the body of the standard tree's store file with the
// row's directive lines replaced by directives, one line each.
func storeBody(directives ...string) string {
	var lines string
	for _, d := range directives {
		lines += d + "\n"
	}
	return "package svc/store\nimport api svc/api\ntype Row api.User int\n" + lines + "const rowmax\n"
}
