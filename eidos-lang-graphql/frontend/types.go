// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"github.com/dokimasia/gqlparser/v2/ast"
	"github.com/dokimasia/gqlparser/v2/lexer"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The values of the stamps of the type system that mark a declaration. The
// values of the schema stamp are [graphql.SchemaStatement] and
// [graphql.ExtendStatement].
const (
	inputValue      = "input"
	repeatableValue = "repeatable"
	extendValue     = "extend"
)

// memberSep joins the members of a union in the spelling of its target.
const memberSep = " | "

// typeSystem lowers the type definitions and the directive definitions of
// the file into the package of the schema. The first definition of a name
// is the declaration that an extension of the unit merges into. The schema
// definitions and the extensions lower after every file of the unit, through
// [lowering.extensions], [lowering.directiveExtensions] and
// [lowering.schemaStatements].
func (s *source) typeSystem(doc *ast.SchemaDocument) {
	file := s.schemaFile()
	for _, d := range doc.Definitions {
		decl := s.definition(d, s.definitionAt(d.Position))
		file.Decls = append(file.Decls, decl)
		if _, held := s.types[d.Name]; !held {
			s.types[d.Name] = declared{decl: decl, kind: d.Kind}
		}
	}
	for _, d := range doc.Directives {
		i := s.directive(d)
		file.Decls = append(file.Decls, i)
		if _, held := s.directives[d.Name]; !held {
			s.directives[d.Name] = i
		}
	}
}

// definition lowers one type definition into its declaration. span is the
// top-level definition of d in the file:
//
//   - a scalar is a defined alias without a target;
//   - an object type is a struct, with its interfaces in Implements;
//   - an interface is an interface, with its interfaces in Extends;
//   - a union is a defined alias whose target is the union of its members;
//   - an enum is an enum with a variant for each value;
//   - an input object is a struct with the input stamp.
//
// A field without arguments is a field, and a field with arguments is a
// method with a parameter for each argument and one result. The
// description of a definition is its documentation, and its directives are
// its annotations.
func (s *source) definition(d *ast.Definition, span document.Definition) symbol.Symbol {
	at := s.at(d.Position)
	doc, notes := s.docOf(d.Description), annotations(d.Directives)
	end := span.Last
	switch d.Kind {
	case ast.Scalar:
		a := &node.Alias{
			Pos: at, Name: d.Name, Visibility: symbol.VisibilityPublic, Defined: true, Doc: doc, Annotations: notes,
		}
		a.Comment = s.notes.attach(a, span.First, s.body(span))
		return a
	case ast.Union:
		a := &node.Alias{
			Pos: at, Name: d.Name, Visibility: symbol.VisibilityPublic, Defined: true, Doc: doc, Annotations: notes,
			Target: s.union(d, at),
		}
		a.Comment = s.notes.attach(a, span.First, s.body(span))
		return a
	case ast.Enum:
		e := &node.Enum{Pos: at, Name: d.Name, Visibility: symbol.VisibilityPublic, Doc: doc, Annotations: notes}
		e.Comment = s.notes.attach(e, span.First, s.body(span))
		e.Variants = s.values(d.EnumValues, end)
		return e
	case ast.Interface:
		i := &node.Interface{
			Pos: at, Name: d.Name, Visibility: symbol.VisibilityPublic, Doc: doc, Annotations: notes,
			Extends: s.interfaces(d, span),
		}
		i.Comment = s.notes.attach(i, span.First, s.body(span))
		i.Fields, i.Methods = s.fields(d.Fields, end, true)
		return i
	case ast.InputObject:
		st := &node.Struct{Pos: at, Name: d.Name, Visibility: symbol.VisibilityPublic, Doc: doc, Annotations: notes}
		st.Comment = s.notes.attach(st, span.First, s.body(span))
		st.Fields = s.inputFields(d.Fields, end)
		s.gb.Stamp(st, meta.RawStamp{Key: graphql.InputKey, Value: inputValue, Pos: at})
		return st
	default:
		st := &node.Struct{
			Pos: at, Name: d.Name, Visibility: symbol.VisibilityPublic, Doc: doc, Annotations: notes,
			Implements: s.interfaces(d, span),
		}
		st.Comment = s.notes.attach(st, span.First, s.body(span))
		st.Fields, st.Methods = s.fields(d.Fields, end, false)
		return st
	}
}

