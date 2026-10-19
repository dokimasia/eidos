// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// A comment of GraphQL is neither documentation nor a directive, so only its
// carriers and the comment on the line of a declaration attach, and a
// carrier that attaches nowhere reports.
func TestComments(t *testing.T) {
	t.Parallel()

	t.Run("attach", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches the carriers of the comments directly above a declaration", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "# A row.\n# +fixture:gen:table name=rows\ntype Row {\n  a: Int\n}\n")
			assert.Empty(t, codesOf(sink), "the carrier has a subject")
			row, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Row").(*node.Struct)
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
			expect.Equal(t, gb.Attachments()[0].Subject, symbol.Symbol(row), "to the declaration below it",
				assert.ByIdentity())
			expect.Equal(t, gb.Attachments()[0].Raw.Pos, position.Pos{File: appFile, Line: 2, Col: 1},
				"the directive is at the line of its carrier")
			expect.Nil(t, row.Doc, "a comment is no documentation")
		})

		t.Run("attaches a negated carrier as a negated instance", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "# -fixture:gen:table\ntype Row {\n  a: Int\n}\n")
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
			assert.True(t, gb.Attachments()[0].Raw.Negated, "the instance is negated")
		})

		t.Run("attaches the carrier after the opening brace of a body to the definition", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type Row { # +fixture:gen:table name=rows\n  a: Int\n}\n")
			assert.Empty(t, codesOf(sink), "the carrier has a subject")
			row, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Row").(*node.Struct)
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
			assert.Equal(t, gb.Attachments()[0].Subject, symbol.Symbol(row), "to the definition of the body",
				assert.ByIdentity())
		})

		t.Run("returns the comment after the last token of a declaration as its comment", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "type Row {\n  a: Int # the count\n  b: Int\n}\n")
			row, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Row").(*node.Struct)
			expect.Equal(t, row.Fields[0].Comment, "the count", "the comment on the line of the field")
			expect.Equal(t, row.Fields[1].Comment, "", "the next field has no comment")
		})

		t.Run("returns no comment text for a trailing comment that is a carrier", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "type Row {\n  a: Int # +fixture:gen:table name=a\n}\n")
			row, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Row").(*node.Struct)
			expect.Equal(t, row.Fields[0].Comment, "", "a carrier is no comment text")
			expect.Length(t, gb.Attachments(), 1, "the carrier attaches to the field")
		})

		t.Run("reports BadCarrier for a carrier outside the kernel grammar", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "# +fixture:gen:table name=\ntype Row {\n  a: Int\n}\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.BadCarrier},
				"the malformed carrier reports under the refusal of the grammar")
			assert.Empty(t, gb.Attachments(), "the declaration takes no directive")
		})
	})

	t.Run("leading", func(t *testing.T) {
		t.Parallel()

		t.Run("reads no comment above a blank line as a leading comment", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "# +fixture:gen:table name=rows\n\ntype Row {\n  a: Int\n}\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnaddressedCarrier},
				"the carrier above the blank line attaches to nothing")
			assert.Empty(t, gb.Attachments(), "the declaration takes no directive")
		})

		t.Run("reads no comment on the line of the token before it as a leading comment", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type Row {\n  a: Int # +fixture:gen:table name=a\n  b: Int\n}\n")
			assert.Empty(t, codesOf(sink), "the carrier has a subject")
			row, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Row").(*node.Struct)
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches once")
			assert.Equal(t, gb.Attachments()[0].Subject, symbol.Symbol(row.Fields[0]),
				"to the field on its line, not to the field below it", assert.ByIdentity())
		})

		t.Run("reads the comment above the description of a declaration", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "# +fixture:gen:table name=rows\n\"A row.\"\ntype Row {\n  a: Int\n}\n")
			assert.Empty(t, codesOf(sink), "the carrier has a subject")
			assert.Length(t, gb.Attachments(), 1, "the carrier above the description attaches")
		})

		t.Run("reads the comment above a description over several lines", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t,
				"# +fixture:gen:table name=rows\n\"\"\"\nA row.\n\"\"\"\ntype Row {\n  a: Int\n}\n")
			assert.Empty(t, codesOf(sink), "the carrier has a subject")
			assert.Length(t, gb.Attachments(), 1, "the carrier above the first line of the description attaches")
		})
	})

	t.Run("trails", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a comment on the line where a block string ends as trailing", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "input Row {\n  a: String = \"\"\"\n  x\n  \"\"\" # the default\n}\n")
			assert.Empty(t, codesOf(sink), "the schema loads clean")
			row, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Row").(*node.Struct)
			assert.Equal(t, row.Fields[0].Comment, "the default", "the comment trails the default over three lines")
		})
	})

	t.Run("split", func(t *testing.T) {
		t.Parallel()

		t.Run("splits a group of comments into carriers at their own lines", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t,
				"# +fixture:gen:table name=one\n# +fixture:gen:table name=two\ntype Row {\n  a: Int\n}\n")
			assert.Length(t, gb.Attachments(), 2, "both carriers attach")
			expect.Equal(t, gb.Attachments()[1].Raw.Pos.Line, 2, "the second carrier is at its own line")
		})
	})

	t.Run("sweep", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnaddressedCarrier for a carrier in a comment of no declaration", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type Row {\n  a: Int\n}\n\n# +fixture:gen:table name=orphan\n")
			found := findingsOf(sink, frontend.UnaddressedCarrier)
			assert.Length(t, found, 1, "the carrier reports")
			expect.Contains(t, found[0].Msg, `"+fixture:gen:table name=orphan"`, "the finding quotes the carrier")
			expect.Equal(t, found[0].Pos, position.Pos{File: appFile, Line: 5, Col: 1}, "the finding is at the carrier")
			expect.Empty(t, gb.Attachments(), "the carrier attaches nothing")
		})

		t.Run("reports nothing for a comment of no declaration without a carrier", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "type Row {\n  a: Int\n}\n\n# a note\n")
			assert.Empty(t, codesOf(sink), "a comment without a carrier is not a finding")
		})
	})
}
