// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The file of a schema at the root of the workspace.
const rootFile = "schema.graphql"

// A unit lowers into the package of its schema and the package of its
// documents, so the declarations of a file land where its schema decides.
func TestLowering(t *testing.T) {
	t.Parallel()

	t.Run("newLowering", func(t *testing.T) {
		t.Parallel()

		t.Run("names the package of a unit after the last segment of its path", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{"api/v1/schema.graphql": {Data: []byte(aSource)}})
			pkg := gbs[0].Packages()[0]
			expect.Equal(t, pkg.ID.Package, "api/v1", "the package of the directory")
			expect.Equal(t, pkg.Name, "v1", "the name is the last segment")
		})

		t.Run("gives the package of the root no name", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{rootFile: {Data: []byte(aSource)}})
			pkg := gbs[0].Packages()[0]
			expect.Equal(t, pkg.ID.Package, "", "the root package has no path")
			expect.Equal(t, pkg.Name, "", "the root package has no name")
		})
	})

	t.Run("schemaFile", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a file without a definition as a file of the package of its schema", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "# nothing\n")
			assert.Empty(t, codesOf(sink), "an empty document loads clean")
			expect.Empty(t, fileIn(t, gb, appPkg, appFile).Decls, "the file declares nothing")
		})

		t.Run("loads no file of the schema for a file of executable definitions alone", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				"app/schema.graphqls": {Data: []byte("type Query {\n  a: Int\n}\n")},
				appFile:               {Data: []byte("query Q {\n  a\n}\n")},
			})
			files := gbs[0].Packages()[0].Files
			schemaFiles := make([]string, 0, len(files))
			for _, f := range files {
				schemaFiles = append(schemaFiles, f.Path)
			}
			expect.Equal(t, schemaFiles, []string{"app/schema.graphqls"}, "the document has no file in the schema")
			expect.Length(t, fileIn(t, gbs[0], appDocs, appFile).Decls, 2, "the document is in the documents")
		})
	})

	t.Run("documentsFile", func(t *testing.T) {
		t.Parallel()

		t.Run("names the package of the documents after its last segment", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "type Query {\n  a: Int\n}\n\nquery Q {\n  a\n}\n")
			pkgs := gb.Packages()
			assert.Length(t, pkgs, 2, "the package of the schema and the package of the documents")
			expect.Equal(t, pkgs[1].ID.Package, appDocs, "the documents follow the package of the schema")
			expect.Equal(t, pkgs[1].Name, "documents", "the name is the last segment")
		})
	})

	t.Run("head", func(t *testing.T) {
		t.Parallel()

		t.Run("positions a file at its first token", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "# a note\n\ntype A {\n  a: Int\n}\n")
			assert.Equal(t, fileIn(t, gb, appPkg, appFile).Pos, position.Pos{File: appFile, Line: 3, Col: 1},
				"the file is at its first token, after its comments")
		})

		t.Run("positions a file at the opening quote of a description", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "  \"\"\"\nThe a.\n\"\"\"\ntype A {\n  a: Int\n}\n")
			assert.Equal(t, fileIn(t, gb, appPkg, appFile).Pos, position.Pos{File: appFile, Line: 1, Col: 3},
				"the file is at the first rune of its first token")
		})

		t.Run("positions a file without a token at its start", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "")
			assert.Equal(t, fileIn(t, gb, appPkg, appFile).Pos, position.Pos{File: appFile, Line: 1, Col: 1},
				"the empty file is at its start")
		})
	})

	t.Run("members", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the comments above the description of a member as leading it", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type A {\n  # +fixture:gen:table name=a\n  \"The a.\"\n  a: Int\n}\n")
			assert.Empty(t, codesOf(sink), "the carrier has a subject")
			a, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "A").(*node.Struct)
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
			assert.Equal(t, gb.Attachments()[0].Subject, symbol.Symbol(a.Fields[0]), "to the member below it",
				assert.ByIdentity())
		})

		t.Run("ends the last member before the closing bracket", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type A {\n  f(\n    a: Int # the a\n    b: Int # the b\n  ): Int\n}\n")
			assert.Empty(t, codesOf(sink), "the schema loads clean")
			a, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "A").(*node.Struct)
			assert.Length(t, a.Methods, 1, "the field with arguments is a method")
			params := a.Methods[0].Params
			assert.Length(t, params, 2, "the method has two parameters")
			expect.Equal(t, params[0].Comment, "the a", "the first argument has its comment")
			expect.Equal(t, params[1].Comment, "the b", "the last argument has its comment")
		})
	})

	t.Run("body", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the comment after the last token of a definition without a body", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "scalar Date # an instant\n")
			date, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Date").(*node.Alias)
			assert.Equal(t, date.Comment, "an instant", "the comment trails the definition")
		})
	})

	t.Run("sortDecls", func(t *testing.T) {
		t.Parallel()

		t.Run("orders the declarations of a file by their positions", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "directive @a on OBJECT\n\ntype A {\n  a: Int\n}\n\ndirective @b on OBJECT\n")
			assert.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)), []string{"a", "A", "b"},
				"the definitions are in the order of the file")
		})
	})
}
