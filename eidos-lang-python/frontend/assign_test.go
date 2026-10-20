// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestAssign pins the declarations of the assignments of a module.
func TestAssign(t *testing.T) {
	t.Parallel()

	t.Run("moduleAssignment", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an annotated assignment to a variable with its type and its value", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count: int = 1\n"), "count")
			expect.Equal(t, v.Type.Spelling, "int", "the type of the annotation")
			expect.Equal(t, v.Value, "1", "the value as written")
			expect.Equal(t, v.Mutability, symbol.MutabilityMutable, "a name of a module is mutable")
		})

		t.Run("lowers an assignment without an annotation to a variable without a type", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count = 1\n"), "count")
			expect.Nil(t, v.Type, "the source states no type")
			expect.Equal(t, v.Value, "1", "the value as written")
		})

		t.Run("lowers an annotation without a value to a variable without a value", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count: int\n"), "count")
			expect.Equal(t, v.Value, "", "the source states no value")
		})

		t.Run("lowers an assignment under a bare Final to a constant without a type", func(t *testing.T) {
			t.Parallel()

			c := named[*node.Constant](t, declsOf(t, "from typing import Final\n\nMAX: Final = 3\n"), "MAX")
			expect.Nil(t, c.Type, "a bare Final states no type")
			expect.Equal(t, c.Value, "3", "the value as written")
		})

		t.Run("lowers an assignment under Final of a type to a typed constant", func(t *testing.T) {
			t.Parallel()

			c := named[*node.Constant](t, declsOf(t, "import typing\n\nMAX: typing.Final[int] = 3\n"), "MAX")
			assert.NotNil(t, c.Type, "Final states the type")
			expect.Equal(t, c.Type.Spelling, "int", "the type inside Final")
		})

		t.Run("lowers a name annotated TypeAlias to an alias of its value", func(t *testing.T) {
			t.Parallel()

			src := "from typing import TypeAlias\n\nIds: TypeAlias = \"list[int]\"\n"
			a := named[*node.Alias](t, declsOf(t, src), "Ids")
			assert.NotNil(t, a.Target, "the alias has a target")
			expect.Equal(t, a.Target.Spelling, "list", "the string annotation names list")
			expect.Equal(t, spellings(a.Target.Args), []string{"int"}, "its argument")
		})

		t.Run("takes the type of a type comment", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count = 1  # type: int\n"), "count")
			assert.NotNil(t, v.Type, "the type comment states the type")
			expect.Equal(t, v.Type.Spelling, "int", "the type of the comment")
			expect.Equal(t, v.Type.Pos.Col, 20, "the type is at its position in the comment")
		})

		t.Run("lowers each name of a chain to a variable of its value", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "a = b = 1\n")
			expect.Equal(t, named[*node.Variable](t, decls, "a").Value, "1", "the first name takes the value")
			expect.Equal(t, named[*node.Variable](t, decls, "b").Value, "1", "the second name takes the value")
		})

		t.Run("lowers each name of a tuple of targets to a variable without a value", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "a, b = 1, 2\n")
			assert.Length(t, decls, 2, "each name is a variable")
			expect.Equal(t, named[*node.Variable](t, decls, "b").Value, "", "a name of a tuple has no value")
		})

		t.Run("takes the docstring of the attribute after the assignment", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count = 1\n\"\"\"The count.\"\"\"\n"), "count")
			assert.Equal(t, v.Doc, []string{"The count."}, "the string after the assignment documents it")
		})

		t.Run("takes the trailing comment of the assignment", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count = 1  # the count\n"), "count")
			assert.Equal(t, v.Comment, "the count", "the comment after the assignment")
		})

		t.Run("attaches the carriers of the assignment to its first name", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, carrier+"a = b = 1\n")
			assert.Empty(t, codesOf(sink), "the carrier attaches")
			attachments := gb.Attachments()
			assert.Length(t, attachments, 1, "one directive attaches")
			v, _ := attachments[0].Subject.(*node.Variable)
			assert.Equal(t, v.Name, "a", "the first name is the subject")
		})

		t.Run("reports UnaddressedCarrier for an assignment that declares no name", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, carrier+"config.debug = True\n")
			assert.Equal(
				t,
				codesOf(sink),
				[]diag.Code{frontend.UnaddressedCarrier},
				"an attribute is no name of the module",
			)
		})

		t.Run("stamps the metadata of an Annotated that wraps the annotation", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from typing import Annotated\n\ncount: Annotated[int, \"positive\", 3] = 1\n")
			v := named[*node.Variable](t, fileOf(t, gb).Decls, "count")
			expect.Equal(t, v.Type.Spelling, "int", "the type inside Annotated")
			expect.Equal(t, stampsOn(gb, v, python.AnnotatedKey), []any{"\"positive\"", "3"}, "each element as written")
		})

		t.Run("stamps the whole annotation for an Annotated inside another type", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from typing import Annotated\n\nids: list[Annotated[int, \"m\"]] = []\n")
			v := named[*node.Variable](t, fileOf(t, gb).Decls, "ids")
			expect.Equal(t, stampsOn(gb, v, python.AnnotatedKey), []any{"list[Annotated[int,\"m\"]]"},
				"the annotation without its whitespace")
			expect.Equal(t, spellings(v.Type.Args), []string{"int"}, "the type of the argument inside Annotated")
		})
	})

	t.Run("implicitAlias", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a type expression under a type comment to a variable of the type", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "X = int  # type: type[int]\n"), "X")
			assert.NotNil(t, v.Type, "the type comment states the type")
			expect.Equal(t, v.Type.Spelling, "type", "the type of the comment")
		})

		t.Run("lowers a type expression of a chain to variables", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "X = Y = int\n")
			expect.Equal(t, named[*node.Variable](t, decls, "X").Value, "int", "the first name binds the class")
			expect.Equal(t, named[*node.Variable](t, decls, "Y").Value, "int", "the second name binds the class")
		})
	})

	t.Run("typeExpression", func(t *testing.T) {
		t.Parallel()

		// want is the spelling of the target of the alias X, and empty where
		// X is a variable.
		tests := []struct {
			name string
			give string
			want string
		}{
			{
				name: "lowers a subscript of a form of typing to an implicit alias",
				give: "from typing import Literal\n\nX = Literal[\"fast\", \"slow\"]\n",
				want: "Literal",
			},
			{
				name: "lowers a subscript of a class of builtins to an implicit alias",
				give: "X = list[int]\n",
				want: "list",
			},
			{
				name: "lowers a subscript of a class of collections.abc to an implicit alias",
				give: "import collections.abc\n\nX = collections.abc.Sequence[int]\n",
				want: "collections.abc.Sequence",
			},
			{
				name: "lowers a subscript of a class of collections to an implicit alias",
				give: "from collections import deque\n\nX = deque[int]\n",
				want: "deque",
			},
			{
				name: "lowers a subscript of a class that the module declares to an implicit alias",
				give: "class Box[T]:\n    pass\n\nX = Box[int]\n",
				want: "Box",
			},
			{name: "lowers a class of builtins to an implicit alias", give: "X = str\n", want: "str"},
			{
				name: "lowers a class of collections.abc to an implicit alias",
				give: "from collections.abc import Sequence\n\nX = Sequence\n",
				want: "Sequence",
			},
			{
				name: "lowers Any of typing to an implicit alias",
				give: "import typing\n\nX = typing.Any\n",
				want: "typing.Any",
			},
			{
				name: "lowers a class that the module declares to an implicit alias",
				give: "class A:\n    pass\n\nX = A\n",
				want: "A",
			},
			{
				name: "lowers an alias that the module declares to an implicit alias",
				give: "type A = int\nX = A\n",
				want: "A",
			},
			{
				name: "lowers a union with None to an implicit alias",
				give: "from shop.models import Order\n\nX = Order | None\n",
				want: "Order|None",
			},
			{
				name: "lowers a union of classes of builtins to an implicit alias",
				give: "X = int | str\n",
				want: "int|str",
			},
			{name: "lowers a function of builtins to a variable", give: "X = len\n"},
			{
				name: "lowers a subscript of a name that an import binds to a variable",
				give: "import os\n\nX = os.environ[\"HOME\"]\n",
			},
			{name: "lowers a subscript of a name that nothing binds to a variable", give: "X = row[0]\n"},
			{name: "lowers a subscript of a variable of the module to a variable", give: "rows = [1]\nX = rows[0]\n"},
			{name: "lowers a function of typing to a variable", give: "from typing import cast\n\nX = cast\n"},
			{
				name: "lowers a class that an import binds to a variable",
				give: "from shop.models import Order\n\nX = Order\n",
			},
			{name: "lowers a union of variables of the module to a variable", give: "A = 1\nB = 2\nX = A | B\n"},
			{name: "lowers a union with an operand that is no name to a variable", give: "X = int | 1\n"},
			{name: "lowers another operator to a variable", give: "X = int + str\n"},
			{name: "lowers a literal to a variable", give: "X = 1\n"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				decls := declsOf(t, tt.give)
				if tt.want == "" {
					v := named[*node.Variable](t, decls, "X")
					expect.Nil(t, v.Type, "the source states no type")
					return
				}
				a := named[*node.Alias](t, decls, "X")
				assert.NotNil(t, a.Target, "the alias has a target")
				expect.Equal(t, a.Target.Spelling, tt.want, "the value is the target")
				expect.False(t, a.Defined, "an implicit alias is no distinct type")
			})
		}
	})

	t.Run("callDeclaration", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{
				name: "stamps the variable of TypeVar with its callee",
				give: "T = TypeVar(\"T\")\n",
				want: python.TypeVar,
			},
			{
				name: "stamps the variable of ParamSpec with its callee",
				give: "T = ParamSpec(\"T\")\n",
				want: python.ParamSpec,
			},
			{
				name: "stamps the variable of TypeVarTuple with its callee",
				give: "T = TypeVarTuple(\"T\")\n",
				want: python.TypeVarTuple,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedSource(t, "from typing import ParamSpec, TypeVar, TypeVarTuple\n\n"+tt.give)
				v := named[*node.Variable](t, fileOf(t, gb).Decls, "T")
				expect.Equal(t, stampsOn(gb, v, python.TypeVarKey), []any{tt.want}, "the callee that declares it")
				expect.Equal(t, v.Value, tt.give[len("T = "):len(tt.give)-1], "the call as written")
			})
		}

		t.Run("lowers NewType to a defined alias of its type", func(t *testing.T) {
			t.Parallel()

			a := named[*node.Alias](t, declsOf(t, "from typing import NewType\n\nUserId = NewType(\"UserId\", int)\n"),
				"UserId")
			expect.True(t, a.Defined, "a NewType is a distinct type")
			expect.Equal(t, a.Target.Spelling, "int", "its base")
		})

		t.Run("lowers NewType without a base to a defined alias without a target", func(t *testing.T) {
			t.Parallel()

			src := "from typing import NewType\n\nUserId = NewType(\"UserId\")\n"
			a := named[*node.Alias](t, declsOf(t, src), "UserId")
			expect.True(t, a.Defined, "a NewType is a distinct type")
			expect.Nil(t, a.Target, "the call states no base")
		})

		t.Run("lowers the functional form of NamedTuple to a struct of immutable fields", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from typing import NamedTuple\n\n"+
				"Point = NamedTuple(\"Point\", [(\"x\", int), (\"y\", int)])\n"), "Point")
			expect.Equal(t, spellings(st.Extends), []string{"NamedTuple"}, "a named tuple extends NamedTuple")
			assert.Length(t, st.Fields, 2, "a field for each pair")
			expect.Equal(t, st.Fields[1].Name, "y", "the name of the second pair")
			expect.Equal(t, st.Fields[1].Mutability, symbol.MutabilityImmutable, "a named tuple is immutable")
		})

		t.Run("lowers the keyword form of NamedTuple to a struct of its keywords", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from typing import NamedTuple\n\nPoint = NamedTuple(\"Point\", x=int)\n")
			st := named[*node.Struct](t, fileOf(t, gb).Decls, "Point")
			assert.Length(t, st.Fields, 1, "a field for each keyword")
			expect.Empty(t, stampsOn(gb, st, python.KeywordsKey), "the keywords are the fields")
		})

		t.Run("lowers the functional form of TypedDict to an interface of its keys", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from typing import TypedDict\n\n"+
				"Movie = TypedDict(\"Movie\", {\"name\": str, \"year\": int}, total=False)\n")
			it := named[*node.Interface](t, fileOf(t, gb).Decls, "Movie")
			assert.Length(t, it.Fields, 2, "a field for each key")
			expect.True(t, it.Fields[0].Optional, "a key of a TypedDict that is not total is optional")
			expect.Equal(t, stampsOn(gb, it, python.TypedDictKey), []any{true}, "the interface is a TypedDict")
			expect.Equal(t, stampsOn(gb, it, python.KeywordsKey), []any{"total=False"}, "the keyword as written")
		})

		t.Run("lowers the names of the functional form of an enum to its variants", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from enum import Enum\n\nColor = Enum(\"Color\", \"RED GREEN, BLUE\")\n")
			e := named[*node.Enum](t, fileOf(t, gb).Decls, "Color")
			want := []string{"RED", "GREEN", "BLUE"}
			expect.Equal(t, variantNames(e), want, "the names separated by spaces or commas")
			expect.Equal(t, stampsOn(gb, e, python.BasesKey), []any{"Enum"}, "the class of enum as written")
		})

		t.Run("lowers the pairs of the functional form of an enum with their values", func(t *testing.T) {
			t.Parallel()

			src := "import enum\n\nColor = enum.IntEnum(\"Color\", [(\"RED\", 1)])\n"
			e := named[*node.Enum](t, declsOf(t, src), "Color")
			assert.Length(t, e.Variants, 1, "a variant for each pair")
			expect.Equal(t, e.Variants[0].Value, "1", "the value as written")
			expect.Equal(t, e.Underlying.Spelling, "int", "IntEnum has members of int")
		})

		t.Run("lowers a dictionary and a list of the functional form of an enum", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "from enum import Enum\n\nA = Enum(\"A\", {\"X\": 1})\nB = Enum(\"B\", [\"Y\"])\n")
			expect.Equal(t, named[*node.Enum](t, decls, "A").Variants[0].Value, "1", "the value of the pair")
			expect.Equal(t, variantNames(named[*node.Enum](t, decls, "B")), []string{"Y"}, "the name of the list")
		})

		t.Run("lowers a call of another callee to a variable of the call", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "registry = make_registry()\n"), "registry")
			expect.Equal(t, v.Value, "make_registry()", "the call as written")
		})
	})

	t.Run("typeAlias", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a type statement to an alias with its type parameters", func(t *testing.T) {
			t.Parallel()

			a := named[*node.Alias](t, declsOf(t, "type Pair[K, V = int] = tuple[K, V]\n"), "Pair")
			assert.Length(t, a.TypeParams, 2, "the type parameters of the statement")
			expect.Equal(t, a.TypeParams[1].Default.Spelling, "int", "the default of the second parameter")
			expect.Equal(t, a.Target.Form, symbol.FormTuple, "the target is a tuple")
		})

		t.Run("lowers a type statement without type parameters", func(t *testing.T) {
			t.Parallel()

			a := named[*node.Alias](t, declsOf(t, "type Ids = list[int]\n"), "Ids")
			expect.Empty(t, a.TypeParams, "the statement has no type parameter")
			expect.Equal(t, a.Target.Spelling, "list", "the target")
		})
	})

	t.Run("alias", func(t *testing.T) {
		t.Parallel()

		t.Run("gives an alias the type variables that its target names", func(t *testing.T) {
			t.Parallel()

			a := named[*node.Alias](t, declsOf(t, "from typing import TypeAlias, TypeVar\n\nT = TypeVar(\"T\")\n"+
				"Items: TypeAlias = list[T]\n"), "Items")
			assert.Length(t, a.TypeParams, 1, "the type variable of the target")
			expect.Equal(t, a.TypeParams[0].Name, "T", "the name of the type variable")
		})

		t.Run("reports DuplicateName for an alias of a declared name", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "Ids = 1\ntype Ids = list[int]\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the second declaration reports")
		})
	})

	t.Run("addAll", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the names of __all__ in every branch and every operand", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "if DEBUG:\n    __all__ = [\"a\"] + [\"b\"]\nelse:\n    __all__ = (\"c\")\n")
			exports := fileOf(t, gb).Exports
			assert.Length(t, exports, 1, "the export of __all__")
			assert.Equal(t, bindingNames(exports[0]), []string{"a", "b", "c"}, "the names of every branch")
		})
	})
}
