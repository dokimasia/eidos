// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The keywords and the marks of the definitions of the type system that the
// backend writes.
const (
	typeKeyword       = "type "
	inputKeyword      = "input "
	interfaceKeyword  = "interface "
	directiveKeyword  = "directive "
	extendKeyword     = "extend "
	implementsKeyword = " implements "
	implementsSep     = " & "
	repeatableKeyword = " repeatable"
	onKeyword         = " on "
	locationSep       = " | "
	bodyOpen          = " {"
	bodyClose         = "}"
	defaultAssign     = " = "
	variableSigil     = "$"
)

// arg is one argument of a field, of a directive or of an executable
// definition as the backend writes it: its description, its name, its type,
// whether the type is nullable, its default, its directives and its
// trailing comment.
type arg struct {
	doc      []string
	name     string
	t        *emit.TypeRef
	nullable bool
	value    string
	as       symbol.Annotations
	comment  string
}

// Struct writes the struct s as an object type, and as an input object
// under the marker graphql:input, after the description of s. The marker
// graphql:extend writes extend before the keyword. The definition lists the
// interfaces of the struct after implements, its directives, and its fields
// and its methods in a body, where a method is a field with arguments. An
// extension without a member has no body.
func (v *Vocabulary) Struct(s *emit.Struct) string {
	b := &v.b
	b.Reset()
	writeDescription(b, s.Doc, "")
	set := markersOf(s.Annotations)
	if set&extendBit != 0 {
		b.WriteString(extendKeyword)
	}
	if set&inputBit != 0 {
		b.WriteString(inputKeyword)
	} else {
		b.WriteString(typeKeyword)
	}
	b.WriteString(s.Name)
	writeImplements(b, s.Implements)
	writeDirectives(b, s.Annotations)
	writeBody(b, s.Fields.Items(), s.Methods.Items(), s.Comment)
	return b.String()
}

// Interface writes the interface i as an interface, and as the definition of
// a directive under the marker graphql:directive, after the description of
// i. The marker graphql:extend writes extend before the keyword. An
// interface lists the interfaces that it implements, which are the
// interfaces that i extends, its directives, and its fields and its methods
// in a body.
func (v *Vocabulary) Interface(i *emit.Interface) string {
	b := &v.b
	b.Reset()
	writeDescription(b, i.Doc, "")
	set := markersOf(i.Annotations)
	if set&directiveBit != 0 {
		writeDirectiveDefinition(b, i, set)
		return b.String()
	}
	if set&extendBit != 0 {
		b.WriteString(extendKeyword)
	}
	b.WriteString(interfaceKeyword)
	b.WriteString(i.Name)
	writeImplements(b, i.Extends)
	writeDirectives(b, i.Annotations)
	writeBody(b, i.Fields.Items(), i.Methods.Items(), i.Comment)
	return b.String()
}

// writeDirectiveDefinition writes the interface i as the definition of a
// directive: the name, which starts with the sigil of a directive after the
// lowering, the fields of i as the arguments, the keyword repeatable under
// the marker graphql:repeatable, and the locations of the marker
// graphql:directive. set is the set of the markers of i.
func writeDirectiveDefinition(b *bytes.Buffer, i *emit.Interface, set markerSet) {
	b.WriteString(directiveKeyword)
	b.WriteString(i.Name)
	fields := i.Fields.Items()
	writeArgs(b, len(fields), func(k int) arg {
		f := fields[k]
		return arg{
			doc: f.Doc, name: f.Name, t: f.Type, nullable: f.Optional, value: f.Value,
			as: f.Annotations, comment: f.Comment,
		}
	}, "", "")
	if set&repeatableBit != 0 {
		b.WriteString(repeatableKeyword)
	}
	b.WriteString(onKeyword)
	for _, a := range i.Annotations {
		if a.Name != string(graphql.DirectiveMarker) {
			continue
		}
		for k, location := range a.Args {
			if k > 0 {
				b.WriteString(locationSep)
			}
			b.WriteString(location)
		}
	}
	writeComment(b, i.Comment)
	b.WriteString(newline)
}

