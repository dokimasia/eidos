// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"path"
	"slices"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/workspace"
	graphqllang "go.dokimi.dev/eidos/lang/graphql"
	graphqlbackend "go.dokimi.dev/eidos/lang/graphql/backend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The plan graphql-mirror, its generator, the family word of the generator,
// and the directory that the plan writes under. The mirror of a file
// <stem>.graphql is the file <stem>_mirror.graphql under the directory, at
// the directory of its source.
const (
	MirrorDir             = "mirror"
	mirrorPlan            = "graphql-mirror"
	mirrorgenID plugin.ID = "mirrorgen"
	mirrorWord            = "mirror"
)

// MirrorPlan returns the plan graphql-mirror of the catalog fixture. It runs
// mirrorgen toward the GraphQL backend, and writes the mirror of each file of
// the tree under the mirror directory. Each call returns a new generator and
// a new backend.
func MirrorPlan() workspace.Plan {
	return workspace.Plan{
		Name:       mirrorPlan,
		Generators: []plugin.Generator{mirrorgen()},
		Backend:    graphqlbackend.New(),
		Layout:     layout.Config{Policy: layout.PolicyCentralised, Dir: MirrorDir},
	}
}

// mirrorgen returns the mirror generator of the catalog fixture. It writes
// each declaration of a GraphQL source in the GraphQL target as the source
// states it, into the mirror of its file: a type, an interface, a directive
// definition, an enum, a scalar, a union and an executable definition. Each
// declaration has the statement of its stamps, and each file has the header
// of its source. The generator writes nothing for a selection, because the
// selection set of an executable definition states it.
//
// The frontend merges an extension into the type that the schema declares.
// The generator writes each field and each method that an extension adds to
// an object type or to an interface in another file as a member that
// receives the type, in the mirror of that file. It writes the interfaces,
// the directives, the input fields, the values and the members that an
// extension adds to any other type in the definition of the type.
func mirrorgen() plugin.Generator {
	return eidos.NewPlugin(mirrorgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: mirrorWord}).
		Handle(
			eidos.OnStruct(mirrorStruct), eidos.OnInterface(mirrorInterface), eidos.OnEnum(mirrorEnum),
			eidos.OnAlias(mirrorAlias), eidos.OnFunction(mirrorDefinition), eidos.OnField(mirrorField),
			eidos.OnMethod(mirrorMethod),
		).
		Generator()
}

// mirrorStruct writes an object type or an input object. An object type has
// the members of its own file, and an input object has every input field.
// mirrorStruct writes nothing for a selection.
func mirrorStruct(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	if path.Base(s.ID.Package) == graphqllang.DocumentsSegment {
		return nil
	}
	_, input := eidos.FactOf(m, s.ID, meta.Named[string](graphqllang.InputKey))
	out := &emit.Struct{
		Origin: s.ID, Doc: s.Doc, Comment: s.Comment, Name: s.Name, Visibility: s.Visibility,
		Annotations: slices.Concat(s.Annotations, graphqllang.Stated(m, s.ID).Annotations),
	}
	for _, ref := range s.Implements {
		out.Implements = append(out.Implements, rules.EmitRef(ref))
	}
	for _, f := range s.Fields {
		if input || f.Pos.File == s.Pos.File {
			out.Fields.Append(mirroredField(f))
		}
	}
	for _, method := range s.Methods {
		if method.Pos.File == s.Pos.File {
			out.Methods.Append(mirroredMethod(method))
		}
	}
	place(m, e, s, out)
	return nil
}

// mirrorInterface writes an interface with the members of its own file, and
// a directive definition with its arguments as the fields of the interface.
func mirrorInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	out := &emit.Interface{
		Origin: i.ID, Doc: i.Doc, Comment: i.Comment, Name: i.Name, Visibility: i.Visibility,
		Annotations: slices.Concat(i.Annotations, graphqllang.Stated(m, i.ID).Annotations),
	}
	for _, ref := range i.Extends {
		out.Extends = append(out.Extends, rules.EmitRef(ref))
	}
	for _, f := range i.Fields {
		if f.Pos.File == i.Pos.File {
			out.Fields.Append(mirroredField(f))
		}
	}
	for _, method := range i.Methods {
		if method.Pos.File == i.Pos.File {
			out.Methods.Append(mirroredMethod(method))
		}
	}
	place(m, e, i, out)
	return nil
}

// mirrorEnum writes an enum with every value, each with its documentation,
// its comment and its directives.
func mirrorEnum(m *eidos.EnumMatch, e *eidos.Emitter) error {
	en := m.Enum
	out := &emit.Enum{
		Origin: en.ID, Doc: en.Doc, Comment: en.Comment, Name: en.Name, Visibility: en.Visibility,
		Annotations: slices.Concat(en.Annotations, graphqllang.Stated(m, en.ID).Annotations),
	}
	for _, v := range en.Variants {
		out.Variants.Append(&emit.EnumVariant{
			Origin: v.ID, Doc: v.Doc, Comment: v.Comment, Name: v.Name, Annotations: v.Annotations,
		})
	}
	place(m, e, en, out)
	return nil
}

