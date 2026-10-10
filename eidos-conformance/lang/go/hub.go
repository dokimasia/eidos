// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/lang/typescript"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The cross-language fixture's TypeScript plan, its client generator, the
// client's family word, and the name of each file of the family. The
// family's word would otherwise follow the stem, so the plan's layout
// names the client of svc/store.go svc/store.ts.
const (
	clientPlan           = "ts-client"
	tsclientID plugin.ID = "tsclient"
	clientWord           = "client"
	clientFile           = "store.ts"
)

// The signature of Get in the store of the cross-language fixture before
// and after [EditHub] renames its parameter.
const (
	getByKeyInContext = "Get(ctx context.Context, key string)"
	getByIDInContext  = "Get(ctx context.Context, id string)"
)

// ComposeHub returns the cross-language fixture's composition over root
// without its plans: the Go frontend and rules, the Go and TypeScript
// targets, a disk sink at root and the state directory's ledger under
// root. [HubPlans] returns the plans that it runs.
func ComposeHub(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target, typescript.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// HubPlans returns the cross-language fixture's plans over one graph.
// The plan go-stubs runs stubgen and the audit weaver toward the Go
// backend, and it writes svc/store_stub_test.go. The plan ts-client,
// scoped to svc, runs the client generator toward the TypeScript
// backend, and it writes svc/store.ts. Each call returns new generators
// and backends.
func HubPlans() []workspace.Plan {
	return []workspace.Plan{
		{
			Name:       pipelinePlan,
			Generators: []plugin.Generator{stubgen(), audit()},
			Backend:    gobackend.New(),
		},
		{
			Name:       clientPlan,
			Sources:    workspace.Sources{Packages: []string{svcSources}},
			Generators: []plugin.Generator{tsclient()},
			Backend:    tsbackend.New(),
			Layout: layout.Config{
				Families: map[layout.Family]layout.Refinement{{Plugin: tsclientID}: {File: clientFile}},
			},
		},
	}
}

// EditHub is the cross-language fixture's edit: it renames the parameter
// of Get in svc/store.go under root. The double of go-stubs and the
// client of ts-client change, and the export of go-stubs, which lists no
// parameter, does not. It reads and writes the one file.
//
// Error modes: the error of a file that does not read or write, and an
// error for a file that declares no Get with the parameter key.
func EditHub(root string) error {
	return rename(root, storeFile, getByKeyInContext, getByIDInContext)
}

// StoreClient returns the variant of the client generator that writes
// the client of each interface alone. The client of Store keeps its
// reference to Session, and the plan does not emit a declaration of
// Session.
func StoreClient() plugin.Generator {
	return generator(eidos.NewPlugin(tsclientID).
		Output(plugin.Output{Per: plugin.PerSource, Word: clientWord}).
		Handle(eidos.OnInterface(interfaceClient)).
		Build())
}

// tsclient returns the cross-language fixture's client generator. It
// writes a TypeScript interface for each struct and each interface of
// its scope, with the exported members alone, and it translates every
// type through the Emitter. Each declaration that it emits has the Go
// declaration as its origin, so a name override on the Go declaration
// applies.
func tsclient() plugin.Generator {
	return generator(eidos.NewPlugin(tsclientID).
		Output(plugin.Output{Per: plugin.PerSource, Word: clientWord}).
		Handle(eidos.OnStruct(structClient), eidos.OnInterface(interfaceClient)).
		Build())
}

// structClient writes the client interface of one struct, with a
// property for each exported field. It withholds the interface where the
// target has no spelling of a field's type, after the Emitter reports
// the refusal.
func structClient(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	client := &emit.Interface{Origin: s.ID, Doc: s.Doc, Name: s.Name, Visibility: s.Visibility}
	for _, f := range s.Fields {
		if f.Visibility != symbol.VisibilityPublic {
			continue
		}
		t, spelled := e.Type(f.ID, f.Type)
		if !spelled {
			return nil
		}
		client.Fields.Append(&emit.Field{Origin: f.ID, Doc: f.Doc, Name: f.Name, Visibility: f.Visibility, Type: t})
	}
	e.File().Append(client)
	return nil
}

// interfaceClient writes the client interface of one interface, with an
// async method for each exported method, because a client calls the
// store across the network. A method of the client drops each parameter
// that the source rules classify as a context and each error return. It
// withholds the interface where the target has no spelling of a type,
// after the Emitter reports the refusal.
func interfaceClient(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	iface := m.Interface
	client := &emit.Interface{Origin: iface.ID, Doc: iface.Doc, Name: iface.Name, Visibility: iface.Visibility}
	bound := m.Rules()
	for _, method := range iface.Methods {
		if method.Visibility != symbol.VisibilityPublic {
			continue
		}
		// A method is callable, so the projection reports true.
		c, _ := bound.CallableOf(method)
		lowered := &emit.Method{
			Origin: method.ID, Doc: method.Doc, Name: method.Name, Visibility: method.Visibility, Async: true,
		}
		for _, p := range c.Params {
			if p.Role == rules.ParamContext {
				continue
			}
			t, spelled := e.Type(method.ID, p.Ref)
			if !spelled {
				return nil
			}
			lowered.Params = append(lowered.Params, &emit.Param{Name: p.Name, Type: t})
		}
		for _, r := range c.Returns {
			if r.Role == rules.ReturnError {
				continue
			}
			t, spelled := e.Type(method.ID, r.Ref)
			if !spelled {
				return nil
			}
			lowered.Returns = append(lowered.Returns, &emit.Return{Type: t})
		}
		client.Methods.Append(lowered)
	}
	e.File().Append(client)
	return nil
}
