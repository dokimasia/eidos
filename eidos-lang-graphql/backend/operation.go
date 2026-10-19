// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/emit"
)

// The keywords of a fragment.
const (
	fragmentKeyword = "fragment "
	conditionOn     = " on "
)

// Function writes the function f as an executable definition after its
// description, with the verbatim body of f as its selection set:
//
//   - Under the marker graphql:fragment, f is a fragment with the argument
//     of the marker as its type condition and the parameters of f as its
//     variables.
//   - Under the marker graphql:operation, f is an operation of the kind of
//     the argument of the marker, with the parameters of f as its variables.
//     The marker graphql:anonymous leaves the name out, and the variables of
//     an anonymous operation follow its keyword after a space. An anonymous
//     query without a description, variables and directives is its selection
//     set alone, because the shorthand of a query takes none of them.
//
// The annotations of f are the directives of the definition, and the
// trailing comment of f follows the opening brace of the selection set. A
// comment ends its line, so the selection set continues on the next line
// where the brace has text after it on its line. The results of f write
// nothing, because the selection set states what a response contains.
func (v *Vocabulary) Function(f *emit.Function) string {
	b := &v.b
	b.Reset()
	writeDescription(b, f.Doc, "")
	set := markersOf(f.Annotations)
	params := f.Params.Items()
	variables := func(k int) arg {
		p := params[k]
		return arg{
			doc: p.Doc, name: p.Name, t: p.Type, nullable: p.Optional, value: p.Default,
			as: p.Annotations, comment: p.Comment,
		}
	}
	kind := markerArg(f.Annotations, graphql.OperationMarker)
	shorthand := set&anonymousBit != 0 && kind == string(graphql.OperationQuery) && len(params) == 0 &&
		len(f.Doc) == 0 && !directives(f.Annotations)
	switch {
	case set&fragmentBit != 0:
		b.WriteString(fragmentKeyword)
		b.WriteString(f.Name)
		writeArgs(b, len(params), variables, "", variableSigil)
		b.WriteString(conditionOn)
		b.WriteString(markerArg(f.Annotations, graphql.FragmentMarker))
	case !shorthand:
		b.WriteString(kind)
		switch {
		case set&anonymousBit == 0:
			b.WriteString(space)
			b.WriteString(f.Name)
		case len(params) > 0:
			b.WriteString(space)
		default:
			// An anonymous operation without variables writes its
			// directives or its selection set after the keyword.
		}
		writeArgs(b, len(params), variables, "", variableSigil)
	}
	writeDirectives(b, f.Annotations)
	if !shorthand {
		b.WriteString(space)
	}
	body := f.Body.Verbatim
	b.WriteString(body[:1])
	writeComment(b, f.Comment)
	if f.Comment != "" && !strings.HasPrefix(body[1:], newline) {
		b.WriteString(newline)
	}
	b.WriteString(body[1:])
	b.WriteString(newline)
	return b.String()
}