// mirrorAlias writes a scalar, an alias without a target, and a union, an
// alias whose target is the union of its members.
func mirrorAlias(m *eidos.AliasMatch, e *eidos.Emitter) error {
	a := m.Alias
	place(m, e, a, &emit.Alias{
		Origin: a.ID, Doc: a.Doc, Comment: a.Comment, Name: a.Name, Visibility: a.Visibility, Defined: a.Defined,
		Target:      rules.EmitRef(a.Target),
		Annotations: slices.Concat(a.Annotations, graphqllang.Stated(m, a.ID).Annotations),
	})
	return nil
}

// mirrorDefinition writes an operation or a fragment with its variables, its
// directives, its markers and its selection set as written. The header is
// the header of the file in the package of the schema.
func mirrorDefinition(m *eidos.FunctionMatch, e *eidos.Emitter) error {
	f := m.Function
	stated := graphqllang.Stated(m, f.ID)
	place(m, e, f, &emit.Function{
		Origin: f.ID, Doc: f.Doc, Comment: f.Comment, Name: f.Name, Visibility: f.Visibility,
		Params: emit.SlotOf(mirroredParams(f.Params)...), Annotations: slices.Concat(f.Annotations, stated.Annotations),
		Body: stated.Body,
	})
	return nil
}

// mirrorField writes a field that an extension in another file adds to an
// object type or to an interface, as a field that receives the type, in the
// mirror of the file of the extension. The mirror of the type writes every
// other field.
func mirrorField(m *eidos.FieldMatch, e *eidos.Emitter) error {
	f := m.Field
	owner, extended := extendedOwner(m, f.ID, f.Pos.File)
	if !extended {
		return nil
	}
	out := mirroredField(f)
	out.Receives = &emit.TypeRef{Spelling: owner.Name, Target: owner}
	place(m, e, f, out)
	return nil
}

// mirrorMethod writes a field with arguments that an extension in another
// file adds to an object type or to an interface, as a method that receives
// the type, as [mirrorField] writes a field.
func mirrorMethod(m *eidos.MethodMatch, e *eidos.Emitter) error {
	method := m.Method
	owner, extended := extendedOwner(m, method.ID, method.Pos.File)
	if !extended {
		return nil
	}
	out := mirroredMethod(method)
	out.Receives = &emit.TypeRef{Spelling: owner.Name, Target: owner}
	place(m, e, method, out)
	return nil
}

// place appends out to the mirror of the file of the declaration d, after the
// header of the file where the file has a schema statement. [graphqllang.Header]
// returns nil for a file without one, and an accumulator takes no nil
// declaration.
func place(m graphqllang.Match, e *eidos.Emitter, d node.Declaration, out symbol.Symbol) {
	file := e.File()
	if h := graphqllang.Header(m, d); h != nil {
		file.Append(h, out)
		return
	}
	file.Append(out)
}

// extendedOwner returns the identity of the object type or the interface of
// the schema that declares the member id. It reports whether an extension
// adds the member, which is so where the type is in a file other than in,
// the file of the member. It reports false for a member of an input object
// and of a directive definition, whose mirror has every member, for a member
// of a selection, and for a member without an owner. Each read records on
// the invocation of m.
func extendedOwner(m graphqllang.Match, id symbol.Identity, in string) (symbol.Identity, bool) {
	if id.Owner == "" || path.Base(id.Package) == graphqllang.DocumentsSegment {
		return symbol.Identity{}, false
	}
	owner := symbol.Identity{Lang: id.Lang, Package: id.Package, Name: id.Owner, Kind: symbol.KindStruct}
	found, held := m.Reader().Lookup(owner)
	if !held {
		owner.Kind = symbol.KindInterface
		found, held = m.Reader().Lookup(owner)
	}
	_, input := eidos.FactOf(m, owner, meta.Named[string](graphqllang.InputKey))
	decl, declares := found.(node.Declaration)
	if !held || !declares || input || directiveDefinition(m, owner) {
		return symbol.Identity{}, false
	}
	return owner, decl.Position().File != in
}

// mirroredField returns the mirror of the field f: its type as written, its
// default, its documentation, its comment and its directives.
func mirroredField(f *node.Field) *emit.Field {
	return &emit.Field{
		Origin: f.ID, Doc: f.Doc, Comment: f.Comment, Name: f.Name, Visibility: f.Visibility,
		Type: rules.EmitRef(f.Type), Value: f.Value, Annotations: f.Annotations,
	}
}

// mirroredMethod returns the mirror of a field with arguments: its arguments,
// its result as written, its documentation, its comment and its directives.
func mirroredMethod(method *node.Method) *emit.Method {
	out := &emit.Method{
		Origin: method.ID, Doc: method.Doc, Comment: method.Comment, Name: method.Name,
		Visibility: method.Visibility, Params: emit.SlotOf(mirroredParams(method.Params)...),
		Annotations: method.Annotations,
	}
	for _, r := range method.Returns {
		out.Returns.Append(&emit.Return{Type: rules.EmitRef(r.Type)})
	}
	return out
}

// mirroredParams returns the mirror of each parameter of params: its type as
// written, its default, its documentation, its comment and its directives.
func mirroredParams(params []*node.Param) []*emit.Param {
	out := make([]*emit.Param, 0, len(params))
	for _, p := range params {
		out = append(out, &emit.Param{
			Doc: p.Doc, Comment: p.Comment, Name: p.Name, Type: rules.EmitRef(p.Type), Default: p.Default,
			Annotations: p.Annotations,
		})
	}
	return out
}
