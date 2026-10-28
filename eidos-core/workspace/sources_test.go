// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"io/fs"
	"path"
	"strconv"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// otherLang is a language beside the fixture's own, which no frontend
// of the fixture loads.
const otherLang symbol.Lang = "proto"

// The packages, modules and plan of the scope cases. They are a package
// at the root of the tree and two below svc, a package that a store
// provides, two module paths, and the plan that the cases scope.
const (
	rootPkg    = "root"
	svcPkg     = "svc"
	storePkg   = "svc/store"
	contextPkg = "context"
	billingMod = "billing"
	sharedMod  = "shared"
	scopedPlan = "scoped"
)

// outsideSource is a file of the store package outside the directory of
// the store package.
const outsideSource = "other/col.zz"

// The packages, the files and the generator of the dependency cases. The
// application imports the library, which a store provides or a file of
// the tree declares.
const (
	appPkg              = "svc/app"
	libPkg              = "ext/lib"
	appSource           = "svc/app/app.zz"
	libSource           = "ext/lib/lib.zz"
	libBody             = "package " + libPkg + "\ntype Lib string\n"
	rosterID  plugin.ID = "roster"
)

// relocatorID is the name of the annotator that stamps the module of the
// store package.
const relocatorID plugin.ID = "relocator"

// billedLine is the line of the struct that moves the store package into
// the billing module.
const billedLine = "type Billed int\n"

// contextFile is the one file of the package that a store provides, a
// path qualified with the name of the store.
var contextFile = plugin.StorePath("goroot", "context/context.go")

// billedID is the struct whose presence moves the store package into the
// billing module.
var billedID = symbol.Identity{
	Lang: frontendtest.ScriptedLang, Package: storePkg, Name: "Billed", Kind: symbol.KindStruct,
}

// probe is a generator that records the package paths that its reader
// returns, which are the packages that the scope of its plan admits.
type probe struct {
	seen []string
}

// Name returns the probe's name.
func (*probe) Name() plugin.ID { return "probe" }

// Generate records every package the reader returns.
func (p *probe) Generate(ctx *plugin.GeneratorContext) error {
	for s := range ctx.Reader.ByKind(symbol.KindPackage) {
		if d, named := s.(node.Declaration); named {
			p.seen = append(p.seen, d.Identity().Package)
		}
	}
	return nil
}

// relocator is an annotator that implements its role directly. It stamps
// the module of the store package. The module is the billing module
// where the package declares Billed, and the shared module otherwise.
type relocator struct{}

// Name returns the name of the relocator.
func (relocator) Name() plugin.ID { return relocatorID }

// Annotate looks Billed up and stamps the module of the store package.
func (relocator) Annotate(ctx *plugin.AnnotatorContext) error {
	module := sharedMod
	if _, billed := ctx.Reader.Lookup(billedID); billed {
		module = billingMod
	}
	claim := meta.Claim{Subject: storePackage, Turn: ctx.Turn, Plugin: relocatorID}
	return meta.Stamp(ctx.Facts, ctx.Kernel.Module, module, claim)
}

