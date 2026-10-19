// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/lexer"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The values and the suffixes of the executable definitions. The struct of
// the selection of an operation is named after the operation and its kind,
// and the struct of a fragment after the fragment, as graphql-codegen names
// the result types of both.
const (
	anonymousValue       = "anonymous"
	fragmentSuffix       = "Fragment"
	querySuffix          = "Query"
	mutationSuffix       = "Mutation"
	subscriptionSuffix   = "Subscription"
	subscriptionSpelling = "stream "
)

// suffixOf returns the suffix of the name of the result of an operation of
// the kind k.
func suffixOf(k graphql.Operation) string {
	switch k {
	case graphql.OperationMutation:
		return mutationSuffix
	case graphql.OperationSubscription:
		return subscriptionSuffix
	default:
		return querySuffix
	}
}

// rootOf returns the root operation type of the kind k in the schema, and
// nil for a schema without the root.
func rootOf(schema *ast.Schema, k graphql.Operation) *ast.Definition {
	switch k {
	case graphql.OperationMutation:
		return schema.Mutation
	case graphql.OperationSubscription:
		return schema.Subscription
	default:
		return schema.Query
	}
}

// executable lowers the fragments and then the operations of the file into
// the package of the documents. The fragments lower first, so the comments
// in a fragment attach to the fields of its own struct and not to the
// fields of a struct that spreads it.
func (s *source) executable(doc *ast.QueryDocument, sel *selector) {
	file := s.documentsFile()
	for _, fr := range doc.Fragments {
		file.Decls = append(file.Decls, s.fragment(fr, sel)...)
	}
	for _, op := range doc.Operations {
		file.Decls = append(file.Decls, s.operation(op, sel)...)
	}
}

// operation lowers one operation into a function and the declaration of its
// result. The function is named after the operation, and an anonymous
// operation takes the base name of its file without its extension. It has a
// parameter for each variable and one result: the struct of its selection,
// named after the operation and its kind. The result of a subscription is
// an asynchronous stream of the struct, because a subscription responds
// with a sequence of results. The directives of the operation are the
// annotations of the function. The function has the kind of the operation
// and its selection set as written as stamps, and an anonymous operation
// has the anonymous stamp.
func (s *source) operation(op *ast.OperationDefinition, sel *selector) []symbol.Symbol {
	span := s.definitionAt(op.Position)
	at := s.at(op.Position)
	name := op.Name
	if name == "" {
		name = strings.TrimSuffix(path.Base(s.path), path.Ext(s.path))
	}
	kind := graphql.Operation(op.Operation)
	result := name + suffixOf(kind)
	fn := &node.Function{
		Pos: at, Name: name, Visibility: symbol.VisibilityPublic, Annotations: annotations(op.Directives),
	}
	fn.Comment = s.notes.attach(fn, span.First, s.body(span))
	fn.Params = s.variables(op.VariableDefinitions, s.variablesEnd(span))
	ret := named(result, at)
	if kind == graphql.OperationSubscription {
		ret = plugin.Composite(subscriptionSpelling+result, at, symbol.FormStream, ret)
		ret.Async = true
	}
	fn.Returns = []*node.Return{{Pos: at, Type: ret}}

	var root *ast.Definition
	if sel.schema != nil {
		root = rootOf(sel.schema, kind)
	}
	decl := sel.value(result, at, []subset{{set: op.SelectionSet, direct: true, end: span.Last}}, root, s.declare)
	s.gb.Stamp(fn, meta.RawStamp{Key: graphql.OperationKey, Value: string(kind), Pos: at})
	s.gb.Stamp(fn, meta.RawStamp{Key: graphql.SelectionKey, Value: s.lexed.Text(s.body(span), span.Last), Pos: at})
	if op.Name == "" {
		s.gb.Stamp(fn, meta.RawStamp{Key: graphql.AnonymousKey, Value: anonymousValue, Pos: at})
	}
	return []symbol.Symbol{fn, decl}
}

// fragment lowers one fragment into a function and the declaration of its
// result, the struct or the sum of its selection, named after the fragment.
// The variables of the fragment, which gqlparser parses beyond the
// specification, are the parameters of the function. The function has the
// type condition of the fragment and its selection set as written as
// stamps.
func (s *source) fragment(fr *ast.FragmentDefinition, sel *selector) []symbol.Symbol {
	span := s.definitionAt(fr.Position)
	at := s.at(fr.Position)
	result := fr.Name + fragmentSuffix
	fn := &node.Function{
		Pos: at, Name: fr.Name, Visibility: symbol.VisibilityPublic, Annotations: annotations(fr.Directives),
		Returns: []*node.Return{{Pos: at, Type: named(result, at)}},
	}
	fn.Comment = s.notes.attach(fn, span.First, s.body(span))
	fn.Params = s.variables(fr.VariableDefinition, s.variablesEnd(span))
	var condition *ast.Definition
	if sel.schema != nil {
		condition = sel.schema.Types[fr.TypeCondition]
	}
	decl := sel.value(result, at, []subset{{set: fr.SelectionSet, direct: true, end: span.Last}}, condition, s.declare)
	s.gb.Stamp(fn, meta.RawStamp{Key: graphql.FragmentKey, Value: fr.TypeCondition, Pos: at})
	s.gb.Stamp(fn, meta.RawStamp{Key: graphql.SelectionKey, Value: s.lexed.Text(s.body(span), span.Last), Pos: at})
	return []symbol.Symbol{fn, decl}
}

// variables lowers the variables of an operation or of a fragment into
// parameters, each with its type, its default and its directives. end is
// the index in Sig of the closing parenthesis of the variables.
func (s *source) variables(list ast.VariableDefinitionList, end int) []*node.Param {
	if len(list) == 0 {
		return nil
	}
	nameAt := func(i int) *ast.Position { return list[i].Position }
	described := func(int) bool { return false }
	out := make([]*node.Param, 0, len(list))
	for i, v := range list {
		p := &node.Param{
			Pos: s.at(v.Position), Name: v.Variable, Type: typeRef(v.Type, s.at, named),
			Default: valueText(v.DefaultValue), Annotations: annotations(v.Directives),
		}
		b := s.memberAt(i, len(list), nameAt, described, end)
		p.Comment = s.notes.attach(p, b.first, b.last)
		out = append(out, p)
	}
	return out
}

// variablesEnd returns the index in Sig of the closing parenthesis of the
// variables of an executable definition, which open before its directives
// and its selection set. A definition without variables returns the index
// of its first token.
func (s *source) variablesEnd(span document.Definition) int {
	k := span.First
	for kind := s.lexed.At(k).Kind; kind != lexer.ParenL; kind = s.lexed.At(k).Kind {
		if kind == lexer.BraceL || kind == lexer.At || kind == lexer.EOF {
			return span.First
		}
		k++
	}
	return s.lexed.Pair[k]
}

// declare declares t in the file of the documents, as a sum at file level
// declares the nested types of the fields of its variants.
func (s *source) declare(t symbol.Symbol) {
	file := s.documentsFile()
	file.Decls = append(file.Decls, t)
}
