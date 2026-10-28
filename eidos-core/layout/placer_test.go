// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"path"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The directories that the out directives of the cases name, below the
// directory of the subject.
const (
	outDir = "gen/"
	pkgDir = "pkgout/"
	ownDir = "own/"
)

// placeAllocs is one preview of the per-source family of a subject without
// routing: the parts of the filename of the speller, their join and the
// path of the file.
const placeAllocs = 3

// The packages of the directories that the cases preview, as the package
// half directories names them.
var (
	storeDirPkg = dirPackage("svc/store")
	multiDirPkg = dirPackage("svc/multi/a")
)

// TestPlacer checks that a preview of a route gives the file of a unit
// the directory and the package that Route gives it, before the store is
// routed.
func TestPlacer(t *testing.T) {
	t.Parallel()

	t.Run("Place", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package that Route gives the file of each unit", func(t *testing.T) {
			t.Parallel()

			multi := coretest.PackageID(multiPkg)
			f := newFixture(
				stubOf(storeFile, storePkg, generated(storeID, "StoreStub")),
				stubOf(rowFile, storePkg, generated(rowID, "RowStub")),
				stubOf(cacheFile, cachePkg, generated(cacheID, "CacheStub")),
				unitOf(stubgen, families()[stubgen][2], multiPkg, multi, generated(zedID, "Suite")),
				unitOf(stubgen, families()[stubgen][3], "", symbol.Identity{}, generated(aceID, "Index")),
			).on(rowID, written(directive.KernelOut, 2, string(directive.OutPath), outDir))
			f.packager, f.config = directories{}, layout.Config{Dir: genDir}
			files, sink := f.route(t)
			assert.False(t, sink.Failed(), "the fixture routes clean")
			in, _ := f.input(t)
			p := layout.NewPlacer(in)
			assert.Length(t, files, 5, "the fixture routes one file for each unit")
			for _, file := range files {
				u := file.Units[0]
				origin, _ := emit.OriginOf(u.Decls[0])
				placed, err := p.Place(origin, previewOf(u))
				expect.NoError(t, err, "the preview of "+file.Path+" routes")
				expect.Equal(t, placed.Package, file.Pkg, "the preview of "+file.Path+" has the package of the file")
				expect.Equal(t, placed.Dir, path.Dir(file.Path), "the preview of "+file.Path+" has its directory")
			}
		})

		tests := []struct {
			name        string
			give        func() *fixture
			giveSubject symbol.Identity
			giveUnit    plugin.Unit
			want        plugin.Placed
		}{
			{
				name:        "returns the package of the directory of the source of the subject",
				give:        func() *fixture { return newFixture() },
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        plugin.Placed{Package: storeDirPkg, Origin: storeID, Dir: "svc/store"},
			},
			{
				name: "returns the package of the directory that the out directive of the subject names",
				give: func() *fixture {
					return newFixture().on(storeID, written(directive.KernelOut, 2, string(directive.OutPath), outDir))
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        plugin.Placed{Package: dirPackage("svc/store/gen"), Origin: storeID, Dir: "svc/store/gen"},
			},
			{
				name: "returns the package of the directory that the out directive of the package names",
				give: func() *fixture {
					return newFixture().onPackage(storePkg, written(directive.KernelOut, 1, string(directive.OutPath), pkgDir))
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want: plugin.Placed{
					Package: dirPackage("svc/store/pkgout"), Origin: storeID, Dir: "svc/store/pkgout",
				},
			},
			{
				name: "returns the package of the directory that the reserved out key of a directive of the plugin names",
				give: func() *fixture {
					return newFixture().on(storeID,
						written(stubDirective, 2, string(directive.ReservedOut), ownDir),
						written(directive.KernelOut, 3, string(directive.OutPath), outDir))
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        plugin.Placed{Package: dirPackage("svc/store/own"), Origin: storeID, Dir: "svc/store/own"},
			},
			{
				name:        "returns the package of the first directory of the package of a family of one file per package",
				give:        func() *fixture { return newFixture() },
				giveSubject: zedID,
				giveUnit: plugin.Unit{
					Plugin: stubgen, Tag: tagPkg, Per: plugin.PerPackage, Word: wordSuite, Key: multiPkg,
					Pkg: coretest.PackageID(multiPkg),
				},
				want: plugin.Placed{
					Package: multiDirPkg, Origin: zedID, Dir: "svc/multi/a", Files: coretest.PackageID(multiPkg),
				},
			},
			{
				name: "returns the package of the directory that the configuration gives a family of the plan",
				give: func() *fixture {
					f := newFixture()
					f.config = layout.Config{Dir: genDir}
					return f
				},
				giveUnit: plugin.Unit{Plugin: stubgen, Tag: tagPlan, Per: plugin.PerPlan, Word: wordIndex},
				want:     plugin.Placed{Package: dirPackage(genDir), Dir: genDir},
			},
			{
				name: "returns the package of the directory below the output directory of a centralised family",
				give: func() *fixture {
					f := newFixture()
					f.config = layout.Config{Policy: layout.PolicyCentralised, Dir: genDir}
					return f
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        plugin.Placed{Package: dirPackage("gen/svc/store"), Origin: storeID, Dir: "gen/svc/store"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := tt.give()
				f.packager = directories{}
				in, _ := f.input(t)
				got, err := layout.NewPlacer(in).Place(tt.giveSubject, tt.giveUnit)
				assert.NoError(t, err, "the preview routes")
				assert.Equal(t, got, tt.want, "the preview has the directory and the package of the file")
			})
		}

		t.Run("hands the target the path that the speller spells", func(t *testing.T) {
			t.Parallel()

			var seen []plugin.Placement
			f := newFixture()
			f.packager = recording{seen: &seen}
			in, _ := f.input(t)
			_, err := layout.NewPlacer(in).Place(storeID, stubUnit())
			assert.NoError(t, err, "the preview routes")
			assert.Length(t, seen, 1, "the target names one package")
			assert.Equal(t, seen[0].Path, "svc/store/store_stub.go", "the file has the name of the speller")
		})

		t.Run("hands the target the filename that the configuration fixes", func(t *testing.T) {
			t.Parallel()

			var seen []plugin.Placement
			f := newFixture()
			f.packager = recording{seen: &seen}
			fixed := map[layout.Family]layout.Refinement{{Plugin: stubgen}: {File: "fixed.go"}}
			f.config = layout.Config{Families: fixed}
			in, _ := f.input(t)
			_, err := layout.NewPlacer(in).Place(storeID, stubUnit())
			assert.NoError(t, err, "the preview routes")
			assert.Length(t, seen, 1, "the target names one package")
			assert.Equal(t, seen[0].Path, "svc/store/fixed.go", "the file has the name of the configuration")
		})

		external := []struct {
			name       string
			giveInside []symbol.Identity
			want       bool
		}{
			{name: "marks the placement of a family that tests from outside", want: true},
			{name: "clears the mark of a unit that tests from inside", giveInside: []symbol.Identity{storeID}},
		}
		for _, tt := range external {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var seen []plugin.Placement
				f := newFixture()
				f.outputs = map[plugin.ID][]plugin.Output{
					stubgen: {{Tag: tagTest, Per: plugin.PerSource, Word: wordStub, External: true}},
				}
				f.packager = recording{seen: &seen}
				in, _ := f.input(t)
				u := plugin.Unit{
					Plugin: stubgen, Tag: tagTest, Per: plugin.PerSource, Word: wordStub, Key: storeFile,
					Pkg: coretest.PackageID(storePkg), Inside: tt.giveInside,
				}
				_, err := layout.NewPlacer(in).Place(storeID, u)
				assert.NoError(t, err, "the preview routes")
				assert.Length(t, seen, 1, "the target names one package")
				assert.Equal(t, seen[0].External, tt.want, "the placement has the mark of the family")
			})
		}

		t.Run("returns the package of the unit for a placer without a packager", func(t *testing.T) {
			t.Parallel()

			in, _ := newFixture().input(t)
			got, err := layout.NewPlacer(in).Place(storeID, stubUnit())
			assert.NoError(t, err, "the preview routes")
			assert.Equal(t, got.Package, coretest.PackageID(storePkg), "the file has the package of the unit")
		})

		t.Run("returns the zero package for the file of a data family", func(t *testing.T) {
			t.Parallel()

			f := newFixture()
			f.outputs[stubgen][3].Name, f.outputs[stubgen][3].Template = dataName, claimTemplate
			f.config = layout.Config{Dir: genDir}
			f.packager = directories{}
			in, _ := f.input(t)
			u := plugin.Unit{Plugin: stubgen, Tag: tagPlan, Per: plugin.PerPlan, Word: wordIndex}
			got, err := layout.NewPlacer(in).Place(storeID, u)
			assert.NoError(t, err, "the preview routes")
			expect.Equal(t, got.Dir, genDir, "the file is in the directory of the plan")
			expect.Equal(t, got.Package, symbol.Identity{}, "a data file declares no package")
		})

		refused := []struct {
			name        string
			give        func() *fixture
			giveSubject symbol.Identity
			giveUnit    plugin.Unit
			want        string
			wantDir     string
		}{
			{
				name:        "returns an error for a subject without a workspace source",
				give:        func() *fixture { return newFixture() },
				giveSubject: depID,
				giveUnit: plugin.Unit{
					Plugin: stubgen, Per: plugin.PerSource, Word: wordStub, Key: depFile,
					Pkg: coretest.PackageID(depPkg),
				},
				want: "Struct Dep has no workspace source to be written beside",
			},
			{
				name: "returns an error for a path that leaves the workspace root",
				give: func() *fixture {
					return newFixture().on(storeID, written(directive.KernelOut, 2, string(directive.OutPath), "../../../x/"))
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        "is absolute or leaves the workspace root, and Struct Store is refused",
			},
			{
				name: "returns an error for a tag that names no family",
				give: func() *fixture {
					return newFixture().on(storeID, written(directive.KernelOut, 2, string(directive.OutTag), "nosuch"))
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        `tag "nosuch" names no family stubgen declares, and Struct Store is refused`,
			},
			{
				name: "returns the error of the package rule of the target",
				give: func() *fixture {
					f := newFixture()
					f.packager = directories{refuse: storePkg}
					return f
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        "no module contains svc/store",
				wantDir:     "svc/store",
			},
			{
				name: "returns an error for a filename that is not one path element",
				give: func() *fixture {
					f := newFixture()
					f.speller = nested{}
					return f
				},
				giveSubject: storeID,
				giveUnit:    stubUnit(),
				want:        `"sub/stub.go", which is not one path element`,
				wantDir:     "svc/store",
			},
			{
				name:        "returns an error for a family that the plugin does not declare",
				give:        func() *fixture { return newFixture() },
				giveSubject: storeID,
				giveUnit:    plugin.Unit{Plugin: stubgen, Tag: "nosuch", Per: plugin.PerSource, Key: storeFile},
				want:        `stubgen places into family "nosuch", which it does not declare`,
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				in, _ := tt.give().input(t)
				got, err := layout.NewPlacer(in).Place(tt.giveSubject, tt.giveUnit)
				assert.HasError(t, err, "the layout derives no package")
				assert.Contains(t, err.Error(), tt.want, "the preview states the reason of the layout")
				expect.Equal(t, got.Package, symbol.Identity{}, "a refused preview has no package")
				expect.Equal(t, got.Origin, tt.giveSubject, "a refused preview lists its origin")
				expect.Equal(t, got.Dir, tt.wantDir, "a refused preview lists the directory that it read")
			})
		}

		unspelled, _ := newFixture().input(t)
		unspelled.Speller = nil
		incomplete := []struct {
			name string
			give layout.Input
		}{
			{name: "returns an error for a placer without an index", give: layout.Input{Speller: spelling{}}},
			{name: "returns an error for a placer without a speller", give: unspelled},
		}
		for _, tt := range incomplete {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := layout.NewPlacer(tt.give).Place(storeID, stubUnit())
				assert.HasError(t, err, "the input lacks a half of the route")
				assert.Contains(t, err.Error(), "needs an index and a filename speller", "the error names the halves")
			})
		}
	})
}

