// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"errors"
	"path"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture's generators and the families they declare, and two
// weavers that append into the generators' slots.
const (
	stubgen plugin.ID = "stubgen"
	docgen  plugin.ID = "docgen"
	pkggen  plugin.ID = "pkggen"
	auditor plugin.ID = "auditor"
	weaver  plugin.ID = "weaver"

	// tagTest is the per-source companion family of stubgen.
	tagTest = "test"
	// tagPkg is stubgen's per-package family.
	tagPkg = "pkg"
	// tagPlan is stubgen's per-plan family.
	tagPlan = "plan"
	// tagFile is the per-source companion of pkggen, whose primary
	// family is per-package.
	tagFile = "file"

	wordStub     = "stub"
	wordSuite    = "suite"
	wordIndex    = "index"
	wordDoc      = "doc"
	wordRegistry = "registry"
	wordPart     = "part"
)

// The fixture workspace: two packages in two directories, a package
// whose files span two directories, and one package of a dependency
// store.
const (
	storePkg  = "svc/store"
	cachePkg  = "svc/cache"
	multiPkg  = "svc/multi"
	depPkg    = "example.com/dep"
	storeFile = "svc/store/store.go"
	rowFile   = "svc/store/row.go"
	cacheFile = "svc/cache/cache.go"
	zedFile   = "svc/multi/z/zed.go"
	aceFile   = "svc/multi/a/ace.go"
	depFile   = "gomod://example.com/dep@v1.0.0/dep.go"
	genDir    = "gen"

	// stubDirective is stubgen's directive, docDirective docgen's and
	// regDirective pkggen's.
	stubDirective directive.Name = "stubgen:stub"
	docDirective  directive.Name = "docgen:doc"
	regDirective  directive.Name = "pkggen:reg"
)

// The canonical scale every layer benches at: per-package files of
// per-file declarations.
const (
	benchPackages = 1_000
	benchFiles    = 10
	benchDecls    = 20
)

// The ceilings of one routing pass, the same in each of 8 runs. Each
// routed file allocates three times, its path, the target's filename
// and the target's split, and no declaration allocates.
const (
	// routeAllocs is one pass over the canonical corpus: 30,000 for its
	// 10,000 files, and 177 for the pass's tables.
	routeAllocs = 30_177
	// routeOneAllocs is one pass over one package: 30 for its 10 files,
	// and 21 for the pass's tables.
	routeOneAllocs = 51
)

// The fixture's source declarations, one struct per source file.
var (
	storeID = coretest.ID(storePkg, "Store", symbol.KindStruct)
	rowID   = coretest.ID(storePkg, "Row", symbol.KindStruct)
	cacheID = coretest.ID(cachePkg, "Cache", symbol.KindStruct)
	zedID   = coretest.ID(multiPkg, "Zed", symbol.KindStruct)
	aceID   = coretest.ID(multiPkg, "Ace", symbol.KindStruct)
	depID   = coretest.ID(depPkg, "Dep", symbol.KindStruct)

	// ghostID names a declaration of a package the graph does not
	// contain.
	ghostID = coretest.ID("svc/ghost", "Ghost", symbol.KindStruct)
)

// source is one file of a fixture package, declaring one struct.
type source struct {
	file string
	id   symbol.Identity
}

// fixture is one routing case: the plan's units, the directives on
// the source declarations, and the routing inputs a case varies.
type fixture struct {
	units      []plugin.Unit
	directives plugin.ValidatedMap
	outputs    map[plugin.ID][]plugin.Output
	config     layout.Config
	speller    plugin.FileSpeller
	packager   plugin.Packager
	residents  map[string][]plugin.Resident
	modules    []plugin.Module
	others     plugin.Names
	// target is the plan's target, and zero for a plan that translates
	// no reference.
	target plugin.Target
	// unregistered routes without a directive registry.
	unregistered bool
}

// newFixture returns a case over the fixture families with the test
// target's speller and no packager.
func newFixture(units ...plugin.Unit) *fixture {
	return &fixture{
		units:      units,
		directives: plugin.ValidatedMap{},
		outputs:    families(),
		speller:    spelling{},
	}
}

// on attaches validated directives to a source declaration.
func (f *fixture) on(id symbol.Identity, ds ...directive.Directive) *fixture {
	f.directives[id] = append(f.directives[id], ds...)
	return f
}

