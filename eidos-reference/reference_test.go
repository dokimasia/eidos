// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	"go.dokimi.dev/assert/golden"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	golang "go.dokimi.dev/eidos/lang/go"
	goannotate "go.dokimi.dev/eidos/lang/go/annotate"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/reference"
	"go.dokimi.dev/eidos/reference/defaults"
	"go.dokimi.dev/eidos/reference/mockgen"
	"go.dokimi.dev/eidos/reference/shapewriter"
	"go.dokimi.dev/eidos/reference/stubgen"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The tree, its config file, and the directory of the files that the
// plans generate from it.
const (
	tree       = "testdata/tree"
	configFile = ".reference.yaml"
	want       = "testdata/want"
)

// The file that the edit of the tree changes, the signature of Get before
// and after the edit, and the mode of the file.
const (
	storeFile  = "catalog/store.go"
	getByID    = "Get(ctx context.Context, id string)"
	getByKey   = "Get(ctx context.Context, key string)"
	sourceMode = 0o644
)

// The extensions of the Go and the TypeScript files.
const (
	goExtension = ".go"
	tsExtension = ".ts"
)

// The package of the catalog, its writers, and the struct and the fields
// with defaults.
const (
	catalogPackage = "example.com/shop/catalog"
	sinkName       = "Sink"
	journalName    = "Journal"
	mirrorName     = "Mirror"
	addressName    = "Address"
	streetName     = "Street"
	countryName    = "Country"
	streetDefault  = "Dam 1"
	countryDefault = "NL"
)

// The misplaced case writes a module whose source file has builder on an
// interface, enum on a struct and mock on a function. No schema of the
// three admits the kind of its declaration.
const (
	goModFile       = "go.mod"
	misplacedModule = "module example.com/shop\n\ngo 1.27\n"
	misplacedFile   = "misplaced/misplaced.go"
	misplacedSource = `package misplaced

// Reader reads.
//
//+reference:builder
type Reader interface {
	Read() error
}

// Point is a point.
//
//+reference:enum
type Point struct {
	X int
}

// Run runs.
//
//+reference:mock
func Run() {}
`
)

// misplacedLines are the lines of the carriers of misplacedSource.
var misplacedLines = []int{5, 12, 19}

// The test of the method String that enum writes on Priority, beside the
// generated files. A priority outside the declared set has a value above
// the range of int64, and its text contains that value.
const (
	priorityTestFile = "catalog/priority_text_test.go"
	priorityTest     = `package catalog

import "testing"

func TestPriorityText(t *testing.T) {
	if got := Priority(1<<63 + 1).String(); got != "Priority(9223372036854775809)" {
		t.Errorf("String returns %q for a priority outside the declared set", got)
	}
}
`
)

// config is the config file of the tree as the command line reads it: the
// version, and the options of the plugins.
type config struct {
	Version int                       `yaml:"version"`
	Options map[string]map[string]any `yaml:"options"`
}

