// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"slices"

	"go.dokimi.dev/eidos/core/workspace"
	pythonlang "go.dokimi.dev/eidos/lang/python"
	pythonbackend "go.dokimi.dev/eidos/lang/python/backend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The plan python-mirror, its generator and the family word of the
// generator. The mirror of a file <stem>.py or <stem>.pyi is the module
// <stem>_mirror.py beside it.
const (
	mirrorPlan            = "python-mirror"
	mirrorgenID plugin.ID = "mirrorgen"
	mirrorWord            = "mirror"
)

// MirrorPlan returns the plan python-mirror of the shop fixture. It runs
// mirrorgen toward the Python backend of modules, and writes the mirror of
// each Python file of the tree beside the file. A mirror is a module of the
// package of its source, so its imports sort as the imports of the source
// do. A composition renders the Python target through one backend, so the
// mirror of a stub file is a module too. Each call returns a new generator
// and a new backend.
func MirrorPlan() workspace.Plan {
	return workspace.Plan{
		Name:       mirrorPlan,
		Generators: []plugin.Generator{mirrorgen()},
		Backend:    pythonbackend.New(),
	}
}

// mirrorgen returns the mirror generator of the shop fixture. It writes each
// declaration of a Python source as the source states it, into the mirror of
// its file: a class, a Protocol, a TypedDict, an enum, an alias, a function,
// a variable and a constant. Each declaration has the markers that restate
// its stamps, and each file has the header of its source.
//
// The class of a nested declaration writes it, so a handler writes no
// declaration that a class declares. The generator writes no variable that
// declares a type variable, because the backend declares the type variable
// of each type parameter that it writes as one.
func mirrorgen() plugin.Generator {
	return eidos.NewPlugin(mirrorgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: mirrorWord}).
		Handle(
			eidos.OnStruct(mirrorStruct), eidos.OnInterface(mirrorInterface), eidos.OnEnum(mirrorEnum),
			eidos.OnAlias(mirrorAlias), eidos.OnFunction(mirrorFunction), eidos.OnVariable(mirrorVariable),
			eidos.OnConstant(mirrorConstant),
		).
		Generator()
}

// mirrorStruct writes a class with its members and its nested declarations.
func mirrorStruct(m *eidos.StructMatch, e *eidos.Emitter) error {
	if m.Struct.ID.Owner == "" {
		place(m, e, m.Struct, restatedStruct(m, m.Struct))
	}
	return nil
}

// mirrorInterface writes a Protocol or a TypedDict with its members and its
// nested declarations.
func mirrorInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	if m.Interface.ID.Owner == "" {
		place(m, e, m.Interface, restatedInterface(m, m.Interface))
	}
	return nil
}

// mirrorEnum writes an enum with its members, its fields and its methods.
func mirrorEnum(m *eidos.EnumMatch, e *eidos.Emitter) error {
	if m.Enum.ID.Owner == "" {
		place(m, e, m.Enum, restatedEnum(m, m.Enum))
	}
	return nil
}

// mirrorAlias writes a type alias, or a NewType where the alias is defined.
func mirrorAlias(m *eidos.AliasMatch, e *eidos.Emitter) error {
	if m.Alias.ID.Owner == "" {
		place(m, e, m.Alias, restatedAlias(m, m.Alias))
	}
	return nil
}

// mirrorFunction writes a function of a module with its signature and its
// decorators. The backend writes the body ..., because the model has no
// body.
func mirrorFunction(m *eidos.FunctionMatch, e *eidos.Emitter) error {
	f := m.Function
	place(m, e, f, &emit.Function{
		Origin: f.ID, Doc: f.Doc, Comment: f.Comment, Name: f.Name, Visibility: f.Visibility, Async: f.Async,
		Overload:    f.Overload,
		TypeParams:  restatedTypeParams(f.TypeParams, f.ID.Package),
		Params:      emit.SlotOf(restatedParams(m, f.Params, f.ID.Package)...),
		Returns:     emit.SlotOf(restatedReturns(f.Returns, f.ID.Package)...),
		Annotations: f.Annotations,
	})
	return nil
}

// mirrorVariable writes a variable of a module, and nothing for the variable
// of a type variable.
func mirrorVariable(m *eidos.VariableMatch, e *eidos.Emitter) error {
	v := m.Variable
	if _, typeVar := eidos.FactOf(m, v.ID, meta.Named[string](pythonlang.TypeVarKey)); typeVar {
		return nil
	}
	place(m, e, v, &emit.Variable{
		Origin: v.ID, Doc: v.Doc, Comment: v.Comment, Name: v.Name, Visibility: v.Visibility,
		Mutability: v.Mutability, Type: restated(v.Type, v.ID.Package), Value: v.Value,
		Annotations: slices.Concat(v.Annotations, pythonlang.Stated(m, v.ID)),
	})
	return nil
}