// input returns the routing input over a settled store of the case's
// units, and the sink it reports into.
func (f *fixture) input(tb testing.TB) (layout.Input, *diag.Sink) {
	tb.Helper()

	ix, err := plugin.NewIndex(graph(tb), meta.NewFacts(meta.NewRegistry()), f.directives, nil)
	assert.NoError(tb, err, "the fixture index builds")
	e := plugin.NewEmit()
	assert.Total(tb, e.Add, f.units, "the fixture unit arrives")
	sink := diag.NewSink()
	var settler plugin.Backend
	if f.target != "" {
		settler = targeting{target: f.target}
	}
	assert.NoError(tb, plugin.Settle(e, settler, nil, sink), "a store without hooks settles as emitted")
	in := layout.Input{
		Emit:       e,
		Config:     f.config,
		Outputs:    f.outputs,
		Speller:    f.speller,
		Packager:   f.packager,
		Index:      ix,
		Directives: registry(tb),
		Modules:    f.modules,
		Others:     f.others,
		Target:     f.target,
		Sink:       sink,
	}
	if f.residents != nil {
		in.Residents = func(dir string) []plugin.Resident { return f.residents[dir] }
	}
	if f.unregistered {
		in.Directives = nil
	}
	return in, sink
}

// route routes the case and returns the files and the sink.
func (f *fixture) route(tb testing.TB) ([]plugin.File, *diag.Sink) {
	tb.Helper()

	in, sink := f.input(tb)
	files, err := layout.Route(in)
	assert.NoError(tb, err, "the fixture plan routes")
	coretest.AssertPositioned(tb, sink)
	return files, sink
}

// spelling is the test target's filename half: one file per unit,
// named by the stem of its file key, its word and its tag, joined by
// underscores.
type spelling struct{}

// SplitUnit returns the unit whole.
func (spelling) SplitUnit(u plugin.Unit) []plugin.Unit { return []plugin.Unit{u} }

// FileName joins the unit's stem, word and tag.
func (spelling) FileName(u plugin.Unit) string {
	var parts []string
	if key := u.FileKey(); key != "" {
		parts = append(parts, strings.TrimSuffix(path.Base(key), path.Ext(key)))
	}
	parts = append(parts, u.Word)
	if u.Tag != "" {
		parts = append(parts, u.Tag)
	}
	return strings.Join(parts, "_") + ".go"
}

// lean is the benchmark's target: one file per unit, named by its
// source stem and its family word in one allocation, as a target's
// spelling costs at least.
type lean struct{}

// SplitUnit returns the unit whole.
func (lean) SplitUnit(u plugin.Unit) []plugin.Unit { return []plugin.Unit{u} }

// FileName joins the unit's stem and word.
func (lean) FileName(u plugin.Unit) string {
	return strings.TrimSuffix(path.Base(u.Key), ".go") + "_" + u.Word + ".go"
}

// perDecl is a target that writes each declaration in a file of its
// own, named after the declaration.
type perDecl struct{}

// SplitUnit returns one part per declaration.
func (perDecl) SplitUnit(u plugin.Unit) []plugin.Unit {
	parts := make([]plugin.Unit, 0, len(u.Decls))
	for _, d := range u.Decls {
		part := u
		part.Decls = []symbol.Symbol{d}
		parts = append(parts, part)
	}
	return parts
}

// FileName returns the declaration's name.
func (perDecl) FileName(u plugin.Unit) string { return emit.DeclaredName(u.Decls[0]) + ".go" }

// lossy is a target whose split drops every declaration.
type lossy struct{ spelling }

// SplitUnit returns the unit without its declarations.
func (lossy) SplitUnit(u plugin.Unit) []plugin.Unit {
	u.Decls = nil
	return []plugin.Unit{u}
}

// nested is a target that spells a filename with a directory in it.
type nested struct{ spelling }

// FileName returns a path of two elements.
func (nested) FileName(plugin.Unit) string { return "sub/stub.go" }

// directories is the test target's package half: a file declares the
// package of its directory, named after the directory's last element,
// and a directory at or below refuse derives none.
type directories struct{ refuse string }

