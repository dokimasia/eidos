// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"cmp"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The files of the fake typeshed store of the round cases, and its table of
// versions: builtins and os in every version, the taskgroups of asyncio
// from Python 3.11, and distutils up to Python 3.11.
const (
	builtinsStub   = "stdlib/builtins.pyi"
	osInit         = "stdlib/os/__init__.pyi"
	osPathStub     = "stdlib/os/path.pyi"
	taskGroupsStub = "stdlib/asyncio/taskgroups.pyi"
	distutilsInit  = "stdlib/distutils/__init__.pyi"
	versionsTable  = "builtins: 3.0-\nos: 3.0-\nasyncio: 3.4-\nasyncio.taskgroups: 3.11-\ndistutils: 3.0-3.11\n"
)

// The modules that the round cases need, the files of the site-packages
// directories that place them, and the paths of a virtual environment
// beside a project file.
const (
	sixNeed       = "six"
	sixSource     = "six.py"
	sixStub       = "six.pyi"
	requestsNeed  = "requests"
	apiNeed       = "requests/api"
	stubsInit     = "requests-stubs/__init__.pyi"
	stubsAPI      = "requests-stubs/api.pyi"
	pkgNeed       = "pkg"
	pkgInit       = "pkg/__init__.py"
	pkgModule     = "pkg.py"
	osNeed        = "os"
	osPathNeed    = "os/path"
	taskGroups    = "asyncio/taskgroups"
	distutilsNeed = "distutils"
	unknownNeed   = "unknown"
	venvCfgPath   = ".venv/pyvenv.cfg"
	venvSiteDir   = ".venv/lib/python3.12/site-packages"
	windowsSite   = ".venv/Lib/site-packages/"
)

// The outputs of the brand in the workspace that the placement reports
// read: a module and the module of a package directory below the root of
// the project, the module of a package directory outside every project, a
// module in that directory, a module in a directory that is not a package
// directory, and the needs that the cases name.
const (
	stubsOutput    = "shop/stubs.py"
	fakesOutput    = "shop/fakes/__init__.py"
	testsInit      = "tests/__init__.py"
	testsOutput    = "tests/fakes.py"
	scriptsOutput  = "scripts/gen.py"
	shopNeed       = "shop"
	shopStubsNeed  = "shop/stubs"
	shopFakesNeed  = "shop/fakes"
	shopOtherNeed  = "shop/other"
	testsFakesNeed = "tests/fakes"
	genNeed        = "gen"
)

// The reasons with which a round reports a need that it places nowhere,
// with a site-packages directory and without one, for the version of the
// empty options.
const (
	unplacedReason = "neither typeshed for Python 3.15 nor a site-packages directory has the module"
	noVenvReason   = "typeshed for Python 3.15 does not have the module, and the load reads no virtual environment"
)

// The qualified paths of the units of typeshed that the round cases
// expect.
var (
	builtinsUnit   = plugin.StorePath(frontend.TypeshedStore, builtinsStub)
	osInitUnit     = plugin.StorePath(frontend.TypeshedStore, osInit)
	osPathUnit     = plugin.StorePath(frontend.TypeshedStore, osPathStub)
	taskGroupsUnit = plugin.StorePath(frontend.TypeshedStore, taskGroupsStub)
)

// errBroken is the error of the failing path of a failing file system.
var errBroken = errors.New("frontend_test: the file system is broken")

// roundReader is the door of a dependency round over a test workspace and
// its stores.
type roundReader struct {
	tree fs.FS
}

// Read returns the bytes of one file from the workspace or a store.
func (r roundReader) Read(p string) ([]byte, error) { return plugin.ReadFile(r.tree, p) }

// ReadDir returns the entries of one directory from the workspace or a
// store.
func (r roundReader) ReadDir(p string) ([]fs.DirEntry, error) { return plugin.ReadDir(r.tree, p) }

// failingFS is a file system in which one path fails to open with
// errBroken, an error that differs from the error of a missing file.
type failingFS struct {
	fs.FS
	fail string
}

// Open opens a path of the file system, and returns errBroken for the
// failing path.
func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.fail {
		return nil, errBroken
	}
	return f.FS.Open(name)
}

