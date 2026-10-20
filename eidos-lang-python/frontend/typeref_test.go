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
	"go.dokimi.dev/eidos/sdk/symbol"
)

// typingImports is the head of the cases of the types, which binds the
// forms of typing that they spell.
const typingImports = "import typing as t\nfrom collections.abc import Callable\n" +
	"from typing import Annotated, Literal, Optional, Tuple, Union\n\n\n"

// TestTyperef pins the structural forms of the type references that the
// annotations of Python state.
func TestTyperef(t *testing.T) {
	t.Parallel()

	t.Run("typeRef", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			wantForm  symbol.TypeForm
			wantElems []string
		}{
			{
				name:      "lowers X | None to Optional of X",
				give:      "int | None",
				wantForm:  symbol.FormOptional,
				wantElems: []string{"int"},
			},
			{
				name: "lowers Optional[X] to Optional of X", give: "Optional[int]", wantForm: symbol.FormOptional,
				wantElems: []string{"int"},
			},
			{
				name: "lowers Union[X, None] to Optional of X", give: "Union[int, None]", wantForm: symbol.FormOptional,
				wantElems: []string{"int"},
			},
			{
				name: "lowers a union to a Union of its members", give: "int | str | bytes", wantForm: symbol.FormUnion,
				wantElems: []string{"int", "str", "bytes"},
			},
			{
				name: "flattens a nested Union into the union", give: "Union[int, Union[str, bytes]]",
				wantForm: symbol.FormUnion, wantElems: []string{"int", "str", "bytes"},
			},
			{
				name:      "keeps None in a union of two other members",
				give:      "int | str | None",
				wantForm:  symbol.FormUnion,
				wantElems: []string{"int", "str", "None"},
			},
			{
				name: "lowers a union of generic classes", give: "list[int] | None", wantForm: symbol.FormOptional,
				wantElems: []string{"list"},
			},
			{
				name: "lowers Callable to a Func of its parameters and its result", give: "Callable[[int, str], bool]",
				wantForm: symbol.FormFunc, wantElems: []string{"int", "str", "bool"},
			},
			{
				name: "lowers a Callable of the result None to a Func without results", give: "Callable[[int], None]",
				wantForm: symbol.FormFunc, wantElems: []string{"int"},
			},
			{
				name: "lowers a Callable of any parameters to Named", give: "Callable[..., int]",
				wantForm: symbol.FormNamed,
			},
			{
				name: "lowers tuple to a Tuple of its members", give: "tuple[int, str]", wantForm: symbol.FormTuple,
				wantElems: []string{"int", "str"},
			},
			{
				name:      "lowers Tuple of typing to a Tuple of its members",
				give:      "Tuple[int]",
				wantForm:  symbol.FormTuple,
				wantElems: []string{"int"},
			},
			{name: "lowers the empty tuple to a Tuple without members", give: "tuple[()]", wantForm: symbol.FormTuple},
			{
				name: "lowers tuple of an ellipsis to a List", give: "tuple[int, ...]", wantForm: symbol.FormList,
				wantElems: []string{"int"},
			},
			{name: "lowers a generic class to Named", give: "dict[str, int]", wantForm: symbol.FormNamed},
			{name: "lowers None to Named", give: "None", wantForm: symbol.FormNamed},
			{
				name:      "lowers a splat to a member of a Tuple",
				give:      "tuple[int, *Ts]",
				wantForm:  symbol.FormTuple,
				wantElems: []string{"int", "*Ts"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := classField(t, typingImports+"class C:\n    x: "+tt.give+"\n").Type
				assert.Equal(t, ref.Form, tt.wantForm, "the form of the reference")
				assert.Equal(t, spellings(ref.Elems), tt.wantElems, "the children of the form", assert.EquateEmpty())
			})
		}

		t.Run("lowers a tuple of an ellipsis to an immutable List", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class C:\n    x: tuple[int, ...]\n").Type
			assert.Equal(t, ref.Mutability, symbol.MutabilityImmutable, "a tuple is immutable")
		})

		t.Run("lists the None members of a union in Absent", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class C:\n    x: None | int | None\n").Type
			assert.Length(t, ref.Absent, 2, "each None member")
		})

		t.Run("records the arguments of a generic class with its spelling", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, typingImports+"class C:\n    x: t.Dict[str, list[int]]\n").Type
			expect.Equal(t, ref.Spelling, "t.Dict", "the class as written")
			expect.Equal(t, spellings(ref.Args), []string{"str", "list"}, "the arguments")
			expect.Equal(t, ref.Package, "typing", "the module that the import binds")
		})

		t.Run("records the values of Literal as written", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, typingImports+"class C:\n    x: Literal[\"a\", 1, True]\n").Type
			expect.Equal(t, ref.Spelling, "Literal", "the form")
			expect.Equal(t, spellings(ref.Args), []string{"\"a\"", "1", "True"}, "each value as written")
		})

		t.Run("lowers Annotated inside another type to the type of its argument", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, typingImports+"class C:\n    x: list[Annotated[int, \"m\"]]\n").Type
			expect.Equal(t, spellings(ref.Args), []string{"int"}, "the metadata leaves the type")
		})

		t.Run("records the module of the import of a name", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "from shop.models import Order\n\n\nclass C:\n    x: Order\n").Type
			expect.Equal(t, ref.Package, "shop/models", "the module of the import")
		})

		t.Run("records no package for a name of the module", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class Order: ...\n\n\nclass C:\n    x: Order\n").Type
			expect.Equal(t, ref.Package, "", "the module declares the name")
		})
	})

	t.Run("inString", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a string annotation to the type that it contains", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class C:\n    x: \"list[Order] | None\"\n").Type
			expect.Equal(t, ref.Form, symbol.FormOptional, "the form of the content")
			assert.NotEmpty(t, ref.Elems, "the member of the optional")
			expect.Equal(t, ref.Elems[0].Spelling, "list", "the generic class of the content")
		})

		t.Run("positions a string annotation at its content", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class C:\n    x: \"Order\"\n").Type
			expect.Equal(t, ref.Pos.Line, 2, "the line of the string")
			expect.Equal(t, ref.Pos.Col, 9, "the column after the quote")
		})

		t.Run("lowers a nested string annotation", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class C:\n    x: \"list['Order']\"\n").Type
			assert.Length(t, ref.Args, 1, "the argument of the content")
			expect.Equal(t, ref.Args[0].Spelling, "Order", "the content of the inner string")
			expect.Equal(t, ref.Args[0].Pos.Col, 15, "the column of the inner content")
		})

		t.Run("lowers a string annotation of several lines", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class C:\n    x: \"\"\"dict[\n        str, int]\"\"\"\n").Type
			expect.Equal(t, spellings(ref.Args), []string{"str", "int"}, "the arguments of the content")
			expect.Equal(t, ref.Args[0].Pos.Line, 3, "the line of the argument")
		})

		t.Run("reports UnparsedFile for a string annotation that does not parse", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "class C:\n    x: \"list[\"\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnparsedFile}, "the annotation does not parse")
			st := named[*node.Struct](t, fileOf(t, gb).Decls, "C")
			expect.Equal(t, st.Fields[0].Type.Spelling, "\"list[\"", "the type keeps the string")
		})

		t.Run("keeps a string inside a type that does not parse as the spelling of the argument", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "class C:\n    x: list[\"dict[\"]\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnparsedFile}, "the string does not parse")
			ref := named[*node.Struct](t, fileOf(t, gb).Decls, "C").Fields[0].Type
			assert.Length(t, ref.Args, 1, "the argument of the list")
			expect.Equal(t, ref.Args[0].Spelling, "\"dict[\"", "the argument keeps the string")
		})

		t.Run("reports UnparsedFile for an empty string annotation", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "class C:\n    x: \"\"\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnparsedFile}, "the empty string states no type")
		})

		t.Run("keeps an f-string as the spelling of the type", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "class C:\n    x: f\"{name}\"\n").Type
			expect.Equal(t, ref.Spelling, "f\"{name}\"", "an f-string is no constant annotation")
		})
	})

	t.Run("annotationOf", func(t *testing.T) {
		t.Parallel()

		t.Run("unwraps the qualifiers inside a string annotation", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "from typing import ClassVar\n\n\nclass C:\n    x: \"ClassVar[int]\"\n")
			expect.Equal(t, f.Level, symbol.LevelType, "the qualifier inside the string")
			expect.Equal(t, f.Type.Spelling, "int", "the type inside the qualifier")
		})

		t.Run("unwraps a bare qualifier to no type", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "from typing import ClassVar\n\n\nclass C:\n    x: ClassVar = 1\n")
			expect.Equal(t, f.Level, symbol.LevelType, "the bare qualifier")
			expect.Nil(t, f.Type, "a bare ClassVar states no type")
		})
	})
}
