// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// These constants name the workspace fixture's trees, the files its
// plans generate, the file its edit changes, the line of Get in that
// file before and after the edit, the probe generator, the two roots of
// the repository that the sibling workspaces share, and their modules.
const (
	workspaceTree            = "testdata/workspace/tree"
	checkedTree              = "testdata/workspace/checked"
	genTree                  = "testdata/workspace/gen"
	workspaceWant            = "testdata/workspace/want"
	doubleFile               = "svc/store_stub.go"
	registryFile             = "admin/registry.go"
	auditorFile              = "admin/auditor.go"
	hookDoubleFile           = "hook/hook_stub.go"
	storeFile                = "svc/store.go"
	getByKey                 = "\tGet(key string) (Session, error)\n"
	getByID                  = "\tGet(id string) (Session, error)\n"
	peekID         plugin.ID = "peek"
	platformRoot             = "platform"
	genRoot                  = "tools/gen"
	acmeModule               = "example.com/acme"
	genModule                = "example.com/tools/gen"
)

// The generator of a test family that tests svc from outside, the word of
// the family, the file that it writes beside the store, the name of the
// field of each check, the extension of a Go file, and the lines of the
// file that declare the external test package of svc, import svc and
// qualify Session.
const (
	checksID       plugin.ID = "checks"
	checkWord                = "check"
	checkFile                = "svc/store_check_test.go"
	subjectName              = "subject"
	goExt                    = ".go"
	externalClause           = "package svc_test\n"
	svcImport                = "\t\"" + acmeModule + "/svc\"\n"
	subjectField             = "\tSubject svc.Session\n"
)

// The files of a tree that vendors the module example.com/dep, the pattern
// that admits every directory of the tree, the generator that writes a
// probe of each struct, the word of its family, and the probes of the
// struct of svc and of the struct of the vendored package.
const (
	goModFile       = "go.mod"
	modulesFile     = "vendor/modules.txt"
	pointFile       = "vendor/example.com/dep/dep.go"
	vendoredMod     = "module " + acmeModule + "\n\ngo 1.27\n\nrequire example.com/dep v1.0.0\n"
	vendoredModules = "# example.com/dep v1.0.0\n## explicit; go 1.27\nexample.com/dep\n"
	vendoredStore   = "package svc\n\nimport \"example.com/dep\"\n\n" +
		"// Store keeps a point.\ntype Store struct {\n\tAt dep.Point\n}\n"
	vendoredPoint           = "package dep\n\n// Point is a point.\ntype Point struct {\n\tX int\n}\n"
	allSources              = "./..."
	probesID      plugin.ID = "probes"
	probeWord               = "probe"
	storeProbe              = "svc/store_probe.go"
	pointProbe              = "vendor/example.com/dep/dep_probe.go"
)

// sight is what the peek generator's reader returned: whether it
// returned the struct the generator matched, how many origins of the
// export of golang.StubsPlan the generator looked up, and how many of
// them it returned.
type sight struct {
	own     atomic.Bool
	origins atomic.Int64
	found   atomic.Int64
}

