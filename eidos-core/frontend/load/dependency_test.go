// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The dependency tree: a workspace file importing a package the store
// declares, which imports a second package the store declares, and a
// third store package nothing imports.
const (
	appPath    = "svc/app"
	appFile    = "svc/app/main.zz"
	appName    = "App"
	libPath    = "ext/lib"
	libFile    = "ext/lib/lib.zz"
	libName    = "Lib"
	deepPath   = "ext/deep"
	deepFile   = "ext/deep/deep.zz"
	deepName   = "Deep"
	unusedFile = "ext/unused/unused.zz"
)

// The paths the cases that vary the dependency tree add: a package no
// store declares, a file of another frontend and the package it
// imports, the file a round's listing gains, and a member no store
// contains.
const (
	absentPath = "ext/absent"
	otherFile  = "other/x.zz"
	otherPath  = "ext/other"
	noteFile   = "ext/lib/notes.txt"
	goneFile   = "ext/gone/gone.zz"
)

// The directory whose three files import one package: the first and
// the third declare one package and the second another, so the
// packages list the files out of path order.
const (
	firstFile  = "svc/a/a.zz"
	secondFile = "svc/a/b.zz"
	thirdFile  = "svc/a/c.zz"
	onePath    = "svc/one"
	twoPath    = "svc/two"
)

// The selections that split a tree between two frontends: the
// dependent language claims the service tree, and the other frontend
// claims its own directory or the store's paths in the workspace.
const (
	serviceSelection = "svc/**/*.zz"
	otherSelection   = "other/**/*.zz"
	extSelection     = "ext/**/*.zz"
)

// The names a case composes a second frontend under, another
// language's name, and the shared input only that frontend declares.
const (
	otherID     = plugin.ID("other")
	otherLang   = symbol.Lang("other")
	otherShared = "other.mod"
)

