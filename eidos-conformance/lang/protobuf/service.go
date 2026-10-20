// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	protolang "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
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

// Brand is the brand under which the service fixture stamps its files. A
// carrier of the fixture starts with it, as in //+acme:typescript.
const Brand output.Brand = "acme"

// The plans of the service fixture, its generators, the family word of
// each generator, the name of the file of the client, and the import path
// of the root of the tree. The Go backend derives the package of each file
// from that import path, because a tree of schemas has no Go module. The
// layout of the plan gives the client of svc/store.proto the file name
// store.ts. Without it, the file name of the client would join the stem and
// the family word.
const (
	serverPlan            = "go-server"
	clientPlan            = "ts-client"
	servergenID plugin.ID = "servergen"
	clientgenID plugin.ID = "clientgen"
	serverWord            = "server"
	clientWord            = "client"
	clientFile            = "store.ts"
	serviceBase           = "example.com/acme"
	schemaPlan            = "proto-schema"
	storeFile             = "store.proto"
)

// The plans of the binding fixture, their handler generators, the family
// word of each generator, and the file that each plan writes for
// svc/store.proto. Each generator writes the interface <Service><Word> of
// each service, so the files of the two Go plans declare two names in one
// package.
const (
	grpcPlan                 = "go-grpc"
	connectPlan              = "go-connect"
	connectESPlan            = "ts-connect"
	grpcgenID      plugin.ID = "grpcgen"
	connectgenID   plugin.ID = "connectgen"
	connectesgenID plugin.ID = "connectesgen"
	grpcWord                 = "grpc"
	connectWord              = "connect"
	handlerWord              = "handler"
	grpcFile                 = "store_grpc.go"
	connectFile              = "store_connect.go"
	connectESFile            = "store_handler.ts"
)

// serverDir is the directory below which the plan go-connect writes. A
// handler of connect-go names the stream types of the connect package, and
// that package imports the messages, so the handler is in a package of its
// own.
const serverDir = "server"

// The schema that the edit of the service fixture changes, the field of
// Summary before and after the edit, and the mode of the file that the
// edit writes.
const (
	schemaFile = "svc/store.proto"
	wideSize   = "int64 size = 1;"
	narrowSize = "int32 size = 1;"
	schemaMode = 0o644
)