// PackageAt returns the directory's package.
func (d directories) PackageAt(p plugin.Placement) (symbol.Identity, error) {
	dir := path.Dir(p.Path)
	if d.refuse != "" && (dir == d.refuse || strings.HasPrefix(dir, d.refuse+"/")) {
		return symbol.Identity{}, errors.New("no module contains " + dir)
	}
	return symbol.Identity{Lang: coretest.Lang, Package: dir, Name: path.Base(dir), Kind: symbol.KindPackage}, nil
}

// targeting is a backend of one target without a hook, which settles a
// store as emitted and records whether the store translates.
type targeting struct{ target plugin.Target }

var _ plugin.Backend = targeting{}

// Name returns the backend's name.
func (targeting) Name() plugin.ID { return "targeting" }

// Target returns the backend's target.
func (b targeting) Target() plugin.Target { return b.target }

// recording is a target whose package half records each placement it
// is handed and returns the origin.
type recording struct{ seen *[]plugin.Placement }

// PackageAt records the placement.
func (r recording) PackageAt(p plugin.Placement) (symbol.Identity, error) {
	*r.seen = append(*r.seen, p)
	return p.Origin, nil
}

// A plan's declarations route to files by family, configuration and
// target, before anything renders.
func TestRoute(t *testing.T) {
	t.Parallel()

	t.Run("Route", func(t *testing.T) {
		t.Parallel()

		layouts := []struct {
			name string
			give func() *fixture
			want []string
		}{
			{
				name: "returns a per-source file beside its source",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
				},
				want: []string{"svc/store/store_stub.go: StoreStub"},
			},
			{
				name: "returns the files in path order",
				give: func() *fixture {
					return newFixture(
						stubOf(storeFile, storePkg, generated(storeID, "StoreStub")),
						stubOf(cacheFile, cachePkg, generated(cacheID, "CacheStub")),
					)
				},
				want: []string{"svc/cache/cache_stub.go: CacheStub", "svc/store/store_stub.go: StoreStub"},
			},
			{
				name: "returns a per-package file in its package directory without a stem",
				give: func() *fixture {
					return newFixture(unitOf(stubgen, families()[stubgen][2], storePkg,
						coretest.PackageID(storePkg), generated(storeID, "StoreSuite")))
				},
				want: []string{"svc/store/suite_pkg.go: StoreSuite"},
			},
			{
				name: "returns a per-package file in the first directory of a package that spans two",
				give: func() *fixture {
					return newFixture(unitOf(stubgen, families()[stubgen][2], multiPkg,
						coretest.PackageID(multiPkg), generated(zedID, "ZedSuite"), generated(aceID, "AceSuite")))
				},
				want: []string{"svc/multi/a/suite_pkg.go: ZedSuite AceSuite"},
			},
			{
				name: "returns a per-plan file in the plan's directory",
				give: func() *fixture {
					f := newFixture(unitOf(stubgen, families()[stubgen][3], "", symbol.Identity{},
						generated(storeID, "StoreIndex")))
					f.config = layout.Config{Dir: genDir}
					return f
				},
				want: []string{"gen/index_plan.go: StoreIndex"},
			},
			{
				name: "returns a centralised file under the output directory at its source directory",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
					f.config = layout.Config{Policy: layout.PolicyCentralised, Dir: genDir}
					return f
				},
				want: []string{"gen/svc/store/store_stub.go: StoreStub"},
			},
			{
				name: "returns a family's file under the family's own policy",
				give: func() *fixture {
					f := newFixture(
						stubOf(storeFile, storePkg, generated(storeID, "StoreStub")),
						unitOf(stubgen, families()[stubgen][1], storeFile, coretest.PackageID(storePkg),
							generated(storeID, "StoreStubTest")),
					)
					f.config = layout.Config{Families: map[layout.Family]layout.Refinement{
						{Plugin: stubgen, Tag: tagTest}: {Policy: layout.PolicyCentralised, Dir: genDir},
					}}
					return f
				},
				want: []string{
					"gen/svc/store/store_stub_test.go: StoreStubTest",
					"svc/store/store_stub.go: StoreStub",
				},
			},
			{
				name: "returns the filename the family's refinement names",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
					f.config = layout.Config{Families: map[layout.Family]layout.Refinement{
						{Plugin: stubgen}: {File: "stubs.go"},
					}}
					return f
				},
				want: []string{"svc/store/stubs.go: StoreStub"},
			},
			{
				name: "returns the filename the generator's refinement names",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
					f.config = layout.Config{Plugins: map[plugin.ID]layout.Refinement{
						stubgen: {File: "all.go"},
					}}
					return f
				},
				want: []string{"svc/store/all.go: StoreStub"},
			},
			{
				name: "returns two plugins' units at one path as one file in store order",
				give: func() *fixture {
					f := newFixture(
						stubOf(storeFile, storePkg, generated(storeID, "StoreStub")),
						unitOf(docgen, families()[docgen][0], storeFile, coretest.PackageID(storePkg),
							generated(storeID, "StoreDoc")),
					)
					f.config = layout.Config{Families: map[layout.Family]layout.Refinement{
						{Plugin: docgen}: {File: "store_stub.go"},
					}}
					return f
				},
				want: []string{"svc/store/store_stub.go: StoreDoc StoreStub"},
			},
			{
				name: "returns one file per part the target splits a unit into",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg,
						generated(storeID, "StoreStub"), generated(storeID, "StoreFake")))
					f.speller = perDecl{}
					return f
				},
				want: []string{"svc/store/StoreFake.go: StoreFake", "svc/store/StoreStub.go: StoreStub"},
			},
		}
		for _, tt := range layouts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := tt.give().route(t)
				assert.Equal(t, layoutOf(files), tt.want, "the routed files")
				coretest.AssertCodes(t, sink)
			})
		}

		t.Run("returns the package the target names for a file", func(t *testing.T) {
			t.Parallel()

			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
			f.packager = directories{}
			files, _ := f.route(t)
			assert.Length(t, files, 1, "one file routes")
			assert.Equal(t, files[0].Pkg, symbol.Identity{
				Lang: coretest.Lang, Package: storePkg, Name: "store", Kind: symbol.KindPackage,
			}, "the file declares its directory's package")
		})

		t.Run("returns the first unit's package for a file of a target without a packager", func(t *testing.T) {
			t.Parallel()

			files, _ := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub"))).route(t)
			assert.Length(t, files, 1, "one file routes")
			assert.Equal(t, files[0].Pkg, coretest.PackageID(storePkg), "the file declares its origin package")
		})

		t.Run("returns the zero package for a file the target derives none for", func(t *testing.T) {
			t.Parallel()

			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
			f.packager = directories{refuse: storePkg}
			files, sink := f.route(t)
			assert.Length(t, files, 1, "the file still routes")
			assert.Equal(t, files[0].Pkg, symbol.Identity{}, "the file declares no package")
			coretest.AssertCodes(t, sink)
		})

		t.Run("hands the target the placement of a file", func(t *testing.T) {
			t.Parallel()

			var seen []plugin.Placement
			residents := []plugin.Resident{{File: storeFile, Pkg: coretest.PackageID(storePkg)}}
			modules := []plugin.Module{{Lang: coretest.Lang, Path: "example.com/acme", Root: "."}}
			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
			f.packager = recording{seen: &seen}
			f.residents = map[string][]plugin.Resident{storePkg: residents}
			f.modules = modules
			f.config = layout.Config{Dir: genDir, ImportBase: "example.com/gen"}
			f.route(t)
			assert.Equal(t, seen, []plugin.Placement{{
				Path:       "svc/store/store_stub.go",
				Origin:     coretest.PackageID(storePkg),
				Residents:  residents,
				Modules:    modules,
				ImportBase: "example.com/gen",
				BaseDir:    genDir,
			}}, "the placement names the file, its origin, its directory's residents and the modules")
		})

		t.Run("hands the target no residents for an input without a residents function", func(t *testing.T) {
			t.Parallel()

			var seen []plugin.Placement
			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
			f.packager = recording{seen: &seen}
			f.route(t)
			assert.Length(t, seen, 1, "one file is placed")
			assert.Empty(t, seen[0].Residents, "no directory has residents")
		})

		t.Run("hands the target no base directory for a plan without an import base", func(t *testing.T) {
			t.Parallel()

			var seen []plugin.Placement
			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
			f.packager = recording{seen: &seen}
			f.config = layout.Config{Policy: layout.PolicyCentralised, Dir: genDir}
			f.route(t)
			assert.Length(t, seen, 1, "one file is placed")
			assert.Equal(t, seen[0].BaseDir, "", "the base directory is empty")
		})

		t.Run("returns a unit's origins for a file that contains the whole unit", func(t *testing.T) {
			t.Parallel()

			u := stubOf(storeFile, storePkg, generated(storeID, "StoreStub"))
			u.Origins = []symbol.Identity{rowID, storeID}
			files, _ := newFixture(u).route(t)
			assert.Length(t, files, 1, "one file routes")
			assert.Equal(t, files[0].Units[0].Origins, []symbol.Identity{rowID, storeID},
				"the source unit's provenance is kept")
		})

		t.Run("returns the origins of the declarations a tag moves", func(t *testing.T) {
			t.Parallel()

			f := newFixture(stubOf(storeFile, storePkg,
				generated(storeID, "StoreStub"), generated(rowID, "RowStub"))).
				on(rowID, written(stubDirective, 1, string(directive.ReservedTag), tagTest))
			files, _ := f.route(t)
			assert.Length(t, files, 2, "the moved declaration has a file of its own")
			assert.Equal(t, files[0].Units[0].Origins, []symbol.Identity{storeID}, "the primary file's provenance")
			assert.Equal(t, files[1].Units[0].Origins, []symbol.Identity{rowID}, "the moved file's provenance")
		})

		t.Run("returns the source units' origins for a file of two whole units", func(t *testing.T) {
			t.Parallel()

			f := newFixture(stubOf(rowFile, storePkg, generated(rowID, "RowStub")),
				unitOf(stubgen, families()[stubgen][2], storePkg, coretest.PackageID(storePkg),
					generated(storeID, "StoreSuite"))).
				on(rowID, written(stubDirective, 1, string(directive.ReservedTag), tagPkg))
			files, _ := f.route(t)
			assert.Length(t, files, 1, "both units route to the package's file")
			assert.Equal(t, files[0].Units[0].Origins, []symbol.Identity{rowID, storeID},
				"the provenance joins both units'")
		})

		t.Run("returns a unit's contributors for a file that contains the whole unit", func(t *testing.T) {
			t.Parallel()

			u := stubOf(storeFile, storePkg, generated(storeID, "StoreStub"))
			u.Contributors = []plugin.ID{weaver}
			files, _ := newFixture(u).route(t)
			assert.Length(t, files, 1, "one file routes")
			assert.Equal(t, files[0].Units[0].Contributors, []plugin.ID{weaver},
				"the weaver's attribution is kept")
		})

		t.Run("returns the source units' contributors for a file of two whole units", func(t *testing.T) {
			t.Parallel()

			row := stubOf(rowFile, storePkg, generated(rowID, "RowStub"))
			row.Contributors = []plugin.ID{weaver}
			suite := unitOf(stubgen, families()[stubgen][2], storePkg, coretest.PackageID(storePkg),
				generated(storeID, "StoreSuite"))
			suite.Contributors = []plugin.ID{auditor}
			f := newFixture(row, suite).
				on(rowID, written(stubDirective, 1, string(directive.ReservedTag), tagPkg))
			files, _ := f.route(t)
			assert.Length(t, files, 1, "both units route to the package's file")
			assert.Equal(t, files[0].Units[0].Contributors, []plugin.ID{auditor, weaver},
				"the attribution joins both units'")
		})

		t.Run("returns no origins for a unit whose declarations have none", func(t *testing.T) {
			t.Parallel()

			f := newFixture(stubOf(storeFile, storePkg,
				generated(symbol.Identity{}, "Loose"), generated(storeID, "StoreStub"))).
				on(storeID, written(stubDirective, 1, string(directive.ReservedTag), tagTest))
			files, _ := f.route(t)
			assert.Equal(t, layoutOf(files), []string{
				"svc/store/store_stub.go: Loose",
				"svc/store/store_stub_test.go: StoreStub",
			}, "the tagged declaration moves")
			assert.Empty(t, files[0].Units[0].Origins, "the loose declaration has no provenance")
		})

		t.Run("reports UndeclaredFamily for a unit of a family its plugin does not declare", func(t *testing.T) {
			t.Parallel()

			u := stubOf(storeFile, storePkg, generated(storeID, "StoreStub"))
			u.Tag = "bogus"
			files, sink := newFixture(u).route(t)
			assert.Empty(t, files, "the unit is refused")
			coretest.AssertCodes(t, sink, layout.UndeclaredFamily)
		})

		t.Run("reports UndeclaredFamily for a unit of a plugin that declares no family", func(t *testing.T) {
			t.Parallel()

			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub")))
			delete(f.outputs, stubgen)
			files, sink := f.route(t)
			assert.Empty(t, files, "the unit is refused")
			coretest.AssertCodes(t, sink, layout.UndeclaredFamily)
		})

		t.Run("reports UndeclaredFamily at the unit's first origin", func(t *testing.T) {
			t.Parallel()

			u := stubOf(storeFile, storePkg, generated(storeID, "StoreStub"))
			u.Tag = "bogus"
			_, sink := newFixture(u).route(t)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, at(storeFile), "the finding is at Store")
				assert.Equal(t, d.Origin, diag.PhaseLayout, "the layout reports it")
			}
		})

		t.Run("reports UndeclaredFamily at the unit's key for a unit without declarations", func(t *testing.T) {
			t.Parallel()

			u := stubOf(storeFile, storePkg)
			u.Tag = "bogus"
			_, sink := newFixture(u).route(t)
			coretest.AssertCodes(t, sink, layout.UndeclaredFamily)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, position.Pos{File: storeFile}, "the finding is at the unit's key")
			}
		})

		t.Run("reports NoDestination at the plugin for a per-source unit without a key", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(stubOf("", storePkg, generated(symbol.Identity{}, "Loose"))).route(t)
			assert.Empty(t, files, "nothing routes")
			coretest.AssertCodes(t, sink, layout.NoDestination)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, position.Pos{File: string(stubgen)}, "the finding is at the plugin")
			}
		})

		t.Run("reports NoDestination for a per-source unit keyed by a dependency file", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(stubOf(depFile, depPkg, generated(depID, "DepStub"))).route(t)
			assert.Empty(t, files, "nothing routes")
			coretest.AssertCodes(t, sink, layout.NoDestination)
		})

		t.Run("reports NoDestination for a declaration beside a redirected one without a source", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(stubOf(depFile, depPkg,
				generated(depID, "DepStub"), generated(storeID, "StoreStub"))).
				on(storeID, written(stubDirective, 1, string(directive.ReservedOut), "mocks/")).route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/store/mocks/dep_stub.go: StoreStub"},
				"the redirected declaration routes")
			coretest.AssertCodes(t, sink, layout.NoDestination)
		})

		t.Run("reports NoDestination for a per-package unit of a package with no workspace file", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(unitOf(stubgen, families()[stubgen][2], depPkg,
				coretest.PackageID(depPkg), generated(depID, "DepSuite"))).route(t)
			assert.Empty(t, files, "nothing routes")
			coretest.AssertCodes(t, sink, layout.NoDestination)
		})

		defects := []struct {
			name string
			give func(in *layout.Input)
		}{
			{name: "returns an error for no store", give: func(in *layout.Input) { in.Emit = nil }},
			{name: "returns an error for an unsettled store", give: func(in *layout.Input) {
				in.Emit = plugin.NewEmit()
			}},
			{name: "returns an error for no speller", give: func(in *layout.Input) { in.Speller = nil }},
			{name: "returns an error for no index", give: func(in *layout.Input) { in.Index = nil }},
			{name: "returns an error for no sink", give: func(in *layout.Input) { in.Sink = nil }},
			{name: "returns an error for a split that drops a declaration", give: func(in *layout.Input) {
				in.Speller = lossy{}
			}},
			{name: "returns an error for a filename of two path elements", give: func(in *layout.Input) {
				in.Speller = nested{}
			}},
		}
		for _, tt := range defects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				in, _ := newFixture(stubOf(storeFile, storePkg, generated(storeID, "StoreStub"))).input(t)
				tt.give(&in)
				_, err := layout.Route(in)
				assert.HasError(t, err, "the input is a defect")
			})
		}
	})
}