// The dependency rounds load what the workspace imports and no store
// file more, so each round's needs, units and keys are pinned.
func TestDependency(t *testing.T) {
	t.Parallel()

	t.Run("dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a returned unit at signature depth", func(t *testing.T) {
			t.Parallel()

			g, report, sink := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, unitOf(t, report, member(libFile)).Depth, plugin.DepthSignatures,
				"the report records the dependency unit's depth")
			_, held := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: libPath, Name: hiddenName, Kind: symbol.KindConstant,
			})
			assert.False(t, held, "the signature-only parse skips the constant")
		})

		t.Run("resolves a workspace reference to a dependency's declaration", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			app, _ := g.Lookup(declIn(appPath, appName))
			assert.Equal(t, app.(*node.Struct).Fields[0].Type.Target, declIn(libPath, libName),
				"the import resolves to the store's declaration")
		})

		t.Run("runs no round for a frontend outside the dependent role", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, depTree(), stores(depStore()))
			assert.Length(t, report.Units, 1, "the workspace's own unit is the load's only unit")
		})

		t.Run("hands a later frontend no need an earlier frontend's rounds declared", func(t *testing.T) {
			t.Parallel()

			tree := depTree()
			tree[otherFile] = &fstest.MapFile{Data: []byte("package other\nimport ext " + libPath + "\n")}
			first := recorded()
			first.Sel = []string{serviceSelection}
			second := recorded()
			second.ID = otherID
			second.Sel = []string{otherSelection}
			loadTree(t, tree, with(first, second), stores(depStore()))
			assert.Empty(t, needPaths(second.rounds[0]), "the earlier frontend's rounds loaded the library")
		})
	})

	t.Run("reuse", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the recorded rounds of an unchanged tree", func(t *testing.T) {
			t.Parallel()

			prior := committed(t, loadOf(t, depTree(), with(recorded()), stores(depStore())).report)
			rec := recorded()
			warm := loadOf(t, depTree(), with(rec), stores(depStore()), func(cfg *load.Config) { cfg.Prior = prior })
			assert.Empty(t, rec.rounds, "every round would return its recorded units")
			assertSameLoad(t, warm, loadOf(t, depTree(), with(recorded()), stores(depStore())))
		})

		t.Run("reports the recorded rounds' findings again", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t, absentTree(), absentTree(), with(recorded()))
			coretest.AssertReports(t, warm.sink, load.UnplacedNeed)
			assertSameLoad(t, warm, cold)
		})

		t.Run("runs the rounds again where a unit's imports change", func(t *testing.T) {
			t.Parallel()

			after := depTree()
			after[appFile] = &fstest.MapFile{Data: []byte("package " + appPath + "\nimport ext " + libPath +
				" " + deepPath + "\ntype " + appName + " ext." + libName + "\n")}
			warm, cold := warmCold(t, depTree(), after, with(recorded()), stores(depStore()))
			assertSameLoad(t, warm, cold)
		})

		t.Run("runs the rounds again where a directory a round listed changes", func(t *testing.T) {
			t.Parallel()

			prior := committed(t, loadOf(t, depTree(), with(recorded()), stores(depStore())).report)
			grown := depStore()
			grown[path.Join(path.Dir(libFile), "more.zz")] = &fstest.MapFile{
				Data: []byte("package " + libPath + "\ntype More string\n"),
			}
			rec := recorded()
			warm := loadOf(t, depTree(), with(rec), stores(grown), func(cfg *load.Config) { cfg.Prior = prior })
			assert.NotEmpty(t, rec.rounds, "the listing of the library's directory moved")
			assertSameLoad(t, warm, loadOf(t, depTree(), with(recorded()), stores(grown)))
		})
	})

	t.Run("run", func(t *testing.T) {
		t.Parallel()

		t.Run("runs a round for the needs a dependency unit imports", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			loadTree(t, depTree(), with(rec), stores(depStore()))
			assert.Equal(t, rec.rounds[1].Needs, []plugin.Need{{Path: deepPath, From: []string{member(libFile)}}},
				"the library's import is the second round's need")
		})

		t.Run("runs the first round without a need", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			loadTree(t, stdTree(), with(rec))
			assert.Length(t, rec.rounds, 1, "the workspace declares every import, so one round runs")
			assert.Equal(t, rec.rounds[0].Number, 1, "the rounds count from one")
		})

		t.Run("ends the rounds at a round without a need", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			loadTree(t, depTree(), with(rec), stores(depStore()))
			assert.Length(t, rec.rounds, 2, "the deep package imports nothing, so no third round runs")
		})

		t.Run("records each unit's round in the report", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			rounds := map[string]int{}
			for _, u := range report.Units {
				rounds[u.Files[0].Path] = u.Round
			}
			assert.Equal(t, rounds, map[string]int{appFile: 0, member(libFile): 1, member(deepFile): 2},
				"a partition's unit is round zero, and a dependency unit is its round's number")
		})

		t.Run("passes a need to one round alone", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				appFile: {Data: []byte("package " + appPath + "\nimport ext " + libPath + " " + absentPath + "\n")},
			}
			rec := recorded()
			loadTree(t, tree, with(rec), stores(depStore()))
			assert.Equal(t, needPaths(rec.rounds[0]), []string{absentPath, libPath}, "the first round has both needs")
			assert.Equal(t, needPaths(rec.rounds[1]), []string{deepPath},
				"the store declares no absent package, and the second round is not asked again")
		})

		t.Run("re-keys a round's units when the round's listing changes", func(t *testing.T) {
			t.Parallel()

			_, base, _ := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			grown := depStore()
			grown[noteFile] = &fstest.MapFile{Data: []byte("not source\n")}
			_, moved, _ := loadTree(t, depTree(), with(recorded()), stores(grown))
			assert.NotEqual(t, keysOf(moved)[member(libFile)], keysOf(base)[member(libFile)],
				"the listing the round read folds into the unit's key")
		})

		t.Run("keeps the key of a later round's unit when an earlier round's listing changes", func(t *testing.T) {
			t.Parallel()

			_, base, _ := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			grown := depStore()
			grown[noteFile] = &fstest.MapFile{Data: []byte("not source\n")}
			_, moved, _ := loadTree(t, depTree(), with(recorded()), stores(grown))
			assert.Equal(t, keysOf(moved)[member(deepFile)], keysOf(base)[member(deepFile)],
				"a unit folds the reads of its own round alone")
		})

		t.Run("keeps a dependency unit's key when a workspace file changes", func(t *testing.T) {
			t.Parallel()

			_, base, _ := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			edited := depTree()
			edited[appFile] = &fstest.MapFile{Data: append(edited[appFile].Data, "// an edit\n"...)}
			_, moved, _ := loadTree(t, edited, with(recorded()), stores(depStore()))
			assert.Equal(t, keysOf(moved)[member(libFile)], keysOf(base)[member(libFile)],
				"the unit read nothing of the workspace")
		})

		t.Run("folds the frontend's version into a dependency unit's key", func(t *testing.T) {
			t.Parallel()

			_, base, _ := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			bumped := recorded()
			bumped.Ver += "+next"
			_, moved, _ := loadTree(t, depTree(), with(bumped), stores(depStore()))
			assert.NotEqual(t, keysOf(moved)[member(libFile)], keysOf(base)[member(libFile)],
				"a frontend's new version re-keys its dependency units")
		})

		t.Run("returns an error wrapping a dependency unit's parse error", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			rec.units = map[int][][]plugin.SourceRef{1: {{{Path: member(goneFile)}}}}
			err := refuse(t, depTree(), with(rec), stores(depStore()))
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error wraps the read's cause")
			assert.Contains(t, err.Error(), member(goneFile), "the error names the member")
		})

		t.Run("returns an error naming a frontend whose options cannot encode", func(t *testing.T) {
			t.Parallel()

			idleDependent := &unencodableDependent{recorded()}
			idleDependent.Sel = []string{otherSelection}
			err := refuse(t, depTree(), with(frontendtest.NewScripted(), idleDependent))
			assert.Contains(t, err.Error(), "encode",
				"a dependent frontend that claims nothing meets the encoding in its first round")
		})
	})

	t.Run("shared", func(t *testing.T) {
		t.Parallel()

		t.Run("hands every round the shared inputs the partition declared", func(t *testing.T) {
			t.Parallel()

			tree := depTree()
			tree[modFile] = &fstest.MapFile{Data: []byte("mod v1\n")}
			rec := recorded()
			loadTree(t, tree, with(rec), stores(depStore()))
			for _, round := range rec.rounds {
				assert.Equal(t, round.Shared, []string{modFile}, "the manifest is the workspace's shared input")
			}
		})

		t.Run("hands the rounds no shared input of another frontend's units", func(t *testing.T) {
			t.Parallel()

			tree := depTree()
			tree[otherFile] = &fstest.MapFile{Data: []byte("package other\n")}
			rec := recorded()
			rec.Sel = []string{serviceSelection}
			other := partitions(func(files []plugin.SourceRef) [][]plugin.SourceRef {
				unit := make([]plugin.SourceRef, len(files))
				for i, f := range files {
					unit[i] = plugin.SourceRef{Path: f.Path, Shared: []string{otherShared}}
				}
				return [][]plugin.SourceRef{unit}
			})
			other.ID = otherID
			other.Sel = []string{otherSelection}
			loadTree(t, tree, with(rec, other), stores(depStore()))
			assert.Empty(t, rec.rounds[0].Shared, "the other frontend's shared input is not the dependent's")
		})
	})

	t.Run("needs", func(t *testing.T) {
		t.Parallel()

		t.Run("hands the first round no need for a package the workspace declares", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			loadTree(t, stdTree(), with(rec))
			assert.Empty(t, rec.rounds[0].Needs, "the workspace declares the one package it imports")
		})

		t.Run("lists each file importing a need once in path order", func(t *testing.T) {
			t.Parallel()

			twice := "package " + twoPath + "\nimport x " + libPath + "\nimport y " + libPath + "\n"
			tree := fstest.MapFS{
				firstFile:  {Data: []byte("package " + onePath + "\nimport x " + libPath + "\n")},
				secondFile: {Data: []byte(twice)},
				thirdFile:  {Data: []byte("package " + onePath + "\nimport x " + libPath + "\n")},
			}
			rec := recorded()
			loadTree(t, tree, with(rec))
			assert.Equal(t, rec.rounds[0].Needs, []plugin.Need{
				{Path: libPath, From: []string{firstFile, secondFile, thirdFile}},
			}, "a file importing the need twice is listed once")
		})

		t.Run("names a need only another language declares", func(t *testing.T) {
			t.Parallel()

			tree := depTree()
			tree[libFile] = &fstest.MapFile{Data: []byte("package " + libPath + "\ntype " + libName + " string\n")}
			rec := recorded()
			rec.Sel = []string{serviceSelection}
			other := &otherLanguage{frontendtest.NewScripted()}
			other.ID = otherID
			other.Sel = []string{extSelection}
			loadTree(t, tree, with(rec, other))
			assert.Equal(t, needPaths(rec.rounds[0]), []string{libPath},
				"a package of another language declares nothing the import names")
		})

		t.Run("names no need another frontend's files import", func(t *testing.T) {
			t.Parallel()

			tree := depTree()
			tree[otherFile] = &fstest.MapFile{Data: []byte("package other\nimport o " + otherPath + "\n")}
			rec := recorded()
			rec.Sel = []string{serviceSelection}
			plain := frontendtest.NewScripted()
			plain.ID = plainID
			plain.Sel = []string{otherSelection}
			loadTree(t, tree, with(rec, plain), stores(depStore()))
			assert.Equal(t, needPaths(rec.rounds[0]), []string{libPath}, "the round asks for its own files' imports")
		})
	})

	t.Run("warnUnplaced", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnplacedNeed for a need the frontend places nowhere", func(t *testing.T) {
			t.Parallel()

			_, _, sink := loadTree(t, absentTree(), with(recorded()), stores(depStore()))
			coretest.AssertCodes(t, sink, load.UnplacedNeed)
		})

		t.Run("reports nothing for a round that places every need", func(t *testing.T) {
			t.Parallel()

			_, _, sink := loadTree(t, depTree(), with(recorded()), stores(depStore()))
			coretest.AssertCodes(t, sink)
		})

		t.Run("positions the finding at the need's import in the first file in path order", func(t *testing.T) {
			t.Parallel()

			_, _, sink := loadTree(t, absentTree(), with(recorded()), stores(depStore()))
			found, _ := findingOf(sink, load.UnplacedNeed)
			expect.Equal(t, found.Pos.File, firstFile, "the first file imports it")
			expect.Equal(t, found.Pos.Line, 2, "on its second line")
		})

		t.Run("positions the finding at the first import of the need in its file", func(t *testing.T) {
			t.Parallel()

			twice := "package " + onePath + "\nimport x " + absentPath + "\nimport y " + absentPath + "\n"
			_, _, sink := loadTree(t, fstest.MapFS{firstFile: {Data: []byte(twice)}}, with(recorded()),
				stores(depStore()))
			found, _ := findingOf(sink, load.UnplacedNeed)
			assert.Equal(t, found.Pos.Line, 2, "the file imports it on lines 2 and 3")
		})

		t.Run("names how many files import the need", func(t *testing.T) {
			t.Parallel()

			_, _, sink := loadTree(t, absentTree(), with(recorded()), stores(depStore()))
			found, _ := findingOf(sink, load.UnplacedNeed)
			assert.Contains(t, found.Msg, "imported by 3 of the load's files", "each importing file once")
		})

		t.Run("quotes the frontend's reason", func(t *testing.T) {
			t.Parallel()

			_, _, sink := loadTree(t, absentTree(), with(recorded()), stores(depStore()))
			found, _ := findingOf(sink, load.UnplacedNeed)
			assert.Contains(t, found.Msg, "no directory is at "+member(absentPath), "the frontend states why")
		})

		t.Run("reports a need once that the frontend reports twice", func(t *testing.T) {
			t.Parallel()

			again := &reporting{
				ScriptedDependent: frontendtest.NewScriptedDependent(),
				extra:             []plugin.Unplaced{{Path: absentPath, Reason: "it is reported again"}},
			}
			_, _, sink := loadTree(t, absentTree(), with(again), stores(depStore()))
			coretest.AssertCodes(t, sink, load.UnplacedNeed)
		})

		t.Run("returns an error naming a reported path that is no need of the round", func(t *testing.T) {
			t.Parallel()

			stray := &reporting{
				ScriptedDependent: frontendtest.NewScriptedDependent(),
				extra:             []plugin.Unplaced{{Path: otherPath, Reason: "nothing imports it"}},
			}
			err := refuse(t, depTree(), with(stray), stores(depStore()))
			assert.Contains(t, err.Error(), otherPath, "the error names the path")
			assert.Contains(t, err.Error(), "no need of the round", "a frontend reports only the round's needs")
		})
	})

	t.Run("admit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a unit without members", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			rec.units = map[int][][]plugin.SourceRef{1: {{}}}
			err := refuse(t, depTree(), with(rec), stores(depStore()))
			assert.Contains(t, err.Error(), "no members", "an empty unit has nothing to parse")
		})

		t.Run("returns an error naming a member the selection claims", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			rec.units = map[int][][]plugin.SourceRef{1: {{{Path: appFile}}}}
			err := refuse(t, depTree(), with(rec), stores(depStore()))
			assert.Contains(t, err.Error(), appFile, "the error names the member")
			assert.Contains(t, err.Error(), "the selection claims", "a dependency is never the workspace's source")
		})

		t.Run("returns an error naming a member the round returns twice", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			lib := []plugin.SourceRef{{Path: member(libFile)}}
			rec.units = map[int][][]plugin.SourceRef{1: {lib, lib}}
			err := refuse(t, depTree(), with(rec), stores(depStore()))
			assert.Contains(t, err.Error(), member(libFile), "the error names the member")
			assert.Contains(t, err.Error(), "twice", "every file is in exactly one unit")
		})

		t.Run("returns an error naming a unit that shares some of its members with loaded units", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			rec.units = map[int][][]plugin.SourceRef{2: {{{Path: member(libFile)}, {Path: member(deepFile)}}}}
			err := refuse(t, depTree(), with(rec), stores(depStore()))
			assert.Contains(t, err.Error(), "shares 1 of its 2 members", "the error counts the overlap")
		})

		t.Run("drops a unit whose every member is loaded", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			rec.units = map[int][][]plugin.SourceRef{2: {{{Path: member(libFile)}}, {{Path: member(deepFile)}}}}
			_, report, sink := loadTree(t, depTree(), with(rec), stores(depStore()))
			coretest.AssertCodes(t, sink)
			loaded := 0
			for _, u := range report.Units {
				if u.Files[0].Path == member(libFile) {
					loaded++
				}
			}
			assert.Equal(t, loaded, 1, "the library loads once, in the round that first returned it")
		})

		t.Run("returns an error wrapping the frontend's own", func(t *testing.T) {
			t.Parallel()

			rec := recorded()
			rec.err = errDependencies
			err := refuse(t, depTree(), with(rec), stores(depStore()))
			assert.ErrorIs(t, err, errDependencies, "the error wraps the frontend's cause")
			assert.Contains(t, err.Error(), "dependencies", "the error names the phase")
		})
	})
}