// union returns the target of a union: the union form of a named reference
// to each member, at the position of the member, spelled with the members
// joined by |.
func (s *source) union(d *ast.Definition, at position.Pos) *node.TypeRef {
	elems := make([]*node.TypeRef, len(d.Types))
	for i, name := range d.Types {
		elems[i] = named(name, s.at(d.TypePositions[i]))
	}
	return &node.TypeRef{Spelling: strings.Join(d.Types, memberSep), Pos: at, Form: symbol.FormUnion, Elems: elems}
}

// interfaces returns a named reference to each interface that the
// definition implements, at the position of the name of the interface in
// its implements clause.
func (s *source) interfaces(d *ast.Definition, span document.Definition) []*node.TypeRef {
	if len(d.Interfaces) == 0 {
		return nil
	}
	out := make([]*node.TypeRef, 0, len(d.Interfaces))
	k := span.First
	for k <= span.Last && !s.lexed.Is(k, document.KeywordImplements) {
		k++
	}
	for k++; k <= span.Last && len(out) < len(d.Interfaces); k++ {
		if tok := s.lexed.At(k); tok.Kind == lexer.Name {
			out = append(out, named(tok.Value, s.at(&tok.Pos)))
		}
	}
	return out
}

// fields lowers the fields of an object type or of an interface. A field
// without arguments is a field, and a field with arguments is a method. The
// method of an interface is abstract. end is the index in Sig of the
// closing brace of the body.
func (s *source) fields(list ast.FieldList, end int, abstract bool) ([]*node.Field, []*node.Method) {
	nameAt := func(i int) *ast.Position { return list[i].Position }
	described := func(i int) bool { return list[i].Description != "" }
	var fields []*node.Field
	var methods []*node.Method
	for i, fd := range list {
		at := s.at(fd.Position)
		b := s.memberAt(i, len(list), nameAt, described, end)
		if len(fd.Arguments) == 0 {
			f := s.field(fd, at)
			f.Comment = s.notes.attach(f, b.first, b.last)
			fields = append(fields, f)
			continue
		}
		m := &node.Method{
			Pos: at, Name: fd.Name, Visibility: symbol.VisibilityPublic, Level: symbol.LevelInstance,
			Abstract: abstract, Doc: s.docOf(fd.Description), Annotations: annotations(fd.Directives),
			Returns: []*node.Return{{Pos: s.at(fd.Type.Position), Type: typeRef(fd.Type, s.at, named)}},
		}
		m.Comment = s.notes.attach(m, b.first, b.last)
		m.Params = s.params(fd.Arguments, s.argumentsEnd(fd.Position))
		methods = append(methods, m)
	}
	return fields, methods
}

// inputFields lowers the fields of an input object, each with its default as
// its value. end is the index in Sig of the closing brace of the body.
func (s *source) inputFields(list ast.FieldList, end int) []*node.Field {
	nameAt := func(i int) *ast.Position { return list[i].Position }
	described := func(i int) bool { return list[i].Description != "" }
	out := make([]*node.Field, 0, len(list))
	for i, fd := range list {
		f := s.field(fd, s.at(fd.Position))
		f.Value = valueText(fd.DefaultValue)
		b := s.memberAt(i, len(list), nameAt, described, end)
		f.Comment = s.notes.attach(f, b.first, b.last)
		out = append(out, f)
	}
	return out
}

