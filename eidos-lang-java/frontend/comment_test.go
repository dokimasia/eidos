// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The carriers of the comment cases, in a line comment and in a Javadoc
// comment, the text of the carrier as the author wrote it, and the name of
// its directive.
const (
	carrierLine   = "// +fixture:gen:table name=t\n"
	carrierDoc    = "/** +fixture:gen:table name=t */\n"
	carrierText   = "+fixture:gen:table name=t"
	directiveName = "gen:table"
)

// The subjects of the refusal messages, as the messages describe them, so
// that the author learns where the carrier is.
const (
	onStatement   = "cannot address a statement outside a method"
	onInitializer = "cannot address an initializer block"
	onParameter   = "cannot address a parameter"
	onEnumType    = "cannot address a type that an enum declares"
	onNothing     = "cannot address a comment that no declaration takes"
)

// Comments contain the documentation and the directives. The cases pin
// which comments a declaration takes, and what becomes of a comment that
// no declaration takes.
func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("commentWalk", func(t *testing.T) {
		t.Parallel()

		t.Run("documents a declaration with the Javadoc directly above it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "/** A is one record. */\npublic class A {}\n", "A").Doc,
				[]string{"A is one record."}, "the text of the Javadoc")
		})

		t.Run("documents a declaration with the line comments directly above it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "// A is one\n// record.\npublic class A {}\n", "A").Doc,
				[]string{"A is one", "record."}, "each line of the run")
		})

		t.Run("ends the documentation at a blank line", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, classOf(t, "// File.\n\npublic class A {}\n", "A").Doc, "the blank line ends the run")
		})

		t.Run("takes the comment between an annotation and its keyword", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "@Deprecated\n// A.\npublic class A {}\n", "A").Doc, []string{"A."},
				"a comment among the modifiers")
		})

		t.Run("leaves the trailing comment of the member before to that member", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    int a; // the a\n    int b;\n")
			assert.Empty(t, st.Fields[1].Doc, "b takes no comment from the line of a")
		})

		t.Run("records the comment on the last line of a declaration as its trailing comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    int a; // the a\n").Fields[0].Comment, "the a", "the comment after a")
		})

		t.Run("records no comment on the next line as a trailing comment", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    int a;\n    // The b.\n    int b;\n")
			assert.Equal(t, st.Fields[0].Comment, "", "the comment starts below a")
		})

		t.Run("passes over a carrier inside a method body", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    void f() {\n        "+carrierLine+
				"        int x = 1;\n    }\n}\n")
			assert.Empty(t, found, "the comments of a body belong to its statements")
		})

		t.Run("passes over a carrier inside a constructor body", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    A() {\n        "+carrierLine+
				"        super();\n    }\n}\n")
			assert.Empty(t, found, "the comments of a constructor body belong to its statements")
		})
	})

	t.Run("declParts", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches a carrier from the Javadoc above a declaration", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, pkgClause+carrierDoc+publicClass)
			assert.Empty(t, found, "the carrier parses")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			assert.Equal(t, string(gb.Attachments()[0].Raw.Name), directiveName, "the name of the directive")
		})

		t.Run("attaches a carrier from a line comment above a declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+carrierLine+publicClass)
			assert.Length(t, gb.Attachments(), 1, "one directive")
		})

		t.Run("attaches a carrier from a trailing comment", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public class A {\n    int a; "+carrierLine+"}\n")
			assert.Length(t, gb.Attachments(), 1, "the directive of the trailing comment")
			_, onField := gb.Attachments()[0].Subject.(*node.Field)
			assert.True(t, onField, "the subject is the field")
		})

		t.Run("reports BadCarrier for a carrier that the kernel grammar refuses", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"// +fixture:gen:table name=\n"+publicClass)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadCarrier}, "the carrier does not parse")
		})
	})

	t.Run("parts", func(t *testing.T) {
		t.Parallel()

		t.Run("folds the continuation of a carrier across consecutive line comments", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, pkgClause+"// +fixture:gen:table \\\n// name=t\n"+publicClass)
			assert.Empty(t, found, "the folded carrier parses")
			assert.Length(t, gb.Attachments()[0].Raw.Args, 1, "the argument on the second line")
		})

		t.Run("positions a folded carrier at its first line", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"// +fixture:gen:table \\\n// name=t\n"+publicClass)
			assert.Length(t, gb.Attachments(), 1, "one directive from two lines")
			assert.Equal(t, gb.Attachments()[0].Raw.Pos.Line, 3, "the line below the package clause")
		})

		t.Run("reads a block comment apart from the line comments around it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "// A.\n/** B. */\n// C.\npublic class A {}\n", "A").Doc,
				[]string{"A.", "B.", "C."}, "each comment reads alone, in source order")
		})

		t.Run("reads a comment among the modifiers apart from the run above them", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"// A.\n@Deprecated\n"+carrierLine+publicClass)
			assert.Length(t, gb.Attachments(), 1, "the carrier among the modifiers")
			assert.Equal(t, gb.Attachments()[0].Raw.Pos.Line, 5, "the position is the line of the carrier")
		})
	})

	t.Run("skip", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the carriers of a declaration that the signature depth leaves out", func(t *testing.T) {
			t.Parallel()

			gb, found := shallowSource(t, pkgClause+carrierLine+"class Hidden {}\n")
			assert.Empty(t, found, "the carrier leaves with its declaration")
			assert.Empty(t, gb.Attachments(), "the carrier attaches nowhere")
		})

		t.Run("takes the trailing carrier of a declaration that the signature depth leaves out", func(t *testing.T) {
			t.Parallel()

			_, found := shallowSource(t, pkgClause+"class Hidden {} "+carrierLine)
			assert.Empty(t, found, "the trailing carrier leaves with its declaration")
		})

		t.Run("takes the carriers inside a declaration that the signature depth leaves out", func(t *testing.T) {
			t.Parallel()

			_, found := shallowSource(t, pkgClause+"class Hidden {\n    int a;\n\n    "+carrierLine+"}\n")
			assert.Empty(t, found, "the sweep passes over the declaration")
		})
	})

	t.Run("refuse", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnaddressedCarrier for a carrier on a statement outside a method", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, carrierLine+"int x = 1;\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"a statement is not a declaration")
		})

		t.Run("describes a statement outside a method as the subject of the carrier", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, carrierLine+"int x = 1;\n")
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, onStatement, "the finding states where the author put the carrier")
		})
	})
}

// shallowSource parses one file at src/main/java/com/acme/A.java at
// signature depth.
func shallowSource(tb testing.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{srcFile: {Data: []byte(src)}}, srcFile, plugin.DepthSignatures)
}