// depTree returns the workspace tree whose one file imports the
// store's library.
func depTree() fstest.MapFS {
	return fstest.MapFS{
		appFile: {Data: []byte("package " + appPath + "\nimport ext " + libPath +
			"\ntype " + appName + " ext." + libName + "\n")},
	}
}

// depStore returns the store the library, the package it imports and
// an unused package are in.
func depStore() fstest.MapFS {
	return fstest.MapFS{
		libFile: {Data: []byte("package " + libPath + "\nimport deep " + deepPath +
			"\ntype " + libName + " deep." + deepName + "\nconst " + hiddenName + "\n")},
		deepFile:   {Data: []byte("package " + deepPath + "\ntype " + deepName + " string\n")},
		unusedFile: {Data: []byte("package ext/unused\ntype Unused string\n")},
	}
}

// absentTree returns the directory whose three files import a package
// no store declares, the packages listing the files out of path order.
func absentTree() fstest.MapFS {
	return fstest.MapFS{
		firstFile:  {Data: []byte("package " + onePath + "\nimport x " + absentPath + "\n")},
		secondFile: {Data: []byte("package " + twoPath + "\nimport x " + absentPath + "\n")},
		thirdFile:  {Data: []byte("package " + onePath + "\nimport x " + absentPath + "\n")},
	}
}