// ComposeService returns the composition of the service fixture over root
// without its plans. The composition has the protobuf frontend and rules,
// the Go, TypeScript and protobuf targets, a disk sink at root and the
// ledger of the state directory under root. [ServicePlans] returns the
// plans that write the same files from every version of the schema, and
// [SchemaPlan] returns the plan that mirrors the schema.
func ComposeService(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(protofrontend.New()).
		Rules(protorules.New()).
		Targets(golang.Target, typescript.Target, protolang.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// ServicePlans returns the plans of the service fixture over one graph. The
// plan go-server runs servergen toward the Go backend, and it writes
// svc/store_server.go. The plan ts-client runs clientgen toward the
// TypeScript backend, and it writes svc/store.ts. Each call returns new
// generators and backends.
func ServicePlans() []workspace.Plan {
	return []workspace.Plan{
		{
			Name:       serverPlan,
			Generators: []plugin.Generator{servergen()},
			Backend:    gobackend.New(),
			Layout:     layout.Config{ImportBase: serviceBase},
		},
		{
			Name:       clientPlan,
			Generators: []plugin.Generator{clientgen()},
			Backend:    tsbackend.New(),
			Layout: layout.Config{
				Families: map[layout.Family]layout.Refinement{{Plugin: clientgenID}: {File: clientFile}},
			},
		},
	}
}

// BindingPlans returns the plans of the binding fixture over the graph of
// the service fixture, which write the handlers of each rpc under the gRPC
// binding that [BindingConfig] selects for each plan. The plan go-grpc runs
// grpcgen toward the Go backend, and it writes svc/store_grpc.go. The plan
// go-connect runs connectgen toward the same Go backend, and it writes
// server/svc/store_connect.go. The plan ts-connect runs connectesgen toward the
// TypeScript backend, and it writes svc/store_handler.ts. Each call returns
// new generators and backends.
func BindingPlans() []workspace.Plan {
	gob := gobackend.New()
	return []workspace.Plan{
		{
			Name:       grpcPlan,
			Generators: []plugin.Generator{handlergen(grpcgenID, grpcWord)},
			Backend:    gob,
			Layout: layout.Config{
				ImportBase: serviceBase,
				Families:   map[layout.Family]layout.Refinement{{Plugin: grpcgenID}: {File: grpcFile}},
			},
		},
		{
			Name:       connectPlan,
			Generators: []plugin.Generator{handlergen(connectgenID, connectWord)},
			Backend:    gob,
			Layout: layout.Config{
				ImportBase: serviceBase,
				Families: map[layout.Family]layout.Refinement{{Plugin: connectgenID}: {
					Policy: layout.PolicyCentralised, Dir: serverDir, File: connectFile,
				}},
			},
		},
		{
			Name:       connectESPlan,
			Generators: []plugin.Generator{handlergen(connectesgenID, handlerWord)},
			Backend:    tsbackend.New(),
			Layout: layout.Config{
				Families: map[layout.Family]layout.Refinement{{Plugin: connectesgenID}: {File: connectESFile}},
			},
		},
	}
}

// BindingConfig returns the config of the binding fixture. It selects
// grpc-go for the plan go-grpc, connect-go for go-connect, and connect-es
// for ts-connect.
func BindingConfig() workspace.Config {
	return workspace.Config{Plans: map[string]workspace.PlanConfig{
		grpcPlan:      {Policies: map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCGo}},
		connectPlan:   {Policies: map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCConnect}},
		connectESPlan: {Policies: map[plugin.PolicyKey]plugin.Choice{typescript.GRPC: typescript.GRPCConnect}},
	}}
}

// SchemaPlan returns the plan proto-schema of the service fixture. It runs
// mirrorgen toward the protobuf backend, and it writes the mirror of
// svc/store.proto at mirror/svc/store.proto. The mirror differs from the
// schema of each version, so the plan is not one of [ServicePlans]. Each
// call returns a new generator and a new backend.
func SchemaPlan() workspace.Plan {
	return MirrorPlan(schemaPlan, storeFile)
}

// EditService is the edit of the service fixture. It narrows the field size
// of the message Summary in svc/store.proto under root from int64 to int32.
// The struct of go-server and the interface of ts-client change, and the
// export of go-server does not, because the export lists the names of the
// declarations and their members and not their types. It reads and writes
// the one file through an [os.Root] of root.
//
// Error modes: the error of a root or a file that does not open, read or
// write, and an error for a schema that does not declare the field size
// with the type int64.
func EditService(root string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("protobuf: %w", err)
	}
	defer r.Close()
	name := filepath.FromSlash(schemaFile)
	b, err := r.ReadFile(name)
	if err != nil {
		return fmt.Errorf("protobuf: %w", err)
	}
	src := string(b)
	if !strings.Contains(src, wideSize) {
		return fmt.Errorf("protobuf: %s declares no field %q to narrow", schemaFile, wideSize)
	}
	if err := r.WriteFile(name, []byte(strings.Replace(src, wideSize, narrowSize, 1)), schemaMode); err != nil {
		return fmt.Errorf("protobuf: %w", err)
	}
	return nil
}

// servergen returns the server generator of the service fixture. It writes
// a Go struct for each message, a sum for each oneof and an enum for each
// enum, each under its flat name. It writes the interface <Service>Server
// for each service.
func servergen() plugin.Generator {
	return eidos.NewPlugin(servergenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: serverWord}).
		Handle(
			eidos.OnStruct(serverMessage), eidos.OnSum(sumOf), eidos.OnEnum(enumOf),
			eidos.OnInterface(serverService),
		).
		Generator()
}

