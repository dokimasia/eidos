// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/lexer"
	"github.com/vektah/gqlparser/v2/parser"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/lang/graphql"
	graphqltesting "go.dokimi.dev/eidos/lang/graphql/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The files of the healthy fixture, the files that a case adds, and the
// files that the adapter does not claim or cannot read.
const (
	schemaFile    = "app/schema.graphqls"
	documentsFile = "app/documents.graphql"
	searchFile    = "app/search.gql"
	brokenStem    = "app/broken"
	brokenFile    = brokenStem + graphql.Extension
	otherFile     = "app/other.gql"
	notesFile     = "app/notes.txt"
	escapeFile    = "../escape.graphql"
	danglingFile  = "dangling.graphql"
	missingFile   = "missing.graphql"
)

// The sources of the healthy fixture. The schema declares an interface that
// another interface implements, an object type and a union. The documents
// mix an extension of a type with a fragment and a named query. The search
// is an anonymous query that spreads the fragment of the documents.
const (
	schemaSource = `schema {
  query: Query
}

"A record with an identity."
interface Node {
  id: ID!
}

"A node with a name."
interface Named implements Node {
  id: ID!
  name: String!
}

"A product of the catalog."
type Product implements Named & Node {
  id: ID!
  name: String!
}

"A result of a search."
union Result = Product

type Query {
  product(id: ID!): Product
  search(text: String!): [Result!]!
}
`
	documentsSource = `extend type Product {
  price: Int
}

fragment ProductName on Product {
  name
}

query ProductById($id: ID!) {
  product(id: $id) {
    id
    price
    ...ProductName
  }
}
`
	searchSource = `{
  search(text: "lamp") {
    ... on Product {
      ...ProductName
    }
  }
}
`
)

// The sources that a case adds to break one check or to pass one rule of a
// client: a definition without a name, a string that does not close, a
// field of an undeclared type, a selection of an undeclared field, an
// anonymous operation beside a named one, a second anonymous operation, and
// a fragment that no operation spreads.
const (
	syntaxErrorSource   = "type Row {\n  id: ID!\n}\n\ntype {\n"
	lexErrorSource      = "type Row {\n  id: ID!\n}\n\n\"open\n"
	schemaErrorSource   = "type Query {\n  owner: Missing\n}\n"
	fieldErrorSource    = "query Broken {\n  missing\n}\n"
	twoOperationsSource = "query Named {\n  product(id: \"1\") {\n    id\n  }\n}\n\n{\n  product(id: \"2\") {\n    id\n  }\n}\n"
	anonymousSource     = "{\n  product(id: \"3\") {\n    name\n  }\n}\n"
	unusedSource        = "fragment Unused on Product {\n  id\n}\n"
)

// The names of the types of the healthy schema, and the names that it does
// not declare.
const (
	productName = "Product"
	nodeName    = "Node"
	namedName   = "Named"
	resultName  = "Result"
	gadgetName  = "Gadget"
	pricedName  = "Priced"
)

// The texts of the refusals of the adapter, and the reason that the error
// of a layout contains.
const (
	noGraphQL    = "testing: the laid-out project has no GraphQL file"
	noTests      = "testing: a GraphQL document declares no test, so GraphQL runs none"
	noType       = "testing: the schema does not declare the type "
	leavesReason = "leaves the directory"
	errorPrefix  = "testing: "
)

