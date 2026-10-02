// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// These constants name the workspace fixture's two plans and their
// sources, the registry's generator, family word and probe, the check
// that every marked interface has a double, the fixture's trees, the
// files the plans generate, the two roots of the repository that the
// sibling workspaces share, and their modules.
const (
	workspaceStubs              = "stubs"
	workspaceRegistry           = "registry"
	svcSources                  = "./svc/..."
	adminSources                = "./admin/..."
	registrarID       plugin.ID = "registrar"
	registryWord                = "registry"
	peekID            plugin.ID = "peek"
	stubbedID         plugin.ID = "stubbed"
	workspaceTree               = "testdata/workspace/go/tree"
	checkedTree                 = "testdata/workspace/go/checked"
	genTree                     = "testdata/workspace/go/gen"
	workspaceWant               = "testdata/workspace/go/want/"
	doubleFile                  = "svc/store_stub.go"
	registryFile                = "admin/registry.go"
	auditorFile                 = "admin/auditor.go"
	hookDoubleFile              = "hook/hook_stub.go"
	platformRoot                = "platform"
	genRoot                     = "tools/gen"
	acmeModule                  = "example.com/acme"
	genModule                   = "example.com/tools/gen"
)

// unstubbed is the code the stubbed check reports under.
var unstubbed = diag.MustRegister(diag.Prefix("ACME"), diag.CodeSpec{
	Number:  1,
	Meaning: "an interface under the stub directive has no double that the stubs plan exports",
})

// sight is what the peek generator's reader returned: whether it
// returned the struct the generator matched, how many origins of the
// stubs plan's export the generator looked up, and how many of them it
// returned.
type sight struct {
	own     atomic.Bool
	origins atomic.Int64
	found   atomic.Int64
}

// stubbed is the fixture's workspace check: it reads the record of the
// stubs plan and reports an Error at each interface under the stub
// directive from which the plan exports no double. It builds each key
// from the doubles generator's naming convention, the way a dependent
// knows a declaration before the producing plan runs.
type stubbed struct{}

// Name returns the check's name.
func (stubbed) Name() plugin.ID { return stubbedID }

// Reads returns the stubs plan.
func (stubbed) Reads() []string { return []string{workspaceStubs} }

// Check reports each marked interface without a double, under each
// spelling the stub directive's schema recognises.
func (stubbed) Check(ctx *plugin.CheckContext) error {
	export := ctx.Plans[0].Export
	for _, spelled := range []directive.Name{stubSchema.Name, stubSchema.Canonical()} {
		for s := range ctx.Index.ByDirective(spelled) {
			iface, is := s.(*node.Interface)
			if !is {
				continue
			}
			key := plugin.ExportKey{Origin: iface.ID, Plugin: stubgenID, Name: doubleName(iface.Name)}
			if len(export.Find(key)) == 0 {
				ctx.Sink.Errorf(unstubbed, iface.Position(), ctx.Plugin,
					"interface %s is under the stub directive, and plan %q exports no double of it",
					iface.Name, workspaceStubs)
			}
		}
	}
	return nil
}