// TestSources tests the sources of a plan, which decide the packages
// that its generators see. Build checks the fields against the
// registries, and each run binds them to the graph and its facts. A warm
// run whose sources admit or refuse a package otherwise than the
// generation did leaves the files of a cold run.
func TestSources(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming the plan for a language the composition does not know", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(scopedPlanOf(workspace.Sources{Lang: "kotlin"}, &probe{})).Build()
			assert.HasError(t, err, "the composition knows no kotlin")
			assert.Contains(t, err.Error(), `plan "scoped" scopes the language "kotlin"`, "the error names the plan")
		})

		t.Run("returns no error for a language a registered rules value declares", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Rules(native{}).
				Plans(scopedPlanOf(workspace.Sources{Lang: coretest.Lang}, &probe{})).Build()
			assert.NoError(t, err, "the rules declare the language")
		})

		t.Run("returns no error for a language a registered frontend loads", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Frontends(frontendtest.NewScripted()).
				Plans(scopedPlanOf(workspace.Sources{Lang: frontendtest.ScriptedLang}, &probe{})).Build()
			assert.NoError(t, err, "the frontend loads the language")
		})

		refused := []string{"", "./", "../svc", "/svc", "svc/../x", "svc/.../x", `svc\x`, "svc/", "/..."}
		for _, pattern := range refused {
			t.Run("returns an error naming the pattern "+pattern, func(t *testing.T) {
				t.Parallel()

				_, err := valid().
					Plans(scopedPlanOf(workspace.Sources{Packages: []string{pattern}}, &probe{})).Build()
				assert.HasError(t, err, "the pattern names no directory")
				assert.Contains(t, err.Error(), `plan "scoped" scopes the pattern `+strconv.Quote(pattern),
					"the error names the plan and the pattern")
			})
		}

		accepted := []string{".", "...", "./...", "svc", "./svc", "svc/...", "./svc/..."}
		for _, pattern := range accepted {
			t.Run("returns no error for the pattern "+pattern, func(t *testing.T) {
				t.Parallel()

				_, err := valid().
					Plans(scopedPlanOf(workspace.Sources{Packages: []string{pattern}}, &probe{})).Build()
				assert.NoError(t, err, "the pattern names a directory")
			})
		}
	})

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		golangRoot := func() *node.Package { return pkgIn(coretest.Lang, rootPkg, "a.go") }
		golangSvc := func() *node.Package { return pkgIn(coretest.Lang, svcPkg, "svc/a.go") }
		golangStore := func() *node.Package { return pkgIn(coretest.Lang, storePkg, "svc/store/b.go") }
		protoSvc := func() *node.Package { return pkgIn(otherLang, svcPkg, "svc/c.proto") }
		stored := func() *node.Package { return pkgIn(coretest.Lang, contextPkg, contextFile) }

		tests := []struct {
			name    string
			sources workspace.Sources
			pkgs    func() []*node.Package
			want    []string
		}{
			{
				name:    "admits every package for the zero value",
				sources: workspace.Sources{},
				pkgs:    func() []*node.Package { return []*node.Package{golangRoot(), protoSvc(), stored()} },
				want:    []string{contextPkg, rootPkg, svcPkg},
			},
			{
				name:    "admits the packages of its language",
				sources: workspace.Sources{Lang: coretest.Lang},
				pkgs:    func() []*node.Package { return []*node.Package{golangStore(), protoSvc()} },
				want:    []string{storePkg},
			},
			{
				name:    "admits a dependency package of its language for a language alone",
				sources: workspace.Sources{Lang: coretest.Lang},
				pkgs:    func() []*node.Package { return []*node.Package{stored()} },
				want:    []string{contextPkg},
			},
			{
				name:    "admits the packages below a directory for its trailing wildcard",
				sources: workspace.Sources{Packages: []string{"./svc/..."}},
				pkgs:    func() []*node.Package { return []*node.Package{golangRoot(), golangSvc(), golangStore()} },
				want:    []string{svcPkg, storePkg},
			},
			{
				name:    "admits one directory's packages for a pattern without the wildcard",
				sources: workspace.Sources{Packages: []string{"svc"}},
				pkgs:    func() []*node.Package { return []*node.Package{golangSvc(), golangStore()} },
				want:    []string{svcPkg},
			},
			{
				name:    "admits the packages of the whole tree for the root's wildcard",
				sources: workspace.Sources{Packages: []string{"./..."}},
				pkgs:    func() []*node.Package { return []*node.Package{golangRoot(), golangStore()} },
				want:    []string{rootPkg, storePkg},
			},
			{
				name:    "admits no package with a file outside every named directory",
				sources: workspace.Sources{Packages: []string{"./svc/..."}},
				pkgs: func() []*node.Package {
					return []*node.Package{pkgIn(coretest.Lang, svcPkg, "svc/a.go", "other/b.go")}
				},
				want: nil,
			},
			{
				name:    "admits no package a store provides for a pattern",
				sources: workspace.Sources{Packages: []string{"./..."}},
				pkgs:    func() []*node.Package { return []*node.Package{stored()} },
				want:    nil,
			},
			{
				name:    "admits a package whose files outside a store are in a named directory",
				sources: workspace.Sources{Packages: []string{"./svc/..."}},
				pkgs: func() []*node.Package {
					return []*node.Package{pkgIn(coretest.Lang, svcPkg, "svc/a.go", contextFile)}
				},
				want: []string{svcPkg},
			},
			{
				name:    "admits no package without a file for a pattern",
				sources: workspace.Sources{Packages: []string{"./..."}},
				pkgs:    func() []*node.Package { return []*node.Package{pkgIn(coretest.Lang, svcPkg)} },
				want:    nil,
			},
			{
				name:    "admits a package whose every field matches",
				sources: workspace.Sources{Lang: coretest.Lang, Packages: []string{"svc/..."}},
				pkgs:    func() []*node.Package { return []*node.Package{golangSvc(), protoSvc(), golangRoot()} },
				want:    []string{svcPkg},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Permutation(t, admitted(t, tt.sources, graphOf(t, nil, tt.pkgs()...)), tt.want,
					"the packages the plan's reader returns", assert.EquateEmpty())
			})
		}

		t.Run("admits the packages whose module fact names its module", func(t *testing.T) {
			t.Parallel()

			billing, shared := golangSvc(), golangStore()
			g := graphOf(t, map[*node.Package]string{billing: billingMod, shared: sharedMod}, billing, shared)
			assert.Equal(t, admitted(t, workspace.Sources{Module: billingMod}, g), []string{svcPkg},
				"the packages of the billing module")
		})

		t.Run("admits no package without a module fact for a module", func(t *testing.T) {
			t.Parallel()

			g := graphOf(t, nil, golangSvc())
			assert.Empty(t, admitted(t, workspace.Sources{Module: billingMod}, g),
				"the packages a module scope admits")
		})

		dependencies := []struct {
			name    string
			sources workspace.Sources
		}{
			{
				name:    "admits a dependency package of its language for a pattern",
				sources: workspace.Sources{Lang: coretest.Lang, Packages: []string{"./svc/..."}},
			},
			{
				name:    "admits a dependency package of its language for a module",
				sources: workspace.Sources{Lang: coretest.Lang, Module: billingMod},
			},
			{
				name:    "admits a dependency package of every language for a pattern without a language",
				sources: workspace.Sources{Packages: []string{"./svc/..."}},
			},
		}
		for _, tt := range dependencies {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				g := coretest.Dependent([]*node.Package{stored()}, golangRoot())
				assert.Equal(t, admitted(t, tt.sources, g), []string{contextPkg},
					"the scope admits the dependency package and refuses the root package")
			})
		}

		t.Run("admits no dependency package of another language", func(t *testing.T) {
			t.Parallel()

			g := coretest.Dependent([]*node.Package{pkgIn(otherLang, contextPkg, contextFile)}, golangRoot())
			assert.Equal(t, admitted(t, workspace.Sources{Lang: coretest.Lang, Packages: []string{"./..."}}, g),
				[]string{rootPkg}, "the scope of a plan of one language refuses a package of another")
		})

		t.Run("returns a dependency package to the reader of a plan that a pattern narrows", func(t *testing.T) {
			t.Parallel()

			p := &probe{}
			sealedRun(t, dependentOf(t, workspace.Sources{Packages: []string{"svc/..."}}, p),
				workspace.Input{Tree: dependentTree(), Stores: libStore()})
			assert.Permutation(t, p.seen, []string{appPkg, libPkg}, "the reader returns the library of the store")
		})

		t.Run("writes no file for a declaration of a dependency package", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, dependentOf(t, workspace.Sources{Lang: frontendtest.ScriptedLang}, mirror(mirrorID)),
				workspace.Input{Tree: dependentTree(), Stores: libStore()})
			assert.Length(t, report.Manifest.Files, 1, "the mirror writes the file of the application alone")
			assert.HasPrefix(t, report.Manifest.Files[0].Path, appPkg+"/", "beside the application")
		})

		narrowed := workspace.Sources{Packages: []string{"svc/..."}}
		moved := []struct {
			name          string
			before, after func() fstest.MapFS
		}{
			{
				name:   "renders the files of a package that a dependency round brings into its sources",
				before: vendoredTree,
				after:  dependentTree,
			},
			{
				name:   "renders the files of a package that the partition takes out of its sources",
				before: dependentTree,
				after:  vendoredTree,
			},
		}
		for _, tt := range moved {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				w := dependentOf(t, narrowed, roster())
				first := sealedRun(t, w, workspace.Input{Tree: tt.before(), Stores: libStore()})
				warm, err := w.Run(t.Context(), workspace.Input{
					Tree: editedAfter(tt.before(), tt.after()), Stores: libStore(),
				})
				assert.NoError(t, err, "the warm run is clean")
				assert.False(t, warm.Stats.Cold, "the run after the edit reads the sealed state")
				cold := sealedRun(t, dependentOf(t, narrowed, roster()),
					workspace.Input{Tree: tt.after(), Stores: libStore()})
				assert.NotEqual(t, cold.Manifest.Files, first.Manifest.Files,
					"the edit moves the library across the sources", assert.EquateEmpty())
				assert.Equal(t, warm.Manifest.Files, cold.Manifest.Files,
					"the warm run records the same entries as the cold run", assert.EquateEmpty())
			})
		}

		t.Run("decodes no region for a scoped plan over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingPlans(ledger.NewMem(), workspace.Plan{
				Name: scopedPlan, Generators: []plugin.Generator{planMirror()}, Backend: fixturePrinter(),
				Layout: layout.Config{Dir: planDir}, Sources: workspace.Sources{Packages: []string{"svc/..."}},
			}))
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Decoded, 0, "the scope decides no package that no reader asks about")
		})

		inStore := workspace.Sources{Packages: []string{storePkg}}
		billing := workspace.Sources{Module: billingMod}
		inside := func() fstest.MapFS { return roundsTree(rowLine, userLine) }
		billed := func() fstest.MapFS { return roundsTree(rowLine+billedLine, userLine) }
		rebound := []struct {
			name          string
			sources       workspace.Sources
			before, after func() fstest.MapFS
		}{
			{
				name:    "removes the files of a package that a new file takes out of its sources",
				sources: inStore,
				before:  inside,
				after:   outsideTree,
			},
			{
				name:    "renders the files of a package that a removed file brings into its sources",
				sources: inStore,
				before:  outsideTree,
				after:   inside,
			},
			{
				name:    "renders the files of a package that a new module fact brings into its sources",
				sources: billing,
				before:  inside,
				after:   billed,
			},
			{
				name:    "removes the files of a package that a new module fact takes out of its sources",
				sources: billing,
				before:  billed,
				after:   inside,
			},
		}
		for _, tt := range rebound {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				w := relocating(t, tt.sources)
				first := sealedRun(t, w, workspace.Input{Tree: tt.before()})
				warm, err := w.Run(t.Context(), workspace.Input{Tree: editedAfter(tt.before(), tt.after())})
				assert.NoError(t, err, "the warm run is clean")
				assert.False(t, warm.Stats.Cold, "the run after the edit reads the sealed state")
				cold := sealedRun(t, relocating(t, tt.sources), workspace.Input{Tree: tt.after()})
				assert.NotEqual(t, cold.Manifest.Files, first.Manifest.Files,
					"the edit moves the store package across the sources", assert.EquateEmpty())
				assert.Equal(t, warm.Manifest.Files, cold.Manifest.Files,
					"the warm run records the same entries as the cold run", assert.EquateEmpty())
			})
		}
	})
}