// Route allocates within its ceiling over one package of the canonical
// shape in the ordinary run, which runs no benchmark.
func TestRouteAllocs(t *testing.T) {
	in := benchInput(t, 1)
	var (
		files []plugin.File
		err   error
	)
	assert.MaxAllocs(t, func() { files, err = layout.Route(in) }, routeOneAllocs,
		"Route allocates each file's path, filename and split, and the pass's tables")
	assert.NoError(t, err, "the input routes")
	assert.Length(t, files, benchFiles, "to one file per source file")
}

// BenchmarkRoute measures one plan's routing: per-source units beside
// their sources, each file's package named by the target. No
// declaration allocates.
func BenchmarkRoute(b *testing.B) {
	b.Run("Route", func(b *testing.B) {
		b.Run("the canonical corpus of 200,000 declarations", func(b *testing.B) {
			benchRoute(b, benchPackages, routeAllocs)
		})

		b.Run("one package of 200 declarations", func(b *testing.B) {
			benchRoute(b, 1, routeOneAllocs)
		})
	})
}

// families returns the families each fixture generator declares.
func families() map[plugin.ID][]plugin.Output {
	return map[plugin.ID][]plugin.Output{
		stubgen: {
			{Tag: "", Per: plugin.PerSource, Word: wordStub},
			{Tag: tagTest, Per: plugin.PerSource, Word: wordStub},
			{Tag: tagPkg, Per: plugin.PerPackage, Word: wordSuite},
			{Tag: tagPlan, Per: plugin.PerPlan, Word: wordIndex},
		},
		docgen: {
			{Tag: "", Per: plugin.PerSource, Word: wordDoc},
		},
		pkggen: {
			{Tag: "", Per: plugin.PerPackage, Word: wordRegistry},
			{Tag: tagFile, Per: plugin.PerSource, Word: wordPart},
		},
	}
}

