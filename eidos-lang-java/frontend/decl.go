// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// hostKind is what a declaration is in, which decides the visibility
// and the level a member takes where its modifiers state none.
type hostKind uint8

// The places a declaration is in: a file's top level, a class or a
// record body, an interface or an annotation type body, whose members
// are public and whose types are static, and an enum body, whose
// constructors are private.
const (
	hostFile      hostKind = 1
	hostClass     hostKind = 2
	hostInterface hostKind = 3
	hostEnum      hostKind = 4
)

// host is the type declaration members lower into: what kind of body
// it has, the lists the members append to, without a list of types for
// an enum, which has none, and a record's parameter list, whose
// components a compact constructor takes as its parameters.
type host struct {
	kind       hostKind
	fields     *[]*node.Field
	methods    *[]*node.Method
	types      *node.Symbols
	components treesitter.Node
}

// header is what every type declaration states the same way: its name
// node, its documentation and trailing comment, its visibility and its
// modifiers.
type header struct {
	name    treesitter.Node
	doc     []string
	comment string
	vis     symbol.Visibility
	mods    modifiers
}

// implicitClass declares the class a compact source file implicitly
// declares, and returns the host the file's members lower into: a final
// Struct with package access, named after its file without the .java
// extension, as javac names it, because the Java Language Specification
// leaves the name to the host system.
func (l *lowering) implicitClass() *host {
	st := &node.Struct{
		Name: strings.TrimSuffix(path.Base(l.path), java.Extension), Pos: l.file.Pos, Final: true,
		Visibility: symbol.VisibilityPackage,
	}
	l.file.Decls = append(l.file.Decls, st)
	return &host{kind: hostClass, fields: &st.Fields, methods: &st.Methods, types: &st.Types}
}

// typeDeclaration reports whether a node kind declares a type: a class,
// an interface, an enum, a record or an annotation type.
func (l *lowering) typeDeclaration(k treesitter.Kind) bool {
	return k == l.v.classDeclaration || k == l.v.interfaceDeclaration || k == l.v.enumDeclaration ||
		k == l.v.recordDeclaration || k == l.v.annotationTypeDeclaration
}

// typeDecl lowers one type declaration in a host, and returns nil for
// one signature depth leaves out. A type in an interface is public
// where it states no access, and package-visible anywhere else. Its
// carriers attach to the type.
func (l *lowering) typeDecl(n treesitter.Node, in hostKind) symbol.Symbol {
	m := l.modifiersOf(n)
	def := symbol.VisibilityPackage
	if in == hostInterface {
		def = symbol.VisibilityPublic
	}
	h := header{name: n.Child(l.v.fieldName), vis: m.visibility(def), mods: m}
	if !l.kept(h.vis) {
		l.skip(n)
		return nil
	}
	parts, comment := l.declParts(n)
	h.doc, h.comment = parts.Docs, comment
	var decl symbol.Symbol
	switch n.Kind() {
	case l.v.classDeclaration:
		decl = l.class(n, h, in)
	case l.v.interfaceDeclaration, l.v.annotationTypeDeclaration:
		decl = l.iface(n, h)
	case l.v.enumDeclaration:
		decl = l.enum(n, h)
	default:
		decl = l.record(n, h, in)
	}
	l.attach(decl, parts.Carriers, m.sugars)
	return decl
}

// class lowers a class as a Struct: abstract, final and sealed where its
// modifiers state them, its type parameters, its superclass as what it
// extends, its interfaces, the subclasses it permits, and its members.
// A nested class is at the type level where it is static or in an
// interface, and an inner class at the instance level.
func (l *lowering) class(n treesitter.Node, h header, in hostKind) *node.Struct {
	st := &node.Struct{
		Name: h.name.Text(), Pos: h.name.Pos(), Doc: h.doc, Comment: h.comment, Visibility: h.vis,
		Abstract: h.mods.has(modAbstract), Final: h.mods.has(modFinal), Sealed: h.mods.has(modSealed),
		TypeParams: l.typeParams(n), Extends: l.typesOf(n.Child(l.v.fieldSuperclass)),
		Implements: l.typesOf(n.Child(l.v.fieldInterfaces)), Permits: l.typesOf(n.Child(l.v.fieldPermits)),
		Annotations: h.mods.annotations,
	}
	if in == hostInterface || h.mods.has(modStatic) {
		st.Level = symbol.LevelType
	}
	l.members(n.Child(l.v.fieldBody), &host{
		kind: hostClass, fields: &st.Fields, methods: &st.Methods, types: &st.Types,
	})
	return st
}

