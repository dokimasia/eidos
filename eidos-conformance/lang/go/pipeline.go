// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Brand is the brand the package's fixtures stamp their files under,
// and the one their carriers spell, as in //+acme:stub.
const Brand output.Brand = "acme"

// The end-to-end fixture: its one plan, the two plugins of the plan
// and the capability that orders them, the stub's family word and tag,
// the field a stub delegates through and Go's selector before a field
// or a method, and the function the weaver calls.
const (
	pipelinePlan                   = "go-stubs"
	stubgenID    plugin.ID         = "stubgen"
	auditID      plugin.ID         = "acme-audit"
	stubsCap     plugin.Capability = "stubs"
	stubWord                       = "stub"
	testTag      eidos.Tag         = "test"
	nextField                      = "next"
	selector                       = "."
	auditCallee                    = "audit"
)

// stubSchema is stubgen's directive: the interface it doubles. The
// reserved tag key picks the family, so the schema declares no key of
// its own.
var stubSchema = directive.Schema{
	Plugin: string(stubgenID),
	Name:   "stub",
	Doc:    "generates a test double that delegates every method of the interface",
}

// ComposePipeline returns the end-to-end fixture's composition over root
// without its plan: the Go frontend and rules, the Go target, a disk sink
// at root and the state directory's ledger under root.
func ComposePipeline(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// PipelinePlans returns the one plan of the end-to-end fixture: stubgen
// and the audit weaver toward the Go backend, with new plugin instances
// on every call. stubgen doubles each interface under //+acme:stub, and
// the directive's tag picks the double's family. The weaver calls audit
// first in every method stubgen emits.
func PipelinePlans() []workspace.Plan {
	return []workspace.Plan{{
		Name:       pipelinePlan,
		Generators: []plugin.Generator{stubgen(), audit()},
		Backend:    gobackend.New(),
	}}
}

// stubgen returns the end-to-end fixture's generator: per interface
// under its directive, a double that delegates every method to a
// wrapped value of the interface, each method on a pointer receiver
// named apart from its parameters. It declares a primary family and a
// test family, and the directive's tag picks the family.
func stubgen() plugin.Generator {
	return eidos.NewPlugin(stubgenID).
		Provides(stubsCap).
		Output(plugin.Output{Per: plugin.PerSource, Word: stubWord}).
		Output(plugin.Output{Tag: string(testTag), Per: plugin.PerSource, Word: stubWord}).
		Handle(eidos.Directive(stubSchema, eidos.OnInterface(stub))).
		Generator()
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
		args := make([]emit.Expr, 0, mirrored.Params.Len())
		for _, p := range mirrored.Params.Items() {
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

// audit returns the end-to-end fixture's weaver: it runs after stubgen,
// through the capability stubgen provides, and calls audit first in
// every method stubgen emits.
func audit() plugin.Generator {
	return eidos.NewPlugin(auditID).
		Requires(stubsCap).
		Handle(eidos.OnEmit(symbol.KindMethod, weave)).
		Generator()
}

// weave appends the audit call into the prologue of one method through
// the Emitter. The call passes the first parameter of the method as its
// argument, by the name that the parameter has in the file of the
// method. A method without a parameter has nothing to pass, and is left
// as it is.
func weave(m *eidos.EmitMatch, e *eidos.Emitter) error {
	method, is := m.Value.(*emit.Method)
	if !is || method.Params.Len() == 0 {
		return nil
	}
	callee := emit.Expr{Kind: emit.ExprName, Name: auditCallee}
	e.Slot(&method.Body.Prologue).Append(emit.Stmt{
		Kind: emit.StmtExpr,
		Value: emit.Expr{
			Kind: emit.ExprCall,
			Fn:   &callee,
			Args: []emit.Expr{{Kind: emit.ExprName, Name: method.Params.Items()[0].Name}},
		},
	})
	return nil
}
