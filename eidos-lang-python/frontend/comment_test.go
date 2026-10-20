// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
)

// TestComment pins the carriers, the trailing comments and the type
// comments of the declarations of Python.
func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("declParts", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches a carrier above the first decorator to the declaration", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, carrier+"@dataclass\nclass Row: ...\n")
			assert.Empty(t, codesOf(sink), "the carrier attaches")
			_, isStruct := gb.Attachments()[0].Subject.(*node.Struct)
			assert.True(t, isStruct, "the class is the subject")
		})

		t.Run("reads the text of a comment that is no carrier as no documentation", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "# A row.\nclass Row: ...\n"), "Row")
			assert.Empty(t, st.Doc, "the docstring documents a class")
		})

		t.Run("reports UnaddressedCarrier for a carrier between a decorator and its definition", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "@dataclass\n"+carrier+"class Row: ...\n")
			assert.Equal(
				t,
				codesOf(sink),
				[]diag.Code{frontend.UnaddressedCarrier},
				"the carrier is inside the statement",
			)
		})

		t.Run("reports BadCarrier for a carrier outside the grammar of the kernel", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "# +fixture:gen:table name=\nclass Row: ...\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.BadCarrier}, "the payload does not parse")
		})
	})

	t.Run("headComment", func(t *testing.T) {
		t.Parallel()

		t.Run("takes a comment on the line of the name of a class", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class Row:  # one row\n    x: int\n"), "Row")
			expect.Equal(t, st.Comment, "one row", "the comment on the line of the name")
		})

		t.Run("takes no type comment as the comment of a callable", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f(a):  # type: (int) -> None\n    pass\n"), "f")
			expect.Equal(t, fn.Comment, "", "a type comment is the signature")
		})
	})

	t.Run("typeComment", func(t *testing.T) {
		t.Parallel()

		t.Run("reads no type of the directive type: ignore", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count = f()  # type: ignore[attr-defined]\n"), "count")
			expect.Nil(t, v.Type, "the directive silences a type checker")
		})
	})
}
