// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/workspace"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	pythonlang "go.dokimi.dev/eidos/lang/python"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The plan go-model, its generator, the family word of the generator, the
// directory of the file of the plan, and the import path of the root of the
// tree. The Go backend derives the package of the file from that import
// path, because a Python project has no Go module.
const (
	modelPlan            = "go-model"
	modelgenID plugin.ID = "modelgen"
	modelWord            = "model"
	modelDir             = "model"
	modelBase            = "example.com/acme"
)

// The decorator of a dataclass, by its name alone and through its module,
// the base of a NamedTuple, and the Go type of an enum.
const (
	dataclassName      = "dataclass"
	dataclassQualified = "dataclasses.dataclass"
	namedTupleName     = "NamedTuple"
	stringType         = "string"
)

// ModelPlan returns the plan go-model of the shop fixture. It runs modelgen
// toward the Go backend, and writes the one Go file model/model.go. Each
// call returns a new generator and a new backend.
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

// modelgen returns the model generator of the shop fixture. It writes the
// data classes of the shop as Go declarations into the file of the plan:
//
//   - a dataclass and a NamedTuple are a struct with a field for each field
//     that an instance has, which leaves out a field at the type level and
//     a field of InitVar;
//   - a TypedDict is a struct with a field for each key, which is optional
//     where a value may lack the key;
//   - a Protocol is an interface with a method for each method;
//   - an enum is a defined string type with a constant for each member,
//     whose value is the text of the member: its value in an enum of texts,
//     such as a StrEnum, and its name otherwise.
//
// A declaration takes its flat name, as Line.Kind is LineKind, because Go
// declares no type inside another. The generator writes nothing for another
// class, an alias, a function, a variable and a constant.
func modelgen() plugin.Generator {
	return eidos.NewPlugin(modelgenID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: modelWord}).
		Handle(eidos.OnStruct(modelStruct), eidos.OnInterface(modelInterface), eidos.OnEnum(modelEnum)).
		Generator()
}

// modelStruct writes the Go struct of a dataclass or a NamedTuple. It
// withholds the struct where Go has no spelling of a type, after the
// Emitter reports the refusal.
func modelStruct(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	if !record(s) {
		return nil
	}
	fields, spelled := e.Fields(s.Fields, func(f *node.Field) bool { return instanceField(m, f) })
	if !spelled {
		return nil
	}
	e.PlanFile().Append(&emit.Struct{
		Origin: s.ID, Doc: s.Doc, Name: s.ID.FlatName(), Visibility: s.Visibility, Fields: emit.SlotOf(fields...),
	})
	return nil
}

// modelInterface writes the Go struct of a TypedDict, and the Go interface
// of a Protocol with a method for each method. It withholds the declaration
// where Go has no spelling of a type, after the Emitter reports the refusal.
func modelInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	if typed, _ := eidos.FactOf(m, i.ID, meta.Named[bool](pythonlang.TypedDictKey)); typed {
		fields, spelled := e.Fields(i.Fields, nil)
		if spelled {
			e.PlanFile().Append(&emit.Struct{
				Origin: i.ID, Doc: i.Doc, Name: i.ID.FlatName(), Visibility: i.Visibility,
				Fields: emit.SlotOf(fields...),
			})
		}
		return nil
	}
	out := &emit.Interface{Origin: i.ID, Doc: i.Doc, Name: i.ID.FlatName(), Visibility: i.Visibility}
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

// modelEnum writes the Go enum of an enum: a defined string type with a
// constant for each member, whose value is the text of the member.
func modelEnum(m *eidos.EnumMatch, e *eidos.Emitter) error {
	en := m.Enum
	out := &emit.Enum{
		Origin: en.ID, Doc: en.Doc, Name: en.ID.FlatName(), Visibility: en.Visibility,
		Underlying: &emit.TypeRef{Spelling: stringType},
	}
	texts := textsOf(m.Rules(), en)
	for i, v := range en.Variants {
		out.Variants.Append(&emit.EnumVariant{Origin: v.ID, Doc: v.Doc, Name: v.Name, Value: strconv.Quote(texts[i])})
	}
	e.PlanFile().Append(out)
	return nil
}

// record reports whether the class s is a record of data: a dataclass, by
// its decorator, or a NamedTuple, by its base.
func record(s *node.Struct) bool {
	return slices.ContainsFunc(s.Annotations, func(a symbol.Annotation) bool {
		return a.Name == dataclassName || a.Name == dataclassQualified
	}) || slices.ContainsFunc(s.Extends, func(r *node.TypeRef) bool { return r.Spelling == namedTupleName })
}

// instanceField reports whether each instance of the class of the field f
// has the field: a field at the instance level that is no InitVar of a
// dataclass, which the generated __init__ takes and the instance does not
// keep. The read of the stamp records on the invocation of m.
func instanceField(m eidos.Matcher, f *node.Field) bool {
	initVar, _ := eidos.FactOf(m, f.ID, meta.Named[bool](pythonlang.InitVarKey))
	return f.Level != symbol.LevelType && !initVar
}

// textsOf returns the text of each member of the enum en, in order: its
// value in an enum of texts and its name otherwise, as the rules of the
// language of the enum state it. A language without the projection of an
// enum gives each member its name.
func textsOf(bound rules.Bound, en *node.Enum) []string {
	out := make([]string, 0, len(en.Variants))
	enums, projects := bound.Source().(rules.EnumRules)
	if !projects {
		for _, v := range en.Variants {
			out = append(out, v.Name)
		}
		return out
	}
	for _, v := range enums.EnumOf(en, bound.View()).Variants {
		out = append(out, v.Text.Text)
	}
	return out
}

// signature appends the translated parameters and the result of method to
// lowered. method is the owner of each translation. signature reports false
// where the target has no spelling of a type, after the Emitter reports the
// refusal.
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
