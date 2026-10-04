// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"errors"
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

// The brand and the target every fixture composition declares, the
// two plans and their generators, and the family word each generator
// emits under.
const (
	fixtureBrand  output.Brand  = "fixture"
	fixtureTarget plugin.Target = "fixture"
	mirrorsPlan                 = "mirrors"
	stubsPlan                   = "stubs"
	idlePlan                    = "idle"
	mirrorID      plugin.ID     = "mirror"
	stubberID     plugin.ID     = "stubber"
	idleID        plugin.ID     = "idler"
	genWord                     = "gen"
	stubWord                    = "stub"
)

// The fixture's tree, which declares Row, and the files the two plans
// generate beside Row's source.
const (
	rowFile   = "svc/store/row.zz"
	rowSource = "package svc/store\ntype Row int string\n"
	rowPkg    = "svc/store"
	genFile   = "svc/store/gen.txt"
	stubFile  = "svc/store/stub.txt"
	genBody   = "type ForRow struct{}\n"
	stubBody  = "type RowStub struct{}\n"
)

// errNoStatements is what the printer's scaffold returns: the fixture
// emits no statement.
var errNoStatements = errors.New("workspacetest_test: the fixture spells no statements")

// The suite is the conformance bar for a workspace of several plans, so
// two plans that generate, export, check and sweep the way the frame
// promises pass every check.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunWorkspaceSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes two plans of which the second depends on the first", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, fixture(t))
		})
	})
}

// naming returns a generator named id that emits, per struct in scope,
// one struct into the per-package file of the word, under the name that
// name spells from the subject. A nil name emits nothing.
func naming(id plugin.ID, word string, name func(*eidos.StructMatch) string) plugin.Generator {
	p, held := eidos.NewPlugin(id).
		Output(plugin.Output{Per: plugin.PerPackage, Word: word}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if name != nil {
				e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: name(m)})
			}
			return nil
		})).Build().(plugin.Generator)
	if !held {
		panic("workspacetest_test: an emitter rule lowers to the generator role")
	}
	return p
}

// printer returns a backend under a name that spells a struct as one
// line and names every file after its family word, framed through a
// line comment.
func printer(name plugin.ID) plugin.Backend {
	return backend.New(name, fixtureTarget, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) { return nil, errNoStatements }).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: everyFact()}).
		Build()
}

// everyFact renders every fact, which the printer's one template states
// none of.
func everyFact() map[symbol.Fact]render.Verdict {
	out := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		out[f] = render.Renders
	}
	return out
}

// planOf returns a plan under a name, running one generator toward a
// printer of its own and depending on deps.
func planOf(name string, gen plugin.Generator, deps ...string) workspace.Plan {
	return workspace.Plan{
		Name:       name,
		DependsOn:  deps,
		Generators: []plugin.Generator{gen},
		Backend:    printer(plugin.ID(name + "-printer")),
	}
}

// mirrors returns the plan that emits ForRow for Row.
func mirrors() workspace.Plan {
	return planOf(mirrorsPlan, naming(mirrorID, genWord, func(m *eidos.StructMatch) string {
		return "For" + m.Struct.Name
	}))
}

// stubs returns the plan that emits RowStub for Row and depends on
// mirrors.
func stubs() workspace.Plan {
	return planOf(stubsPlan, naming(stubberID, stubWord, func(m *eidos.StructMatch) string {
		return m.Struct.Name + "Stub"
	}), mirrorsPlan)
}

// idle returns a plan whose generator emits nothing, so it routes no
// file.
func idle() workspace.Plan { return planOf(idlePlan, naming(idleID, genWord, nil)) }

// onDisk returns the composition over root without its plans: the
// fixture's brand and target, the scripted frontend, a disk sink at root
// and the state directory's ledger under root.
func onDisk(root string) *workspace.Builder {
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Targets(fixtureTarget).
		Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) })
}

// stamped returns the bytes the output contract stamps for a body one
// plugin assembled from Row's package.
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

// fixture returns the fixture: Row's source, the composition without
// its plans, the two plans, and the two files they generate.
func fixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return workspacetest.Fixture{
		Tree:    fstest.MapFS{rowFile: {Data: []byte(rowSource)}},
		Compose: onDisk,
		Plans:   func() []workspace.Plan { return []workspace.Plan{mirrors(), stubs()} },
		Want: map[string][]byte{
			genFile:  stamped(t, genFile, genBody, mirrorID),
			stubFile: stamped(t, stubFile, stubBody, stubberID),
		},
	}
}