// handlergen returns the handler generator id of the binding fixture under
// the family word word. It writes the interface <Service><Word> of each
// service, with the method that [eidos.Emitter.Handler] returns for each
// rpc. handlergen withholds the interface where the target cannot serve an
// rpc, after the Emitter reports the refusal.
func handlergen(id plugin.ID, word string) plugin.Generator {
	return eidos.NewPlugin(id).
		Output(plugin.Output{Per: plugin.PerSource, Word: word}).
		Handle(eidos.OnInterface(func(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
			service := m.Interface
			out := &emit.Interface{
				Origin: service.ID, Doc: service.Doc, Name: e.JoinName(word, service.Name),
				Visibility: service.Visibility,
			}
			if !handlers(out, service, e) {
				return nil
			}
			e.File().Append(out)
			return nil
		})).
		Generator()
}

// clientgen returns the client generator of the service fixture. It writes
// a TypeScript interface for each message, a sum for each oneof and an enum
// for each enum, each under its flat name. It writes the interface
// <Service>Client for each service.
func clientgen() plugin.Generator {
	return eidos.NewPlugin(clientgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: clientWord}).
		Handle(
			eidos.OnStruct(clientMessage), eidos.OnSum(sumOf), eidos.OnEnum(enumOf),
			eidos.OnInterface(clientService),
		).
		Generator()
}

// serverMessage writes the Go struct of one message under its flat name,
// with a field for each field of the message and a field for each oneof.
// The Emitter translates the type of each field with its presence. The
// field of a oneof refers to the sum of the oneof, whose nil value is the
// absent value. The reference has the flat name of the oneof and the oneof
// as its target, so the settle follows it to the sum that has the oneof as
// its origin. serverMessage withholds the struct where Go has no spelling
// of the type of a field, after the Emitter reports the refusal.
func serverMessage(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	fields, spelled := e.Fields(s.Fields, nil)
	if !spelled {
		return nil
	}
	out := &emit.Struct{
		Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
	}
	for _, nested := range s.Types {
		if sum, oneof := nested.(*node.Sum); oneof {
			out.Fields.Append(&emit.Field{
				Doc: sum.Doc, Name: sum.Name, Visibility: sum.Visibility,
				Type: &emit.TypeRef{Spelling: sum.ID.FlatName(), Target: sum.ID},
			})
		}
	}
	e.File().Append(out)
	return nil
}

// clientMessage writes the TypeScript interface of one message under its
// flat name, with a property for each field of the message and a property
// for each oneof. A property has a question mark where its field has
// presence, which the rules of the language of the field decide. The
// property of a oneof always has one, and it refers to the sum of the
// oneof as the struct of [serverMessage] does. clientMessage withholds the
// interface where TypeScript has no spelling of the type of a field, after
// the Emitter reports the refusal.
func clientMessage(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	fields, spelled := e.Fields(s.Fields, nil)
	if !spelled {
		return nil
	}
	bound := m.Rules()
	for i, f := range fields {
		f.Optional = bound.FieldTypeOf(s.Fields[i]).Form == symbol.FormOptional
	}
	out := &emit.Interface{
		Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
	}
	for _, nested := range s.Types {
		if sum, oneof := nested.(*node.Sum); oneof {
			out.Fields.Append(&emit.Field{
				Doc: sum.Doc, Name: sum.Name, Visibility: sum.Visibility, Optional: true,
				Type: &emit.TypeRef{Spelling: sum.ID.FlatName(), Target: sum.ID},
			})
		}
	}
	e.File().Append(out)
	return nil
}

// sumOf writes the sum of one oneof under its flat name, with a variant
// for each member. The one field of a variant has the type of the member,
// and no documentation, because the variant has the documentation of the
// member. sumOf withholds the sum where the target has no spelling of the
// type of a member, after the Emitter reports the refusal.
func sumOf(m *eidos.SumMatch, e *eidos.Emitter) error {
	s := m.Sum
	out := &emit.Sum{Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility}
	for _, v := range s.Variants {
		variant := &emit.SumVariant{Origin: v.ID, Doc: v.Doc, Name: v.Name}
		for _, f := range v.Fields {
			t, spelled := e.Type(f.ID, f.Type)
			if !spelled {
				return nil
			}
			variant.Fields.Append(&emit.Field{Origin: f.ID, Name: f.Name, Visibility: f.Visibility, Type: t})
		}
		out.Variants.Append(variant)
	}
	e.File().Append(out)
	return nil
}

