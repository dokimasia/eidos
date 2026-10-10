// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
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

// Brand is the brand that the service fixture stamps its files under. A
// carrier of the fixture writes it, as in //+acme:typescript.
const Brand output.Brand = "acme"

// The service fixture's plans, its generators, the family word of each
// generator, the name of the client's file, and the import path of the
// tree's root. The Go backend derives the package of each file from that
// import path, because a tree of schemas has no Go module. The plan's
// layout gives the client of svc/store.proto the file name store.ts,
// because the client's file name would otherwise join the stem and the
// family word.
const (
	serverPlan            = "go-server"
	clientPlan            = "ts-client"
	servergenID plugin.ID = "servergen"
	clientgenID plugin.ID = "clientgen"
	serverWord            = "server"
	clientWord            = "client"
	clientFile            = "store.ts"
	serviceBase           = "example.com/acme"
)

// The parameter that a Go server method takes before its request, and
// the failure that it declares, which the Go backend lowers into the error
// result.
const (
	contextParam   = "ctx"
	contextType    = "context.Context"
	contextPackage = "context"
	errorType      = "error"
)

// The schema that the service fixture's edit changes, the field of
// Summary before and after the edit, and the mode of the file that the
// edit writes.
const (
	schemaFile = "svc/store.proto"
	wideSize   = "int64 size = 1;"
	narrowSize = "int32 size = 1;"
	schemaMode = 0o644
)

// ComposeService returns the service fixture's composition over root
// without its plans: the protobuf frontend and rules, the Go and
// TypeScript targets, a disk sink at root and the state directory's
// ledger under root. [ServicePlans] returns the plans that it runs.
func ComposeService(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(protofrontend.New()).
		Rules(protorules.New()).
		Targets(golang.Target, typescript.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// ServicePlans returns the service fixture's plans over one graph. The
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

// EditService is the service fixture's edit: it narrows the field size of
// the message Summary in svc/store.proto under root from int64 to int32.
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

// servergen returns the service fixture's server generator. It writes a
// Go struct for each message, a sum for each oneof and an enum for each
// enum, all under their flat names, and the interface <Service>Server
// for each service.
func servergen() plugin.Generator {
	return generator(eidos.NewPlugin(servergenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: serverWord}).
		Handle(
			eidos.OnStruct(serverMessage), eidos.OnSum(sumOf), eidos.OnEnum(enumOf),
			eidos.OnInterface(serverService),
		).
		Build())
}

// clientgen returns the service fixture's client generator. It writes a
// TypeScript interface for each message, a sum for each oneof and an enum
// for each enum, all under their flat names, and the interface
// <Service>Client for each service.
func clientgen() plugin.Generator {
	return generator(eidos.NewPlugin(clientgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: clientWord}).
		Handle(
			eidos.OnStruct(clientMessage), eidos.OnSum(sumOf), eidos.OnEnum(enumOf),
			eidos.OnInterface(clientService),
		).
		Build())
}

// serverMessage writes the Go struct of one message under its flat name,
// with a field for each field of the message and a field for each oneof.
// The Emitter translates the type of each field with its presence. A
// oneof's field refers to the oneof's sum, whose nil value is the absent
// one. The reference has the oneof's flat name and the oneof as its
// target, so the settle follows it to the sum that has the oneof as its
// origin. It withholds the struct where Go has no spelling of a field's
// type, after the Emitter reports the refusal.
func serverMessage(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	out := &emit.Struct{Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility}
	for _, f := range s.Fields {
		t, spelled := e.Type(f.ID, f.Type)
		if !spelled {
			return nil
		}
		out.Fields.Append(&emit.Field{Origin: f.ID, Doc: f.Doc, Name: f.Name, Visibility: f.Visibility, Type: t})
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
// presence, which the rules of the field's language decide, and a oneof's
// property always has one. A oneof's property refers to the oneof's sum
// as the struct of [serverMessage] does. It withholds the interface where
// TypeScript has no spelling of a field's type, after the Emitter reports
// the refusal.
func clientMessage(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	bound := m.Rules()
	out := &emit.Interface{Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility}
	for _, f := range s.Fields {
		t, spelled := e.Type(f.ID, f.Type)
		if !spelled {
			return nil
		}
		out.Fields.Append(&emit.Field{
			Origin: f.ID, Doc: f.Doc, Name: f.Name, Visibility: f.Visibility, Type: t,
			Optional: bound.FieldTypeOf(f).Form == symbol.FormOptional,
		})
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
// for each member, whose one field has the member's type. It withholds
// the sum where the target has no spelling of a member's type, after the
// Emitter reports the refusal.
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
// with a method for each rpc. A method takes a context before the rpc's
// request and declares a failure, which the Go backend lowers into the
// error result, because a Go server reports a failed call through it. A
// streaming side translates as an iterator over pairs of a message and an
// error. It withholds the interface where Go has no spelling of a type,
// after the Emitter reports the refusal.
func serverService(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	service := m.Interface
	out := &emit.Interface{
		Origin: service.ID, Doc: service.Doc, Name: e.JoinName(serverWord, service.Name),
		Visibility: service.Visibility,
	}
	bound := m.Rules()
	for _, method := range service.Methods {
		// An rpc is a method, so the projection reports true.
		c, _ := bound.CallableOf(method)
		ctx := &emit.Param{Name: contextParam, Type: &emit.TypeRef{Spelling: contextType, Package: contextPackage}}
		lowered := &emit.Method{
			Origin: method.ID, Doc: method.Doc, Name: method.Name, Visibility: method.Visibility,
			Params: []*emit.Param{ctx}, Throws: []*emit.TypeRef{{Spelling: errorType}},
		}
		if !signature(lowered, c.Params, c.Returns, method.ID, e) {
			return nil
		}
		out.Methods.Append(lowered)
	}
	e.File().Append(out)
	return nil
}

// clientService writes the TypeScript interface <Service>Client of one
// service, with a method for each rpc. A method is asynchronous where its
// rpc is, so the caller awaits a response that is not a stream, and a
// streaming side translates as an asynchronous iterable. It withholds
// the interface where TypeScript has no spelling of a type, after the
// Emitter reports the refusal.
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
		if !signature(lowered, c.Params, c.Returns, method.ID, e) {
			return nil
		}
		out.Methods.Append(lowered)
	}
	e.File().Append(out)
	return nil
}

// signature appends the translated request and response of one rpc to a
// method, each translated with the rpc as its owner. It reports false
// where the target has no spelling of a type, after the Emitter reports
// the refusal.
func signature(
	lowered *emit.Method, params []rules.ParamView, returns []rules.ReturnView, owner symbol.Identity, e *eidos.Emitter,
) bool {
	for _, p := range params {
		t, spelled := e.Type(owner, p.Ref)
		if !spelled {
			return false
		}
		lowered.Params = append(lowered.Params, &emit.Param{Name: p.Name, Type: t})
	}
	for _, r := range returns {
		t, spelled := e.Type(owner, r.Ref)
		if !spelled {
			return false
		}
		lowered.Returns = append(lowered.Returns, &emit.Return{Type: t})
	}
	return true
}

// generator returns a built plugin as the generator that its emitter
// rules make it. Every plugin of the package declares emitter rules, so
// a plugin that is not a generator is a defect in this package.
func generator(p plugin.Plugin) plugin.Generator {
	g, held := p.(plugin.Generator)
	if !held {
		panic("protobuf: a plugin of emitter rules lowers to a generator")
	}
	return g
}