// The composition of eidos-reference generates the files of testdata/want
// from the tree, passes the workspace suites, and generates Go that vets
// with tests that pass and TypeScript that type-checks.
func TestReference(t *testing.T) {
	t.Parallel()

	f := fixture(t)

	t.Run("Plans", func(t *testing.T) {
		t.Parallel()

		t.Run("returns two plans that share one mockgen and one stubgen", func(t *testing.T) {
			t.Parallel()

			plans := reference.Plans()
			assert.Length(t, plans, 2, "the composition has the plan go and the plan typescript")
			expect.Equal(t, generator(t, plans[0], mockgen.ID), generator(t, plans[1], mockgen.ID),
				"both plans run one mockgen", assert.ByIdentity())
			expect.Equal(t, generator(t, plans[0], stubgen.ID), generator(t, plans[1], stubgen.ID),
				"both plans run one stubgen", assert.ByIdentity())
		})
	})

	t.Run("Compose", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a composition that builds", func(t *testing.T) {
			t.Parallel()

			_, err := reference.Compose().Build()
			assert.NoError(t, err, "the composition of the binary builds")
		})

		t.Run("returns a composition that builds with the annotator of the Go satellite", func(t *testing.T) {
			t.Parallel()

			_, err := reference.Compose().Annotators(goannotate.New()).Build()
			assert.NoError(t, err, "the annotator composes beside the Go backend")
		})

		t.Run("writes the files of testdata/want from the tree", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			report := run(t, f, root)
			for _, e := range report.Manifest.Files {
				got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e.Path)))
				assert.NoError(t, err, "the run writes "+e.Path)
				golden.MatchAt(t, filepath.Join(filepath.FromSlash(want), filepath.FromSlash(e.Path)), got,
					golden.ShouldUpdate())
			}
		})

		t.Run("passes the workspace suite over the tree", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, f)
		})

		t.Run("passes the warm and cold suite over the tree", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWarmColdSuite(t, f)
		})

		t.Run("writes Go that vets", func(t *testing.T) {
			t.Parallel()

			a := gotesting.New()
			if !toolchain.Require(t, a) {
				return
			}
			gotesting.AssertVets(t.Context(), t, a, generated(f, goExtension))
		})

		t.Run("writes Go tests that pass", func(t *testing.T) {
			t.Parallel()

			a := gotesting.New()
			if !toolchain.Require(t, a) {
				return
			}
			toolchain.AssertTestsPass(t.Context(), t, a, generated(f, goExtension))
		})

		t.Run("writes a String that spells a value outside the variants as the value", func(t *testing.T) {
			t.Parallel()

			a := gotesting.New()
			if !toolchain.Require(t, a) {
				return
			}
			g := generated(f, goExtension)
			g.Files[priorityTestFile] = []byte(priorityTest)
			toolchain.AssertTestsPass(t.Context(), t, a, g)
		})

		t.Run("writes TypeScript that type-checks", func(t *testing.T) {
			t.Parallel()

			a := tstesting.New()
			if !toolchain.Require(t, a) {
				return
			}
			toolchain.AssertTypeChecks(t.Context(), t, a, generated(f, tsExtension))
		})

		t.Run("stamps the writers through shapewriter", func(t *testing.T) {
			t.Parallel()

			report := run(t, f, t.TempDir())
			key, registered := meta.Lookup[bool](report.Facts.Registry(), shapewriter.KeyWriter)
			assert.True(t, registered, "the composition registers the key of shapewriter")
			for _, tt := range []struct {
				name string
				want bool
			}{{name: sinkName, want: true}, {name: journalName, want: true}, {name: mirrorName}} {
				id := symbol.Identity{
					Lang:    golang.Lang,
					Package: catalogPackage,
					Name:    tt.name,
					Kind:    symbol.KindStruct,
				}
				got, _ := meta.Get(report.Facts, id, key)
				expect.Equal(t, got, tt.want, "shapewriter stamps "+tt.name+" as the tree declares it")
			}
		})

		t.Run("stamps the defaults of the fields through defaults", func(t *testing.T) {
			t.Parallel()

			report := run(t, f, t.TempDir())
			key, registered := meta.Lookup[string](report.Facts.Registry(), defaults.KeyValue)
			assert.True(t, registered, "the composition registers the key of defaults")
			for _, tt := range []struct {
				name string
				want string
			}{{name: streetName, want: streetDefault}, {name: countryName, want: countryDefault}} {
				id := symbol.Identity{
					Lang:    golang.Lang,
					Package: catalogPackage,
					Owner:   addressName,
					Name:    tt.name,
					Kind:    symbol.KindField,
				}
				got, _ := meta.Get(report.Facts, id, key)
				expect.Equal(t, got, tt.want, "defaults stamps the default of "+tt.name+" that takes precedence")
			}
		})

		t.Run("reports MisplacedDirective for each directive on a declaration of another kind", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{
				goModFile:     files.Text(misplacedModule),
				misplacedFile: files.Text(misplacedSource),
			})
			w, err := f.Compose(root).Plans(f.Plans()...).Build()
			assert.NoError(t, err, "the composition builds")
			report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Stores: f.Stores, Cold: true})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails on the misplaced directives")
			findings := slices.Collect(report.Sink.All())
			assert.Length(t, findings, len(misplacedLines), "each misplaced directive reports one finding")
			for i, line := range misplacedLines {
				expect.Equal(t, findings[i].Code, directive.MisplacedDirective,
					"the finding is under MisplacedDirective")
				expect.Equal(t, findings[i].Pos.Line, line, "the finding is at the carrier of the directive")
			}
		})
	})
}

