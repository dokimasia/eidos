// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/lang/naming"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The plan ts-client, its generator, the family word of the generator, the
// directory of the file of the plan, and the word of the interface of the
// variables of an operation.
const (
	clientPlan              = "ts-client"
	clientgenID   plugin.ID = "clientgen"
	clientWord              = "client"
	clientDir               = "client"
	variablesWord           = "variables"
)

// typenameKey is the key of a response that names the type of an object,
// and unionSep joins the members of a TypeScript union.
const (
	typenameKey = "__typename"
	unionSep    = " | "
)

// ClientPlan returns the plan ts-client of the catalog fixture. It runs
// clientgen toward the TypeScript backend, and writes the one TypeScript file
// of the plan under client, as graphql-codegen writes the types of a schema
// and of its operations into one file. Each call returns a new generator and
// a new backend.
func ClientPlan() workspace.Plan {
	return workspace.Plan{
		Name:       clientPlan,
		Generators: []plugin.Generator{clientgen()},
		Backend:    tsbackend.New(),
		Layout: layout.Config{
			Families: map[layout.Family]layout.Refinement{{Plugin: clientgenID}: {Dir: clientDir}},
		},
	}
}

// clientgen returns the client generator of the catalog fixture. It writes
// the types that a TypeScript client of the schema reads into the file of
// the plan:
//
//   - an object type, an interface and an input object are an interface with
//     a property for each field, where a field with arguments has the type of
//     its result, and the property of a nullable field is optional;
//   - an enum is an enum whose value for each value is the name of the value;
//   - a union is an alias of the union of its members, and a custom scalar
//     outside the table of the satellite is an alias of string;
//   - the selection of an executable definition is an interface under its
//     flat name, or a sum where it selects fields of particular types;
//   - the variables of an operation or a fragment are the interface
//     <Definition>Variables with a property for each variable.
//
// A property has the GraphQL name of its field or its variable, because the
// name is a key of the JSON of a response or of the variables. Where the
// camel convention of TypeScript would change the name, as it changes
// __typename, the property is a computed property whose name is the key as
// a string literal. The generator writes nothing for a directive definition
// and for a custom scalar of the table, whose references fold to its shape.
func clientgen() plugin.Generator {
	return eidos.NewPlugin(clientgenID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: clientWord}).
		Handle(
			eidos.OnStruct(clientStruct), eidos.OnInterface(clientInterface), eidos.OnEnum(clientEnum),
			eidos.OnAlias(clientAlias), eidos.OnSum(clientSum), eidos.OnFunction(clientVariables),
		).
		Generator()
}

// clientStruct writes the TypeScript interface of an object type, an input
// object or a selection, under its flat name. It withholds the interface
// where TypeScript has no spelling of a type, after the Emitter reports the
// refusal.
func clientStruct(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	fields, spelled := properties(m.Rules(), e, s.Fields, s.Methods)
	if !spelled {
		return nil
	}
	e.PlanFile().Append(&emit.Interface{
		Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
	})
	return nil
}

// clientInterface writes the TypeScript interface of a GraphQL interface,
// with the interfaces that it extends. It writes nothing for a directive
// definition, and withholds the interface where TypeScript has no spelling of
// a type, after the Emitter reports the refusal.
func clientInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	if directiveDefinition(m, i.ID) {
		return nil
	}
	fields, spelled := properties(m.Rules(), e, i.Fields, i.Methods)
	if !spelled {
		return nil
	}
	out := &emit.Interface{
		Origin: i.ID, Doc: i.Doc, Name: i.ID.FlatName(), Visibility: i.Visibility, Fields: emit.SlotOf(fields...),
	}
	for _, ref := range i.Extends {
		t, spelled := e.Type(i.ID, ref)
		if !spelled {
			return nil
		}
		out.Extends = append(out.Extends, t)
	}
	e.PlanFile().Append(out)
	return nil
}

// clientEnum writes the TypeScript enum of a GraphQL enum, with the name of
// each value as its value.
func clientEnum(m *eidos.EnumMatch, e *eidos.Emitter) error {
	e.PlanFile().Append(stringEnum(m.Enum))
	return nil
}

// clientAlias writes the TypeScript alias of a union, the union of its
// members, and of a custom scalar outside the table of the satellite, an
// alias of string. It writes nothing for a custom scalar of the table, and
// withholds a union where TypeScript has no spelling of a member, after the
// Emitter reports the refusal.
func clientAlias(m *eidos.AliasMatch, e *eidos.Emitter) error {
	a := m.Alias
	target := &emit.TypeRef{Spelling: stringType}
	switch {
	case a.Target != nil:
		t, spelled := e.Type(a.ID, a.Target)
		if !spelled {
			return nil
		}
		target = t
	case tableScalar(a):
		return nil
	default:
		// A custom scalar outside the table is a string in the JSON of a
		// response.
	}
	e.PlanFile().Append(&emit.Alias{Origin: a.ID, Doc: a.Doc, Name: a.Name, Visibility: a.Visibility, Target: target})
	return nil
}

