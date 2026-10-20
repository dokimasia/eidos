// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/workspace"
	pythonlang "go.dokimi.dev/eidos/lang/python"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The plan ts-client, its generator, the family word of the generator, and
// the directory of the file of the plan.
const (
	clientPlan            = "ts-client"
	clientgenID plugin.ID = "clientgen"
	clientWord            = "client"
	clientDir             = "client"
)

// ClientPlan returns the plan ts-client of the shop fixture. It runs
// clientgen toward the TypeScript backend, and writes the one TypeScript
// file of the plan under client. Each call returns a new generator and a new
// backend.
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

// clientgen returns the client generator of the shop fixture. It writes the
// types that a TypeScript client of the JSON of the shop reads into the file
// of the plan:
//
//   - a dataclass, a NamedTuple and a TypedDict are an interface with a
//     property for each field that an instance has, and the property of a
//     key that a value may lack is optional;
//   - a Protocol is an interface with a method for each method, and a
//     property for each property of the Protocol, which is readonly unless
//     the Protocol states a setter of its name;
//   - an enum is an enum whose value for each member is the text of the
//     member;
//   - an alias is a type alias of its target with the type parameters of the
//     alias, and a NewType is an alias of the type that it wraps.
//
// A property has the Python name of its field, because the name is a key of
// the JSON of a value. Each property is a wire field, so the settle keeps its
// name. A declaration takes its flat name, as Line.Kind is LineKind. The
// generator writes nothing for another class, a function, a variable and a
// constant.
func clientgen() plugin.Generator {
	return eidos.NewPlugin(clientgenID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: clientWord}).
		Handle(
			eidos.OnStruct(clientStruct), eidos.OnInterface(clientInterface), eidos.OnEnum(clientEnum),
			eidos.OnAlias(clientAlias),
		).
		Generator()
}

// clientStruct writes the TypeScript interface of a dataclass or a
// NamedTuple. It withholds the interface where TypeScript has no spelling of
// a type, after the Emitter reports the refusal.
func clientStruct(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	if !record(s) {
		return nil
	}
	fields, spelled := properties(m, m.Rules(), e, s.Fields)
	if !spelled {
		return nil
	}
	e.PlanFile().Append(&emit.Interface{
		Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
	})
	return nil
}

// clientInterface writes the TypeScript interface of a TypedDict, with a
// property for each key, and of a Protocol, with a method for each method and
// a property for each property. It withholds the interface where TypeScript
// has no spelling of a type, after the Emitter reports the refusal.
func clientInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	out := &emit.Interface{Origin: i.ID, Doc: i.Doc, Name: i.ID.FlatName(), Visibility: i.Visibility}
	if typed, _ := eidos.FactOf(m, i.ID, meta.Named[bool](pythonlang.TypedDictKey)); typed {
		fields, spelled := properties(m, m.Rules(), e, i.Fields)
		if !spelled {
			return nil
		}
		out.Fields = emit.SlotOf(fields...)
		e.PlanFile().Append(out)
		return nil
	}
	for _, method := range i.Methods {
		switch method.Accessor {
		case symbol.AccessorGet:
			field, spelled := property(e, i, method)
			if !spelled {
				return nil
			}
			out.Fields.Append(field)
		case symbol.AccessorSet, symbol.AccessorDelete:
			// The property of the getter of the name states the setter and
			// the deleter.
		case symbol.AccessorNone:
			lowered := &emit.Method{
				Origin:     method.ID,
				Doc:        method.Doc,
				Name:       method.Name,
				Visibility: method.Visibility,
			}
			if !signature(e, lowered, method) {
				return nil
			}
			out.Methods.Append(lowered)
		}
	}
	e.PlanFile().Append(out)
	return nil
}

// property returns the TypeScript property of the getter of a property of
// the Protocol i: the type of its result, readonly unless i states a setter
// of its name. It reports false where TypeScript has no spelling of the
// type, after the Emitter reports the refusal.
func property(e *eidos.Emitter, i *node.Interface, getter *node.Method) (*emit.Field, bool) {
	var result *node.TypeRef
	if len(getter.Returns) > 0 {
		result = getter.Returns[0].Type
	}
	t, spelled := e.Type(getter.ID, result)
	if !spelled {
		return nil, false
	}
	mutability := symbol.MutabilityImmutable
	setter := func(m *node.Method) bool { return m.Name == getter.Name && m.Accessor == symbol.AccessorSet }
	if slices.ContainsFunc(i.Methods, setter) {
		mutability = symbol.MutabilityMutable
	}
	return &emit.Field{
		Origin: getter.ID, Doc: getter.Doc, Name: getter.Name, Visibility: getter.Visibility, Mutability: mutability,
		Type: t, Wire: true,
	}, true
}

// clientEnum writes the TypeScript enum of an enum, with the text of each
// member as its value.
func clientEnum(m *eidos.EnumMatch, e *eidos.Emitter) error {
	en := m.Enum
	out := &emit.Enum{Origin: en.ID, Doc: en.Doc, Name: en.ID.FlatName(), Visibility: en.Visibility}
	texts := textsOf(m.Rules(), en)
	for i, v := range en.Variants {
		out.Variants.Append(&emit.EnumVariant{Origin: v.ID, Doc: v.Doc, Name: v.Name, Value: strconv.Quote(texts[i])})
	}
	e.PlanFile().Append(out)
	return nil
}

// clientAlias writes the TypeScript alias of a type alias, with the type
// parameters of the alias that the Emitter translates, and of a NewType. It
// withholds the alias where TypeScript has no spelling of its target or of a
// type of a type parameter, after the Emitter reports the refusal.
func clientAlias(m *eidos.AliasMatch, e *eidos.Emitter) error {
	a := m.Alias
	params, spelled := e.TypeParams(a.ID, a.TypeParams)
	if !spelled {
		return nil
	}
	target, spelled := e.Type(a.ID, a.Target)
	if !spelled {
		return nil
	}
	e.PlanFile().Append(&emit.Alias{
		Origin: a.ID, Doc: a.Doc, Name: a.ID.FlatName(), Visibility: a.Visibility, TypeParams: params, Target: target,
	})
	return nil
}

// properties returns the TypeScript properties of the fields of a class that
// an instance has: a property for each field, which is optional where its
// type is optional, as the rules bound of Python fold the presence of a key
// of a TypedDict into the type of the key. Each property is a wire field,
// which keeps its key. properties reports false where TypeScript has no
// spelling of a type, after the Emitter reports the refusal. The reads of
// the stamps record on the invocation of m.
func properties(m eidos.Matcher, bound rules.Bound, e *eidos.Emitter, fields []*node.Field) ([]*emit.Field, bool) {
	kept := make([]*node.Field, 0, len(fields))
	for _, f := range fields {
		if instanceField(m, f) {
			kept = append(kept, f)
		}
	}
	out, spelled := e.Fields(kept, nil)
	if !spelled {
		return nil, false
	}
	for i, f := range out {
		f.Optional = bound.FieldTypeOf(kept[i]).Form == symbol.FormOptional
		f.Wire = true
	}
	return out, true
}