// writeImplements writes the interfaces ts after implements, joined by &,
// and nothing for no interfaces.
func writeImplements(b *bytes.Buffer, ts []*emit.TypeRef) {
	for k, t := range ts {
		if k == 0 {
			b.WriteString(implementsKeyword)
		} else {
			b.WriteString(implementsSep)
		}
		b.WriteString(t.Spelling)
	}
}

// writeBody writes the fields and then the methods of a type in braces,
// with the trailing comment of the type after the opening brace. A type
// without members writes its comment and no braces, as an extension that
// adds directives alone writes it.
func writeBody(b *bytes.Buffer, fields []*emit.Field, methods []*emit.Method, comment string) {
	if len(fields)+len(methods) == 0 {
		writeComment(b, comment)
		b.WriteString(newline)
		return
	}
	b.WriteString(bodyOpen)
	writeComment(b, comment)
	b.WriteString(newline)
	for _, f := range fields {
		writeField(b, f)
	}
	for _, m := range methods {
		writeMethod(b, m)
	}
	b.WriteString(bodyClose)
	b.WriteString(newline)
}

// writeField writes the field f as a member of a body, at the indentation of
// a member: its description, its name, its type, its default, its
// directives and its trailing comment.
func writeField(b *bytes.Buffer, f *emit.Field) {
	writeDescription(b, f.Doc, memberIndent)
	b.WriteString(memberIndent)
	b.WriteString(f.Name)
	b.WriteString(graphql.NameSep)
	writeType(b, f.Type, f.Optional)
	if f.Value != "" {
		b.WriteString(defaultAssign)
		b.WriteString(f.Value)
	}
	writeDirectives(b, f.Annotations)
	writeComment(b, f.Comment)
	b.WriteString(newline)
}

// writeMethod writes the method m as a field with arguments of a body, at
// the indentation of a member: its description, its name, its parameters as
// its arguments, its one result as its type, its directives and its
// trailing comment.
func writeMethod(b *bytes.Buffer, m *emit.Method) {
	writeDescription(b, m.Doc, memberIndent)
	b.WriteString(memberIndent)
	b.WriteString(m.Name)
	params := m.Params.Items()
	writeArgs(b, len(params), func(k int) arg {
		p := params[k]
		return arg{
			doc: p.Doc, name: p.Name, t: p.Type, nullable: p.Optional, value: p.Default,
			as: p.Annotations, comment: p.Comment,
		}
	}, memberIndent, "")
	b.WriteString(graphql.NameSep)
	writeType(b, m.Returns.Items()[0].Type, m.Optional)
	writeDirectives(b, m.Annotations)
	writeComment(b, m.Comment)
	b.WriteString(newline)
}

// writeArgs writes the n arguments that at returns in parentheses, each
// behind sigil, and nothing for no arguments. The arguments are on one line,
// separated by a comma. Where an argument has a description or a comment,
// each argument is on a line of its own one level deeper than indent, with
// its description before it and its comment after it.
func writeArgs(b *bytes.Buffer, n int, at func(int) arg, indent, sigil string) {
	if n == 0 {
		return
	}
	lines := false
	for k := range n {
		a := at(k)
		lines = lines || len(a.doc) > 0 || a.comment != ""
	}
	var inner string
	if lines {
		inner = indent + memberIndent
	}
	b.WriteString(argsOpen)
	for k := range n {
		a := at(k)
		switch {
		case lines:
			b.WriteString(newline)
			writeDescription(b, a.doc, inner)
			b.WriteString(inner)
		case k > 0:
			b.WriteString(argsSep)
		}
		b.WriteString(sigil)
		b.WriteString(a.name)
		b.WriteString(graphql.NameSep)
		writeType(b, a.t, a.nullable)
		if a.value != "" {
			b.WriteString(defaultAssign)
			b.WriteString(a.value)
		}
		writeDirectives(b, a.as)
		if lines {
			writeComment(b, a.comment)
		}
	}
	if lines {
		b.WriteString(newline)
		b.WriteString(indent)
	}
	b.WriteString(argsClose)
}