// TestAdapter checks the adapter through the assertions of the kernel and
// through each of its methods. The parse and the validation run in process,
// so no case needs a toolchain on the machine.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an adapter that parses a healthy project", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertParses(t.Context(), t, graphqltesting.New(), healthy())
		})

		t.Run("returns an adapter that type-checks a healthy project", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertTypeChecks(t.Context(), t, graphqltesting.New(), healthy())
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns graphql", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphqltesting.New().Lang(), graphql.Lang, "the language that the assertions name")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true without a reason", func(t *testing.T) {
			t.Parallel()

			present, why := graphqltesting.New().Available()
			assert.True(t, present, "the parse and the validation run in process")
			assert.Empty(t, why, "a toolchain that is there states no reason")
		})
	})

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every generated file", func(t *testing.T) {
			t.Parallel()

			files.HasContent(t, filepath.Join(laidOut(t, healthy()), filepath.FromSlash(searchFile)), searchSource,
				"the generated file is on disk with its own bytes")
		})

		t.Run("returns an error for a path that leaves the scratch directory", func(t *testing.T) {
			t.Parallel()

			_, err := graphqltesting.New().Layout(only(escapeFile, schemaSource))
			assert.HasError(t, err, "a fixture writes only inside its project")
			assert.Contains(t, err.Error(), leavesReason, "the error contains the reason")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, graphqltesting.New().Parse(t.Context(), laidOut(t, healthy())),
				"the healthy output parses")
		})

		tests := []struct {
			name     string
			giveFile string
		}{
			{name: "returns the syntax error of a schema file", giveFile: brokenStem + graphql.SchemaExtension},
			{name: "returns the syntax error of a GraphQL file", giveFile: brokenStem + graphql.Extension},
			{
				name:     "returns the syntax error of a file with the short extension",
				giveFile: brokenStem + graphql.ShortExtension,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := graphqltesting.New().Parse(t.Context(), laidOut(t, with(tt.giveFile, syntaxErrorSource)))
				assert.HasError(t, err, "a type definition without a name is a syntax error")
				assert.Equal(t, err.Error(), parseRefusal(t, tt.giveFile, syntaxErrorSource),
					"the error is the error of the parser of gqlparser at its file, its line and its column")
			})
		}

		t.Run("returns the error of the lexer at its position", func(t *testing.T) {
			t.Parallel()

			lx := lexer.New(&ast.Source{Name: brokenFile, Input: lexErrorSource})
			var want error
			for want == nil {
				tok, err := lx.ReadToken()
				assert.NotEqual(t, tok.Kind, lexer.EOF, "the lexer of gqlparser refuses the text before its end")
				want = err
			}
			err := graphqltesting.New().Parse(t.Context(), laidOut(t, with(brokenFile, lexErrorSource)))
			assert.HasError(t, err, "a string that does not close is an error of the lexer")
			assert.Equal(t, err.Error(), errorPrefix+want.Error(),
				"the error is the error of the lexer of gqlparser at its file, its line and its column")
		})

		t.Run("returns nil for a project whose one error is a validation error", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().Parse(t.Context(), laidOut(t, with(brokenFile, fieldErrorSource)))
			assert.NoError(t, err, "a selection of an undeclared field is no syntax error")
		})

		t.Run("returns an error for a project without a GraphQL file", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().Parse(t.Context(), laidOut(t, only(notesFile, schemaSource)))
			assert.HasError(t, err, "a parse of nothing proves nothing")
			assert.Equal(t, err.Error(), noGraphQL, "the error states the reason")
		})

		t.Run("returns an error for a directory that does not exist", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().Parse(t.Context(), filepath.Join(t.TempDir(), "missing"))
			assert.ErrorIs(t, err, os.ErrNotExist, "the walk of a missing directory fails")
		})

		t.Run("returns an error for a GraphQL file that does not read", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{danglingFile: files.Link(missingFile)})
			err := graphqltesting.New().Parse(t.Context(), dir)
			assert.ErrorIs(t, err, os.ErrNotExist, "the read of a link without a target fails")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return graphqltesting.New().Parse(ctx, dir)
			}, "an ended context parses no file")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, graphqltesting.New().TypeCheck(t.Context(), laidOut(t, healthy())),
				"the output validates with a mixed document and a spread across files")
		})

		t.Run("returns the syntax error of a project that does not parse", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().TypeCheck(t.Context(), laidOut(t, with(brokenFile, syntaxErrorSource)))
			assert.HasError(t, err, "a project with a syntax error does not validate")
			assert.Equal(t, err.Error(), parseRefusal(t, brokenFile, syntaxErrorSource),
				"the error is the error of the parser of gqlparser")
		})

		t.Run("returns the error of a schema that does not load", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().TypeCheck(t.Context(), laidOut(t, only(brokenFile, schemaErrorSource)))
			want := loadRefusal(t, brokenFile, schemaErrorSource)
			assert.Equal(t, assert.ErrorAs[*gqlerror.Error](t, err, "the error wraps an error of gqlparser"), want,
				"the error wraps the error of the load of gqlparser")
			assert.Equal(t, err.Error(), errorPrefix+want.Error(),
				"the error contains the file, the line and the column of the fault")
		})

		t.Run("returns the validation error of a document at its position", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().TypeCheck(t.Context(), laidOut(t, with(brokenFile, fieldErrorSource)))
			assert.HasError(t, err, "a selection of an undeclared field does not validate")
			assert.Equal(t, err.Error(), validationRefusal(t, fieldErrorSource),
				"the error is the error of the validator of gqlparser at its file, its line and its column")
		})

		t.Run("returns an error for an anonymous operation beside another operation of its file", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().TypeCheck(t.Context(), laidOut(t, with(brokenFile, twoOperationsSource)))
			assert.HasError(t, err, "an anonymous operation is the only operation of its document")
			assert.Equal(t, err.Error(), validationRefusal(t, twoOperationsSource),
				"the error is the error of the rule of gqlparser at the position of the anonymous operation")
		})

		t.Run("returns nil for an anonymous operation in each of two files", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().TypeCheck(t.Context(), laidOut(t, with(otherFile, anonymousSource)))
			assert.NoError(t, err, "the anonymous operations of two documents do not share a name")
		})

		t.Run("returns nil for a fragment that no operation spreads", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().TypeCheck(t.Context(), laidOut(t, with(brokenFile, unusedSource)))
			assert.NoError(t, err, "a client validates a fragment that no operation of the project spreads")
		})

		t.Run("returns an error for a project without a GraphQL file", func(t *testing.T) {
			t.Parallel()

			err := graphqltesting.New().TypeCheck(t.Context(), laidOut(t, only(notesFile, schemaSource)))
			assert.HasError(t, err, "a validation of nothing proves nothing")
			assert.Equal(t, err.Error(), noGraphQL, "the error states the reason")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return graphqltesting.New().TypeCheck(ctx, dir)
			}, "an ended context validates no file")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for every project", func(t *testing.T) {
			t.Parallel()

			report, err := graphqltesting.New().RunTests(t.Context(), laidOut(t, healthy()))
			assert.HasError(t, err, "a GraphQL document declares no test")
			assert.Equal(t, err.Error(), noTests, "the error states the missing capability")
			assert.False(t, report.OK(), "a run of nothing is not a pass")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()

		dir := laidOut(t, healthy())
		tests := []struct {
			name         string
			giveType     string
			giveContract string
			want         bool
		}{
			{
				name:     "reports true for an interface that an object type implements",
				giveType: productName, giveContract: nodeName, want: true,
			},
			{
				name:     "reports true for an interface that an interface implements",
				giveType: namedName, giveContract: nodeName, want: true,
			},
			{
				name:     "reports true for a union that has the object type as a member",
				giveType: productName, giveContract: resultName, want: true,
			},
			{
				name:     "reports false for an interface that the type does not implement",
				giveType: nodeName, giveContract: namedName,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := graphqltesting.New().Satisfies(t.Context(), dir, tt.giveType, tt.giveContract)
				assert.NoError(t, err, "the healthy schema declares both types")
				assert.Equal(t, got, tt.want,
					"the schema lists the contract among the interfaces and the unions of the type")
			})
		}

		t.Run("returns an error for a type that the schema does not declare", func(t *testing.T) {
			t.Parallel()

			got, err := graphqltesting.New().Satisfies(t.Context(), dir, gadgetName, nodeName)
			assert.HasError(t, err, "a question about an undeclared type has no answer")
			assert.Equal(t, err.Error(), noType+gadgetName, "the error names the type")
			assert.False(t, got, "a refusal reports false")
		})

		t.Run("returns an error for a contract that the schema does not declare", func(t *testing.T) {
			t.Parallel()

			got, err := graphqltesting.New().Satisfies(t.Context(), dir, productName, pricedName)
			assert.HasError(t, err, "a question about an undeclared contract has no answer")
			assert.Equal(t, err.Error(), noType+pricedName, "the error names the contract")
			assert.False(t, got, "a refusal reports false")
		})

		t.Run("returns the syntax error of a project that does not parse", func(t *testing.T) {
			t.Parallel()

			_, err := graphqltesting.New().Satisfies(t.Context(), laidOut(t, with(brokenFile, syntaxErrorSource)),
				productName, nodeName)
			assert.HasError(t, err, "a project with a syntax error has no schema")
			assert.Equal(t, err.Error(), parseRefusal(t, brokenFile, syntaxErrorSource),
				"the error is the error of the parser of gqlparser")
		})

		t.Run("returns the error of a schema that does not load", func(t *testing.T) {
			t.Parallel()

			_, err := graphqltesting.New().Satisfies(t.Context(), laidOut(t, only(brokenFile, schemaErrorSource)),
				productName, nodeName)
			assert.HasError(t, err, "a schema that does not load has no types")
			assert.Equal(t, err.Error(), errorPrefix+loadRefusal(t, brokenFile, schemaErrorSource).Error(),
				"the error is the error of the load of gqlparser")
		})

		t.Run("returns an error for a project without a GraphQL file", func(t *testing.T) {
			t.Parallel()

			_, err := graphqltesting.New().Satisfies(t.Context(), laidOut(t, only(notesFile, schemaSource)),
				productName, nodeName)
			assert.HasError(t, err, "a project without a schema has no types")
			assert.Equal(t, err.Error(), noGraphQL, "the error states the reason")
		})
	})
}

