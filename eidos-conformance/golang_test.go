// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/pipelinetest"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The Go corpus's signature root and its one unexported
// declaration, which a signature-only load drops.
const (
	constantsRoot = "f/constants"
	limitName     = "limit"
)

// repositoryRoot is the root of this repository, relative to the
// module directory the benchmark runs in.
const repositoryRoot = ".."

// The end-to-end fixture: the brand its carriers open with, its one
// plan, the two plugins of the plan and the capability that orders
// them, the stub's family word and tag, the field a stub delegates
// through and Go's selector before a field or a method, the function
// the weaver calls, and the fixture's trees: the workspace, the
// standard library's one package it reads, and the file the plan
// generates.
const (
	acmeBrand    output.Brand      = "acme"
	stubsPlan                      = "go-stubs"
	stubgenID    plugin.ID         = "stubgen"
	auditID      plugin.ID         = "acme-audit"
	stubsCap     plugin.Capability = "stubs"
	stubWord                       = "stub"
	testTag      eidos.Tag         = "test"
	nextField                      = "next"
	selector                       = "."
	auditCallee                    = "audit"
	pipelineTree                   = "testdata/pipeline/go/tree"
	pipelineRoot                   = "testdata/pipeline/go/goroot"
	stubFile                       = "svc/store_stub_test.go"
	stubWant                       = "testdata/pipeline/go/want/" + stubFile
)

// stubSchema is stubgen's directive: the interface it doubles. The
// reserved tag key picks the family, so the schema declares no key of
// its own.
var stubSchema = directive.Schema{
	Plugin: string(stubgenID),
	Name:   "stub",
	Doc:    "generates a test double that delegates every method of the interface",
}

// Go's corpus entry: the first real language against the shared
// inventory. The one refusal is Go's own semantics: overloads do
// not exist. A typed constant group loads as the enum the schema
// names it.
func TestGolang(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Corpus{
		Frontend: gofrontend.New(nil),
		Sources:  os.DirFS("testdata/go"),
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Projects,
			"struct_methods":      conformance.Projects,
			"method_overloads":    conformance.Refuses,
			"constants":           conformance.Projects,
			"cross_package_ref":   conformance.Projects,
			"composite_refs":      conformance.ProjectsPartly,
			"builtin_ref":         conformance.Projects,
			"directive_carrier":   conformance.Projects,
			"test_classification": conformance.Projects,
			"interfaces":          conformance.Projects,
			"enum_values":         conformance.Projects,
		},
		Signatures: []string{constantsRoot},
		Dropped: []symbol.Identity{
			{Lang: gofrontend.Lang, Package: constantsRoot, Name: limitName, Kind: symbol.KindConstant},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    gofrontend.Keys,
		Rules:   gorules.New(),
	})
}

// The stub half of the one-declaration document runs end to end: the
// Go frontend loads an interface under //+acme:stub tag=test, stubgen
// mirrors its methods onto a double on pointer receivers, the audit
// weaver calls audit first in each, and the Go backend writes the
// double beside its source as svc/store_stub_test.go.
func TestGolangPipeline(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(filepath.FromSlash(stubWant))
	assert.NoError(t, err, "the generated file's golden reads")
	pipelinetest.RunPipelineSuite(t, pipelinetest.Fixture{
		Tree:    os.DirFS(pipelineTree),
		Stores:  map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(pipelineRoot)},
		Compose: composeStubs,
		Want:    map[string][]byte{stubFile: want},
	})
}

// BenchmarkGolang loads this repository with the Go frontend over the
// machine's module cache and standard library: every workspace unit at
// full depth, then the dependency rounds at signature depth. It
// reports the rounds, the workspace units and the dependency units
// beside the time and the allocations. It pins no allocation ceiling,
// because its input is the repository and the toolchain the machine
// has installed, and both change without a change to the load.
func BenchmarkGolang(b *testing.B) {
	stores, err := gofrontend.Stores(os.Getenv)
	if err != nil {
		b.Fatalf("the machine's module cache and standard library resolve: %v", err)
	}
	tree := os.DirFS(repositoryRoot)
	fronts := []plugin.Frontend{gofrontend.New(nil)}

	b.Run("Load", func(b *testing.B) {
		var report *load.Report
		b.ReportAllocs()
		for b.Loop() {
			sink := diag.NewSink()
			_, loaded, loadErr := load.Load(context.Background(), load.Config{
				FS:        tree,
				Frontends: fronts,
				Sink:      sink,
				Brand:     frontendtest.Brand,
				Stores:    stores,
			})
			if loadErr != nil || sink.Failed() {
				b.Fatalf("the repository loads without an error: %v", loadErr)
			}
			report = loaded
		}
		var rounds, workspace, dependency int
		for _, u := range report.Units {
			rounds = max(rounds, u.Round)
			if u.Round == 0 {
				workspace++
			} else {
				dependency++
			}
		}
		if dependency == 0 {
			b.Fatal("the dependency rounds return units")
		}
		b.ReportMetric(float64(rounds), "rounds")
		b.ReportMetric(float64(workspace), "workspace-units")
		b.ReportMetric(float64(dependency), "dependency-units")
	})
}