// pkgIn returns a package of one language and path with a file at each
// path of files. Each file declares one struct named after its
// directory.
func pkgIn(lang symbol.Lang, pkg string, files ...string) *node.Package {
	p := &node.Package{ID: symbol.Identity{Lang: lang, Package: pkg, Kind: symbol.KindPackage}}
	for _, file := range files {
		name := "S" + path.Base(path.Dir(file))
		s := &node.Struct{
			ID:   symbol.Identity{Lang: lang, Package: pkg, Name: name, Kind: symbol.KindStruct},
			Name: name,
			Pos:  position.Pos{File: file, Line: 1, Col: 1},
		}
		p.Files = append(p.Files, &node.File{
			ID:    symbol.Identity{Lang: lang, Package: pkg, Name: path.Base(file), Kind: symbol.KindFile},
			Path:  file,
			Decls: node.Symbols{s},
		})
	}
	return p
}

// moduled stamps the module fact of a package as a frontend stamps it,
// for the run to replay.
func moduled(t *testing.T, g *store.Graph, p *node.Package, module string) {
	t.Helper()

	stamp := meta.RawStamp{Key: meta.ModuleKey, Value: module, Origin: "fixture", Pos: position.Pos{File: "go.mod"}}
	assert.NoError(t, g.AttachStamps(p.ID, []meta.RawStamp{stamp}), "the module fact attaches")
}