// mirrorConstant writes a constant of a module.
func mirrorConstant(m *eidos.ConstantMatch, e *eidos.Emitter) error {
	c := m.Constant
	place(m, e, c, &emit.Constant{
		Origin: c.ID, Doc: c.Doc, Comment: c.Comment, Name: c.Name, Visibility: c.Visibility,
		Type: restated(c.Type, c.ID.Package), Value: c.Value,
		Annotations: slices.Concat(c.Annotations, pythonlang.Stated(m, c.ID)),
	})
	return nil
}

// place appends out to the mirror of the file of the declaration d, after
// the header of the file where the file has one. [pythonlang.Header]
// returns nil for a file without a header, and an accumulator takes no nil
// declaration.
func place(m pythonlang.Match, e *eidos.Emitter, d node.Declaration, out symbol.Symbol) {
	file := e.File()
	if h := pythonlang.Header(m, d); h != nil {
		file.Append(h, out)
		return
	}
	file.Append(out)
}

// restatedStruct returns the mirror of the class s, with its members and
// its nested declarations.
func restatedStruct(m eidos.Matcher, s *node.Struct) *emit.Struct {
	home := s.ID.Package
	out := &emit.Struct{
		Origin: s.ID, Doc: s.Doc, Comment: s.Comment, Name: s.Name, Visibility: s.Visibility, Level: s.Level,
		Abstract: s.Abstract, Final: s.Final, Sealed: s.Sealed, TypeParams: restatedTypeParams(s.TypeParams, home),
		Extends: restatedRefs(s.Extends, home), Implements: restatedRefs(s.Implements, home),
		Annotations: slices.Concat(s.Annotations, pythonlang.Stated(m, s.ID)),
	}
	for _, f := range s.Fields {
		out.Fields.Append(restatedField(m, f))
	}
	for _, method := range s.Methods {
		out.Methods.Append(restatedMethod(m, method))
	}
	for _, d := range s.Types {
		if nested, restates := restatedNested(m, d); restates {
			out.Types.Append(nested)
		}
	}
	return out
}

// restatedInterface returns the mirror of the Protocol or the TypedDict i,
// with its members and its nested declarations.
func restatedInterface(m eidos.Matcher, i *node.Interface) *emit.Interface {
	home := i.ID.Package
	out := &emit.Interface{
		Origin: i.ID, Doc: i.Doc, Comment: i.Comment, Name: i.Name, Visibility: i.Visibility, Sealed: i.Sealed,
		TypeParams: restatedTypeParams(i.TypeParams, home), Extends: restatedRefs(i.Extends, home),
		Annotations: slices.Concat(i.Annotations, pythonlang.Stated(m, i.ID)),
	}
	for _, f := range i.Fields {
		out.Fields.Append(restatedField(m, f))
	}
	for _, method := range i.Methods {
		out.Methods.Append(restatedMethod(m, method))
	}
	for _, d := range i.Types {
		if nested, restates := restatedNested(m, d); restates {
			out.Types.Append(nested)
		}
	}
	return out
}

// restatedEnum returns the mirror of the enum en, with its members, its
// fields and its methods.
func restatedEnum(m eidos.Matcher, en *node.Enum) *emit.Enum {
	out := &emit.Enum{
		Origin: en.ID, Doc: en.Doc, Comment: en.Comment, Name: en.Name, Visibility: en.Visibility, Const: en.Const,
		Underlying:  restated(en.Underlying, en.ID.Package),
		Annotations: slices.Concat(en.Annotations, pythonlang.Stated(m, en.ID)),
	}
	for _, v := range en.Variants {
		out.Variants.Append(&emit.EnumVariant{
			Origin: v.ID, Doc: v.Doc, Comment: v.Comment, Name: v.Name, Value: v.Value, Annotations: v.Annotations,
		})
	}
	for _, f := range en.Fields {
		out.Fields.Append(restatedField(m, f))
	}
	for _, method := range en.Methods {
		out.Methods.Append(restatedMethod(m, method))
	}
	return out
}

// restatedAlias returns the mirror of the alias a.
func restatedAlias(m eidos.Matcher, a *node.Alias) *emit.Alias {
	return &emit.Alias{
		Origin: a.ID, Doc: a.Doc, Comment: a.Comment, Name: a.Name, Visibility: a.Visibility, Defined: a.Defined,
		TypeParams: restatedTypeParams(a.TypeParams, a.ID.Package), Target: restated(a.Target, a.ID.Package),
		Annotations: slices.Concat(a.Annotations, pythonlang.Stated(m, a.ID)),
	}
}

// restatedNested returns the mirror of a declaration that a class declares:
// a class, a Protocol, a TypedDict, an enum or an alias. It reports false
// for a declaration of another kind, which a class body does not declare.
func restatedNested(m eidos.Matcher, d symbol.Symbol) (symbol.Symbol, bool) {
	switch x := d.(type) {
	case *node.Struct:
		return restatedStruct(m, x), true
	case *node.Interface:
		return restatedInterface(m, x), true
	case *node.Enum:
		return restatedEnum(m, x), true
	case *node.Alias:
		return restatedAlias(m, x), true
	default:
		return nil, false
	}
}