// TestDepend pins the dependency rounds: the units of typeshed and of the
// site-packages directories that place the needs, the needs that no step
// places, and the errors of the stores.
func TestDepend(t *testing.T) {
	t.Parallel()

	t.Run("dependencies", func(t *testing.T) {
		t.Parallel()

		withCfg := func(cfg, site string) fstest.MapFS {
			return fstest.MapFS{projectFile: {Data: []byte(rootProject)}, venvCfgPath: {Data: []byte(cfg)}, site: {}}
		}
		tests := []struct {
			name       string
			giveTree   fs.FS
			giveShared []string
			giveNumber int
			giveNeeds  []string
			want       [][]string
		}{
			{name: "returns builtins in the first round", giveTree: typeshedTree(), giveNumber: 1, want: [][]string{
				{builtinsUnit},
			}},
			{name: "returns no builtins after the first round", giveTree: typeshedTree(), giveNumber: 2},
			{
				name:       "returns the stub of a module that typeshed lists by its package",
				giveTree:   typeshedTree(),
				giveNumber: 2,
				giveNeeds:  []string{osPathNeed},
				want:       [][]string{{osPathUnit}},
			},
			{
				name:     "returns the stub of the package directory of a module that typeshed lists",
				giveTree: typeshedTree(), giveNumber: 2, giveNeeds: []string{osNeed}, want: [][]string{{osInitUnit}},
			},
			{
				name:     "returns a module that typeshed lists in the range of the version",
				giveTree: typeshedTree(), giveNumber: 2, giveNeeds: []string{taskGroups},
				want: [][]string{{taskGroupsUnit}},
			},
			{
				name:     "returns the package of stubs alone of a module",
				giveTree: venvTree(fstest.MapFS{stubsInit: {}}), giveNumber: 2, giveNeeds: []string{requestsNeed},
				want: [][]string{{plugin.StorePath(frontend.VenvStore, stubsInit)}},
			},
			{
				name:     "returns a module of a package of stubs alone",
				giveTree: venvTree(fstest.MapFS{stubsAPI: {}}), giveNumber: 2, giveNeeds: []string{apiNeed},
				want: [][]string{{plugin.StorePath(frontend.VenvStore, stubsAPI)}},
			},
			{
				name:       "returns the stub of a module before its source",
				giveTree:   venvTree(fstest.MapFS{sixSource: {}, sixStub: {}}),
				giveNumber: 2,
				giveNeeds:  []string{sixNeed},
				want:       [][]string{{plugin.StorePath(frontend.VenvStore, sixStub)}},
			},
			{
				name:       "returns a package directory before a module file",
				giveTree:   venvTree(fstest.MapFS{pkgInit: {}, pkgModule: {}}),
				giveNumber: 2,
				giveNeeds:  []string{pkgNeed},
				want:       [][]string{{plugin.StorePath(frontend.VenvStore, pkgInit)}},
			},
			{
				name:     "returns typeshed before a site-packages directory",
				giveTree: venvTree(fstest.MapFS{"os/__init__.py": {}}), giveNumber: 2, giveNeeds: []string{osNeed},
				want: [][]string{{osInitUnit}},
			},
			{
				name:     "returns a module once for two needs of the module",
				giveTree: venvTree(fstest.MapFS{sixSource: {}}), giveNumber: 2, giveNeeds: []string{sixNeed, sixNeed},
				want: [][]string{{plugin.StorePath(frontend.VenvStore, sixSource)}},
			},
			{
				name:       "returns a module of the virtual environment beside a project file",
				giveTree:   storeTree{withCfg(version312, venvSiteDir+"/"+sixSource), nil},
				giveShared: []string{projectFile}, giveNumber: 2, giveNeeds: []string{sixNeed},
				want: [][]string{{venvSiteDir + "/" + sixSource}},
			},
			{
				name:       "returns a module of a virtual environment of Windows beside a project file",
				giveTree:   storeTree{withCfg("version_info = 3.12.4.final.0\n", windowsSite+sixSource), nil},
				giveShared: []string{projectFile}, giveNumber: 2, giveNeeds: []string{sixNeed},
				want: [][]string{{windowsSite + sixSource}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := placed(t, nil, tt.giveTree, tt.giveShared, tt.giveNumber, tt.giveNeeds...)
				assert.Equal(t, got, tt.want, "the units of the round in the order of the needs", assert.EquateEmpty())
			})
		}

		unplaced := []struct {
			name       string
			giveOpts   *frontend.Options
			giveTree   fs.FS
			giveShared []string
			giveNeed   string
			want       []plugin.Unplaced
		}{
			{
				name:     "reports a need that neither typeshed nor a site-packages directory places",
				giveTree: venvTree(fstest.MapFS{}), giveNeed: unknownNeed,
				want: []plugin.Unplaced{{Path: unknownNeed, Reason: unplacedReason}},
			},
			{
				name:     "reports a need of a load without a virtual environment",
				giveTree: typeshedTree(), giveNeed: unknownNeed,
				want: []plugin.Unplaced{{Path: unknownNeed, Reason: noVenvReason}},
			},
			{
				name:     "reports a module of typeshed after the last version of its range",
				giveOpts: &frontend.Options{Version: "3.12"}, giveTree: typeshedTree(), giveNeed: distutilsNeed,
				want: []plugin.Unplaced{{Path: distutilsNeed, Reason: "typeshed for Python 3.12 does not have the " +
					"module, and the load reads no virtual environment"}},
			},
			{
				name:     "reports a module of typeshed before the first version of its range",
				giveOpts: &frontend.Options{Version: "3.10"}, giveTree: typeshedTree(), giveNeed: taskGroups,
				want: []plugin.Unplaced{{Path: taskGroups, Reason: "typeshed for Python 3.10 does not have the " +
					"module, and the load reads no virtual environment"}},
			},
			{
				name:       "reports a need beside a virtual environment whose pyvenv.cfg states no version",
				giveTree:   storeTree{withCfg("home = /usr/bin\n", venvSiteDir+"/"+sixSource), nil},
				giveShared: []string{projectFile}, giveNeed: sixNeed,
				want: []plugin.Unplaced{{Path: sixNeed, Reason: noVenvReason}},
			},
			{
				name:     "reports nothing for a need that a step places",
				giveTree: venvTree(fstest.MapFS{sixSource: {}}), giveNeed: sixNeed,
			},
		}
		for _, tt := range unplaced {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, round, err := runRound(t, tt.giveOpts, tt.giveTree, tt.giveShared, 2, tt.giveNeed)
				assert.NoError(t, err, "the round places what it can")
				assert.Equal(t, round.Unplaced(), tt.want, "the needs that the round places nowhere with their reasons",
					assert.EquateEmpty())
			})
		}

		inProject := storeTree{
			fstest.MapFS{projectFile: {Data: []byte(rootProject)}},
			map[string]fs.FS{frontend.TypeshedStore: typeshedStore()},
		}
		outside := storeTree{fstest.MapFS{testsInit: {}}, map[string]fs.FS{frontend.TypeshedStore: typeshedStore()}}
		generated := []struct {
			name        string
			giveTree    fs.FS
			giveOutputs []string
			giveNeed    string
			want        []plugin.Unplaced
		}{
			{
				name:     "reports nothing for a need of the module of an output",
				giveTree: inProject, giveOutputs: []string{stubsOutput}, giveNeed: shopStubsNeed,
			},
			{
				name:     "reports nothing for a need of the package above the module of an output",
				giveTree: inProject, giveOutputs: []string{stubsOutput}, giveNeed: shopNeed,
			},
			{
				name:     "reports nothing for a need of the module of an output in a package directory",
				giveTree: inProject, giveOutputs: []string{fakesOutput}, giveNeed: shopFakesNeed,
			},
			{
				name:     "reports nothing for a need of an output in a package directory outside every project",
				giveTree: outside, giveOutputs: []string{testsOutput}, giveNeed: testsFakesNeed,
			},
			{
				name:     "reports nothing for a need of an output in a directory that is not a package directory",
				giveTree: outside, giveOutputs: []string{scriptsOutput}, giveNeed: genNeed,
			},
			{
				name:     "reports a need of a module that no output has",
				giveTree: inProject, giveOutputs: []string{stubsOutput}, giveNeed: shopOtherNeed,
				want: []plugin.Unplaced{{Path: shopOtherNeed, Reason: noVenvReason}},
			},
		}
		for _, tt := range generated {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				round := &plugin.DependencyRound{
					Number: 2, Needs: []plugin.Need{{Path: tt.giveNeed}}, Outputs: tt.giveOutputs,
				}
				_, err := depend(t, nil, tt.giveTree, round)
				assert.NoError(t, err, "the round places what it can")
				assert.Equal(t, round.Unplaced(), tt.want, "the brand writes the modules of the outputs",
					assert.EquateEmpty())
			})
		}

		t.Run("returns the error of the workspace for the module of a package directory of an output",
			func(t *testing.T) {
				t.Parallel()

				round := &plugin.DependencyRound{
					Number: 2, Needs: []plugin.Need{{Path: testsFakesNeed}}, Outputs: []string{testsOutput},
				}
				_, err := depend(t, nil, failingFS{fstest.MapFS{testsInit: {}}, testsInit}, round)
				assert.ErrorIs(t, err, errBroken, "the round returns the failure of the read")
			})

		refused := []struct {
			name     string
			giveOpts *frontend.Options
			giveTree fs.FS
			want     string
		}{
			{
				name:     "returns an error for a version that is not a version of Python 3",
				giveOpts: &frontend.Options{Version: "2.7"}, giveTree: typeshedTree(), want: "2.7",
			},
			{
				name: "returns an error for a table of versions that does not parse",
				giveTree: storeTree{fstest.MapFS{}, map[string]fs.FS{frontend.TypeshedStore: fstest.MapFS{
					typeshed.VersionsFile: {Data: []byte("os 3.0-\n")},
				}}},
				want: "VERSIONS",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, _, err := runRound(t, tt.giveOpts, tt.giveTree, nil, 1)
				assert.HasError(t, err, "the round fails")
				assert.Contains(t, err.Error(), tt.want, "the error states its reason")
			})
		}

		broken := []struct {
			name       string
			giveTree   fs.FS
			giveShared []string
			giveNumber int
			giveNeed   string
		}{
			{
				name: "returns the error of the store for a table of versions that does not read",
				giveTree: storeTree{fstest.MapFS{}, map[string]fs.FS{
					frontend.TypeshedStore: failingFS{typeshedStore(), typeshed.VersionsFile},
				}},
			},
			{
				name: "returns the error of the store for the stub of builtins that does not read",
				giveTree: storeTree{fstest.MapFS{}, map[string]fs.FS{
					frontend.TypeshedStore: failingFS{typeshedStore(), builtinsStub},
				}},
				giveNumber: 1,
			},
			{
				name: "returns the error of the store for a stub that does not read",
				giveTree: storeTree{fstest.MapFS{}, map[string]fs.FS{
					frontend.TypeshedStore: failingFS{typeshedStore(), osPathStub},
				}},
				giveNeed: osPathNeed,
			},
			{
				name: "returns the error of the store for a venv store that does not list",
				giveTree: storeTree{fstest.MapFS{}, map[string]fs.FS{
					frontend.VenvStore: failingFS{fstest.MapFS{sixSource: {}}, "."},
				}},
			},
			{
				name:       "returns the error of the workspace for a pyvenv.cfg that does not read",
				giveTree:   failingFS{withCfg(version312, venvSiteDir+"/"+sixSource), venvCfgPath},
				giveShared: []string{projectFile},
			},
			{
				name:       "returns the error of the workspace for a site-packages directory that does not list",
				giveTree:   failingFS{withCfg(version312, venvSiteDir+"/"+sixSource), venvSiteDir},
				giveShared: []string{projectFile},
			},
		}
		for _, tt := range broken {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var needs []string
				if tt.giveNeed != "" {
					needs = append(needs, tt.giveNeed)
				}
				_, _, err := runRound(t, nil, tt.giveTree, tt.giveShared, cmp.Or(tt.giveNumber, 2), needs...)
				assert.ErrorIs(t, err, errBroken, "the round returns the failure of the read")
			})
		}
	})
}