// graphOf returns an unfrozen graph of the packages, with the module
// facts of modules.
func graphOf(t *testing.T, modules map[*node.Package]string, pkgs ...*node.Package) *store.Graph {
	t.Helper()

	g := store.New()
	for _, p := range pkgs {
		assert.NoError(t, g.AddPackage(p), "the package is admitted")
		if module, stamped := modules[p]; stamped {
			moduled(t, g, p, module)
		}
	}
	return g
}

// outsideTree returns the rounds tree with a file of the store package
// outside the directory of the store package.
func outsideTree() fstest.MapFS {
	tree := roundsTree(rowLine, userLine)
	tree[outsideSource] = &fstest.MapFile{Data: []byte("package svc/store\n" + colLine)}
	return tree
}

// relocating returns a composition over the scripted frontend that
// records into a new memory ledger. It has the relocator, and one plan
// over the sources whose mirror mirrors every struct.
func relocating(tb assert.TB, sources workspace.Sources) *workspace.Workspace {
	tb.Helper()

	return built(tb, roundsBuilder(tb, ledger.NewMem(), sources, mirror(mirrorID)).Annotators(relocator{}))
}

// roster returns a generator whose graph rule writes one struct for each
// package that the reader of its plan returns, into the file of its plan.
func roster() plugin.Generator {
	return eidos.NewPlugin(rosterID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: "roster"}).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
			for s := range m.Reader().ByKind(symbol.KindPackage) {
				if d, named := s.(node.Declaration); named {
					e.PlanFile().Append(&emit.Struct{Name: "For" + path.Base(d.Identity().Package)})
				}
			}
			return nil
		})).
		Generator()
}

