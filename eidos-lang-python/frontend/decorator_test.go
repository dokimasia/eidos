// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestDecorator pins the annotations of the decorators and the directives
// of the decorators of the brand.
func TestDecorator(t *testing.T) {
	t.Parallel()

	t.Run("decoratorsOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want symbol.Annotation
		}{
			{
				name: "lowers a name to an annotation of the name",
				give: "@dataclass",
				want: symbol.Annotation{Name: "dataclass"},
			},
			{
				name: "lowers a call to an annotation of its callee and its arguments as written",
				give: "@dataclasses.dataclass(frozen=True, slots = True)",
				want: symbol.Annotation{Name: "dataclasses.dataclass", Args: []string{"frozen=True", "slots = True"}},
			},
			{
				name: "lowers another expression to an annotation of the expression",
				give: "@buttons[0].clicked", want: symbol.Annotation{Name: "buttons[0].clicked"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				fn := named[*node.Function](t, declsOf(t, tt.give+"\ndef f(): ...\n"), "f")
				assert.Equal(t, fn.Annotations, symbol.Annotations{tt.want}, "the annotation of the decorator")
			})
		}
	})

	t.Run("marker", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches the directive of a decorator of the brand", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "@fixture.gen.table(\"t\", rows=-2, ratio=1.5, live=True, off=False, "+
				"tags=[\"a\", (\"b\")], hex=0x1F)\nclass Row: ...\n")
			assert.Empty(t, codesOf(sink), "the marker lifts")
			attachments := gb.Attachments()
			assert.Length(t, attachments, 1, "the directive of the marker")
			raw := attachments[0].Raw
			expect.Equal(t, string(raw.Name), carrierDirective, "the name of the directive")
			expect.Equal(t, raw.Args, []directive.RawArg{
				{Value: directive.RawValue{Text: "t", Quoted: true}},
				{Key: "rows", Value: directive.RawValue{Text: "-2"}},
				{Key: "ratio", Value: directive.RawValue{Text: "1.5"}},
				{Key: "live", Value: directive.RawValue{Text: "true"}},
				{Key: "off", Value: directive.RawValue{Text: "false"}},
				{Key: "tags", Value: directive.RawValue{List: []directive.RawValue{
					{Text: "a", Quoted: true}, {Text: "b", Quoted: true},
				}}},
				{Key: "hex", Value: directive.RawValue{Text: "31"}},
			}, "each argument lifted from the syntax")
		})

		refused := []struct {
			name string
			give string
		}{
			{name: "reports BadMarker for an argument that does not lift", give: "name=compute()"},
			{name: "reports BadMarker for a list with an element that does not lift", give: "tags=[\"a\", other]"},
			{name: "reports BadMarker for a concatenation with an f-string", give: "\"a\" f\"{b}\""},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := parsedSource(t, "@fixture.gen.table("+tt.give+")\nclass Row: ...\n")
				assert.Equal(t, codesOf(sink), []diag.Code{frontend.BadMarker}, "the argument is no literal")
			})
		}

		t.Run("keeps a decorator of the brand as an annotation", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "@fixture.gen.table\nclass Row: ...\n"), "Row")
			expect.Equal(t, st.Annotations, symbol.Annotations{{Name: "fixture.gen.table"}}, "the decorator as written")
		})

		t.Run("attaches nothing for a decorator outside the brand", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "@other.gen.table\nclass Row: ...\n")
			assert.Empty(t, gb.Attachments(), "the decorator is metadata of another tool")
		})
	})
}