// restatedField returns the mirror of the field f, with the markers of its
// stamps.
func restatedField(m eidos.Matcher, f *node.Field) *emit.Field {
	return &emit.Field{
		Origin: f.ID, Doc: f.Doc, Comment: f.Comment, Name: f.Name, Visibility: f.Visibility, Level: f.Level,
		Mutability: f.Mutability, Hard: f.Hard, Optional: f.Optional, Computed: f.Computed,
		Type: restated(f.Type, f.ID.Package), Value: f.Value,
		Annotations: slices.Concat(f.Annotations, pythonlang.Stated(m, f.ID)),
	}
}

// restatedMethod returns the mirror of the method of a class: its
// signature, its facts and its decorators. The backend writes the body ...,
// because the model has no body.
func restatedMethod(m eidos.Matcher, method *node.Method) *emit.Method {
	home := method.ID.Package
	out := &emit.Method{
		Origin: method.ID, Doc: method.Doc, Comment: method.Comment, Name: method.Name,
		Visibility: method.Visibility, Level: method.Level, Abstract: method.Abstract, Final: method.Final,
		Override: method.Override, HasDefault: method.HasDefault, Async: method.Async, Accessor: method.Accessor,
		Indexer: method.Indexer, Constructs: method.Constructs, Hard: method.Hard, Optional: method.Optional,
		Mutability: method.Mutability, Computed: method.Computed, Overload: method.Overload,
		TypeParams:  restatedTypeParams(method.TypeParams, home),
		Params:      emit.SlotOf(restatedParams(m, method.Params, home)...),
		Returns:     emit.SlotOf(restatedReturns(method.Returns, home)...),
		Annotations: method.Annotations,
	}
	if method.Receiver != nil {
		out.Receiver = &emit.Param{Name: method.Receiver.Name}
	}
	return out
}

// restatedParams returns the mirror of each parameter of params, with the
// markers of its stamps, for the mirror of the module home.
func restatedParams(m eidos.Matcher, params []*node.Param, home string) []*emit.Param {
	out := make([]*emit.Param, 0, len(params))
	for _, p := range params {
		out = append(out, &emit.Param{
			Doc: p.Doc, Comment: p.Comment, Name: p.Name, Label: p.Label, Type: restated(p.Type, home),
			Default: p.Default, Optional: p.Optional, Variadic: p.Variadic, Passing: p.Passing,
			Annotations: slices.Concat(p.Annotations, pythonlang.Stated(m, p.ID)),
		})
	}
	return out
}

// restatedReturns returns the mirror of each result of returns, for the
// mirror of the module home.
func restatedReturns(returns []*node.Return, home string) []*emit.Return {
	out := make([]*emit.Return, 0, len(returns))
	for _, r := range returns {
		out = append(out, &emit.Return{Comment: r.Comment, Name: r.Name, Type: restated(r.Type, home)})
	}
	return out
}

// restatedTypeParams returns the mirror of the type parameters tps, whose
// bounds and defaults refer as [restated] refers, for the mirror of the
// module home.
func restatedTypeParams(tps []*node.TypeParam, home string) []*emit.TypeParam {
	out := eidos.MirrorTypeParams(tps)
	for _, tp := range out {
		for _, b := range tp.Bounds {
			localize(b, home)
		}
		localize(tp.Default, home)
	}
	return out
}

// restatedRefs returns the mirror of each reference of refs, as [restated]
// restates one, and nil for no references.
func restatedRefs(refs []*node.TypeRef, home string) []*emit.TypeRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]*emit.TypeRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, restated(r, home))
	}
	return out
}

// restated returns the emit form of the reference t in the mirror of the
// module home: the reference as written, with its target and its package,
// and bare where it refers to a declaration of home, as [localize] makes it.
// A nil reference returns nil.
func restated(t *node.TypeRef, home string) *emit.TypeRef {
	out := rules.EmitRef(t)
	localize(out, home)
	return out
}

// localize clears the target and the package of each reference in t, t
// included, whose target is a declaration of the module home. The mirror of
// home declares that declaration itself. The settle resolves the bare
// reference to the declaration of the mirror, and the backend writes it
// without an import. A reference to a type parameter keeps its target,
// because the backend reads it to name the type variable of the parameter.
func localize(t *emit.TypeRef, home string) {
	if t == nil {
		return
	}
	if t.Target.Lang == pythonlang.Lang && t.Target.Package == home && t.Target.Kind != symbol.KindTypeParam {
		t.Target, t.Package = symbol.Identity{}, ""
	}
	for _, c := range t.Elems {
		localize(c, home)
	}
	for _, a := range t.Args {
		localize(a, home)
	}
}
