// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// These constants are the brand and the target of every fixture
// composition, the lowering entry of the target, the plans and their
// generators, and the family word of each generator.
const (
	fixtureBrand  output.Brand  = "fixture"
	fixtureTarget plugin.Target = "fixture"
	loweringID    plugin.ID     = "fixture-lowering"
	mirrorsPlan                 = "mirrors"
	stubsPlan                   = "stubs"
	idlePlan                    = "idle"
	mirrorID      plugin.ID     = "mirror"
	stubberID     plugin.ID     = "stubber"
	idleID        plugin.ID     = "idler"
	genWord                     = "gen"
	stubWord                    = "stub"
)

// These constants are the tree of the fixture, which declares Row, the
// source that the edit of the fixture writes to give Row a third field,
// and the files that the two plans generate beside the source of Row.
const (
	rowFile   = "svc/store/row.zz"
	rowSource = "package svc/store\ntype Row int string\n"
	rowEdited = "package svc/store\ntype Row int string bool\n"
	rowPkg    = "svc/store"
	genFile   = "svc/store/gen.txt"
	stubFile  = "svc/store/stub.txt"
	genBody   = "// Row has 2 fields.\ntype ForRow struct{}\n"
	stubBody  = "type RowStub struct{}\n"
)

// errNoStatements is the error of the scaffold of the printer. The
// fixture emits no statement.
var errNoStatements = errors.New("workspacetest_test: the fixture spells no statements")

// fixturePrinter returns the printer of the target fixture. Every call
// returns one value, so the plans of a composition share the backend of
// their target, as a target renders through one backend. A kit backend
// keeps no state of a run, so parallel tests share the value.
var fixturePrinter = sync.OnceValue(func() plugin.Backend { return printing().Build() })

// TestSuite tests the suites of a workspace of several plans. Two plans
// that generate, export, check and sweep as the frame requires pass
// every check.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunWorkspaceSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes two plans of which the second depends on the first", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, fixture(t))
		})
	})

	t.Run("RunWarmColdSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes two plans of which the second depends on the first", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWarmColdSuite(t, fixture(t))
		})
	})
}

// naming returns a generator named id. For each struct in scope, it
// emits the struct that shape returns for the subject into the
// per-package file of word. A nil shape emits nothing.
func naming(id plugin.ID, word string, shape func(*eidos.StructMatch) *emit.Struct) plugin.Generator {
	return eidos.NewPlugin(id).
		Output(plugin.Output{Per: plugin.PerPackage, Word: word}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if shape != nil {
				e.PackageFile().Append(shape(m))
			}
			return nil
		})).Generator()
}

// printing returns the builder of the printer, for a case that adds
// parts to it. The printer spells a struct as one line after its
// documentation, names every file after its family word and frames the
// file through a line comment.
func printing() *backend.Builder {
	return backend.New("printer", fixtureTarget, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "{{range .Doc}}// {{.}}\n{{end}}type {{.Name}} struct{}\n",
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) { return nil, errNoStatements }).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: everyFact()})
}

// everyFact returns a verdict of Renders for every fact. The one template
// of the printer writes no fact.
func everyFact() map[symbol.Fact]render.Verdict {
	out := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		out[f] = render.Renders
	}
	return out
}

// planOf returns a plan of a name that runs gen through the printer of
// the target fixture and depends on deps.
func planOf(name string, gen plugin.Generator, deps ...string) workspace.Plan {
	return workspace.Plan{
		Name:       name,
		DependsOn:  deps,
		Generators: []plugin.Generator{gen},
		Backend:    fixturePrinter(),
	}
}

// mirrors returns the plan that emits ForRow for Row, documented with the
// number of fields of Row, so a new field changes the file of the plan
// and leaves its export unchanged.
func mirrors() workspace.Plan {
	return planOf(mirrorsPlan, naming(mirrorID, genWord, func(m *eidos.StructMatch) *emit.Struct {
		return &emit.Struct{
			Origin: m.Struct.Identity(),
			Name:   "For" + m.Struct.Name,
			Doc:    []string{fmt.Sprintf("%s has %d fields.", m.Struct.Name, len(m.Struct.Fields))},
		}
	}))
}

// stubs returns the plan that emits RowStub for Row and depends on
// mirrors.
func stubs() workspace.Plan {
	return planOf(stubsPlan, naming(stubberID, stubWord, func(m *eidos.StructMatch) *emit.Struct {
		return &emit.Struct{Origin: m.Struct.Identity(), Name: m.Struct.Name + "Stub"}
	}), mirrorsPlan)
}

// idle returns a plan whose generator emits nothing, so it routes no
// file.
func idle() workspace.Plan { return planOf(idlePlan, naming(idleID, genWord, nil)) }

// onDisk returns the composition over root without its plans. It has the
// brand and the target of the fixture, the scripted frontend, a disk
// sink at root and the ledger of the state directory under root.
func onDisk(root string) *workspace.Builder {
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Targets(fixtureTarget).
		Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) })
}

// stamped returns the bytes that the output contract stamps for a body
// that one plugin assembled from the package of Row.
func stamped(t *testing.T, path, body string, p plugin.ID) []byte {
	t.Helper()

	c, err := output.NewContract(fixtureBrand, plugin.CommentSyntax{Line: []string{"//"}})
	assert.NoError(t, err, "the contract builds")
	b, err := c.Stamp(plugin.RenderedFile{
		Path: path, Plugins: []plugin.ID{p}, Sources: []string{rowPkg}, Body: []byte(body),
	})
	assert.NoError(t, err, "the body stamps")
	return b
}

// fixture returns the fixture. It has the source of Row, the composition
// without its plans, the two plans, the two files that they generate and
// the edit that gives Row a third field.
func fixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return workspacetest.Fixture{
		Tree:    fstest.MapFS{rowFile: {Data: []byte(rowSource)}},
		Compose: func(root string) (*workspace.Builder, error) { return onDisk(root), nil },
		Plans:   func() []workspace.Plan { return []workspace.Plan{mirrors(), stubs()} },
		Want: map[string][]byte{
			genFile:  stamped(t, genFile, genBody, mirrorID),
			stubFile: stamped(t, stubFile, stubBody, stubberID),
		},
		Edit: func(root string) error {
			return os.WriteFile(filepath.Join(root, filepath.FromSlash(rowFile)), []byte(rowEdited), 0o644)
		},
	}
}