// The workspace frame over Go: the stubs plan doubles the interfaces of
// svc, the registry plan reads its export and aliases each double in
// admin, each plan's reader returns only its own directories, a check
// reads the records, and neither of two workspaces of one repository
// reads or changes the other's files. A test file of an external family
// declares the external test package of its directory. The fixture's edit
// renames a parameter of an interface the stubs plan doubles.
func TestWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("ComposeWorkspace", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the workspace suite over a registry that aliases the exported doubles", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, workspaceFixture(t))
		})

		t.Run("passes the warm suite over an edit that renames a parameter of Get", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWarmColdSuite(t, workspaceFixture(t))
		})

		t.Run("writes the goldens of the workspace fixture", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertGolden(t, workspaceFixture(t), t.TempDir(), workspaceWant)
		})

		t.Run("runs no invocation of the registry plan after an edit that only the stubs plan reads",
			func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				copyTree(t, root, workspaceTree)
				w, err := golang.ComposeWorkspace(root).Plans(golang.WorkspacePlans()...).Build()
				assert.NoError(t, err, "the composition builds")
				_, err = w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
				assert.NoError(t, err, "the first run is clean")
				assert.NoError(t, golang.EditWorkspace(root), "the edit applies")
				report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
				assert.NoError(t, err, "the run after the edit is clean")
				assert.False(t, report.Stats.Cold, "the run after the edit reads the sealed state")
				invoked := map[string]int{}
				for _, c := range report.Stats.Invoked {
					invoked[c.Plan] += c.Count
				}
				for _, p := range golang.WorkspacePlans() {
					if p.Name == golang.StubsPlan {
						expect.NotEqual(t, invoked[p.Name], 0, "the stubs plan runs its invocation on the edited Store")
						continue
					}
					expect.Equal(t, invoked[p.Name], 0, "plan "+p.Name+" runs no invocation")
				}
			})

		t.Run("records each workspace's manifest under its own root", func(t *testing.T) {
			t.Parallel()

			repo := siblings(t)
			platform := filepath.Join(repo, platformRoot)
			gen := filepath.Join(repo, filepath.FromSlash(genRoot))
			assert.Equal(t, recordedPaths(t, platform), []string{registryFile, doubleFile},
				"the platform's record lists the platform's files")
			assert.Equal(t, recordedPaths(t, gen), []string{hookDoubleFile},
				"the generator's record lists the generator's files")
			files.Absent(t, filepath.Join(repo, ledger.StateDir(golang.Brand)),
				"the repository's root has no state directory")
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
						assert.HasPrefix(t, id.Package, module,
							"the "+root+" workspace generates "+e.Path+" from its own module")
						sources++
					}
				}
				assert.NotEqual(t, sources, 0, "the "+root+" workspace records the sources of its files")
			}
		})

		t.Run("removes no file of a sibling workspace", func(t *testing.T) {
			t.Parallel()

			repo := siblings(t)
			gen := filepath.Join(repo, filepath.FromSlash(genRoot))
			platform := filepath.Join(repo, platformRoot)
			files.Unchanged(t, os.DirFS(gen), func() { runClean(t, platform, golang.WorkspacePlans()[:1]) },
				"the generator's workspace is unchanged")
			files.Absent(t, filepath.Join(platform, filepath.FromSlash(registryFile)),
				"the platform removes the registry plan's file")
		})

		t.Run("writes a test file of an external family into the external test package of svc", func(t *testing.T) {
			t.Parallel()

			expect.That(t, files.Read(t, filepath.Join(externalRoot(t), filepath.FromSlash(checkFile)))).
				Contains(externalClause, "the file declares the external test package of svc").
				Contains(svcImport, "the file imports the package under test").
				Contains(subjectField, "the file qualifies the reference into the package under test")
		})

		t.Run("writes a test file of an external family that go vet passes", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, gotesting.New()) {
				return
			}
			root := externalRoot(t)
			gotesting.AssertVets(t.Context(), t,
				workspacetest.Generated(t, root, golang.Brand, os.DirFS(workspaceTree), goExt))
		})

		t.Run("writes no file for a struct of a vendored package", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{
				goModFile:   files.Text(vendoredMod),
				storeFile:   files.Text(vendoredStore),
				modulesFile: files.Text(vendoredModules),
				pointFile:   files.Text(vendoredPoint),
			})
			read := &atomic.Bool{}
			plan := golang.WorkspacePlans()[0]
			plan.Sources = workspace.Sources{Packages: []string{allSources}}
			plan.Generators = []plugin.Generator{probes(read)}
			runClean(t, root, []workspace.Plan{plan})
			assert.True(t, read.Load(), "the reader of the plan returns the struct of the vendored package")
			files.IsFile(t, filepath.Join(root, filepath.FromSlash(storeProbe)), "the rule runs on the struct of svc")
			files.Absent(t, filepath.Join(root, filepath.FromSlash(pointProbe)),
				"the rule does not run on the struct of the vendored package")
		})
	})

	t.Run("WorkspacePlans", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a registry plan whose reader finds no declaration outside its sources", func(t *testing.T) {
			t.Parallel()

			seen := &sight{}
			root := t.TempDir()
			copyTree(t, root, workspaceTree)
			runClean(t, root, golang.WorkspacePlans(peek(seen)))
			assert.True(t, seen.own.Load(), "the reader returns the registry, inside the plan's sources")
			assert.NotEqual(t, seen.origins.Load(), int64(0), "the stubs plan exports a declaration with an origin")
			assert.Equal(t, seen.found.Load(), int64(0),
				"the reader returns no origin in svc, outside the plan's sources")
		})
	})

	t.Run("EditWorkspace", func(t *testing.T) {
		t.Parallel()

		t.Run("renames the parameter of Get in the stubbed interface", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			copyTree(t, root, workspaceTree)
			want := filesUnder(t, root)
			want[storeFile] = strings.Replace(want[storeFile], getByKey, getByID, 1)
			assert.NoError(t, golang.EditWorkspace(root), "the edit applies")
			assert.Equal(t, filesUnder(t, root), want, "the edit changes the line of Get and no other line")
		})

		t.Run("returns an error for a store whose Get has no parameter key", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			copyTree(t, root, workspaceTree)
			assert.NoError(t, golang.EditWorkspace(root), "the first edit applies")
			err := golang.EditWorkspace(root)
			assert.HasError(t, err, "a second edit finds no parameter key to rename")
			assert.Contains(t, err.Error(), storeFile, "the error names the file")
		})

		t.Run("returns fs.ErrNotExist for a root without the store", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, golang.EditWorkspace(t.TempDir()), fs.ErrNotExist, "an empty root has no store to read")
		})
	})

	t.Run("Stubbed", func(t *testing.T) {
		t.Parallel()

		t.Run("Check", func(t *testing.T) {
			t.Parallel()

			t.Run("reports an Error at an interface whose double no plan exports", func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				copyTree(t, root, workspaceTree)
				copyTree(t, root, checkedTree)
				w, err := golang.ComposeWorkspace(root).Plans(golang.WorkspacePlans()...).
					Checks(golang.Stubbed{}).Build()
				assert.NoError(t, err, "the composition builds")
				report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
				assert.ErrorIs(t, err, workspace.ErrRunFailed, "the check's Error fails the run")
				var reported []diag.Diag
				for d := range report.Sink.All() {
					if d.Code == golang.Unstubbed {
						reported = append(reported, d)
					}
				}
				assert.Length(t, reported, 1, "one Error, for the one interface outside the stubs plan's sources")
				assert.Equal(t, reported[0].Pos.File, auditorFile, "at the interface in admin")
			})
		})
	})
}

