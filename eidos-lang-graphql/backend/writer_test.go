// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// describedAllocs is a definition with a description of one line: the text
// of the definition and the quoted description.
const describedAllocs = 1 + 1

// The opening and the closing of the object type whose member a case of the
// writers writes.
const (
	rowOpen  = "type Row {\n"
	rowClose = "}\n"
)

// TestWriter pins the parts of a definition that every writer shares: the
// type expressions, the descriptions, the trailing comments and the applied
// directives, through the member of an object type.
func TestWriter(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Struct", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.Field
				want string
			}{
				{
					name: "returns a type that is not nullable with the mark of a non-null type",
					give: &emit.Field{Name: nameField, Type: stringRef},
					want: "  name: String!\n",
				},
				{
					name: "returns a nullable field without the mark of a non-null type",
					give: &emit.Field{Name: nameField, Type: stringRef, Optional: true},
					want: "  name: String\n",
				},
				{
					name: "returns an optional form without the mark of a non-null type",
					give: &emit.Field{
						Name: nameField,
						Type: &emit.TypeRef{
							Spelling: "String?",
							Form:     symbol.FormOptional,
							Elems:    []*emit.TypeRef{stringRef},
						},
					},
					want: "  name: String\n",
				},
				{
					name: "returns a list in brackets",
					give: &emit.Field{
						Name: "names",
						Type: &emit.TypeRef{
							Spelling: "[String]",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{stringRef},
						},
					},
					want: "  names: [String!]!\n",
				},
				{
					name: "returns a nullable list of nullable elements",
					give: &emit.Field{
						Name: "names",
						Type: &emit.TypeRef{
							Spelling: "[String?]?", Form: symbol.FormOptional,
							Elems: []*emit.TypeRef{{
								Spelling: "[String?]", Form: symbol.FormList,
								Elems: []*emit.TypeRef{{
									Spelling: "String?", Form: symbol.FormOptional, Elems: []*emit.TypeRef{stringRef},
								}},
							}},
						},
					},
					want: "  names: [String]\n",
				},
				{
					name: "returns a stream as its element",
					give: &emit.Field{
						Name: "changes",
						Type: &emit.TypeRef{
							Spelling: "stream String",
							Form:     symbol.FormStream,
							Elems:    []*emit.TypeRef{stringRef},
						},
					},
					want: "  changes: String!\n",
				},
				{
					name: "returns the stream of a nullable field as a nullable element",
					give: &emit.Field{
						Name:     "changes",
						Optional: true,
						Type: &emit.TypeRef{
							Spelling: "stream String",
							Form:     symbol.FormStream,
							Elems:    []*emit.TypeRef{stringRef},
						},
					},
					want: "  changes: String\n",
				},
				{
					name: "returns a description of one line as a string",
					give: &emit.Field{Name: nameField, Type: stringRef, Doc: []string{"Name keys the row."}},
					want: "  \"Name keys the row.\"\n  name: String!\n",
				},
				{
					name: "returns a description of one line that escapes its quotes",
					give: &emit.Field{Name: nameField, Type: stringRef, Doc: []string{`Says "hi".`}},
					want: "  \"Says \\\"hi\\\".\"\n  name: String!\n",
				},
				{
					name: "returns a description of two lines as a block string",
					give: &emit.Field{
						Name: nameField,
						Type: stringRef,
						Doc:  []string{"Name keys the row.", "It is unique."},
					},
					want: "  \"\"\"\n  Name keys the row.\n  It is unique.\n  \"\"\"\n  name: String!\n",
				},
				{
					name: "returns an empty line of a block string without the indentation",
					give: &emit.Field{Name: nameField, Type: stringRef, Doc: []string{"First.", "", "Third."}},
					want: "  \"\"\"\n  First.\n\n  Third.\n  \"\"\"\n  name: String!\n",
				},
				{
					name: "returns a block string that escapes its triple quotes",
					give: &emit.Field{Name: nameField, Type: stringRef, Doc: []string{`Write """.`, "Done."}},
					want: "  \"\"\"\n  Write \\\"\"\".\n  Done.\n  \"\"\"\n  name: String!\n",
				},
				{
					name: "returns a trailing comment after a space",
					give: &emit.Field{Name: nameField, Type: stringRef, Comment: "unique"},
					want: "  name: String! # unique\n",
				},
				{
					name: "returns each line feed of a comment as a space",
					give: &emit.Field{Name: nameField, Type: stringRef, Comment: "unique\nper store"},
					want: "  name: String! # unique per store\n",
				},
				{
					name: "returns a carriage return before a line feed of a comment as one space",
					give: &emit.Field{Name: nameField, Type: stringRef, Comment: "unique\r\nper store"},
					want: "  name: String! # unique per store\n",
				},
				{
					name: "returns each carriage return of a comment as a space",
					give: &emit.Field{Name: nameField, Type: stringRef, Comment: "unique\rper store"},
					want: "  name: String! # unique per store\n",
				},
				{
					name: "returns a directive without arguments after a space",
					give: &emit.Field{Name: nameField, Type: stringRef, Annotations: symbol.Annotations{shareable}},
					want: "  name: String! @shareable\n",
				},
				{
					name: "returns the argument of a directive in parentheses",
					give: &emit.Field{
						Name: nameField, Type: stringRef,
						Annotations: symbol.Annotations{{Name: "deprecated", Args: []string{`reason: "old"`}}},
					},
					want: "  name: String! @deprecated(reason: \"old\")\n",
				},
				{
					name: "returns the arguments of a directive separated by a comma",
					give: &emit.Field{
						Name: nameField, Type: stringRef,
						Annotations: symbol.Annotations{{Name: "cost", Args: []string{"weight: 2", "complexity: 3"}}},
					},
					want: "  name: String! @cost(weight: 2, complexity: 3)\n",
				},
				{
					name: "returns no directive for a marker of the satellite",
					give: &emit.Field{Name: nameField, Type: stringRef, Annotations: symbol.Annotations{inputMarker}},
					want: "  name: String!\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(tt.give)}
					assert.Equal(t, (&backend.Vocabulary{}).Struct(row), rowOpen+tt.want+rowClose,
						"the member of the object type")
				})
			}
		})
	})
}

// A description of one line allocates the quoted text that gqlparser
// escapes, beside the text of the definition. The ordinary run, which runs
// no benchmark, checks that ceiling here.
func TestWriterAllocs(t *testing.T) {
	v := &backend.Vocabulary{}
	row := &emit.Struct{Name: rowName, Doc: []string{"Row is one record."}, Comment: "one\nper fetch"}
	var got string
	assert.MaxAllocs(t, func() { got = v.Struct(row) }, describedAllocs,
		"Struct allocates the text of the definition and its quoted description")
	assert.Equal(t, got, "\"Row is one record.\"\ntype Row # one per fetch\n", "Struct writes the description")
}

// BenchmarkWriter measures a definition with a description and a comment,
// which every documented declaration of a file writes.
func BenchmarkWriter(b *testing.B) {
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Struct", func(b *testing.B) {
			v := &backend.Vocabulary{}
			row := &emit.Struct{Name: rowName, Doc: []string{"Row is one record."}, Comment: "one\nper fetch"}
			c := bench.Start(b).Warmup(1).MaxAllocs(describedAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = v.Struct(row)
			}
			assert.Equal(b, got, "\"Row is one record.\"\ntype Row # one per fetch\n", "Struct writes the description")
		})
	})
}
