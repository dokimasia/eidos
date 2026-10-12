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
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The TypeScript plan of the cross-language fixture, its client generator,
// the family word of the client, and the name of each file of the family.
// Without the name, the family word would follow the stem, so the layout of
// the plan names the client of svc/store.go svc/store.ts.
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

// ComposeHub returns the composition of the cross-language fixture over
// root without its plans. The composition has the Go frontend and rules,
// the Go and TypeScript targets, a disk sink at root and the ledger of the
// state directory under root. [HubPlans] returns the plans that it runs.
func ComposeHub(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target, typescript.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// HubPlans returns the plans of the cross-language fixture over one graph.
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

// EditHub is the edit of the cross-language fixture. It renames the
// parameter of Get in svc/store.go under root. The double of go-stubs and
// the client of ts-client change. The export of go-stubs lists no
// parameter, so it does not change. EditHub reads and writes the one file.
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
	return eidos.NewPlugin(tsclientID).
		Output(plugin.Output{Per: plugin.PerSource, Word: clientWord}).
		Handle(eidos.OnInterface(interfaceClient)).
		Generator()
}

// tsclient returns the client generator of the cross-language fixture. It
// writes a TypeScript interface for each struct and each interface of
// its scope, with the exported members alone, and it translates every
// type through the Emitter. Each declaration that it emits has the Go
// declaration as its origin, so a name override on the Go declaration
// applies.
func tsclient() plugin.Generator {
	return eidos.NewPlugin(tsclientID).
		Output(plugin.Output{Per: plugin.PerSource, Word: clientWord}).
		Handle(eidos.OnStruct(structClient), eidos.OnInterface(interfaceClient)).
		Generator()
}

// structClient writes the client interface of one struct, with a
// property for each exported field. It withholds the interface where the
// target has no spelling of the type of a field, after the Emitter reports
// the refusal.
func structClient(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	fields, spelled := e.Fields(s.Fields, func(f *node.Field) bool { return f.Visibility == symbol.VisibilityPublic })
	if !spelled {
		return nil
	}
	e.File().Append(&emit.Interface{
		Origin: s.ID, Doc: s.Doc, Name: s.Name, Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
	})
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
			lowered.Params.Append(&emit.Param{Name: p.Name, Type: t})
		}
		for _, r := range c.Returns {
			if r.Role == rules.ReturnError {
				continue
			}
			t, spelled := e.Type(method.ID, r.Ref)
			if !spelled {
				return nil
			}
			lowered.Returns.Append(&emit.Return{Type: t})
		}
		client.Methods.Append(lowered)
	}
	e.File().Append(client)
	return nil
}