// workspaceFixture returns the workspace suite's fixture: the tree, the
// composition, the two plans, the goldens under testdata/workspace/want,
// and the edit that renames a parameter of Get.
func workspaceFixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return workspacetest.Fixture{
		Tree:    os.DirFS(workspaceTree),
		Compose: func(root string) (*workspace.Builder, error) { return golang.ComposeWorkspace(root), nil },
		Plans:   func() []workspace.Plan { return golang.WorkspacePlans() },
		Want:    workspacetest.Want(t, workspaceWant),
		Edit:    golang.EditWorkspace,
	}
}

// peek returns a generator that looks up, through its reader, the
// struct it matches and the origin of every declaration that the export
// of golang.StubsPlan lists, and records what the reader returned in
// seen.
func peek(seen *sight) plugin.Generator {
	return eidos.NewPlugin(peekID).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
			_, own := m.Reader().Lookup(m.Struct.ID)
			seen.own.Store(own)
			export, _ := m.Export(golang.StubsPlan)
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
		Generator()
}

// checks returns a generator that writes a check of each struct of its
// scope into a test family whose files test their package from outside.
// The check has a field of the type of the struct, so the file refers to a
// declaration of the package under test.
func checks() plugin.Generator {
	return eidos.NewPlugin(checksID).
		Output(plugin.Output{Tag: eidos.TagTest, Per: plugin.PerSource, Word: checkWord, External: true}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			s := m.Struct
			check := &emit.Struct{Origin: s.ID, Name: checkWord + s.Name, Visibility: symbol.VisibilityPublic}
			check.Fields.Append(&emit.Field{
				Name:       subjectName,
				Visibility: symbol.VisibilityPublic,
				Type:       &emit.TypeRef{Spelling: s.Name, Target: s.ID},
			})
			e.File(eidos.TagTest).Append(check)
			return nil
		})).
		Generator()
}