// at returns the position of a declaration on line 3 of a file.
func at(file string) position.Pos { return position.Pos{File: file, Line: 3, Col: 6} }

// sourceOf returns one source struct at line 3 of its file.
func sourceOf(id symbol.Identity, file string) *node.Struct {
	return &node.Struct{ID: id, Name: id.Name, Pos: at(file)}
}

// sourcePackage returns a package of one struct per file, its files
// in the order given.
func sourcePackage(pkg, name string, files ...source) *node.Package {
	p := &node.Package{ID: coretest.PackageID(pkg), Path: strings.Split(pkg, "/"), Name: name}
	for _, s := range files {
		p.Files = append(p.Files, &node.File{
			ID:    symbol.Identity{Lang: coretest.Lang, Package: pkg, Name: path.Base(s.file), Kind: symbol.KindFile},
			Path:  s.file,
			Decls: node.Symbols{sourceOf(s.id, s.file)},
		})
	}
	return p
}

// graph returns the fixture workspace, frozen.
func graph(tb testing.TB) *store.Graph {
	tb.Helper()

	return coretest.Frozen(tb,
		sourcePackage(storePkg, "store", source{storeFile, storeID}, source{rowFile, rowID}),
		sourcePackage(cachePkg, "cache", source{cacheFile, cacheID}),
		sourcePackage(multiPkg, "multi", source{zedFile, zedID}, source{aceFile, aceID}),
		sourcePackage(depPkg, "dep", source{depFile, depID}),
	)
}

