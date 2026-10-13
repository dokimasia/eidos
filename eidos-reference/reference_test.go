// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
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

// These constants are the tree, the directory of the files that the plans
// generate from it, and the golden files of the surface of the Go and the
// TypeScript output.
const (
	tree      = "testdata/tree"
	want      = "testdata/want"
	goSurface = "testdata/surface/go.txt"
	tsSurface = "testdata/surface/typescript.txt"
)

// configFile is the name of the config file of the brand of the ensemble,
// which the root of the tree contains.
var configFile = cli.ConfigName(reference.Brand)

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

// The authored values of the tree: the sample and the alternate of City,
// the alternate of Country, the witness of the type parameter of Cache,
// and the error type that the Go annotator stamps.
const (
	cityName         = "City"
	citySample       = "Utrecht"
	cityAlternate    = "Leiden"
	countryAlternate = "DE"
	cacheName        = "Cache"
	cacheParam       = "T"
	witnessType      = "string"
	notFoundName     = "NotFoundError"
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

// The source of the errors of the catalog with ErrConflict declared and
// never assigned. The sentinel test of the package reports it by its name.
const (
	errorsFile       = "catalog/errors.go"
	unassignedErrors = `package catalog

import "errors"

// ErrNotFound reports a product that the catalog does not contain.
var ErrNotFound = errors.New("catalog: product not found")

// ErrConflict reports a product that the catalog contains already.
var ErrConflict error

// NotFoundError reports the product that a lookup of the catalog did not
// find.
type NotFoundError struct {
	// ID identifies the product.
	ID string
	// Err is the cause of the error.
	Err error
}

// Error returns the message of the error.
func (e *NotFoundError) Error() string {
	return "catalog: product " + e.ID + " not found"
}

// Unwrap returns the cause of the error.
func (e *NotFoundError) Unwrap() error {
	return e.Err
}
`
)

// The composition of eidos-reference generates the files of testdata/want
// from the tree and passes the workspace suites. Its Go vets, its tests
// pass and run in parallel, and its TypeScript type-checks. The public
// surface of both languages matches its golden, and every public
// declaration has a doc comment.
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

		t.Run("returns a composition that builds under the config file of the tree", func(t *testing.T) {
			t.Parallel()

			_, err := configured(t).Build()
			assert.NoError(t, err, "the composition of the binary builds")
		})

		t.Run("returns a composition that refuses the empty options of a callee plugin at Build", func(t *testing.T) {
			t.Parallel()

			_, err := reference.Compose().Build()
			assert.HasError(t, err, "the config file sets the required options")
			assert.Contains(t, err.Error(), "plugin: authgen requires the option package",
				"the message contains the plugin and the option")
		})

		t.Run("returns a composition that runs the annotator of the Go satellite", func(t *testing.T) {
			t.Parallel()

			w, err := configured(t).Build()
			assert.NoError(t, err, "the composition of the binary builds")
			names := make([]plugin.ID, 0, len(w.Describe().Annotate))
			for _, c := range w.Describe().Annotate {
				names = append(names, c.Name)
			}
			assert.Contains(t, names, golang.AnnotatorName, "the composition runs the annotator of the Go satellite")
		})

		t.Run("writes the files of testdata/want from the tree", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertGolden(t, f, t.TempDir(), want)
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

			if !toolchain.Require(t, gotesting.New()) {
				return
			}
			gotesting.AssertVets(t.Context(), t, generated(f, goExtension))
		})

		t.Run("writes Go whose public surface matches its golden", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertSurface(t, gofrontend.New(nil), generated(f, goExtension), goSurface)
		})

		t.Run("writes Go whose public declarations have doc comments", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertDocumented(t, gofrontend.New(nil), generated(f, goExtension))
		})

		t.Run("writes TypeScript whose public surface matches its golden", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertSurface(t, tsfrontend.New(), generated(f, tsExtension), tsSurface)
		})

		t.Run("writes TypeScript whose public declarations have doc comments", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertDocumented(t, tsfrontend.New(), generated(f, tsExtension))
		})

		t.Run("writes Go tests that pass", func(t *testing.T) {
			t.Parallel()

			a := gotesting.New()
			if !toolchain.Require(t, a) {
				return
			}
			toolchain.AssertTestsPass(t.Context(), t, a, generated(f, goExtension))
		})

		t.Run("writes Go tests that run in parallel", func(t *testing.T) {
			t.Parallel()

			gotesting.AssertParallel(t, generated(f, goExtension))
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

		t.Run("writes a sentinel test that reports a sentinel that is never assigned", func(t *testing.T) {
			t.Parallel()

			a := gotesting.New()
			if !toolchain.Require(t, a) {
				return
			}
			g := generated(f, goExtension)
			g.Files[errorsFile] = []byte(unassignedErrors)
			dir, cleanup := toolchain.Prepare(t, a, g)
			defer cleanup()
			report, err := a.RunTests(t.Context(), dir)
			assert.NoError(t, err, "the tests of the output build and run")
			expect.Contains(t, report.Output, "ErrConflict is never assigned",
				"the test reports the sentinel by its name")
			expect.NotContains(t, report.Output, "panic:", "the test does not panic")
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

		t.Run("stamps the authored samples of the fields through the sample annotator", func(t *testing.T) {
			t.Parallel()

			report := run(t, f, t.TempDir())
			sampleKey, registered := meta.Lookup[string](report.Facts.Registry(), meta.SampleKey)
			assert.True(t, registered, "the kernel registers the key of a sample")
			alternateKey, registered := meta.Lookup[string](report.Facts.Registry(), meta.AlternateKey)
			assert.True(t, registered, "the kernel registers the key of an alternate")
			field := func(name string) symbol.Identity {
				return symbol.Identity{
					Lang: golang.Lang, Package: catalogPackage, Owner: addressName, Name: name, Kind: symbol.KindField,
				}
			}
			sample, _ := meta.Get(report.Facts, field(cityName), sampleKey)
			expect.Equal(t, sample, citySample, "City has the sample of its directive")
			alternate, _ := meta.Get(report.Facts, field(cityName), alternateKey)
			expect.Equal(t, alternate, cityAlternate, "City has the alternate of its directive")
			alternate, _ = meta.Get(report.Facts, field(countryName), alternateKey)
			expect.Equal(t, alternate, countryAlternate, "Country has the alternate that its directive states alone")
			_, stated := meta.Get(report.Facts, field(countryName), sampleKey)
			expect.False(t, stated, "Country has no stated sample")
		})

		t.Run("stamps the witness of a type parameter through the witness annotator", func(t *testing.T) {
			t.Parallel()

			report := run(t, f, t.TempDir())
			witnessKey, registered := meta.Lookup[symbol.Identity](report.Facts.Registry(), meta.WitnessKey)
			assert.True(t, registered, "the kernel registers the key of a witness")
			param := symbol.Identity{
				Lang: golang.Lang, Package: catalogPackage, Owner: cacheName, Name: cacheParam,
				Kind: symbol.KindTypeParam,
			}
			got, held := meta.Get(report.Facts, param, witnessKey)
			assert.True(t, held, "the type parameter of Cache has a witness")
			assert.Equal(t, got.Name, witnessType, "the witness is the builtin type of the directive")
		})

		t.Run("stamps an error type through the annotator of the Go satellite", func(t *testing.T) {
			t.Parallel()

			report := run(t, f, t.TempDir())
			key, registered := meta.Lookup[bool](report.Facts.Registry(), golang.SatisfiesErrorKey)
			assert.True(t, registered, "the composition registers the keys of the Go annotator")
			id := symbol.Identity{
				Lang: golang.Lang, Package: catalogPackage, Name: notFoundName, Kind: symbol.KindStruct,
			}
			got, _ := meta.Get(report.Facts, id, key)
			assert.True(t, got, "NotFoundError satisfies error")
		})

		t.Run("reports MisplacedDirective for each directive on a declaration of another kind", func(t *testing.T) {
			t.Parallel()

			config, err := os.ReadFile(filepath.Join(filepath.FromSlash(tree), configFile))
			assert.NoError(t, err, "the config file of the tree reads")
			root := files.Workspace(t, files.Tree{
				configFile:    files.Bytes(config),
				goModFile:     files.Text(misplacedModule),
				misplacedFile: files.Text(misplacedSource),
			})
			b, err := f.Compose(root)
			assert.NoError(t, err, "the composition configures")
			w, err := b.Plans(f.Plans()...).Build()
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
// composition of Base with a disk sink and a ledger at the root of a check
// and the config file in the root, as the binary applies it, the stores
// that the frontends of the composition locate in the environment, as the
// command line passes them, the plans of Plans, the files of
// testdata/want, and the edit that renames the parameter of Get in the
// interface Store.
func fixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	w, err := configured(t).Build()
	assert.NoError(t, err, "the composition builds")
	stores, err := w.Stores(os.Getenv)
	assert.NoError(t, err, "the frontends of the composition locate their stores")
	return workspacetest.Fixture{
		Tree:   os.DirFS(filepath.FromSlash(tree)),
		Stores: stores,
		Compose: func(root string) (*workspace.Builder, error) {
			b := reference.Base().
				Output(func() (output.Sink, error) { return output.NewDisk(root, reference.Brand) }).
				Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, reference.Brand) })
			return b, cli.Configure(b, root)
		},
		Plans: reference.Plans,
		Want:  workspacetest.Want(t, want),
		Edit:  edit,
	}
}

// configured returns the composition of the binary under the config file of
// the tree, which sets the required options of the callee plugins.
func configured(t *testing.T) *workspace.Builder {
	t.Helper()

	b := reference.Compose()
	assert.NoError(t, cli.Configure(b, filepath.FromSlash(tree)), "the config file of the tree applies")
	return b
}

// run copies the tree into root, an empty directory, runs the composition
// of the fixture f over it cold with the stores of f, and returns the
// report. It stops the test where the composition does not configure or
// build, or where the run returns an error.
func run(t *testing.T, f workspacetest.Fixture, root string) *workspace.Report {
	t.Helper()

	assert.NoError(t, os.CopyFS(root, f.Tree), "the tree copies into the root")
	b, err := f.Compose(root)
	assert.NoError(t, err, "the composition configures")
	w, err := b.Plans(f.Plans()...).Build()
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
