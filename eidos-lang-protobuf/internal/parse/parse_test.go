// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package parse_test

import (
	"strings"
	"testing"

	"github.com/bufbuild/protocompile/experimental/report"
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf/internal/parse"
)

// sourcePath is the path that every case parses its source under.
const sourcePath = "svc/store.proto"

// TestParse pins the parse of a source and the copy that a file of edition
// 2026 parses from.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			give        string
			wantPatched bool
			wantValue   string
		}{
			{
				name: "parses a file of proto3 from its own text",
				give: "syntax = \"proto3\";\n\nmessage Row {}\n", wantValue: "proto3",
			},
			{
				name: "parses a file of edition 2024 from its own text",
				give: "edition = \"2024\";\n\nlocal message Row {}\n", wantValue: "2024",
			},
			{
				name: "parses a file of edition 2026 from a copy that states 2024",
				give: "edition = \"2026\";\n\nlocal message Row {}\n", wantPatched: true, wantValue: "2024",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree, r, patched := parse.File(sourcePath, tt.give)
				assert.Equal(t, patched, tt.wantPatched, "whether the parse read a copy")
				assert.Equal(t, tree.Syntax().Value().AsLiteral().AsString().Text(), tt.wantValue,
					"the value of the version statement of the parsed text")
				assert.Equal(t, errorsOf(r), 0, "the source parses without an error")
			})
		}

		t.Run("keeps every position of a file of edition 2026", func(t *testing.T) {
			t.Parallel()

			text := "edition = \"2026\";\n\nlocal message Row {}\n"
			tree, _, _ := parse.File(sourcePath, text)
			decls := tree.Decls()
			assert.Equal(t, decls.Len(), 2, "the version statement and the message")
			span := decls.At(1).Span()
			assert.Equal(t, span.Start, strings.Index(text, "local"), "the message starts where the file writes it")
		})

		t.Run("parses a syntax statement that states 2026 from its own text", func(t *testing.T) {
			t.Parallel()

			tree, _, patched := parse.File(sourcePath, "syntax = \"2026\";\n")
			assert.False(t, patched, "only an edition statement reads a copy")
			assert.Equal(t, tree.Syntax().Value().AsLiteral().AsString().Text(), "2026",
				"the parsed text states the value of the file")
		})

		t.Run("parses a file without a version statement from its own text", func(t *testing.T) {
			t.Parallel()

			tree, r, patched := parse.File(sourcePath, "message Row {}\n")
			assert.False(t, patched, "a file without an edition statement parses from its own text")
			assert.True(t, tree.Syntax().IsZero(), "the file has no version statement")
			assert.Equal(t, errorsOf(r), 0, "the source parses without an error")
		})

		t.Run("reports the errors of the parser", func(t *testing.T) {
			t.Parallel()

			_, r, _ := parse.File(sourcePath, "syntax = \"proto3\";\nmessage {\n")
			assert.NotEqual(t, errorsOf(r), 0, "a message without a name is a syntax error")
		})

		t.Run("returns the tree of the file at its path", func(t *testing.T) {
			t.Parallel()

			tree, _, _ := parse.File(sourcePath, "syntax = \"proto3\";\n")
			assert.Equal(t, tree.Path(), sourcePath, "the tree names the file")
		})
	})

	t.Run("Text", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			give        string
			want        string
			wantPatched bool
		}{
			{
				name: "returns the text of a file of proto3",
				give: "syntax = \"proto3\";\n\nmessage Row {}\n",
				want: "syntax = \"proto3\";\n\nmessage Row {}\n",
			},
			{
				name: "returns a copy of a file of edition 2026 that states 2024",
				give: "edition = \"2026\";\n\nlocal message Row {}\n",
				want: "edition = \"2024\";\n\nlocal message Row {}\n", wantPatched: true,
			},
			{
				name: "returns the text of a syntax statement that states 2026",
				give: "syntax = \"2026\";\n", want: "syntax = \"2026\";\n",
			},
			{
				name: "returns a copy of a file of edition 2026 with a syntax error after its edition statement",
				give: "edition = \"2026\";\nmessage {\n", want: "edition = \"2024\";\nmessage {\n", wantPatched: true,
			},
			{
				name: "returns the text of a file whose edition statement does not parse",
				give: "edition = 2026;\n", want: "edition = 2026;\n",
			},
			{name: "returns an empty text", give: "", want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, patched := parse.Text(sourcePath, tt.give)
				assert.Equal(t, got, tt.want, "the text that protocompile reads")
				assert.Equal(t, patched, tt.wantPatched, "whether the text is a copy")
			})
		}
	})
}

// errorsOf counts the diagnostics of a report at the level of an error or
// above.
func errorsOf(r *report.Report) int {
	n := 0
	for i := range r.Diagnostics {
		if level := r.Diagnostics[i].Level(); level == report.Error || level == report.ICE {
			n++
		}
	}
	return n
}
