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
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture trees' paths and sources, so a case names the file it
// perturbs without repeating a literal.
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

	// setCarrier and negatedCarrier are the carrier line as a comment
	// writes it under the suite's brand, set and negated.
	setCarrier     = "+" + string(frontendtest.Brand) + ":" + carrierLine
	negatedCarrier = "-" + string(frontendtest.Brand) + ":" + carrierLine

	// localStatement declares a type referencing another in its own
	// package, for a case that needs an in-package resolution.
	localStatement = "type Rows Row\n"

	// depType and hiddenConst are what the signature root declares: a
	// type the signature-only load keeps, and a constant it drops.
	depType     = "Dep"
	hiddenConst = "hidden"
)

// The store the dependent fixtures read: a package the api file
// imports, its one scripted file, a file and a directory beside it
// that are not source, a package no store declares, and the api
// file's source that imports the package.
const (
	extPath       = "ext/lib"
	extMember     = "ext/lib/lib.zz"
	extNotes      = "ext/lib/notes.txt"
	extNested     = "ext/lib/nested.zz/inner.zz"
	extSource     = "package ext/lib\ntype Lib string\n"
	absentPackage = "ext/absent"
	usesSource    = "package svc/api\nimport ext ext/lib\ntype User ext.Lib\nmethod Get int\n"
)

// vendorRoot is the workspace directory the vendored language reads
// its dependencies from, which the scripted claim carves out, and
// vendoredMember is the library's file under it.
const (
	vendorRoot     = "skip"
	vendoredMember = "skip/ext/lib/lib.zz"
)

// The barrel that publishes the api package in the exporter fixture,
// the store file that spells User through the barrel, and the
// declaration that spelling names.
const (
	barrelFile   = "svc/barrel/index.zz"
	barrelSource = "package svc/barrel\nimport " + frontendtest.ScriptedPublish + " svc/api\n"
	viaSource    = "package svc/store\nimport b svc/barrel\ntype Row b.User int\n"
	apiPath      = "svc/api"
	userName     = "User"
)

// depIdentity returns the identity of a declaration in the
// signature root's package.
func depIdentity(name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: frontendtest.ScriptedLang, Package: depRoot, Name: name, Kind: kind}
}

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
		Dropped:    []symbol.Identity{depIdentity(hiddenConst, symbol.KindConstant)},
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

// dependentFixture is the plain fixture whose api file imports a
// package the one store declares.
func dependentFixture() *frontendtest.Fixture {
	return &frontendtest.Fixture{
		Sources: fstest.MapFS{
			apiFile:   {Data: []byte(usesSource)},
			storeFile: {Data: []byte(crossSource)},
		},
		Stores: map[string]fs.FS{
			frontendtest.ScriptedStore: fstest.MapFS{extMember: {Data: []byte(extSource)}},
		},
	}
}

// vendoredFixture is the dependent fixture with the library in the
// workspace's skip directory and no store.
func vendoredFixture() *frontendtest.Fixture {
	return &frontendtest.Fixture{
		Sources: fstest.MapFS{
			apiFile:        {Data: []byte(usesSource)},
			storeFile:      {Data: []byte(crossSource)},
			vendoredMember: {Data: []byte(extSource)},
		},
	}
}

// exporterFixture is the plain fixture whose store file names the api
// package's User through a barrel that publishes the package.
func exporterFixture() *frontendtest.Fixture {
	return &frontendtest.Fixture{
		Sources: fstest.MapFS{
			apiFile:    {Data: []byte(apiSource)},
			barrelFile: {Data: []byte(barrelSource)},
			storeFile:  {Data: []byte(viaSource)},
		},
		Reexported: []symbol.Identity{{
			Lang: frontendtest.ScriptedLang, Package: apiPath, Name: userName, Kind: symbol.KindStruct,
		}},
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

// The suite is the read side's conformance bar, so the scripted
// language passes it, and the suite skips each check a fixture
// states nothing for, as a named skip, instead of passing it.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunFrontendSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the scripted language through every check", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, setup)
		})

		t.Run("skips the checks the fixture states nothing for", func(t *testing.T) {
			t.Parallel()

			// A fixture with no signature root loads nothing shallow
			// and one with no schema attaches nothing, so a suite that
			// ran either check here would fail.
			frontendtest.RunFrontendSuite(t, setupOver(plainFixture()))
		})

		t.Run("passes the scripted language in the dependent role through every check", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return frontendtest.NewScriptedDependent(), dependentFixture()
			})
		})

		t.Run("passes the scripted language in the exporter role through every check", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return frontendtest.NewScriptedExporter(), exporterFixture()
			})
		})

		t.Run("skips the dependency check for a fixture without stores", func(t *testing.T) {
			t.Parallel()

			// The plain fixture imports nothing outside itself, so a
			// suite that ran the check here would fail it.
			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return frontendtest.NewScriptedDependent(), plainFixture()
			})
		})

		t.Run("skips the re-export check for a fixture that lists no re-exported declaration", func(t *testing.T) {
			t.Parallel()

			// The check fails a fixture that lists nothing, so a suite
			// that ran it here would fail.
			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return frontendtest.NewScriptedExporter(), plainFixture()
			})
		})
	})
}
