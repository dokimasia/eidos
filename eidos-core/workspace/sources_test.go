// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"path"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// otherLang is a language beside the fixture's own, which no frontend
// of the fixture loads.
const otherLang symbol.Lang = "proto"

// The packages, modules and plan the scope cases declare: a package at
// the tree's root and two below svc, a package a store provides, two
// module paths, and the plan the cases scope.
const (
	rootPkg    = "root"
	svcPkg     = "svc"
	storePkg   = "svc/store"
	contextPkg = "context"
	billingMod = "billing"
	sharedMod  = "shared"
	scopedPlan = "scoped"
)

// contextFile is the one file of the package a store provides: a path
// qualified with the store's name.
var contextFile = plugin.StorePath("goroot", "context/context.go")

// probe is a generator recording the package paths its reader returns:
// what its plan's scope admits.
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

// A plan's sources decide which packages its generators see: Build
// checks the fields against the registries, and each run binds them to
// the graph and its facts.
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

		t.Run("decodes no region for a scoped plan over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingPlans(ledger.NewMem(), workspace.Plan{
				Name: scopedPlan, Generators: []plugin.Generator{planMirror()}, Backend: printer(t, "fixture"),
				Layout: layout.Config{Dir: planDir}, Sources: workspace.Sources{Packages: []string{"svc/..."}},
			}))
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Decoded, 0, "the scope decides no package that no reader asks about")
		})
	})
}

// pkgIn returns a package of one language and path whose one file, at
// each of files, declares one struct named after the file.
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

// moduled stamps a package's module fact the way a frontend stamps it,
// for the run to replay.
func moduled(t *testing.T, g *store.Graph, p *node.Package, module string) {
	t.Helper()

	stamp := meta.RawStamp{Key: meta.ModuleKey, Value: module, Origin: "fixture", Pos: position.Pos{File: "go.mod"}}
	assert.NoError(t, g.AttachStamps(p.ID, []meta.RawStamp{stamp}), "the module fact attaches")
}

// graphOf returns an unfrozen graph of the packages, with the module
// facts a stamp map names.
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

// scopedPlanOf returns the plan the scope cases compose: the probe
// under sources, toward the fixture's target.
func scopedPlanOf(sources workspace.Sources, p *probe) workspace.Plan {
	plan := planTo(scopedPlan, "fixture", p)
	plan.Sources = sources
	return plan
}

// admitted runs one plan of the probe under sources over the graph,
// and returns the package paths its reader returned.
func admitted(t *testing.T, sources workspace.Sources, g *store.Graph) []string {
	t.Helper()

	p := &probe{}
	w := built(t, workspace.New().Brand(fixtureBrand).Targets("fixture").Rules(native{}).
		Plans(scopedPlanOf(sources, p)))
	cleanRun(t, w, g)
	return p.seen
}