// healthy returns generated output that parses and validates: a schema, a
// document that mixes an extension with executable definitions, and an
// anonymous query that spreads a fragment of the other document. Each file
// has another of the three extensions.
func healthy() toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{
		schemaFile:    []byte(schemaSource),
		documentsFile: []byte(documentsSource),
		searchFile:    []byte(searchSource),
	}}
}

// with returns the healthy fixture with one file added, for a case that
// breaks one check or passes one rule.
func with(path, body string) toolchain.Generated {
	g := healthy()
	g.Files[path] = []byte(body)
	return g
}

// only returns a fixture with one file.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{path: []byte(body)}}
}

// laidOut lays a fixture out and removes it when the case ends.
func laidOut(t *testing.T, g toolchain.Generated) string {
	t.Helper()

	dir, err := graphqltesting.New().Layout(g)
	assert.NoError(t, err, "the fixture lays out")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// parseRefusal returns the error that the adapter reports for the file name
// with the text, whose one error is the first error of the parser of
// gqlparser.
func parseRefusal(t *testing.T, name, text string) string {
	t.Helper()

	_, err := parser.ParseSchema(&ast.Source{Name: name, Input: text})
	assert.HasError(t, err, "gqlparser refuses the text")
	return errorPrefix + err.Error()
}

// loadRefusal returns the error of the load of gqlparser for a schema of the
// one file name with the text.
func loadRefusal(t *testing.T, name, text string) *gqlerror.Error {
	t.Helper()

	_, err := gqlparser.LoadSchema(&ast.Source{Name: name, Input: text})
	return assert.ErrorAs[*gqlerror.Error](t, err, "gqlparser refuses the schema with an error of its own type")
}

// validationRefusal returns the error that the adapter reports for the
// document text in brokenFile beside the healthy fixture. It is the one
// error of the default rules of gqlparser for the text against the healthy
// schema, at its line and its column in brokenFile.
func validationRefusal(t *testing.T, text string) string {
	t.Helper()

	schema, err := gqlparser.LoadSchema(&ast.Source{Name: schemaFile, Input: schemaSource})
	assert.NoError(t, err, "the healthy schema loads")
	_, errs := gqlparser.LoadQueryWithRules(schema, text, nil)
	assert.Length(t, errs, 1, "gqlparser refuses the document once")
	assert.Length(t, errs[0].Locations, 1, "the error of gqlparser has one position")
	return fmt.Sprintf("%s%s:%d:%d: %s", errorPrefix, brokenFile, errs[0].Locations[0].Line,
		errs[0].Locations[0].Column, errs[0].Message)
}