// probes returns a generator that writes a probe of each struct of its
// scope beside the source of the struct. It stores into read whether its
// reader returns the declaration that the type of the first field of a
// struct refers to.
func probes(read *atomic.Bool) plugin.Generator {
	return eidos.NewPlugin(probesID).
		Output(plugin.Output{Per: plugin.PerSource, Word: probeWord}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			s := m.Struct
			if len(s.Fields) > 0 && s.Fields[0].Type != nil {
				_, found := m.Reader().Lookup(s.Fields[0].Type.Target)
				read.Store(found)
			}
			e.File().Append(&emit.Struct{Origin: s.ID, Name: probeWord + s.Name, Visibility: symbol.VisibilityPublic})
			return nil
		})).
		Generator()
}

// externalRoot copies the workspace fixture's tree into a new directory,
// runs the stubs plan with the generator of checks after its own over it,
// and returns the directory. The generator of the plan registers the
// schema of the stub directive of the tree. The run is clean.
func externalRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	copyTree(t, root, workspaceTree)
	plan := golang.WorkspacePlans()[0]
	plan.Generators = append(plan.Generators, checks())
	runClean(t, root, []workspace.Plan{plan})
	return root
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

	w, err := golang.ComposeWorkspace(root).Plans(plans...).Build()
	assert.NoError(t, err, "the composition over "+root+" builds")
	report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
	for d := range report.Sink.All() {
		expect.NotEqual(t, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s at %s is no Error of the run over %s: %s", d.Code, d.Pos, root, d.Msg))
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
	runClean(t, platform, golang.WorkspacePlans())
	stubs := golang.WorkspacePlans()[0]
	stubs.Sources = workspace.Sources{}
	runClean(t, gen, []workspace.Plan{stubs})
	return repo
}

// recorded returns the manifest the state directory under root records:
// every document of its manifest directory, decoded and joined.
func recorded(t *testing.T, root string) manifest.Manifest {
	t.Helper()

	dir := filepath.Join(root, filepath.FromSlash(ledger.ManifestPath(golang.Brand)))
	listed, err := os.ReadDir(dir)
	assert.NoError(t, err, "the record under "+root+" lists")
	shards := make([]manifest.Shard, 0, len(listed))
	for _, e := range listed {
		shards = append(shards, document(t, filepath.Join(dir, e.Name())))
	}
	m, err := manifest.Join(shards)
	assert.NoError(t, err, "the documents under "+root+" join")
	return m
}

// document returns the manifest document in the file at path.
func document(t *testing.T, path string) manifest.Shard {
	t.Helper()

	b, err := os.ReadFile(path)
	assert.NoError(t, err, "the document "+path+" reads")
	s, err := manifest.DecodeShard(b)
	assert.NoError(t, err, "the document "+path+" decodes")
	return s
}

// recordedPaths returns the paths the record under root lists, in its
// path order.
func recordedPaths(t *testing.T, root string) []string {
	t.Helper()

	files := recorded(t, root).Files
	out := make([]string, 0, len(files))
	for _, e := range files {
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
