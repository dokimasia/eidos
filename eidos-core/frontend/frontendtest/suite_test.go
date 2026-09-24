// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/plugin"
)

// The fixture trees' paths, so a case names the file it perturbs
// rather than repeating a literal.
const (
	modFile      = "mod.zz"
	apiFile      = "svc/api/user.zz"
	storeFile    = "svc/store/row.zz"
	testFile     = "svc/store/row_test.zz"
	depFile      = "svc/dep/dep.zz"
	badFile      = "bad/oops.zz"
	notesFile    = "notes.txt"
	depRoot      = "svc/dep"
	serviceRoot  = "svc"
	absentRoot   = "vendor"
	carrierLine  = "gen:table name=users"
	apiSource    = "package svc/api\ntype User string\nmethod Get int\n"
	crossSource  = "package svc/store\nimport api svc/api\ntype Row api.User int\n"
	singleSource = "package svc/api\ntype User string\ntype Row User int\n"

	// carrierStatement is the carrier line as a scripted statement,
	// for a case appending one to a source the fixtures share.
	carrierStatement = "+" + carrierLine + "\n"

	// localStatement declares a type referencing another in its own
	// package, for a case that needs an in-package resolution.
	localStatement = "type Rows Row\n"
)

// singleFixture declares one package whose one type references
// another of its own.
func singleFixture() *frontendtest.Fixture {
	fx := plainFixture()
	fx.Sources = fstest.MapFS{apiFile: {Data: []byte(singleSource)}}
	return fx
}

// fixture is the scripted language's whole-contract tree: two
// packages, a cross-package reference, a builtin, members, a
// directive carrier, a classification stamp, a signature root and
// a broken file that reports.
func fixture() *frontendtest.Fixture {
	return &frontendtest.Fixture{
		Sources: fstest.MapFS{
			modFile: {Data: []byte("mod v1\n")},
			apiFile: {Data: []byte(apiSource)},
			storeFile: {Data: []byte(
				"package svc/store\nimport api svc/api\ntype Row api.User int\n+" +
					carrierLine + "\nconst rowmax\n",
			)},
			testFile: {Data: []byte(
				"package svc/store\nstamp fake.testFile yes\ntype RowTest string\n",
			)},
			depFile: {Data: []byte(
				"package svc/dep\ntype Dep string\nconst hidden\n",
			)},
			badFile: {Data: []byte("type Lost string\n")},
		},
		Signatures: []string{depRoot},
		Schemas:    frontendtest.ScriptedSchemas(),
		Keys:       frontendtest.ScriptedKeys,
	}
}

// plainFixture states the least a frontend can and still meet every
// check that is not optional: two packages and one cross-package
// reference, with no signature root, no schema and no stamp.
func plainFixture() *frontendtest.Fixture {
	return &frontendtest.Fixture{
		Sources: fstest.MapFS{
			apiFile:   {Data: []byte(apiSource)},
			storeFile: {Data: []byte(crossSource)},
		},
	}
}

// setup builds the scripted frontend over the whole-contract tree.
func setup(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontendtest.NewScripted(), fixture()
}

// setupOver builds the scripted frontend over the stated fixture.
func setupOver(fx *frontendtest.Fixture) frontendtest.Setup {
	return func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
		return frontendtest.NewScripted(), fx
	}
}

// failingFS is a tree whose named path refuses to open, so a walk
// or a read that meets it fails for a filesystem's own reason. It
// implements the one method, leaving Stat, ReadDir and ReadFile to
// the fallbacks in io/fs, which is what puts every access through
// the refusal.
type failingFS struct {
	tree fstest.MapFS
	fail string
}

// Open returns the tree's file, or a refusal for the named path.
func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.fail {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return f.tree.Open(name)
}

// The suite is the read side's conformance bar, so it must hold the
// scripted language, and it must state each check it leaves out
// rather than passing it against a fixture that says nothing.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunFrontendSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("holds the scripted language to every check", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, setup)
		})

		t.Run("skips the checks the fixture states nothing for", func(t *testing.T) {
			t.Parallel()

			// A fixture with no signature root loads nothing shallow
			// and one with no schema attaches nothing, so a suite that
			// ran either check here would fail rather than skip.
			frontendtest.RunFrontendSuite(t, setupOver(plainFixture()))
		})
	})
}