// stores hands the load one store under the scripted dependent's
// store name.
func stores(tree fs.FS) func(*load.Config) {
	return func(cfg *load.Config) {
		cfg.Stores = map[string]fs.FS{frontendtest.ScriptedStore: tree}
	}
}

// member returns the qualified path of a file of the scripted
// dependent's store.
func member(path string) string { return plugin.StorePath(frontendtest.ScriptedStore, path) }

// declIn returns the identity of a struct one package declares.
func declIn(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: frontendtest.ScriptedLang, Package: pkg, Name: name, Kind: symbol.KindStruct}
}

// unitOf returns the report of the unit whose first member is a path.
func unitOf(tb testing.TB, report *load.Report, first string) load.UnitReport {
	tb.Helper()

	at := slices.IndexFunc(report.Units, func(u load.UnitReport) bool { return u.Files[0].Path == first })
	assert.NotEqual(tb, at, -1, "a unit of the report opens with "+first)
	return report.Units[at]
}

// needPaths returns the paths of a round's needs, in round order.
func needPaths(round *plugin.DependencyRound) []string {
	out := make([]string, len(round.Needs))
	for i, n := range round.Needs {
		out[i] = n.Path
	}
	return out
}

// errDependencies is what a failing round returns, so a case can
// follow the cause up through the driver.
var errDependencies = errors.New("load_test: the dependency round fails")

