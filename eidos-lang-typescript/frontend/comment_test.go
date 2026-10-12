// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The carrier of the comment cases, and the name of its directive.
const (
	carrierLine   = "// +fixture:gen:table name=t\n"
	directiveName = "gen:table"
)

// Comments contain the documentation and the directives. The cases pin
// which comments a declaration takes.
func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("commentWalk", func(t *testing.T) {
		t.Parallel()

		t.Run("documents a declaration with the block comment directly above it", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "/** A is one record. */\nexport class A {}\n"), "A")
			assert.Equal(t, st.Doc, []string{"A is one record."}, "the text of the block")
		})

		t.Run("documents a declaration with a run of line comments", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "// A is one\n// record.\nexport class A {}\n"), "A")
			assert.Equal(t, st.Doc, []string{"A is one", "record."}, "each line of the run")
		})

		t.Run("ends the documentation at a blank line", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "// File header.\n\nexport class A {}\n"), "A")
			assert.Empty(t, st.Doc, "a comment above a blank line documents nothing below it")
		})

		t.Run("leaves the trailing comment of the member before to that member", func(t *testing.T) {
			t.Parallel()

			i := ifaceOf(t, "  a: string; // the a\n  b: number;\n")
			assert.Empty(t, i.Fields[1].Doc, "b takes no comment from the line of a")
		})

		t.Run("skips the decorators between a member and its comment", func(t *testing.T) {
			t.Parallel()

			c := classOf(t, "  /** Runs. */\n  @log\n  run(): void {}\n")
			assert.Equal(t, c.Methods[0].Doc, []string{"Runs."}, "the comment above the decorator")
		})

		t.Run("records the comment on the last line of a declaration as its trailing comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ifaceOf(t, "  a: string; // the a\n").Fields[0].Comment, "the a",
				"the comment after the member")
		})

		t.Run("records a trailing comment that the grammar folds into a declaration without a semicolon",
			func(t *testing.T) {
				t.Parallel()

				c := named[*node.Constant](t, declsOf(t, "export const x = 1 // the x\n"), "x")
				assert.Equal(t, c.Comment, "the x", "the comment is the last child of the statement")
			})

		t.Run("passes over a carrier inside the body of a function", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export function f(): void {\n  "+carrierLine+"  return;\n}\n")
			assert.Empty(t, found, "the comments of a body belong to its statements")
		})
	})

	t.Run("declParts", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches a carrier from the comment above a declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, carrierLine+"export class A {}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			assert.Equal(t, string(gb.Attachments()[0].Raw.Name), directiveName, "the name of the directive")
		})

		t.Run("attaches a carrier from a trailing comment", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export class A {} "+carrierLine)
			assert.Length(t, gb.Attachments(), 1, "the directive of the trailing comment")
		})

		t.Run("attaches a carrier above a namespace to its package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export {};\n"+carrierLine+"namespace N {}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onPackage := gb.Attachments()[0].Subject.(*node.Package)
			assert.True(t, onPackage, "a namespace is a package")
		})
	})

	t.Run("parts", func(t *testing.T) {
		t.Parallel()

		t.Run("folds the continuation of a carrier across consecutive line comments", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "// +fixture:gen:table \\\n// name=t\nexport class A {}\n")
			assert.Empty(t, found, "the folded carrier parses")
			assert.Length(t, gb.Attachments(), 1, "one directive from two lines")
			assert.Length(t, gb.Attachments()[0].Raw.Args, 1, "the argument on the second line")
		})

		t.Run("reads a line comment after a gap apart from the run before it", func(t *testing.T) {
			t.Parallel()

			parts := named[*node.Struct](t, declsOf(t, "/** A. */\n// More.\nexport class A {}\n"), "A").Doc
			assert.Equal(t, parts, []string{"A.", "More."}, "a block comment and a line comment each read alone")
		})
	})

	t.Run("skip", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the carriers of a declaration that the signature depth leaves out", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export {};\n" + carrierLine + "class Hidden {}\n",
			)}}, aFile, plugin.DepthSignatures)
			assert.Empty(t, found, "the carrier leaves with its declaration")
			assert.Empty(t, gb.Attachments(), "the carrier attaches nowhere")
		})

		t.Run("takes the trailing carrier of a declaration that the signature depth leaves out", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export {};\nclass Hidden {} " + carrierLine,
			)}}, aFile, plugin.DepthSignatures)
			assert.Empty(t, found, "the trailing carrier leaves with its declaration")
		})
	})
}