// The workspace frame over Go: the stubs plan doubles the interfaces
// of svc, the registry plan reads its export and aliases each double in
// admin, each plan's reader returns only its own directories, a check
// reads the records, and neither of two workspaces of one repository
// reads or changes the other's files.
func TestGolangWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("RunWorkspaceSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a registry that aliases the doubles its producer exports", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, workspaceFixture(t))
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false for a declaration outside the plan's sources", func(t *testing.T) {
			t.Parallel()

			seen := &sight{}
			root := t.TempDir()
			copyTree(t, root, workspaceTree)
			runClean(t, root, workspacePlans(peek(seen)))
			assert.True(t, seen.own.Load(), "the reader returns the registry, inside the plan's sources")
			assert.True(t, seen.origins.Load() > 0, "the stubs plan exports a declaration with an origin")
			assert.Equal(t, seen.found.Load(), int64(0),
				"the reader returns no origin in svc, outside the plan's sources")
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("reports an Error at an interface whose double no plan exports", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			copyTree(t, root, workspaceTree)
			copyTree(t, root, checkedTree)
			w, err := composeWorkspace(root).Plans(workspacePlans()...).Checks(stubbed{}).Build()
			assert.NoError(t, err, "the composition builds")
			report, err := w.Run(context.Background(), workspace.Input{Tree: os.DirFS(root)})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the check's Error fails the run")
			var reported []diag.Diag
			for d := range report.Sink.All() {
				if d.Code == unstubbed {
					reported = append(reported, d)
				}
			}
			assert.Length(t, reported, 1, "one Error, for the one interface outside the stubs plan's sources")
			assert.Equal(t, reported[0].Pos.File, auditorFile, "at the interface in admin")
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("records each workspace's manifest under its own root", func(t *testing.T) {
			t.Parallel()

			repo := siblings(t)
			platform := filepath.Join(repo, platformRoot)
			gen := filepath.Join(repo, filepath.FromSlash(genRoot))
			assert.Equal(t, recordedPaths(t, platform), []string{registryFile, doubleFile},
				"the platform's record lists the platform's files")
			assert.Equal(t, recordedPaths(t, gen), []string{hookDoubleFile},
				"the generator's record lists the generator's files")
			_, err := os.Stat(filepath.Join(repo, ledger.StateDir(acmeBrand)))
			assert.True(t, errors.Is(err, fs.ErrNotExist), "the repository's root has no state directory")
		})

		t.Run("reads no source of a sibling workspace", func(t *testing.T) {
			t.Parallel()

			repo := siblings(t)
			for root, module := range map[string]string{platformRoot: acmeModule, genRoot: genModule} {
				sources := 0
				for _, e := range recorded(t, filepath.Join(repo, filepath.FromSlash(root))).Files {
					for _, source := range e.Sources {
						id, err := symbol.Parse(source)
						assert.NoError(t, err, "the source "+source+" parses")
						assert.True(t, strings.HasPrefix(id.Package, module),
							"the "+root+" workspace generates "+e.Path+" from its own module, not "+id.Package)
						sources++
					}
				}
				assert.True(t, sources > 0, "the "+root+" workspace records the sources of its files")
			}
		})

		t.Run("removes no file of a sibling workspace", func(t *testing.T) {
			t.Parallel()

			repo := siblings(t)
			gen := filepath.Join(repo, filepath.FromSlash(genRoot))
			before := filesUnder(t, gen)
			platform := filepath.Join(repo, platformRoot)
			runClean(t, platform, workspacePlans()[:1])
			_, err := os.Stat(filepath.Join(platform, filepath.FromSlash(registryFile)))
			assert.True(t, errors.Is(err, fs.ErrNotExist), "the platform removes the registry plan's file")
			assert.Equal(t, filesUnder(t, gen), before, "the generator's workspace is unchanged")
		})
	})
}

// workspaceFixture returns the workspace suite's fixture: the tree, the
// composition, the two plans, and the two files they generate.
func workspaceFixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	want := map[string][]byte{}
	for _, path := range []string{doubleFile, registryFile} {
		b, err := os.ReadFile(filepath.FromSlash(workspaceWant + path))
		assert.NoError(t, err, "the golden of "+path+" reads")
		want[path] = b
	}
	return workspacetest.Fixture{
		Tree:    os.DirFS(workspaceTree),
		Compose: composeWorkspace,
		Plans:   func() []workspace.Plan { return workspacePlans() },
		Want:    want,
	}
}

// composeWorkspace is the workspace fixture's composition over root,
// without its plans: the Go frontend and rules, a disk sink at root and
// the state directory's ledger under root.
func composeWorkspace(root string) *workspace.Builder {
	return workspace.New().
		Brand(acmeBrand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, acmeBrand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, acmeBrand) })
}

// workspacePlans returns the workspace fixture's plans: stubs, scoped to
// svc, which doubles every interface under the stub directive, and
// registry, scoped to admin, which depends on stubs and aliases every
// double its export lists. extra joins the registry's generators. Both
// plans render through one Go backend, because a composition admits a
// plugin name twice only for the same provider.
func workspacePlans(extra ...plugin.Generator) []workspace.Plan {
	backend := gobackend.New()
	return []workspace.Plan{
		{
			Name:       workspaceStubs,
			Sources:    workspace.Sources{Packages: []string{svcSources}},
			Generators: []plugin.Generator{doubles()},
			Backend:    backend,
		},
		{
			Name:       workspaceRegistry,
			Sources:    workspace.Sources{Packages: []string{adminSources}},
			DependsOn:  []string{workspaceStubs},
			Generators: append([]plugin.Generator{registrar()}, extra...),
			Backend:    backend,
		},
	}
}

// doubles returns the workspace fixture's stub generator: per interface
// under the stub directive, a double in the primary family, under the
// name doubleName gives it.
func doubles() plugin.Generator {
	p, held := eidos.NewPlugin(stubgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: stubWord}).
		Handle(eidos.Directive(stubSchema, eidos.OnInterface(func(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
			e.File().Append(double(doubleName(m.Interface.Name), m))
			return nil
		}))).
		Build().(plugin.Generator)
	if !held {
		panic("conformance_test: the doubles generator lowers to the generator role")
	}
	return p
}

// doubleName returns the name the doubles generator emits for the
// double of an interface: the stub word, then the interface's name, in
// the neutral convention. The Go settle respells it for a public
// declaration, so stubStore declares StubStore.
func doubleName(iface string) string { return stubWord + iface }

