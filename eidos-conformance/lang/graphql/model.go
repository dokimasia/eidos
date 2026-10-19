// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"path"
	"strconv"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/workspace"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	graphqllang "go.dokimi.dev/eidos/lang/graphql"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The plan go-model, its generator, the family word of the generator, the
// directory of the file of the plan, and the import path of the root of the
// tree. The Go backend derives the package of the file from that import
// path, because a tree of schemas has no Go module.
const (
	modelPlan            = "go-model"
	modelgenID plugin.ID = "modelgen"
	modelWord            = "model"
	modelDir             = "model"
	modelBase            = "example.com/acme"
)

// The word of a resolver interface, the parameter that a resolver takes
// before the arguments of its field, the failure that it declares, and the
// Go type of an enum and of a custom scalar outside the table.
const (
	resolverWord   = "resolver"
	contextParam   = "ctx"
	contextType    = "context.Context"
	contextPackage = "context"
	errorType      = "error"
	stringType     = "string"
)

// ModelPlan returns the plan go-model of the catalog fixture. It runs
// modelgen toward the Go backend, and writes the one Go file model/model.go,
// as gqlgen writes the models of a schema into one file. Each call returns a
// new generator and a new backend.
func ModelPlan() workspace.Plan {
	return workspace.Plan{
		Name:       modelPlan,
		Generators: []plugin.Generator{modelgen()},
		Backend:    gobackend.New(),
		Layout: layout.Config{
			ImportBase: modelBase,
			Families:   map[layout.Family]layout.Refinement{{Plugin: modelgenID}: {Dir: modelDir}},
		},
	}
}

// modelgen returns the model generator of the catalog fixture. It writes the
// types of the schema as Go declarations into the file of the plan:
//
//   - an object type and an input object are a struct with a field for each
//     field of the type, where a field with arguments has the type of its
//     result;
//   - a root operation type is the interface <Type>Resolver with a method for
//     each field, which takes a context and the arguments of the field and
//     declares a failure;
//   - an interface is an interface with a method for each field;
//   - an enum is a defined string type with a constant for each value, whose
//     value is the name of the value, as GraphQL serializes an enum value;
//   - a union is an interface without methods, and a custom scalar outside
//     the table of the satellite is a defined string type.
//
// The generator writes nothing for a directive definition, for a custom
// scalar of the table, whose references fold to its shape, and for the
// executable definitions and their selections.
func modelgen() plugin.Generator {
	return eidos.NewPlugin(modelgenID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: modelWord}).
		Handle(
			eidos.OnStruct(modelStruct), eidos.OnInterface(modelInterface), eidos.OnEnum(modelEnum),
			eidos.OnAlias(modelAlias),
		).
		Generator()
}

// modelStruct writes the Go struct of an object type or an input object, and
// the resolver interface of a root operation type. It withholds the
// declaration where Go has no spelling of a type, after the Emitter reports
// the refusal.
func modelStruct(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	if path.Base(s.ID.Package) == graphqllang.DocumentsSegment {
		return nil
	}
	if rootOperation(m, s.ID) {
		if resolver, spelled := resolverOf(e, s); spelled {
			e.PlanFile().Append(resolver)
		}
		return nil
	}
	fields, spelled := e.Fields(s.Fields, nil)
	if !spelled {
		return nil
	}
	out := &emit.Struct{
		Origin: s.ID, Doc: s.Doc, Name: s.Name, Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
	}
	for _, method := range s.Methods {
		for _, r := range method.Returns {
			t, spelled := e.Type(method.ID, r.Type)
			if !spelled {
				return nil
			}
			out.Fields.Append(&emit.Field{
				Origin: method.ID, Doc: method.Doc, Name: method.Name, Visibility: method.Visibility, Type: t,
			})
		}
	}
	e.PlanFile().Append(out)
	return nil
}

// resolverOf returns the Go interface <Type>Resolver of the root operation
// type s, with a method for each field of s. A method takes a context, and
// the arguments of a field with arguments after it. Each method declares a
// failure, which the Go backend lowers into the error result. It reports
// false where Go has no spelling of a type, after the Emitter reports the
// refusal.
func resolverOf(e *eidos.Emitter, s *node.Struct) (*emit.Interface, bool) {
	out := &emit.Interface{Origin: s.ID, Doc: s.Doc, Name: e.JoinName(resolverWord, s.Name), Visibility: s.Visibility}
	for _, f := range s.Fields {
		t, spelled := e.Type(f.ID, f.Type)
		if !spelled {
			return nil, false
		}
		out.Methods.Append(&emit.Method{
			Origin:     f.ID,
			Doc:        f.Doc,
			Name:       f.Name,
			Visibility: f.Visibility,
			Params: emit.SlotOf(
				&emit.Param{Name: contextParam, Type: &emit.TypeRef{Spelling: contextType, Package: contextPackage}},
			),
			Returns: emit.SlotOf(&emit.Return{Type: t}),
			Throws:  []*emit.TypeRef{{Spelling: errorType}},
		})
	}
	for _, method := range s.Methods {
		lowered := &emit.Method{
			Origin:     method.ID,
			Doc:        method.Doc,
			Name:       method.Name,
			Visibility: method.Visibility,
			Params: emit.SlotOf(
				&emit.Param{Name: contextParam, Type: &emit.TypeRef{Spelling: contextType, Package: contextPackage}},
			),
			Throws: []*emit.TypeRef{{Spelling: errorType}},
		}
		if !signature(e, lowered, method) {
			return nil, false
		}
		out.Methods.Append(lowered)
	}
	return out, true
}