// TestPlacerAllocs checks the ceiling of a preview in the ordinary run,
// which does not run a benchmark.
func TestPlacerAllocs(t *testing.T) {
	f := newFixture()
	f.packager = directories{}
	in, _ := f.input(t)
	p := layout.NewPlacer(in)
	u := stubUnit()
	var (
		got plugin.Placed
		err error
	)
	assert.MaxAllocs(t, func() { got, err = p.Place(storeID, u) }, placeAllocs,
		"a preview allocates the filename and the path of the file")
	assert.NoError(t, err, "the preview routes")
	assert.Equal(t, got.Package, storeDirPkg, "the preview has the package of the directory")
}

// BenchmarkPlacer measures one preview of the per-source family of a
// subject without routing.
func BenchmarkPlacer(b *testing.B) {
	b.Run("Place", func(b *testing.B) {
		f := newFixture()
		f.packager = directories{}
		in, _ := f.input(b)
		p := layout.NewPlacer(in)
		u := stubUnit()
		c := bench.Start(b).MaxAllocs(placeAllocs)
		defer c.End()
		var (
			got plugin.Placed
			err error
		)
		for c.Loop() {
			got, err = p.Place(storeID, u)
		}
		assert.NoError(b, err, "the preview routes")
		assert.Equal(b, got.Package, storeDirPkg, "the preview has the package of the directory")
	})
}

// stubUnit returns the unit of the primary family of stubgen for the
// source of Store, without declarations.
func stubUnit() plugin.Unit {
	return plugin.Unit{
		Plugin: stubgen, Per: plugin.PerSource, Word: wordStub, Key: storeFile, Pkg: coretest.PackageID(storePkg),
	}
}

// previewOf returns the unit of a routed file as a generator previews it:
// its family, its key and its package, without declarations.
func previewOf(u plugin.Unit) plugin.Unit {
	return plugin.Unit{
		Plugin: u.Plugin, Tag: u.Tag, Per: u.Per, Word: u.Word, Key: u.Key, Pkg: u.Pkg, Template: u.Template,
	}
}