// registrar returns the registry plan's generator: for each struct of
// its scope, a type alias in the struct's package for each double the
// stubs plan exports, under the export's spelling and qualified with the
// export's package. The registry plan's scope declares one struct, and
// the rule takes no directive, so a composition without the plan still
// validates every directive of the tree.
func registrar() plugin.Generator {
	p, held := eidos.NewPlugin(registrarID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: registryWord}).
		Handle(eidos.OnStruct(register)).
		Build().(plugin.Generator)
	if !held {
		panic("conformance_test: the registrar lowers to the generator role")
	}
	return p
}

// register aliases each double of the stubs plan's export, reading its
// spelling and its import path from the export instead of applying the
// doubles generator's naming and the Go settle a second time.
func register(m *eidos.StructMatch, e *eidos.Emitter) error {
	export, _ := m.Export(workspaceStubs)
	out := e.PackageFile()
	for _, s := range export.Symbols {
		if s.Plugin != stubgenID || s.Kind != symbol.KindStruct || s.Host != "" {
			continue
		}
		out.Append(&emit.Alias{
			Origin: m.Struct.ID,
			Name:   s.Spelling,
			Target: &emit.TypeRef{Spelling: s.Spelling, Package: s.Package.Package},
		})
	}
	return nil
}

// peek returns a generator that looks up, through its reader, the
// struct it matches and the origin of every declaration the stubs
// plan's export lists, and records what the reader returned in seen.
func peek(seen *sight) plugin.Generator {
	p, held := eidos.NewPlugin(peekID).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
			_, own := m.Reader().Lookup(m.Struct.ID)
			seen.own.Store(own)
			export, _ := m.Export(workspaceStubs)
			for _, s := range export.Symbols {
				if s.Origin.IsZero() {
					continue
				}
				seen.origins.Add(1)
				if _, found := m.Reader().Lookup(s.Origin); found {
					seen.found.Add(1)
				}
			}
			return nil
		})).
		Build().(plugin.Generator)
	if !held {
		panic("conformance_test: the peek generator lowers to the generator role")
	}
	return p
}

// copyTree copies a testdata tree into root, beside what root already
// contains.
func copyTree(t *testing.T, root, tree string) {
	t.Helper()

	assert.NoError(t, os.CopyFS(root, os.DirFS(tree)), "the tree "+tree+" copies into "+root)
}

// runClean runs the workspace fixture's composition of plans over the
// tree at root, and checks that the run is clean: each Error it reports
// fails the test on a line of its own, before the run's error does.
func runClean(t *testing.T, root string, plans []workspace.Plan) {
	t.Helper()

	w, err := composeWorkspace(root).Plans(plans...).Build()
	assert.NoError(t, err, "the composition over "+root+" builds")
	report, err := w.Run(context.Background(), workspace.Input{Tree: os.DirFS(root)})
	for d := range report.Sink.All() {
		if d.Severity == diag.SeverityError {
			t.Errorf("the run over %s reports the Error %s at %s: %s", root, d.Code, d.Pos, d.Msg)
		}
	}
	assert.NoError(t, err, "the run over "+root+" is clean")
}

// siblings copies the workspace fixture's tree to the platform root and
// the generator's tree to the tools/gen root of one repository, runs a
// composition over each root, and returns the repository's root. The
// platform runs both of the fixture's plans, and the generator runs the
// stubs plan over its whole tree.
func siblings(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	platform := filepath.Join(repo, platformRoot)
	gen := filepath.Join(repo, filepath.FromSlash(genRoot))
	copyTree(t, platform, workspaceTree)
	copyTree(t, gen, genTree)
	runClean(t, platform, workspacePlans())
	stubs := workspacePlans()[0]
	stubs.Sources = workspace.Sources{}
	runClean(t, gen, []workspace.Plan{stubs})
	return repo
}

// recorded returns the manifest the state directory under root records.
func recorded(t *testing.T, root string) manifest.Manifest {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ledger.ManifestPath(acmeBrand))))
	assert.NoError(t, err, "the record under "+root+" reads")
	m, err := manifest.Decode(b)
	assert.NoError(t, err, "the record under "+root+" decodes")
	return m
}

// recordedPaths returns the paths the record under root lists, in its
// path order.
func recordedPaths(t *testing.T, root string) []string {
	t.Helper()

	var out []string
	for _, e := range recorded(t, root).Files {
		out = append(out, e.Path)
	}
	return out
}

// filesUnder returns every file under dir and its bytes, keyed by its
// slash-separated path relative to dir.
func filesUnder(t *testing.T, dir string) map[string]string {
	t.Helper()

	fsys := os.DirFS(dir)
	out := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, path)
		out[path] = string(b)
		return err
	})
	assert.NoError(t, err, "the files under "+dir+" read")
	return out
}
