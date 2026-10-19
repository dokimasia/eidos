// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"cmp"
	"slices"

	"github.com/vektah/gqlparser/v2/ast"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the root operation types of a schema without a schema
// definition, as the specification of GraphQL defaults them.
const (
	defaultQuery        = "Query"
	defaultMutation     = "Mutation"
	defaultSubscription = "Subscription"
)

// statement is one schema definition or schema extension of a file of the
// unit. extension reports a schema extension.
type statement struct {
	src       *source
	def       *ast.SchemaDefinition
	extension bool
}

// extension is one type extension of a file of the unit.
type extension struct {
	src *source
	def *ast.Definition
}

// nameOf returns the name of a root operation type of a loaded schema, and
// the empty name for a schema without the root.
func nameOf(d *ast.Definition) string {
	if d == nil {
		return ""
	}
	return d.Name
}

// extensions merges each extension of the unit into the declaration of its
// type, where the unit declares the type with the kind of the extension. The
// type then has the members, the interfaces and the directives of the
// extension, and the carriers of the extension attach to it. An extension of
// a type that the unit does not declare, or declares with another kind,
// loads as a declaration of its own in the file of the extension, with the
// extension stamp. A later extension of the same type merges into that
// declaration. The bindings of the unit record the name of a type that the
// unit declares by an extension alone.
func (l *lowering) extensions(exts []extension) {
	for _, x := range exts {
		span := x.src.definitionAt(x.def.Position)
		target, held := l.types[x.def.Name]
		if held && target.kind == x.def.Kind {
			x.src.merge(target.decl, x.def, span)
			continue
		}
		decl := x.src.definition(x.def, span)
		file := x.src.schemaFile()
		file.Decls = append(file.Decls, decl)
		l.gb.Stamp(decl, meta.RawStamp{Key: graphql.ExtensionKey, Value: extendValue, Pos: decl.Position()})
		if !held {
			l.types[x.def.Name] = declared{decl: decl, kind: x.def.Kind}
			l.schemaScope.extended[x.def.Name] = true
		}
	}
}

// schemaStatements lowers the schema definitions and the schema extensions
// of the unit into the files that state them. The description of a schema
// definition is the documentation of its file, and the directives of a
// statement are annotations of its file. The carriers of a statement attach
// to its file. A file with a schema definition has the schema stamp with the
// value [graphql.SchemaStatement], and a file with schema extensions alone
// has it with the value [graphql.ExtendStatement]. The roots stamp of a file
// lists the root operation types of its statements as written, in source
// order.
func (l *lowering) schemaStatements(statements []statement) {
	slices.SortStableFunc(statements, func(a, b statement) int {
		return cmp.Or(cmp.Compare(a.src.path, b.src.path), cmp.Compare(a.def.Position.Start, b.def.Position.Start))
	})
	byFile := map[*source][]statement{}
	var order []*source
	for _, st := range statements {
		if len(byFile[st.src]) == 0 {
			order = append(order, st.src)
		}
		byFile[st.src] = append(byFile[st.src], st)
	}
	for _, src := range order {
		file := src.schemaFile()
		value := graphql.ExtendStatement
		var roots []string
		for _, st := range byFile[src] {
			span := src.definitionAt(st.def.Position)
			file.Doc = append(file.Doc, src.docOf(st.def.Description)...)
			file.Annotations = append(file.Annotations, annotations(st.def.Directives)...)
			src.notes.attach(file, span.First, src.body(span))
			if !st.extension {
				value = graphql.SchemaStatement
			}
			for _, op := range st.def.OperationTypes {
				roots = append(roots, string(op.Operation)+graphql.NameSep+op.Type)
			}
		}
		l.gb.Stamp(file, meta.RawStamp{Key: graphql.SchemaKey, Value: value, Pos: file.Pos})
		if len(roots) > 0 {
			l.gb.Stamp(file, meta.RawStamp{Key: graphql.RootsKey, Value: roots, Pos: file.Pos})
		}
	}
}

// operations stamps the kind of operation on each root operation type that
// the unit declares as an object type. loaded is the schema that gqlparser
// loaded for the unit, and nil where the schema does not load. Without it,
// the roots are the root operation types of the statements of the unit,
// and the default names where the unit has no schema definition.
func (l *lowering) operations(loaded *ast.Schema, statements []statement) {
	roots := map[graphql.Operation]string{}
	switch {
	case loaded != nil:
		roots[graphql.OperationQuery] = nameOf(loaded.Query)
		roots[graphql.OperationMutation] = nameOf(loaded.Mutation)
		roots[graphql.OperationSubscription] = nameOf(loaded.Subscription)
	default:
		defined := slices.ContainsFunc(statements, func(st statement) bool { return !st.extension })
		if !defined {
			roots[graphql.OperationQuery] = defaultQuery
			roots[graphql.OperationMutation] = defaultMutation
			roots[graphql.OperationSubscription] = defaultSubscription
		}
		for _, st := range statements {
			for _, op := range st.def.OperationTypes {
				roots[graphql.Operation(op.Operation)] = op.Type
			}
		}
	}
	for _, op := range [...]graphql.Operation{
		graphql.OperationQuery, graphql.OperationMutation, graphql.OperationSubscription,
	} {
		t, held := l.types[roots[op]]
		if !held || t.kind != ast.Object {
			continue
		}
		l.gb.Stamp(t.decl, meta.RawStamp{Key: graphql.OperationKey, Value: string(op), Pos: t.decl.Position()})
	}
}

// merge adds the extension x to the declaration of its type. span is the
// top-level definition of x in the file. The comment of the extension is
// the comment of the declaration where the declaration has none.
func (s *source) merge(decl symbol.Symbol, x *ast.Definition, span document.Definition) {
	comment := s.notes.attach(decl, span.First, s.body(span))
	notes := annotations(x.Directives)
	switch d := decl.(type) {
	case *node.Struct:
		d.Annotations = append(d.Annotations, notes...)
		d.Comment = cmp.Or(d.Comment, comment)
		if x.Kind == ast.InputObject {
			d.Fields = append(d.Fields, s.inputFields(x.Fields, span.Last)...)
			return
		}
		d.Implements = append(d.Implements, s.interfaces(x, span)...)
		fields, methods := s.fields(x.Fields, span.Last, false)
		d.Fields, d.Methods = append(d.Fields, fields...), append(d.Methods, methods...)
	case *node.Interface:
		d.Annotations = append(d.Annotations, notes...)
		d.Comment = cmp.Or(d.Comment, comment)
		d.Extends = append(d.Extends, s.interfaces(x, span)...)
		fields, methods := s.fields(x.Fields, span.Last, true)
		d.Fields, d.Methods = append(d.Fields, fields...), append(d.Methods, methods...)
	case *node.Enum:
		d.Annotations = append(d.Annotations, notes...)
		d.Comment = cmp.Or(d.Comment, comment)
		d.Variants = append(d.Variants, s.values(x.EnumValues, span.Last)...)
	case *node.Alias:
		d.Annotations = append(d.Annotations, notes...)
		d.Comment = cmp.Or(d.Comment, comment)
		if x.Kind == ast.Union {
			members := s.union(x, s.at(x.Position))
			d.Target.Elems = append(d.Target.Elems, members.Elems...)
			if d.Target.Spelling != "" && members.Spelling != "" {
				d.Target.Spelling += memberSep
			}
			d.Target.Spelling += members.Spelling
		}
	}
}