// composeStubs is the end-to-end fixture's composition over root: the
// Go frontend and rules, one plan of stubgen and the audit weaver
// toward the Go backend, a disk sink at root and the state directory's
// ledger under root.
func composeStubs(root string) (*workspace.Workspace, error) {
	return workspace.New().
		Brand(acmeBrand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target).
		Plans(workspace.Plan{
			Name:       stubsPlan,
			Generators: []plugin.Generator{stubgen(), audit()},
			Backend:    gobackend.New(),
		}).
		Output(func() (output.Sink, error) { return output.NewDisk(root, acmeBrand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, acmeBrand) }).
		Build()
}

// stubgen returns the fixture's generator: per interface under its
// directive, a double that delegates every method to a wrapped value of
// the interface, each method on a pointer receiver named apart from its
// parameters. It declares a primary family and a test family, and the
// directive's tag picks the family.
func stubgen() plugin.Generator {
	p, held := eidos.NewPlugin(stubgenID).
		Provides(stubsCap).
		Output(plugin.Output{Per: plugin.PerSource, Word: stubWord}).
		Output(plugin.Output{Tag: string(testTag), Per: plugin.PerSource, Word: stubWord}).
		Handle(eidos.Directive(stubSchema, eidos.OnInterface(stub))).
		Build().(plugin.Generator)
	if !held {
		panic("conformance_test: stubgen lowers to the generator role")
	}
	return p
}

// stub emits the double of one interface into the primary family, and
// the directive's tag redirects it.
func stub(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	e.File().Append(double(e.JoinName(stubWord, m.Interface.Name), m))
	return nil
}

// double returns the double of one interface under a name: a struct
// that wraps a value of the interface and delegates every method to it,
// each method on a pointer receiver named apart from its parameters.
func double(name string, m *eidos.InterfaceMatch) *emit.Struct {
	d := &emit.Struct{Origin: m.Interface.ID, Name: name}
	d.Fields.Append(&emit.Field{
		Name:       nextField,
		Visibility: symbol.VisibilityPackage,
		Type:       &emit.TypeRef{Spelling: m.Interface.Name, Target: m.Interface.ID},
	})
	for _, method := range m.Interface.Methods {
		mirrored := golang.PointerReceiver(eidos.Mirror(name, method))
		args := make([]emit.Expr, 0, len(mirrored.Params))
		for _, p := range mirrored.Params {
			args = append(args, emit.Expr{Kind: emit.ExprName, Name: p.Name})
		}
		callee := emit.Expr{
			Kind: emit.ExprName,
			Name: mirrored.Receiver.Name + selector + nextField + selector + mirrored.Name,
		}
		mirrored.Body.Stmts = []emit.Stmt{{
			Kind:  emit.StmtReturn,
			Value: emit.Expr{Kind: emit.ExprCall, Fn: &callee, Args: args},
		}}
		d.Methods.Append(mirrored)
	}
	return d
}

// audit returns the fixture's weaver: it runs after stubgen, through the
// capability stubgen provides, and calls audit first in every method
// stubgen emits.
func audit() plugin.Generator {
	p, held := eidos.NewPlugin(auditID).
		Requires(stubsCap).
		Handle(eidos.OnEmit(symbol.KindMethod, weave)).
		Build().(plugin.Generator)
	if !held {
		panic("conformance_test: the audit weaver lowers to the generator role")
	}
	return p
}

// weave appends the audit call into one method's prologue through the
// Emitter, passing the method's first parameter, which names the weaver
// in the frame of the file the method is written in. A method without a
// parameter has nothing to pass, and is left as it is.
func weave(m *eidos.EmitMatch, e *eidos.Emitter) error {
	method, is := m.Value.(*emit.Method)
	if !is || len(method.Params) == 0 {
		return nil
	}
	callee := emit.Expr{Kind: emit.ExprName, Name: auditCallee}
	e.Slot(&method.Body.Prologue).Append(emit.Stmt{
		Kind: emit.StmtExpr,
		Value: emit.Expr{
			Kind: emit.ExprCall,
			Fn:   &callee,
			Args: []emit.Expr{{Kind: emit.ExprName, Name: method.Params[0].Name}},
		},
	})
	return nil
}
