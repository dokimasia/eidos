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

// The carriers the comment cases write, in a line comment and in a
// Javadoc comment, the carrier's text as the author wrote it, and the
// directive it names.
const (
	carrierLine   = "// +fixture:gen:table name=t\n"
	carrierDoc    = "/** +fixture:gen:table name=t */\n"
	carrierText   = "+fixture:gen:table name=t"
	directiveName = "gen:table"
)

// The subjects a refused carrier's message names, as the message spells
// them, which tell the author where the carrier is.
const (
	onStatement   = "is on a statement outside a method"
	onInitializer = "is on an initializer block"
	onParameter   = "is on a parameter"
	onEnumType    = "is on a type an enum declares"
	onNothing     = "is on a comment no declaration takes"
)

// Comments are where documentation and directives are, so which comment
// a declaration takes, and what becomes of one no declaration takes, is
// pinned.
func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("refuseCarriers", func(t *testing.T) {
		t.Parallel()

		t.Run("quotes a refused carrier as the author wrote it", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+carrierLine+"\n"+publicClass)
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, carrierText, "quoting the carrier")
		})
	})

	t.Run("leading", func(t *testing.T) {
		t.Parallel()

		t.Run("documents a declaration with the Javadoc directly above it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "/** A is one record. */\npublic class A {}\n", "A").Doc,
				[]string{"A is one record."}, "the Javadoc's text")
		})

		t.Run("documents a declaration with the line comments directly above it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "// A is one\n// record.\npublic class A {}\n", "A").Doc,
				[]string{"A is one", "record."}, "each line of the run")
		})

		t.Run("ends the documentation at a blank line", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, classOf(t, "// File.\n\npublic class A {}\n", "A").Doc, "the blank line is the gap")
		})

		t.Run("takes the comment between an annotation and its keyword", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "@Deprecated\n// A.\npublic class A {}\n", "A").Doc, []string{"A."},
				"a comment among the modifiers")
		})

		t.Run("leaves the trailing comment of the member before to that member", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    int a; // the a\n    int b;\n")
			assert.Empty(t, st.Fields[1].Doc, "b takes no comment of a's line")
		})
	})

	t.Run("trailing", func(t *testing.T) {
		t.Parallel()

		t.Run("records the comment on a declaration's last line as its trailing comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    int a; // the a\n").Fields[0].Comment, "the a", "the comment after a")
		})

		t.Run("records no comment on the next line as a trailing comment", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    int a;\n    // The b.\n    int b;\n")
			assert.Equal(t, st.Fields[0].Comment, "", "the comment opens below a")
		})
	})

	t.Run("declParts", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches a carrier from the Javadoc above a declaration", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, pkgClause+carrierDoc+publicClass)
			assert.Empty(t, found, "the carrier parses")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			assert.Equal(t, string(gb.Attachments()[0].Raw.Name), directiveName, "under its spelling")
		})

		t.Run("attaches a carrier from a line comment above a declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+carrierLine+publicClass)
			assert.Length(t, gb.Attachments(), 1, "one directive")
		})

		t.Run("attaches a carrier from a trailing comment", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public class A {\n    int a; "+carrierLine+"}\n")
			assert.Length(t, gb.Attachments(), 1, "the trailing comment's directive")
			_, onField := gb.Attachments()[0].Subject.(*node.Field)
			assert.True(t, onField, "on the field")
		})

		t.Run("reports BadCarrier for a carrier the kernel grammar refuses", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"// +fixture:gen:table name=\n"+publicClass)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadCarrier}, "the carrier does not parse")
		})
	})

	t.Run("parts", func(t *testing.T) {
		t.Parallel()

		t.Run("folds a carrier's continuation across consecutive line comments", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, pkgClause+"// +fixture:gen:table \\\n// name=t\n"+publicClass)
			assert.Empty(t, found, "the folded carrier parses")
			assert.Length(t, gb.Attachments()[0].Raw.Args, 1, "with the argument the second line states")
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
				[]string{"A.", "B.", "C."}, "each reads alone, in source order")
		})

		t.Run("reads a comment among the modifiers apart from the run above them", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"// A.\n@Deprecated\n"+carrierLine+publicClass)
			assert.Length(t, gb.Attachments(), 1, "the carrier among the modifiers")
			assert.Equal(t, gb.Attachments()[0].Raw.Pos.Line, 5, "positioned at its own line")
		})
	})

	t.Run("skip", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the carriers of a declaration signature depth leaves out with it", func(t *testing.T) {
			t.Parallel()

			gb, found := shallowSource(t, pkgClause+carrierLine+"class Hidden {}\n")
			assert.Empty(t, found, "the carrier leaves with its declaration")
			assert.Empty(t, gb.Attachments(), "and attaches nowhere")
		})

		t.Run("takes the trailing carrier of a declaration signature depth leaves out with it", func(t *testing.T) {
			t.Parallel()

			_, found := shallowSource(t, pkgClause+"class Hidden {} "+carrierLine)
			assert.Empty(t, found, "the trailing carrier leaves with its declaration")
		})

		t.Run("takes the carriers inside a declaration signature depth leaves out with it", func(t *testing.T) {
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
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a statement is no declaration")
		})

		t.Run("names a statement outside a method as the refused carrier's subject", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, carrierLine+"int x = 1;\n")
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, onStatement, "where the author put it")
		})
	})

	t.Run("sweep", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnaddressedCarrier for a carrier no declaration takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+carrierLine+"\n"+publicClass)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"a blank line keeps the carrier from the class")
		})

		t.Run("names no subject for a carrier no declaration takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+carrierLine+"\n"+publicClass)
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, onNothing, "the comment is apart from every declaration")
		})

		t.Run("reports UnaddressedCarrier for a carrier in a class body no member takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    int a;\n\n    "+carrierLine+"}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "the carrier is on no member")
		})

		t.Run("passes over a carrier inside a method body", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    void f() {\n        "+carrierLine+
				"        int x = 1;\n    }\n}\n")
			assert.Empty(t, found, "a body's comments belong to its statements")
		})

		t.Run("passes over a carrier inside a constructor body", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    A() {\n        "+carrierLine+
				"        super();\n    }\n}\n")
			assert.Empty(t, found, "a constructor body's comments belong to its statements")
		})
	})
}

// shallowSource parses one file at src/main/java/com/acme/A.java, at
// signature depth.
func shallowSource(tb testing.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{srcFile: {Data: []byte(src)}}, srcFile, plugin.DepthSignatures)
}