// dependentTree returns a tree whose one file of the application imports
// the library.
func dependentTree() fstest.MapFS {
	return fstest.MapFS{
		appSource: {Data: []byte("package " + appPkg + "\nimport lib " + libPkg + "\ntype App lib.Lib\n")},
	}
}

// vendoredTree returns the dependent tree with the library as a file of
// the tree, which the partition loads.
func vendoredTree() fstest.MapFS {
	tree := dependentTree()
	tree[libSource] = &fstest.MapFile{Data: []byte(libBody)}
	return tree
}

// libStore returns the stores of a run, in which the store of the
// scripted dependent provides the library.
func libStore() map[string]fs.FS {
	return map[string]fs.FS{frontendtest.ScriptedStore: fstest.MapFS{libSource: {Data: []byte(libBody)}}}
}

// dependentOf returns a composition over the scripted dependent that
// records into a new memory ledger and renders into memory. Its one plan
// runs gen over the sources.
func dependentOf(tb assert.TB, sources workspace.Sources, gen plugin.Generator) *workspace.Workspace {
	tb.Helper()

	l := ledger.NewMem()
	return built(tb, workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScriptedDependent()).
		Targets("fixture").
		Plans(workspace.Plan{
			Name: scopedPlan, Sources: sources, Generators: []plugin.Generator{gen}, Backend: fixturePrinter(),
			Layout: layout.Config{Dir: planDir},
		}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil }))
}

// scopedPlanOf returns the plan that the scope cases compose, the probe
// under sources toward the target fixture.
func scopedPlanOf(sources workspace.Sources, p *probe) workspace.Plan {
	plan := planTo(scopedPlan, "fixture", p)
	plan.Sources = sources
	return plan
}

// admitted runs one plan of the probe under sources over the graph, and
// returns the package paths that its reader returned.
func admitted(t *testing.T, sources workspace.Sources, g *store.Graph) []string {
	t.Helper()

	p := &probe{}
	w := built(t, workspace.New().Brand(fixtureBrand).Targets("fixture").Rules(native{}).
		Plans(scopedPlanOf(sources, p)))
	cleanRun(t, w, g)
	return p.seen
}