// enumOf writes one enum under its flat name, with a variant for each
// value and the number that the schema declares.
func enumOf(m *eidos.EnumMatch, e *eidos.Emitter) error {
	en := m.Enum
	out := &emit.Enum{Origin: en.ID, Doc: en.Doc, Name: en.ID.FlatName(), Visibility: en.Visibility}
	for _, v := range en.Variants {
		out.Variants.Append(&emit.EnumVariant{Origin: v.ID, Doc: v.Doc, Name: v.Name, Value: v.Value})
	}
	e.File().Append(out)
	return nil
}

// serverService writes the Go interface <Service>Server of one service,
// with the method that [eidos.Emitter.Handler] returns for each rpc. Under
// the binding none, a method takes the context of the call and the request
// of the rpc, as the projection states them, and declares the failure of
// the projection, which the Go backend lowers into the error result. A
// streaming side translates as an iterator over pairs of a message and an
// error. serverService withholds the interface where Go cannot serve an
// rpc, after the Emitter reports the refusal.
func serverService(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	service := m.Interface
	out := &emit.Interface{
		Origin: service.ID, Doc: service.Doc, Name: e.JoinName(serverWord, service.Name),
		Visibility: service.Visibility,
	}
	if !handlers(out, service, e) {
		return nil
	}
	e.File().Append(out)
	return nil
}

// handlers appends the method that [eidos.Emitter.Handler] returns for each
// rpc of service to out, in the order of the rpcs. It reports false where
// the target cannot serve an rpc, after the Emitter reports the refusal.
func handlers(out *emit.Interface, service *node.Interface, e *eidos.Emitter) bool {
	for _, rpc := range service.Methods {
		h, served := e.Handler(rpc)
		if !served {
			return false
		}
		out.Methods.Append(h)
	}
	return true
}

// clientService writes the TypeScript interface <Service>Client of one
// service, with a method for each rpc. A method is asynchronous where its
// rpc is, so the caller awaits a response that is not a stream. A method
// leaves out the context of the call, because a caller of a TypeScript
// client passes the signal of a call in the options of the call. A
// streaming side translates as an asynchronous iterable. clientService
// withholds the interface where TypeScript has no spelling of a type, after
// the Emitter reports the refusal.
func clientService(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	service := m.Interface
	out := &emit.Interface{
		Origin: service.ID, Doc: service.Doc, Name: e.JoinName(clientWord, service.Name),
		Visibility: service.Visibility,
	}
	bound := m.Rules()
	for _, method := range service.Methods {
		// An rpc is a method, so the projection reports true.
		c, _ := bound.CallableOf(method)
		lowered := &emit.Method{
			Origin: method.ID, Doc: method.Doc, Name: method.Name, Visibility: method.Visibility, Async: c.Async,
		}
		params := slices.DeleteFunc(slices.Clone(c.Params), func(p rules.ParamView) bool {
			return p.Role == rules.ParamContext
		})
		if !signature(lowered, params, c.Returns, method.ID, e) {
			return nil
		}
		out.Methods.Append(lowered)
	}
	e.File().Append(out)
	return nil
}

// signature appends the translated request and response of one rpc to a
// method. The rpc is the owner of each translation. signature reports
// false where the target has no spelling of a type, after the Emitter
// reports the refusal.
func signature(
	lowered *emit.Method, params []rules.ParamView, returns []rules.ReturnView, owner symbol.Identity, e *eidos.Emitter,
) bool {
	for _, p := range params {
		t, spelled := e.Type(owner, p.Ref)
		if !spelled {
			return false
		}
		lowered.Params.Append(&emit.Param{Name: p.Name, Type: t})
	}
	for _, r := range returns {
		t, spelled := e.Type(owner, r.Ref)
		if !spelled {
			return false
		}
		lowered.Returns.Append(&emit.Return{Type: t})
	}
	return true
}