// clientSum writes the TypeScript type of a selection that selects fields of
// particular types: an interface for each possible type with the
// properties of its variant, named after the selection and the type, and
// the alias of the union of the interfaces under the flat name of the
// selection. Every output has the origin of the selection, as the lowering
// of a sum by the TypeScript backend gives it. A response discriminates the
// possible types by the key __typename alone, so the property __typename of
// a variant has the name of its type as its literal type, and no property
// of its own discriminates the union. clientSum withholds the type where
// TypeScript has no spelling of a type, after the Emitter reports the
// refusal.
func clientSum(m *eidos.SumMatch, e *eidos.Emitter) error {
	s := m.Sum
	name := s.ID.FlatName()
	bound := m.Rules()
	decls := make([]symbol.Symbol, 1, 1+len(s.Variants))
	union := &emit.TypeRef{Form: symbol.FormUnion, Elems: make([]*emit.TypeRef, 0, len(s.Variants))}
	spellings := make([]string, 0, len(s.Variants))
	for _, v := range s.Variants {
		fields, spelled := properties(bound, e, v.Fields, nil)
		if !spelled {
			return nil
		}
		for _, f := range fields {
			if f.Computed && f.Name == strconv.Quote(typenameKey) {
				f.Type = &emit.TypeRef{Spelling: strconv.Quote(v.Name)}
			}
		}
		// The union is a composite spelling, which the settle leaves as
		// written, so each interface takes its final Pascal spelling here,
		// and each member of the union names it by that spelling.
		variant := naming.Pascal(name + naming.Pascal(v.Name))
		decls = append(decls, &emit.Interface{
			Origin: s.ID, Doc: v.Doc, Name: variant, Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
		})
		union.Elems = append(union.Elems, &emit.TypeRef{Spelling: variant})
		spellings = append(spellings, variant)
	}
	union.Spelling = strings.Join(spellings, unionSep)
	decls[0] = &emit.Alias{Origin: s.ID, Doc: s.Doc, Name: name, Visibility: s.Visibility, Target: union}
	e.PlanFile().Append(decls...)
	return nil
}

// clientVariables writes the TypeScript interface <Definition>Variables of
// an operation or a fragment with variables, with a property for each
// variable. The property of a nullable variable is optional. It writes
// nothing for a definition without variables, and withholds the interface
// where TypeScript has no spelling of a type, after the Emitter reports the
// refusal.
func clientVariables(m *eidos.FunctionMatch, e *eidos.Emitter) error {
	f := m.Function
	if len(f.Params) == 0 {
		return nil
	}
	bound := m.Rules()
	out := &emit.Interface{Origin: f.ID, Doc: f.Doc, Name: e.JoinName(variablesWord, f.Name), Visibility: f.Visibility}
	for _, p := range f.Params {
		t, spelled := e.Type(p.ID, p.Type)
		if !spelled {
			return nil
		}
		property := &emit.Field{
			Origin: p.ID, Doc: p.Doc, Name: p.Name, Visibility: f.Visibility, Type: t,
			Optional: bound.TypeOf(p.Type).Form == symbol.FormOptional,
		}
		keepKey(property)
		out.Fields.Append(property)
	}
	e.PlanFile().Append(out)
	return nil
}

// properties returns the TypeScript properties of the fields and the methods
// of a GraphQL type: a property for each field, and one for each method with
// the type of its result. A property is optional where its type is
// nullable, which the rules bound of GraphQL decide, and keeps its key, as
// [keepKey] keeps it. properties reports false where TypeScript has no
// spelling of a type, after the Emitter reports the refusal.
func properties(
	bound rules.Bound,
	e *eidos.Emitter,
	fields []*node.Field,
	methods []*node.Method,
) ([]*emit.Field, bool) {
	out, spelled := e.Fields(fields, nil)
	if !spelled {
		return nil, false
	}
	for i, f := range out {
		f.Optional = bound.FieldTypeOf(fields[i]).Form == symbol.FormOptional
	}
	for _, method := range methods {
		for _, r := range method.Returns {
			t, spelled := e.Type(method.ID, r.Type)
			if !spelled {
				return nil, false
			}
			out = append(out, &emit.Field{
				Origin: method.ID, Doc: method.Doc, Name: method.Name, Visibility: method.Visibility, Type: t,
				Optional: bound.TypeOf(r.Type).Form == symbol.FormOptional,
			})
		}
	}
	for _, f := range out {
		keepKey(f)
	}
	return out, true
}

// keepKey keeps the GraphQL name of the property f, which is a key of the
// JSON of a response or of the variables. Where the camel convention of
// TypeScript would change the name, f becomes a computed property whose name
// is the key as a string literal, which the respell of TypeScript keeps and
// the backend writes as written.
func keepKey(f *emit.Field) {
	if naming.Camel(f.Name) != f.Name {
		f.Name, f.Computed = strconv.Quote(f.Name), true
	}
}