// field lowers one field definition without arguments into a field.
func (s *source) field(fd *ast.FieldDefinition, at position.Pos) *node.Field {
	return &node.Field{
		Pos: at, Name: fd.Name, Visibility: symbol.VisibilityPublic, Level: symbol.LevelInstance,
		Type: typeRef(fd.Type, s.at, named), Doc: s.docOf(fd.Description), Annotations: annotations(fd.Directives),
	}
}

// params lowers the arguments of a field into parameters, each with its
// default, its documentation and its directives. end is the index in Sig
// of the closing parenthesis of the arguments.
func (s *source) params(list ast.ArgumentDefinitionList, end int) []*node.Param {
	nameAt := func(i int) *ast.Position { return list[i].Position }
	described := func(i int) bool { return list[i].Description != "" }
	out := make([]*node.Param, 0, len(list))
	for i, ad := range list {
		p := &node.Param{
			Pos: s.at(ad.Position), Name: ad.Name, Type: typeRef(ad.Type, s.at, named),
			Default: valueText(ad.DefaultValue), Doc: s.docOf(ad.Description), Annotations: annotations(ad.Directives),
		}
		b := s.memberAt(i, len(list), nameAt, described, end)
		p.Comment = s.notes.attach(p, b.first, b.last)
		out = append(out, p)
	}
	return out
}

// values lowers the values of an enum into its variants. end is the index
// in Sig of the closing brace of the body.
func (s *source) values(list ast.EnumValueList, end int) []*node.EnumVariant {
	nameAt := func(i int) *ast.Position { return list[i].Position }
	described := func(i int) bool { return list[i].Description != "" }
	out := make([]*node.EnumVariant, 0, len(list))
	for i, ev := range list {
		v := &node.EnumVariant{
			Pos: s.at(ev.Position), Name: ev.Name, Doc: s.docOf(ev.Description),
			Annotations: annotations(ev.Directives),
		}
		b := s.memberAt(i, len(list), nameAt, described, end)
		v.Comment = s.notes.attach(v, b.first, b.last)
		out = append(out, v)
	}
	return out
}

// directive lowers a directive definition into an interface with a field for
// each argument, with the default of the argument as the value of its field.
// The directives that apply to the definition are the annotations of the
// interface. The locations of the directive are its directive stamp as
// written, and a repeatable directive has the repeatable stamp.
func (s *source) directive(d *ast.DirectiveDefinition) *node.Interface {
	span := s.definitionAt(d.Position)
	at := s.at(d.Position)
	i := &node.Interface{
		Pos: at, Name: d.Name, Visibility: symbol.VisibilityPublic, Doc: s.docOf(d.Description),
		Annotations: annotations(d.Directives),
	}
	i.Comment = s.notes.attach(i, span.First, span.Last)
	nameAt := func(k int) *ast.Position { return d.Arguments[k].Position }
	described := func(k int) bool { return d.Arguments[k].Description != "" }
	end := s.argumentsEnd(d.Position)
	for k, ad := range d.Arguments {
		f := &node.Field{
			Pos: s.at(ad.Position), Name: ad.Name, Visibility: symbol.VisibilityPublic, Level: symbol.LevelInstance,
			Type: typeRef(ad.Type, s.at, named), Value: valueText(ad.DefaultValue), Doc: s.docOf(ad.Description),
			Annotations: annotations(ad.Directives),
		}
		b := s.memberAt(k, len(d.Arguments), nameAt, described, end)
		f.Comment = s.notes.attach(f, b.first, b.last)
		i.Fields = append(i.Fields, f)
	}
	locations := make([]string, len(d.Locations))
	for k, loc := range d.Locations {
		locations[k] = string(loc)
	}
	s.gb.Stamp(i, meta.RawStamp{Key: graphql.DirectiveKey, Value: locations, Pos: at})
	if d.IsRepeatable {
		s.gb.Stamp(i, meta.RawStamp{Key: graphql.RepeatableKey, Value: repeatableValue, Pos: at})
	}
	return i
}