// registry returns the schemas of the fixture generators' directives.
func registry(tb testing.TB) *directive.Registry {
	tb.Helper()

	r := directive.NewRegistry()
	assert.Total(tb, r.Register, []directive.Schema{
		{Plugin: string(stubgen), Name: "stub", Doc: "marks a declaration stubgen stubs"},
		{Plugin: string(docgen), Name: "doc", Doc: "marks a declaration docgen documents"},
		{Plugin: string(pkggen), Name: "reg", Doc: "marks a declaration pkggen registers"},
	}, "the fixture schema registers")
	return r
}

// written returns a validated directive instance at a carrier line,
// with its string params.
func written(name directive.Name, line int, params ...string) directive.Directive {
	d := directive.Directive{
		Name:   name,
		Pos:    position.Pos{File: storeFile, Line: line, Col: 1},
		Params: map[directive.ParamKey]directive.Value{},
	}
	for i := 0; i+1 < len(params); i += 2 {
		d.Params[directive.ParamKey(params[i])] = directive.Value{Kind: directive.TypeString, Str: params[i+1]}
	}
	return d
}

// generated returns an emitted struct deriving from origin.
func generated(origin symbol.Identity, name string, fields ...*emit.Field) *emit.Struct {
	s := &emit.Struct{Origin: origin, Name: name}
	for _, f := range fields {
		s.Fields.Append(f)
	}
	return s
}

