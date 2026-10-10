// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Brand is the brand that the package's fixtures stamp their files
// under, and the one that their carriers spell, as in // +acme:stub.
const Brand output.Brand = "acme"

// The TypeScript pipeline fixture: its one plan, its stub generator, the
// stub's family word and tag, the field that a stub delegates through,
// the name of a stub's constructor, and the receiver and the selector
// of a delegating call.
const (
	pipelinePlan           = "ts-stubs"
	stubgenID    plugin.ID = "stubgen"
	stubWord               = "stub"
	testTag      eidos.Tag = "test"
	nextField              = "next"
	constructor            = "constructor"
	thisReceiver           = "this"
	selector               = "."
)

// stubSchema is stubgen's directive: the interface that it doubles. The
// reserved tag key picks the family, so the schema declares no key of
// its own.
var stubSchema = directive.Schema{
	Plugin: string(stubgenID),
	Name:   "stub",
	Doc:    "generates a test double that delegates every method of the interface",
}

// ComposeStubs returns the TypeScript pipeline fixture's composition
// over root: the TypeScript frontend and rules, one plan of stubgen
// toward the TypeScript backend, a disk sink at root and the state
// directory's ledger under root. stubgen doubles each interface under
// // +acme:stub, and the directive's tag picks the double's family.
//
// It returns the error of Build. The fixture's composition builds
// without one.
func ComposeStubs(root string) (*workspace.Workspace, error) {
	return workspace.New().
		Brand(Brand).
		Frontends(tsfrontend.New()).
		Rules(tsrules.New()).
		Targets(typescript.Target).
		Plans(workspace.Plan{
			Name:       pipelinePlan,
			Generators: []plugin.Generator{stubgen()},
			Backend:    tsbackend.New(),
		}).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) }).
		Build()
}

// stubgen returns the TypeScript pipeline fixture's generator: for each
// interface under its directive, a class that implements the interface
// and delegates every method to a wrapped value of it. It declares a
// primary family and a test family, and the directive's tag picks the
// family.
func stubgen() plugin.Generator {
	return generator(eidos.NewPlugin(stubgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: stubWord}).
		Output(plugin.Output{Tag: string(testTag), Per: plugin.PerSource, Word: stubWord}).
		Handle(eidos.Directive(stubSchema, eidos.OnInterface(stub))).
		Build())
}

// stub emits the double of one interface into the primary family, and
// the directive's tag redirects it.
func stub(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	e.File().Append(double(e.JoinName(stubWord, m.Interface.Name), m))
	return nil
}

// double returns the double of one interface under a name: a class that
// implements the interface, keeps a value of it in a private readonly
// field, which its constructor assigns, and delegates every method to
// that value.
func double(name string, m *eidos.InterfaceMatch) *emit.Struct {
	iface := &emit.TypeRef{Spelling: m.Interface.Name, Target: m.Interface.ID}
	d := &emit.Struct{
		Origin: m.Interface.ID, Name: name, Visibility: symbol.VisibilityPublic,
		Implements: []*emit.TypeRef{iface},
	}
	d.Fields.Append(&emit.Field{
		Name: nextField, Visibility: symbol.VisibilityPrivate, Mutability: symbol.MutabilityImmutable, Type: iface,
	})
	ctor := &emit.Method{
		Origin: m.Interface.ID, Name: constructor, Visibility: symbol.VisibilityPublic, Constructs: true,
		Params: []*emit.Param{{Name: nextField, Type: iface}},
	}
	ctor.Body.Stmts = []emit.Stmt{{
		Kind:  emit.StmtAssign,
		Names: []string{thisReceiver + selector + nextField},
		Value: emit.Expr{Kind: emit.ExprName, Name: nextField},
	}}
	d.Methods.Append(ctor)
	for _, method := range m.Interface.Methods {
		mirrored := eidos.Mirror(name, method)
		args := make([]emit.Expr, 0, len(mirrored.Params))
		for _, p := range mirrored.Params {
			args = append(args, emit.Expr{Kind: emit.ExprName, Name: p.Name})
		}
		callee := emit.Expr{Kind: emit.ExprName, Name: thisReceiver + selector + nextField + selector + mirrored.Name}
		mirrored.Body.Stmts = []emit.Stmt{{
			Kind:  emit.StmtReturn,
			Value: emit.Expr{Kind: emit.ExprCall, Fn: &callee, Args: args},
		}}
		d.Methods.Append(mirrored)
	}
	return d
}

// generator returns a built plugin as the generator that its emitter
// rules make it. Every plugin of the package declares emitter rules, so
// a plugin that is not a generator is a defect in this package.
func generator(p plugin.Plugin) plugin.Generator {
	g, held := p.(plugin.Generator)
	if !held {
		panic("typescript: a plugin of emitter rules lowers to a generator")
	}
	return g
}