// modelInterface writes the Go interface of a GraphQL interface, with the
// interfaces that it implements and a method for each field: a field without
// arguments takes nothing, and a field with arguments takes its arguments. It
// writes nothing for a directive definition, and withholds the interface
// where Go has no spelling of a type, after the Emitter reports the refusal.
func modelInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	if directiveDefinition(m, i.ID) {
		return nil
	}
	out := &emit.Interface{Origin: i.ID, Doc: i.Doc, Name: i.Name, Visibility: i.Visibility}
	for _, ref := range i.Extends {
		t, spelled := e.Type(i.ID, ref)
		if !spelled {
			return nil
		}
		out.Extends = append(out.Extends, t)
	}
	for _, f := range i.Fields {
		t, spelled := e.Type(f.ID, f.Type)
		if !spelled {
			return nil
		}
		out.Methods.Append(&emit.Method{
			Origin:     f.ID,
			Doc:        f.Doc,
			Name:       f.Name,
			Visibility: f.Visibility,
			Returns:    emit.SlotOf(&emit.Return{Type: t}),
		})
	}
	for _, method := range i.Methods {
		lowered := &emit.Method{Origin: method.ID, Doc: method.Doc, Name: method.Name, Visibility: method.Visibility}
		if !signature(e, lowered, method) {
			return nil
		}
		out.Methods.Append(lowered)
	}
	e.PlanFile().Append(out)
	return nil
}

// modelEnum writes the Go enum of a GraphQL enum: a defined string type with
// a constant for each value.
func modelEnum(m *eidos.EnumMatch, e *eidos.Emitter) error {
	out := stringEnum(m.Enum)
	out.Underlying = &emit.TypeRef{Spelling: stringType}
	e.PlanFile().Append(out)
	return nil
}

// modelAlias writes the Go declaration of a union, an interface without
// methods, and of a custom scalar outside the table of the satellite, a
// defined string type. It writes nothing for a custom scalar of the table.
func modelAlias(m *eidos.AliasMatch, e *eidos.Emitter) error {
	a := m.Alias
	switch {
	case a.Target != nil:
		e.PlanFile().Append(&emit.Interface{Origin: a.ID, Doc: a.Doc, Name: a.Name, Visibility: a.Visibility})
	case !tableScalar(a):
		e.PlanFile().Append(&emit.Alias{
			Origin: a.ID, Doc: a.Doc, Name: a.Name, Visibility: a.Visibility, Defined: true,
			Target: &emit.TypeRef{Spelling: stringType},
		})
	default:
		// A reference to a scalar of the table folds to the shape of the
		// table, so no declaration serves it.
	}
	return nil
}

// signature appends the translated arguments and the result of the field
// with arguments method to lowered. method is the owner of each
// translation. signature reports false where the target has no spelling of
// a type, after the Emitter reports the refusal.
func signature(e *eidos.Emitter, lowered *emit.Method, method *node.Method) bool {
	for _, p := range method.Params {
		t, spelled := e.Type(method.ID, p.Type)
		if !spelled {
			return false
		}
		lowered.Params.Append(&emit.Param{Doc: p.Doc, Name: p.Name, Type: t})
	}
	for _, r := range method.Returns {
		t, spelled := e.Type(method.ID, r.Type)
		if !spelled {
			return false
		}
		lowered.Returns.Append(&emit.Return{Type: t})
	}
	return true
}

// stringEnum returns the enum of a GraphQL enum for a target with string
// enums: a variant for each value, whose value is the name of the value as a
// string literal, as GraphQL serializes an enum value.
func stringEnum(en *node.Enum) *emit.Enum {
	out := &emit.Enum{Origin: en.ID, Doc: en.Doc, Name: en.Name, Visibility: en.Visibility}
	for _, v := range en.Variants {
		out.Variants.Append(&emit.EnumVariant{Origin: v.ID, Doc: v.Doc, Name: v.Name, Value: strconv.Quote(v.Name)})
	}
	return out
}
