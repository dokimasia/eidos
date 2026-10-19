// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The marks of the canonical form of a GraphQL value. A variable opens with
// its sigil, and a list and an object separate their parts with a comma and
// a space. An argument and a field of an object separate their name from
// their value with [graphql.NameSep].
const (
	variableSigil = "$"
	listStart     = "["
	listEnd       = "]"
	objectStart   = "{"
	objectEnd     = "}"
	partSep       = ", "
)

// valueText returns the canonical form of a GraphQL value: a string with
// the escapes that graphql-js writes, a list and an object with a comma and
// a space between their parts, an object field with a colon and a space
// after its name, and every other value as written. A block string is
// written as a string of the same value. A nil value is the empty text.
func valueText(v *ast.Value) string {
	if v == nil {
		return ""
	}
	var b strings.Builder
	writeValue(&b, v)
	return b.String()
}

// writeValue writes the canonical form of v.
func writeValue(b *strings.Builder, v *ast.Value) {
	switch v.Kind {
	case ast.Variable:
		b.WriteString(variableSigil)
		b.WriteString(v.Raw)
	case ast.StringValue, ast.BlockValue:
		quoted := ast.Value{Kind: ast.StringValue, Raw: v.Raw}
		b.WriteString(quoted.String())
	case ast.ListValue:
		b.WriteString(listStart)
		for i, item := range v.Children {
			if i > 0 {
				b.WriteString(partSep)
			}
			writeValue(b, item.Value)
		}
		b.WriteString(listEnd)
	case ast.ObjectValue:
		b.WriteString(objectStart)
		for i, field := range v.Children {
			if i > 0 {
				b.WriteString(partSep)
			}
			b.WriteString(field.Name)
			b.WriteString(graphql.NameSep)
			writeValue(b, field.Value)
		}
		b.WriteString(objectEnd)
	default:
		b.WriteString(v.Raw)
	}
}

// annotations returns the applied directives ds as annotations: each with
// the name of the directive and one argument for each of its arguments, as
// the name of the argument, [graphql.NameSep] and the canonical form of the
// value. A list without directives returns nil.
func annotations(ds ast.DirectiveList) symbol.Annotations {
	if len(ds) == 0 {
		return nil
	}
	out := make(symbol.Annotations, 0, len(ds))
	for _, d := range ds {
		a := symbol.Annotation{Name: d.Name}
		if len(d.Arguments) > 0 {
			a.Args = make([]string, 0, len(d.Arguments))
			for _, arg := range d.Arguments {
				a.Args = append(a.Args, arg.Name+graphql.NameSep+valueText(arg.Value))
			}
		}
		out = append(out, a)
	}
	return out
}
