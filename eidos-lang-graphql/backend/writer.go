// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The marks of the type expressions of GraphQL. A list writes its element in
// brackets, and a type that is not nullable ends in the mark of a non-null
// type.
const (
	listOpen  = "["
	listClose = "]"
	nonNull   = "!"
)

// The syntax of the comments, the descriptions and the directives of
// GraphQL.
const (
	commentMark  = "#"
	commentGap   = " "
	blockQuotes  = `"""`
	escapedQuote = `\"""`
	argsOpen     = "("
	argsClose    = ")"
	argsSep      = ", "
	space        = " "
	newline      = "\n"
)

// The line terminators of GraphQL are a line feed, a carriage return, and a
// carriage return before a line feed, which ends one line.
const (
	carriageReturn = "\r"
	crlf           = carriageReturn + newline
)

// memberIndent is the indentation of a member of a body, as graphql-js and
// prettier write it.
const memberIndent = "  "

// writeType writes the GraphQL type expression of the reference t as the
// type of a field, an argument or a variable. A named reference writes its
// spelling, a nullable form writes its child without the mark !, a list
// writes its child in brackets, and a stream writes its child, which the
// root of subscriptions streams. Every form that is not nullable ends in
// the mark !. writeType writes the expression from the children, because
// the settle respells the spelling of each named reference and not the
// spelling of a form. The lowering leaves no other form.
func writeType(b *bytes.Buffer, t *emit.TypeRef, nullable bool) {
	switch t.Form {
	case symbol.FormOptional:
		writeType(b, t.Elems[0], true)
		return
	case symbol.FormStream:
		writeType(b, t.Elems[0], nullable)
		return
	case symbol.FormList:
		b.WriteString(listOpen)
		writeType(b, t.Elems[0], false)
		b.WriteString(listClose)
	default:
		b.WriteString(t.Spelling)
	}
	if !nullable {
		b.WriteString(nonNull)
	}
}

// writeDescription writes the documentation lines as the description of a
// declaration at the indentation indent: one line as a string, and two or
// more lines as a block string with each line at the indentation, so the
// parser removes the indentation again. A line of a block string escapes
// its triple quotes. No lines write nothing. One line allocates its quoted
// text, which gqlparser escapes as graphql-js does.
func writeDescription(b *bytes.Buffer, lines []string, indent string) {
	switch len(lines) {
	case 0:
		return
	case 1:
		b.WriteString(indent)
		quoted := ast.Value{Kind: ast.StringValue, Raw: lines[0]}
		b.WriteString(quoted.String())
		b.WriteString(newline)
		return
	}
	b.WriteString(indent)
	b.WriteString(blockQuotes)
	b.WriteString(newline)
	for _, line := range lines {
		if line != "" {
			b.WriteString(indent)
			b.WriteString(strings.ReplaceAll(line, blockQuotes, escapedQuote))
		}
		b.WriteString(newline)
	}
	b.WriteString(indent)
	b.WriteString(blockQuotes)
	b.WriteString(newline)
}

// writeComment writes the trailing comment of a declaration after a space,
// and nothing for an empty comment. A comment of GraphQL ends at the end of
// its line, so each line terminator of the comment writes a space.
func writeComment(b *bytes.Buffer, comment string) {
	if comment == "" {
		return
	}
	b.WriteString(commentGap)
	b.WriteString(commentMark)
	b.WriteString(commentGap)
	for {
		i := strings.IndexAny(comment, crlf)
		if i < 0 {
			b.WriteString(comment)
			return
		}
		b.WriteString(comment[:i])
		b.WriteString(commentGap)
		if strings.HasPrefix(comment[i:], crlf) {
			i++
		}
		comment = comment[i+1:]
	}
}

// writeDirectives writes each annotation of as that is not a marker as an
// applied directive after a space: the sigil, the name, and the arguments in
// parentheses where it has arguments. Each argument is the name of an
// argument, [graphql.NameSep] and a value, as the checks of the annotations
// require.
func writeDirectives(b *bytes.Buffer, as symbol.Annotations) {
	for _, a := range as {
		if strings.HasPrefix(a.Name, graphql.MarkerPrefix) {
			continue
		}
		b.WriteString(space)
		b.WriteString(graphql.DirectiveSigil)
		b.WriteString(a.Name)
		if len(a.Args) == 0 {
			continue
		}
		b.WriteString(argsOpen)
		for i, arg := range a.Args {
			if i > 0 {
				b.WriteString(argsSep)
			}
			b.WriteString(arg)
		}
		b.WriteString(argsClose)
	}
}

// directives reports whether the annotations as state an applied directive,
// an annotation that is not a marker. It allocates nothing.
func directives(as symbol.Annotations) bool {
	for _, a := range as {
		if !strings.HasPrefix(a.Name, graphql.MarkerPrefix) {
			return true
		}
	}
	return false
}