// fixedRounds is the scripted language in the dependent role with the
// units of each round the case fixes, recording every round the load
// hands it. A round the case fixes no units for returns what the
// scripted dependent returns, and a stated error fails every round.
// The load calls Dependencies from one goroutine, so the record needs
// no lock.
type fixedRounds struct {
	*frontendtest.ScriptedDependent
	units  map[int][][]plugin.SourceRef
	err    error
	rounds []*plugin.DependencyRound
}

// recorded returns the scripted language in the dependent role with
// no round fixed.
func recorded() *fixedRounds {
	return &fixedRounds{ScriptedDependent: frontendtest.NewScriptedDependent()}
}

// Dependencies records the round, then returns the stated error, the
// fixed units, or the scripted dependent's units.
func (f *fixedRounds) Dependencies(
	ctx context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	f.rounds = append(f.rounds, round)
	if f.err != nil {
		return nil, f.err
	}
	if units, fixed := f.units[round.Number]; fixed {
		return units, nil
	}
	return f.ScriptedDependent.Dependencies(ctx, round, r)
}

// reporting is the scripted language in the dependent role that adds
// the stated reports to each round after the scripted dependent's own.
type reporting struct {
	*frontendtest.ScriptedDependent
	extra []plugin.Unplaced
}

// Dependencies returns the scripted dependent's units and reports the
// stated needs placed nowhere.
func (f *reporting) Dependencies(
	ctx context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	units, err := f.ScriptedDependent.Dependencies(ctx, round, r)
	for _, u := range f.extra {
		round.Unplace(u.Path, u.Reason)
	}
	return units, err
}

// unencodableDependent declares options the driver cannot fold into a
// key, in the dependent role.
type unencodableDependent struct {
	*fixedRounds
}

// Options returns the unencodable declaration.
func (*unencodableDependent) Options() any { return &unencodableOptions{} }

// otherLanguage is the scripted grammar under another language's name.
type otherLanguage struct {
	*frontendtest.Scripted
}

// Lang returns the other language's name.
func (*otherLanguage) Lang() symbol.Lang { return otherLang }
