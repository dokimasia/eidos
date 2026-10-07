// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The carriers the comment cases write, in a doc comment and in a plain
// one, and the directive they name.
const (
	carrierDoc    = "/// +fixture:gen:table name=t\n"
	carrierPlain  = "// +fixture:gen:table name=t\n"
	directiveName = "gen:table"
)

// Comments are where documentation and directives are, so which comment
// an item takes, and what becomes of one no item takes, is pinned.
func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("leading", func(t *testing.T) {
		t.Parallel()

		t.Run("documents an item with the doc comments directly above it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "/// A is one\n/// record.\npub struct A;\n", "A").Doc,
				[]string{"A is one", "record."}, "each line of the run")
		})

		t.Run("documents an item with a doc block comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "/** A is one record. */\npub struct A;\n", "A").Doc,
				[]string{"A is one record."}, "the block's text")
		})

		t.Run("skips the attributes between an item and its doc comments", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "/// A.\n#[derive(Debug)]\npub struct A;\n", "A").Doc, []string{"A."},
				"the comment above the attribute")
		})

		t.Run("takes a comment between two attributes into the run", func(t *testing.T) {
			t.Parallel()

			src := "/// A.\n#[derive(Debug)]\n// B.\n#[repr(C)]\npub struct A;\n"
			assert.Equal(t, structOf(t, src, "A").Doc, []string{"A.", "B."}, "both comments document the item")
		})

		t.Run("takes the comment after an attribute on its line", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[derive(Debug)] // A.\npub struct A;\n", "A").Doc, []string{"A."},
				"an attribute is no item, so the comment is not its trailing comment")
		})

		t.Run("ends the documentation at a blank line", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, structOf(t, "/// File.\n\npub struct A;\n", "A").Doc,
				"a doc line comment's newline is its own, so the blank line is the gap")
		})

		t.Run("leaves an inner doc comment to the module", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "//! Crate.\npub struct A;\n")
			assert.Empty(t, named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A").Doc,
				"the item takes no inner doc comment")
			assert.Equal(t, packageIn(t, gb, crateName).Doc, []string{"Crate."}, "the module does")
		})

		t.Run("leaves the trailing comment of the field before to that field", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A {\n    pub a: u8, // the a\n    pub b: u8,\n}\n", "A")
			assert.Empty(t, st.Fields[1].Doc, "b takes no comment of a's line")
		})

		t.Run("documents the field after a doc comment on the line of the field before", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A {\n    pub a: u8, /// the b\n    pub b: u8,\n}\n", "A")
			assert.Equal(t, st.Fields[1].Doc, []string{"the b"}, "a doc comment documents the item after it")
		})

		t.Run("documents a positional field past its visibility", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct T(\n    /// the zero\n    pub u8,\n);\n", "T")
			assert.Equal(t, st.Fields[0].Doc, []string{"the zero"}, "the comment above the field's modifier")
		})
	})

	t.Run("trailing", func(t *testing.T) {
		t.Parallel()

		t.Run("records the plain comment on an item's last line as its trailing comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "pub struct A; // the a\n", "A").Comment, "the a", "the comment after the item")
		})

		t.Run("records the plain comment after a field as the field's trailing comment", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A {\n    pub a: u8, // the a\n}\n", "A")
			assert.Equal(t, st.Fields[0].Comment, "the a", "the comment after the comma")
		})

		t.Run("records the plain comment after a last field without a comma", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A {\n    pub a: u8 // the a\n}\n", "A")
			assert.Equal(t, st.Fields[0].Comment, "the a", "the comment before the brace")
		})

		t.Run("records no comment on the next line as a trailing comment", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "pub struct A;\n// The b.\npub struct B;\n")
			assert.Equal(t, named[*node.Struct](t, decls, "A").Comment, "", "the comment opens below A")
			assert.Equal(t, named[*node.Struct](t, decls, "B").Doc, []string{"The b."}, "and documents B")
		})

		t.Run("records no doc comment as a trailing comment", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A {\n    pub a: u8, /// the b\n    pub b: u8,\n}\n", "A")
			assert.Equal(t, st.Fields[0].Comment, "", "the doc comment is b's")
		})
	})

	t.Run("declParts", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches a carrier from the doc comment above an item", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, carrierDoc+"pub struct A;\n")
			assert.Empty(t, found, "the carrier parses")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			assert.Equal(t, string(gb.Attachments()[0].Raw.Name), directiveName, "under its spelling")
		})

		t.Run("attaches a carrier from a plain comment above an item", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, carrierPlain+"pub struct A;\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
		})

		t.Run("attaches a carrier from a trailing comment", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub struct A; "+carrierPlain)
			assert.Length(t, gb.Attachments(), 1, "the trailing comment's directive")
		})

		t.Run("documents an item with its #[doc] attributes after its doc comments", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "/// One.\n#[doc = \"Two.\"]\npub struct A;\n", "A").Doc,
				[]string{"One.", "Two."}, "the comments, then the attributes")
		})
	})

	t.Run("parts", func(t *testing.T) {
		t.Parallel()

		t.Run("folds a carrier's continuation across consecutive doc comments", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "/// +fixture:gen:table \\\n/// name=t\npub struct A;\n")
			assert.Empty(t, found, "the folded carrier parses")
			assert.Length(t, gb.Attachments(), 1, "one directive from two lines")
			assert.Length(t, gb.Attachments()[0].Raw.Args, 1, "with the argument the second line states")
		})

		t.Run("reads a block comment apart from the line comments around it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "/// A.\n/** B. */\n/// C.\npub struct A;\n", "A").Doc,
				[]string{"A.", "B.", "C."}, "each reads alone, in source order")
		})

		t.Run("reads line comments after a gap apart from the run before it", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "//! Crate.\n\n//! +fixture:gen:table name=t\n")
			assert.Length(t, gb.Attachments(), 1, "the module's carrier")
			assert.Equal(t, gb.Attachments()[0].Raw.Pos.Line, 3, "positioned at its own line")
		})
	})

	t.Run("skip", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the carriers of an item signature depth leaves out with it", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{libRoot: carrierPlain + "struct Hidden;\n"}),
				libRoot, plugin.DepthSignatures, nil)
			assert.Empty(t, found, "the carrier leaves with its item")
			assert.Empty(t, gb.Attachments(), "and attaches nowhere")
		})

		t.Run("takes the trailing carrier of an item signature depth leaves out with it", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, crateTree(map[string]string{libRoot: "struct Hidden; " + carrierPlain}),
				libRoot, plugin.DepthSignatures, nil)
			assert.Empty(t, found, "the trailing carrier leaves with its item")
		})

		t.Run("takes the carriers inside an item signature depth leaves out with it", func(t *testing.T) {
			t.Parallel()

			src := "struct Hidden {\n    a: u8,\n\n    " + carrierPlain + "}\n"
			_, found := parsedTree(t, crateTree(map[string]string{libRoot: src}), libRoot, plugin.DepthSignatures, nil)
			assert.Empty(t, found, "the sweep passes over the item")
		})

		t.Run("takes the carriers of an item a cfg predicate keeps out with it", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, carrierPlain+"#[cfg(feature = \"x\")]\npub struct A;\n")
			assert.Empty(t, found, "the carrier leaves with its item")
		})
	})

	t.Run("refuse", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnaddressedCarrier for a carrier on a macro", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, carrierPlain+"macro_rules! m { () => {} }\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a macro is not in the model")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a parameter", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub fn f(\n    "+carrierPlain+"    a: u8,\n) {}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a parameter takes no carrier")
		})

		t.Run("reports UnaddressedCarrier for a carrier on an impl block", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub struct A;\n"+carrierPlain+"impl A {}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "an impl block is no declaration")
		})
	})

	t.Run("sweep", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnaddressedCarrier for a carrier no item takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, carrierPlain+"\npub struct A;\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"a blank line keeps the carrier from the struct")
		})

		t.Run("reports UnaddressedCarrier for a carrier in a struct body no field takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub struct A {\n    a: u8,\n\n    "+carrierPlain+"}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "the carrier is on no field")
		})

		t.Run("passes over a carrier inside a function body", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub fn f() {\n    "+carrierPlain+"    let x = 1;\n}\n")
			assert.Empty(t, found, "a body's comments belong to its statements")
		})

		t.Run("passes over the carriers of an inline module a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{
				manifestPath: {Data: []byte(manifestSrc)},
				libRoot: {Data: []byte("mod gated {\n    #![cfg(feature = \"x\")]\n    " + carrierPlain +
					"    pub struct A;\n}\n")},
			}, libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "the carrier leaves with the module")
			assert.Empty(t, gb.Attachments(), "and attaches nowhere")
		})
	})
}

// structOf parses the fixture crate whose library root is src and
// returns the struct of a name its root declares.
func structOf(tb testing.TB, src, name string) *node.Struct {
	tb.Helper()

	return named[*node.Struct](tb, declsOf(tb, src), name)
}