// typeshedStore returns the fake typeshed store.
func typeshedStore() fstest.MapFS {
	return fstest.MapFS{
		typeshed.VersionsFile: {Data: []byte(versionsTable)},
		builtinsStub:          {}, osInit: {}, osPathStub: {}, taskGroupsStub: {}, distutilsInit: {},
	}
}

// typeshedTree returns an empty workspace beside the fake typeshed store.
func typeshedTree() storeTree {
	return storeTree{fstest.MapFS{}, map[string]fs.FS{frontend.TypeshedStore: typeshedStore()}}
}

// venvTree returns an empty workspace beside the fake typeshed store and a
// venv store.
func venvTree(venv fstest.MapFS) storeTree {
	return storeTree{
		fstest.MapFS{},
		map[string]fs.FS{frontend.TypeshedStore: typeshedStore(), frontend.VenvStore: venv},
	}
}

// runRound runs a round of a number through a Python frontend with
// options, over a workspace and its stores, with the shared inputs and the
// needs, under the context of the test. It returns the units of the round
// and the round, whose reports the load reads.
func runRound(
	tb testing.TB, opts *frontend.Options, tree fs.FS, shared []string, number int, needs ...string,
) ([][]plugin.SourceRef, *plugin.DependencyRound, error) {
	tb.Helper()

	round := &plugin.DependencyRound{Number: number, Shared: shared}
	for _, need := range needs {
		round.Needs = append(round.Needs, plugin.Need{Path: need})
	}
	units, err := depend(tb, opts, tree, round)
	return units, round, err
}

// depend runs a round through a Python frontend with options, over a
// workspace and its stores, under the context of the test.
func depend(
	tb testing.TB, opts *frontend.Options, tree fs.FS, round *plugin.DependencyRound,
) ([][]plugin.SourceRef, error) {
	tb.Helper()

	dependent, depends := frontend.New(opts).(plugin.Dependent)
	assert.True(tb, depends, "the Python frontend is a dependent")
	return dependent.Dependencies(tb.Context(), round, roundReader{tree})
}

// placed runs a round that succeeds in the case and returns the member
// paths of each unit.
func placed(
	tb testing.TB,
	opts *frontend.Options,
	tree fs.FS,
	shared []string,
	number int,
	needs ...string,
) [][]string {
	tb.Helper()

	units, _, err := runRound(tb, opts, tree, shared, number, needs...)
	assert.NoError(tb, err, "the round places its needs")
	out := make([][]string, 0, len(units))
	for _, unit := range units {
		members := make([]string, 0, len(unit))
		for _, ref := range unit {
			members = append(members, ref.Path)
		}
		out = append(out, members)
	}
	return out
}