// unitOf returns one unit of a generator's family, its declarations
// and their origins.
func unitOf(p plugin.ID, out plugin.Output, key string, pkg symbol.Identity, decls ...symbol.Symbol) plugin.Unit {
	u := plugin.Unit{Plugin: p, Tag: out.Tag, Per: out.Per, Word: out.Word, Key: key, Pkg: pkg, Decls: decls}
	seen := map[symbol.Identity]bool{}
	for _, d := range decls {
		if origin, _ := emit.OriginOf(d); !origin.IsZero() && !seen[origin] {
			seen[origin] = true
			u.Origins = append(u.Origins, origin)
		}
	}
	return u
}

// stubOf returns stubgen's primary unit of one source file.
func stubOf(file, pkg string, decls ...symbol.Symbol) plugin.Unit {
	return unitOf(stubgen, families()[stubgen][0], file, coretest.PackageID(pkg), decls...)
}

// layoutOf returns each file's path and the names of its declarations,
// unit by unit, which is what a case compares against.
func layoutOf(files []plugin.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		var names []string
		for _, u := range f.Units {
			for _, d := range u.Decls {
				names = append(names, emit.DeclaredName(d))
			}
		}
		out = append(out, f.Path+": "+strings.Join(names, " "))
	}
	return out
}

// benchRoute measures Route over an input of the given number of
// packages against a ceiling of allocs per pass.
func benchRoute(b *testing.B, packages int, allocs uint64) {
	b.Helper()

	in := benchInput(b, packages)
	c := bench.Start(b).MaxAllocs(allocs)
	defer c.End()
	var (
		files []plugin.File
		err   error
	)
	for c.Loop() {
		files, err = layout.Route(in)
	}
	assert.NoError(b, err, "the input routes")
	assert.Length(b, files, packages*benchFiles, "to one file per source file")
}

