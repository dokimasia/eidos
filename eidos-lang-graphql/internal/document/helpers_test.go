// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"github.com/dokimasia/gqlparser/v2/ast"
	"github.com/dokimasia/gqlparser/v2/parser"
	"go.dokimi.dev/assert"
)

// fileName is the name of the source of each case.
const fileName = "app/schema.graphql"

// The sources that two or more specs read: a small type, a mixed file of the
// type and an anonymous query, and a root of queries.
const (
	smallType   = "type A {\n  a: Int\n}\n"
	mixedFile   = "type A {\n  a: Int\n}\n{ a }\n"
	querySchema = "type Query {\n  a: Int\n}\n"
)

// parsedSchema returns the document of the type system text, which gqlparser
// parses.
func parsedSchema(tb assert.TB, text string) *ast.SchemaDocument {
	tb.Helper()

	doc, err := parser.ParseSchema(&ast.Source{Name: fileName, Input: text})
	assert.NoError(tb, err, "the schema parses")
	return doc
}