// iface lowers an interface, or an annotation type, as an Interface:
// sealed where its modifiers state it, its type parameters, the
// interfaces it extends, the subtypes it permits, and its members. An
// annotation type's elements are its methods, and java.annotationType
// stamps it.
func (l *lowering) iface(n treesitter.Node, h header) *node.Interface {
	it := &node.Interface{
		Name: h.name.Text(), Pos: h.name.Pos(), Doc: h.doc, Comment: h.comment, Visibility: h.vis,
		Sealed: h.mods.has(modSealed), TypeParams: l.typeParams(n),
		Extends: l.typesOf(l.firstOf(n, l.v.extendsInterfaces)), Permits: l.typesOf(n.Child(l.v.fieldPermits)),
		Annotations: h.mods.annotations,
	}
	if n.Kind() == l.v.annotationTypeDeclaration {
		l.u.Graph().Stamp(it, meta.RawStamp{Key: java.AnnotationTypeKey, Value: true, Pos: it.Pos})
	}
	l.members(n.Child(l.v.fieldBody), &host{
		kind: hostInterface, fields: &it.Fields, methods: &it.Methods, types: &it.Types,
	})
	return it
}

// enum lowers an enum as an Enum: each constant a variant whose value
// is its constructor arguments verbatim, and the fields, methods and
// constructors its body declares. A type an enum declares has no place
// in the model, which reports it under [UnmodeledItem].
func (l *lowering) enum(n treesitter.Node, h header) *node.Enum {
	e := &node.Enum{
		Name: h.name.Text(), Pos: h.name.Pos(), Doc: h.doc, Comment: h.comment, Visibility: h.vis,
		Annotations: h.mods.annotations,
	}
	for child := range n.Child(l.v.fieldBody).NamedChildren() {
		switch child.Kind() {
		case l.v.enumConstant:
			e.Variants = append(e.Variants, l.variant(child))
		case l.v.enumBodyDeclarations:
			l.members(child, &host{kind: hostEnum, fields: &e.Fields, methods: &e.Methods})
		}
	}
	return e
}

// variant lowers one enum constant: its name, its constructor arguments
// verbatim as its value, its annotations and its comments. A body the
// constant declares is an anonymous class, which the model does not
// contain.
func (l *lowering) variant(n treesitter.Node) *node.EnumVariant {
	parts, comment := l.declParts(n)
	name := n.Child(l.v.fieldName)
	m := l.modifiersOf(n)
	v := &node.EnumVariant{
		Name: name.Text(), Pos: name.Pos(), Doc: parts.Docs, Comment: comment,
		Value: l.inner(n.Child(l.v.fieldArguments)), Annotations: m.annotations,
	}
	l.attach(v, parts.Carriers, m.sugars)
	return v
}

// record lowers a record as a Struct, final and stamped java.record:
// each component a public field that is immutable, its type
// parameters, its interfaces and its members. A nested record is at the
// type level, because a record is static.
func (l *lowering) record(n treesitter.Node, h header, in hostKind) *node.Struct {
	st := &node.Struct{
		Name: h.name.Text(), Pos: h.name.Pos(), Doc: h.doc, Comment: h.comment, Visibility: h.vis,
		Final: true, TypeParams: l.typeParams(n), Implements: l.typesOf(n.Child(l.v.fieldInterfaces)),
		Annotations: h.mods.annotations,
	}
	if in != hostFile {
		st.Level = symbol.LevelType
	}
	components := n.Child(l.v.fieldParameters)
	params, _ := l.params(components)
	for _, p := range params {
		f := &node.Field{
			Name: p.Name, Pos: p.Pos, Visibility: symbol.VisibilityPublic, Mutability: symbol.MutabilityImmutable,
			Type: p.Type, Annotations: p.Annotations,
		}
		if p.Variadic == symbol.VariadicPositional {
			f.Type = listOf(p.Type)
		}
		st.Fields = append(st.Fields, f)
	}
	l.u.Graph().Stamp(st, meta.RawStamp{Key: java.RecordKey, Value: true, Pos: st.Pos})
	l.members(n.Child(l.v.fieldBody), &host{
		kind: hostClass, fields: &st.Fields, methods: &st.Methods, types: &st.Types,
		components: components,
	})
	return st
}

// inner returns an argument list's contents as written, between its
// parentheses, and empty for no argument list.
func (l *lowering) inner(args treesitter.Node) string {
	parts := l.children(args)
	if len(parts) == 0 {
		return ""
	}
	return parts[0].TextThrough(parts[len(parts)-1])
}