// benchTree returns a frozen graph of the given number of packages of
// the canonical shape, and the settled emit store of one per-source
// unit per source file, each declaration deriving from a source struct.
func benchTree(tb testing.TB, packages int) (*store.Graph, *plugin.Emit) {
	tb.Helper()

	g := store.New()
	e := plugin.NewEmit()
	for i := range packages {
		pkg := storePkg + strconv.Itoa(i)
		p := &node.Package{ID: coretest.PackageID(pkg), Path: strings.Split(pkg, "/"), Name: path.Base(pkg)}
		for f := range benchFiles {
			file := pkg + "/unit" + strconv.Itoa(f) + ".go"
			decls := make(node.Symbols, 0, benchDecls)
			emitted := make([]symbol.Symbol, 0, benchDecls)
			for d := range benchDecls {
				id := coretest.ID(pkg, "Decl"+strconv.Itoa(f)+"_"+strconv.Itoa(d), symbol.KindStruct)
				decls = append(decls, sourceOf(id, file))
				emitted = append(emitted, generated(id, id.Name+"Stub"))
			}
			p.Files = append(p.Files, &node.File{
				ID:    symbol.Identity{Lang: coretest.Lang, Package: pkg, Name: path.Base(file), Kind: symbol.KindFile},
				Path:  file,
				Decls: decls,
			})
			assert.NoError(tb, e.Add(stubOf(file, pkg, emitted...)), "the unit adds")
		}
		assert.NoError(tb, g.AddPackage(p), "the package is admitted")
	}
	g.Freeze()
	assert.NoError(tb, plugin.Settle(e, nil, nil, diag.NewSink()), "the store settles")
	return g, e
}

// benchInput returns the routing input over benchTree's graph and
// store, with one sink every pass reports into: the input routes clean,
// so the sink receives no finding.
func benchInput(tb testing.TB, packages int) layout.Input {
	tb.Helper()

	g, e := benchTree(tb, packages)
	ix, err := plugin.NewIndex(g, meta.NewFacts(meta.NewRegistry()), nil, nil)
	assert.NoError(tb, err, "the index builds")
	return layout.Input{
		Emit: e, Outputs: families(), Speller: lean{}, Packager: directories{}, Index: ix,
		Sink: diag.NewSink(),
	}
}