// fixture returns the fixture of the workspace suites over the tree: the
// composition of Base with the config of the tree, a disk sink and a ledger
// at the root of a check, the stores that the frontends of the composition
// locate in the environment, as the command line passes them, the plans of
// Plans, the files of testdata/want, and the edit that renames the
// parameter of Get in the interface Store.
func fixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	w, err := reference.Compose().Build()
	assert.NoError(t, err, "the composition builds")
	stores, err := w.Stores(os.Getenv)
	assert.NoError(t, err, "the frontends of the composition locate their stores")
	b, err := os.ReadFile(filepath.Join(filepath.FromSlash(tree), configFile))
	assert.NoError(t, err, "the config file of the tree reads")
	var c config
	assert.NoError(t, yaml.Unmarshal(b, &c), "the config file of the tree decodes")
	wanted := map[string][]byte{}
	if !golden.ShouldUpdate() {
		err = fs.WalkDir(os.DirFS(filepath.FromSlash(want)), ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			wanted[p], err = fs.ReadFile(os.DirFS(filepath.FromSlash(want)), p)
			return err
		})
		assert.NoError(t, err, "the files of testdata/want read")
	}
	return workspacetest.Fixture{
		Tree:   os.DirFS(filepath.FromSlash(tree)),
		Stores: stores,
		Compose: func(root string) *workspace.Builder {
			return reference.Base().
				Config(workspace.Config{Options: c.Options}).
				Output(func() (output.Sink, error) { return output.NewDisk(root, reference.Brand) }).
				Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, reference.Brand) })
		},
		Plans: reference.Plans,
		Want:  wanted,
		Edit:  edit,
	}
}

// run copies the tree into root, an empty directory, runs the composition
// of the fixture f over it cold with the stores of f, and returns the
// report. It stops the test where the composition does not build or the run
// returns an error.
func run(t *testing.T, f workspacetest.Fixture, root string) *workspace.Report {
	t.Helper()

	assert.NoError(t, os.CopyFS(root, f.Tree), "the tree copies into the root")
	w, err := f.Compose(root).Plans(f.Plans()...).Build()
	assert.NoError(t, err, "the composition builds")
	report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Stores: f.Stores, Cold: true})
	assert.NoError(t, err, "the plans run clean")
	return report
}

// edit renames the parameter of Get in the interface Store of the tree at
// root, the way a person edits a source file. It opens the file through an
// os.Root of root, and keeps the mode of the file.
//
// Error modes: the error of a root or a file that does not open, read or
// write, and an error for a file without the signature of Get.
func edit(root string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("reference: %w", err)
	}
	defer r.Close()
	name := filepath.FromSlash(storeFile)
	b, err := r.ReadFile(name)
	if err != nil {
		return fmt.Errorf("reference: %w", err)
	}
	src := string(b)
	if !strings.Contains(src, getByID) {
		return fmt.Errorf("reference: %s declares no %s to rename", storeFile, getByID)
	}
	if err := r.WriteFile(name, []byte(strings.Replace(src, getByID, getByKey, 1)), sourceMode); err != nil {
		return fmt.Errorf("reference: %w", err)
	}
	return nil
}

// generated returns the files of the fixture f with the extension ext as
// the output of a toolchain run, with the tree as its sources. The tree
// has the go.mod of its module.
func generated(f workspacetest.Fixture, ext string) toolchain.Generated {
	files := map[string][]byte{}
	for p, b := range f.Want {
		if path.Ext(p) == ext {
			files[p] = b
		}
	}
	return toolchain.Generated{Files: files, Sources: f.Tree}
}

// generator returns the generator named id of the plan p. It stops the
// test where the plan has no such generator.
func generator(t *testing.T, p workspace.Plan, id plugin.ID) plugin.Generator {
	t.Helper()

	at := slices.IndexFunc(p.Generators, func(g plugin.Generator) bool { return g.Name() == id })
	assert.NotEqual(t, at, -1, "the plan "+p.Name+" runs "+string(id))
	return p.Generators[at]
}
