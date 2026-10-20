// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestClass pins the kinds of the classes and their members.
func TestClass(t *testing.T) {
	t.Parallel()

	t.Run("class", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a class to a struct with its bases in Extends", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class Base: ...\n\n\nclass Order(Base, dict):\n"+
				"    \"\"\"An order.\"\"\"\n"), "Order")
			expect.Equal(t, spellings(st.Extends), []string{"Base", "dict"}, "the bases in the order of the statement")
			expect.Equal(t, st.Doc, []string{"An order."}, "the docstring of the class")
		})

		t.Run("lowers a class with the base Protocol to an interface without the base", func(t *testing.T) {
			t.Parallel()

			src := "from typing import Protocol\n\n\nclass Shape(Sized, Protocol):\n    def area(self) -> float: ...\n"
			it := named[*node.Interface](t, declsOf(t, src), "Shape")
			expect.Equal(t, spellings(it.Extends), []string{"Sized"}, "the other bases")
			assert.Length(t, it.Methods, 1, "the method of the protocol")
			expect.True(t, it.Methods[0].Abstract, "a method without a body is abstract")
		})

		t.Run("lowers a class with the base TypedDict to an interface stamped python.typedDict", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from typing import NotRequired, ReadOnly, Required, TypedDict\n\n\n"+
				"class Movie(TypedDict, total=False):\n    name: Required[str]\n    year: NotRequired[int]\n"+
				"    rating: ReadOnly[float]\n")
			it := named[*node.Interface](t, fileOf(t, gb).Decls, "Movie")
			expect.Equal(t, stampsOn(gb, it, python.TypedDictKey), []any{true}, "the interface is a TypedDict")
			expect.Equal(t, stampsOn(gb, it, python.KeywordsKey), []any{"total=False"}, "the keywords of the statement")
			assert.Length(t, it.Fields, 3, "a field for each key")
			expect.False(t, it.Fields[0].Optional, "Required keeps a key of a TypedDict that is not total")
			expect.True(t, it.Fields[1].Optional, "NotRequired makes a key optional")
			expect.Equal(t, it.Fields[2].Mutability, symbol.MutabilityImmutable, "ReadOnly makes a key immutable")
			expect.Empty(t, it.Extends, "TypedDict is no supertype")
		})

		t.Run("lowers a class with the base NamedTuple to a struct of immutable fields", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from typing import NamedTuple\n\n\nclass Point(NamedTuple):\n"+
				"    x: int\n    y: int = 0\n"), "Point")
			expect.Equal(t, spellings(st.Extends), []string{"NamedTuple"}, "a named tuple extends NamedTuple")
			expect.Equal(t, st.Fields[1].Value, "0", "the default of the field")
			expect.Equal(t, st.Fields[0].Mutability, symbol.MutabilityImmutable, "a named tuple is immutable")
		})

		t.Run("lowers a class with a base of enum to an enum with its members", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from enum import StrEnum, auto\n\n\nclass Kind(StrEnum):\n    A = \"a\"\n"+
				"    B = auto()\n    _ignore_ = [\"C\"]\n    C = 3\n\n    def label(self) -> str: ...\n")
			e := named[*node.Enum](t, fileOf(t, gb).Decls, "Kind")
			expect.Equal(t, variantNames(e), []string{"A", "B"}, "the members that the typing specification counts")
			expect.Equal(t, e.Variants[1].Value, "auto()", "the value as written")
			expect.Equal(t, e.Underlying.Spelling, "str", "StrEnum has members of str")
			expect.Equal(t, stampsOn(gb, e, python.BasesKey), []any{"StrEnum"}, "the bases as written")
			assert.Length(t, e.Methods, 1, "the method of the enum")
			assert.Length(t, e.Fields, 1, "the attribute that _ignore_ lists is a field")
		})

		t.Run("gives an enum the mixin type of its bases as its underlying type", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "import enum\n\n\nclass Flags(int, enum.Enum, boundary=STRICT):\n"+
				"    X = 1\n"), "Flags")
			assert.NotNil(t, e.Underlying, "the mixin is the data type")
			expect.Equal(t, e.Underlying.Spelling, "int", "the mixin type")
		})

		t.Run("lowers a class with a metaclass of enum to an enum", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "from enum import EnumMeta\n\n\nclass Color(metaclass=EnumMeta):\n"+
				"    RED = 1\n"), "Color")
			expect.Nil(t, e.Underlying, "the metaclass states no data type")
		})

		t.Run("lowers a subclass of an enum of the module to an enum", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "from enum import Enum\n\n\nclass Base(Enum):\n    pass\n\n\nclass Color(Base):\n"+
				"    RED = 1\n")
			color := named[*node.Enum](t, decls, "Color")
			expect.Equal(t, variantNames(color), []string{"RED"}, "the subclass is an enum")
		})

		t.Run("lowers a subclass of an enum of another module to a struct", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from shop.kinds import Base\n\n\nclass Color(Base):\n"+
				"    RED = 1\n"), "Color")
			expect.Equal(t, spellings(st.Extends), []string{"Base"}, "the parse reads one module")
		})

		t.Run("stamps the unpacked arguments of a class statement as keywords", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "class C(*bases, **options):\n    pass\n")
			st := named[*node.Struct](t, fileOf(t, gb).Decls, "C")
			got := stampsOn(gb, st, python.KeywordsKey)
			expect.Equal(t, got, []any{"*bases", "**options"}, "each unpacking as written")
			expect.Empty(t, st.Extends, "an unpacking names no base")
		})

		t.Run("gives a class the type variables of Generic", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from typing import Generic, TypeVar\n\nK = TypeVar(\"K\")\n"+
				"V = TypeVar(\"V\")\n\n\nclass Box(Base[V], Generic[K, V]):\n    pass\n"), "Box")
			assert.Length(t, st.TypeParams, 2, "the type variables of Generic")
			expect.Equal(t, st.TypeParams[0].Name, "K", "in the order of Generic")
			expect.Equal(t, spellings(st.Extends), []string{"Base"}, "Generic is no supertype")
		})

		t.Run("gives a class without Generic the type variables of its bases", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from typing import TypeVar\n\nT = TypeVar(\"T\")\n\n\n"+
				"class Box(list[T]):\n    pass\n"), "Box")
			assert.Length(t, st.TypeParams, 1, "the type variable of the base")
		})

		t.Run("gives a class the type parameters of the syntax of PEP 695", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class Box[T: int = bool]:\n    item: T\n"), "Box")
			assert.Length(t, st.TypeParams, 1, "the type parameter of the statement")
			expect.Equal(t, spellings(st.TypeParams[0].Bounds), []string{"int"}, "its bound")
			expect.Equal(t, st.TypeParams[0].Default.Spelling, "bool", "its default")
		})

		t.Run("lowers a struct with an abstract method as abstract", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from abc import ABC, abstractmethod\n\n\nclass Base(ABC):\n"+
				"    @abstractmethod\n    def run(self) -> None: ...\n"), "Base")
			expect.True(t, st.Abstract, "a type checker refuses to instantiate the class")
		})

		t.Run("lowers a class under final as final", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from typing import final\n\n\n@final\nclass Leaf: ...\n"), "Leaf")
			expect.True(t, st.Final, "the class cannot be subclassed")
			expect.Equal(t, st.Annotations, symbol.Annotations{{Name: "final"}}, "the decorator is an annotation")
		})

		t.Run("lowers a nested class to a declaration of the class", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class Outer:\n    class Inner:\n        x: int\n"), "Outer")
			assert.Length(t, st.Types, 1, "the nested class")
			inner, _ := st.Types[0].(*node.Struct)
			expect.Equal(t, inner.Name, "Inner", "the name of the nested class")
		})

		tests := []struct {
			name string
			give string
			want []diag.Code
		}{
			{
				name: "reports UnmodeledConstruct for a class nested in an enum",
				give: "    class Inner: ...\n",
				want: []diag.Code{frontend.UnmodeledConstruct},
			},
			{
				name: "reports UnaddressedCarrier for a carrier of a class nested in an enum",
				give: "    " + carrier + "    class Inner: ...\n",
				want: []diag.Code{frontend.UnmodeledConstruct, frontend.UnaddressedCarrier},
			},
			{
				name: "reports UnaddressedCarrier for a decorator of the brand on a class nested in an enum",
				give: "    @fixture.gen.table\n    class Inner: ...\n",
				want: []diag.Code{frontend.UnmodeledConstruct, frontend.UnaddressedCarrier},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := parsedSource(t, "from enum import Enum\n\n\nclass Color(Enum):\n    RED = 1\n\n"+tt.give)
				assert.Equal(t, codesOf(sink), tt.want, "an enum of the model has no types")
			})
		}
	})

	t.Run("classAssignment", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an annotated attribute to a field with its default", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "class C:\n    count: int = 0\n")
			expect.Equal(t, f.Type.Spelling, "int", "the type of the annotation")
			expect.Equal(t, f.Value, "0", "the default as written")
			expect.Equal(t, f.Level, symbol.LevelInstance, "an annotated attribute belongs to an instance")
		})

		t.Run("lowers an attribute under ClassVar to a field at type level", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "from typing import ClassVar\n\n\nclass C:\n    count: ClassVar[int] = 0\n")
			expect.Equal(t, f.Level, symbol.LevelType, "ClassVar shares the attribute")
			expect.Equal(t, f.Type.Spelling, "int", "the type inside ClassVar")
		})

		t.Run("lowers an attribute under Final to an immutable field", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "from typing import Final\n\n\nclass C:\n    count: Final[int] = 0\n")
			expect.Equal(t, f.Mutability, symbol.MutabilityImmutable, "Final fixes the attribute")
		})

		t.Run("lowers an attribute without an annotation to a field at type level", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "class C:\n    __slots__ = (\"x\",)\n")
			expect.Equal(t, f.Level, symbol.LevelType, "the instances share the attribute")
			expect.Nil(t, f.Type, "the source states no type")
			expect.Equal(t, f.Value, "(\"x\",)", "the value as written")
		})

		t.Run("stamps python.initVar on a field of InitVar", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from dataclasses import InitVar\n\n\nclass C:\n    seed: InitVar[int]\n")
			st := named[*node.Struct](t, fileOf(t, gb).Decls, "C")
			expect.Equal(t, stampsOn(gb, st.Fields[0], python.InitVarKey), []any{true}, "the instance keeps no seed")
		})

		t.Run("takes the docstring and the trailing comment of an attribute", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "class C:\n    count: int  # the count\n    \"\"\"The count.\"\"\"\n")
			expect.Equal(t, f.Doc, []string{"The count."}, "the docstring of the attribute")
			expect.Equal(t, f.Comment, "the count", "the trailing comment")
		})

		t.Run("reports DuplicateName for a second declaration of a member", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "class C:\n    x: int\n    x: str\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the second declaration reports")
		})

		t.Run("takes the type of the type comment of an attribute", func(t *testing.T) {
			t.Parallel()

			f := classField(t, "class C:\n    ids = []  # type: list[int]\n")
			assert.NotNil(t, f.Type, "the type comment states the type")
			expect.Equal(t, f.Type.Spelling, "list", "the type of the comment")
		})

		t.Run("lowers each name of a chain to a field", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class C:\n    a = b = 1\n"), "C")
			assert.Length(t, st.Fields, 2, "a field for each name of the chain")
			expect.Equal(t, st.Fields[1].Name, "b", "the second name")
		})

		t.Run("lowers an attribute annotated TypeAlias to an alias of the class", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from typing import TypeAlias\n\n\nclass C:\n"+
				"    Ids: TypeAlias = list[int]\n"), "C")
			expect.Empty(t, st.Fields, "an alias is no field")
			assert.Length(t, st.Types, 1, "the alias is a type of the class")
			alias, _ := st.Types[0].(*node.Alias)
			assert.NotNil(t, alias, "the type is an alias")
			expect.Equal(t, alias.Target.Spelling, "list", "the target of the alias")
		})

		t.Run("lowers an attribute whose value is a type expression to an alias of the class", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class C:\n    Ids = list[int]\n"), "C")
			expect.Empty(t, st.Fields, "an implicit alias is no field")
			assert.Length(t, st.Types, 1, "the alias is a type of the class")
			alias, _ := st.Types[0].(*node.Alias)
			assert.NotNil(t, alias, "the type is an alias")
			expect.Equal(t, alias.Target.Spelling, "list", "the target of the alias")
		})

		t.Run("lowers an attribute of an enum whose value is a type expression to a field", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "from enum import Enum\n\n\nclass E(Enum):\n    __kind = int\n"), "E")
			assert.Length(t, e.Fields, 1, "a private name of an enum is no member")
			expect.Equal(t, e.Fields[0].Name, "__kind", "an enum declares no aliases")
		})

		nonmembers := []struct {
			name string
			give string
			want string
		}{
			{
				name: "lowers an attribute of an enum that nonmember wraps to a field of the value inside it",
				give: "count = nonmember(0)", want: "0",
			},
			{
				name: "lowers the value of a cast inside nonmember to the value of the field",
				give: "count = nonmember(cast(\"int\", 0))", want: "0",
			},
			{
				name: "lowers a call inside nonmember that is no cast to the value of the field",
				give: "count = nonmember(start(0))", want: "start(0)",
			},
		}
		for _, tt := range nonmembers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				e := named[*node.Enum](t, declsOf(t, "from enum import Enum, nonmember\nfrom typing import cast\n\n\n"+
					"class Kind(Enum):\n    A = 1\n    "+tt.give+"\n"), "Kind")
				assert.Length(t, e.Fields, 1, "an attribute that nonmember wraps is no member")
				expect.Equal(t, e.Fields[0].Value, tt.want, "the value inside nonmember")
			})
		}

		t.Run("takes the type of a cast inside nonmember for an attribute of an enum", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "from enum import Enum, nonmember\nfrom typing import cast\n\n\n"+
				"class Kind(Enum):\n    A = 1\n    count = nonmember(cast(\"int\", 0))\n"), "Kind")
			assert.Length(t, e.Fields, 1, "an attribute that nonmember wraps is no member")
			assert.NotNil(t, e.Fields[0].Type, "the cast states the type")
			expect.Equal(t, e.Fields[0].Type.Spelling, "int", "the type that the string of the cast contains")
		})

		t.Run("leaves out a member that is not public at signature depth", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				projectFile: {Data: []byte(rootProject)},
				appFile:     {Data: []byte("class C:\n    _x: int\n    y: int\n")},
			}, plugin.DepthSignatures)
			st := named[*node.Struct](t, fileOf(t, gbs[0]).Decls, "C")
			assert.Length(t, st.Fields, 1, "the protected field is left out")
		})
	})

	t.Run("classStatements", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the attributes of every clause of a try statement", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class C:\n    try:\n        x: int\n"+
				"    except ImportError:\n        y: int\n"), "C")
			assert.Length(t, st.Fields, 2, "every clause declares")
			expect.Equal(t, st.Fields[1].Name, "y", "the attribute of the except clause")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a statement that declares nothing", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "class C:\n    "+carrier+"    print(1)\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnaddressedCarrier}, "a call declares no member")
		})
	})

	t.Run("ignored", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the names of _ignore_ in a string", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "from enum import Enum\n\n\nclass E(Enum):\n"+
				"    _ignore_ = \"B C\"\n    A = 1\n    B = 2\n"), "E")
			expect.Equal(t, variantNames(e), []string{"A"}, "the names that _ignore_ lists are no members")
		})
	})

	t.Run("variant", func(t *testing.T) {
		t.Parallel()

		t.Run("reports DuplicateName for a second member of a name", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "from enum import Enum\n\n\nclass E(Enum):\n    A = 1\n    A = 2\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the second member reports")
		})
	})

	t.Run("functionalClass", func(t *testing.T) {
		t.Parallel()

		t.Run("reports DuplicateName for a functional class of a declared name", func(t *testing.T) {
			t.Parallel()

			src := "from typing import NamedTuple\n\nP = 1\nP = NamedTuple(\"P\", [(\"x\", int)])\n"
			_, sink := parsedSource(t, src)
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the second declaration reports")
		})

		t.Run("leaves out a functional class that is not public at signature depth", func(t *testing.T) {
			t.Parallel()

			src := "from typing import NamedTuple\n\n_P = NamedTuple(\"_P\", [(\"x\", int)])\n"
			gbs, _ := parsedTree(t, fstest.MapFS{
				projectFile: {Data: []byte(rootProject)},
				appFile:     {Data: []byte(src)},
			}, plugin.DepthSignatures)
			expect.Empty(t, fileOf(t, gbs[0]).Decls, "the private class is left out")
		})

		t.Run("lowers namedtuple of collections to a struct that extends NamedTuple of typing", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "import collections\n\n"+
				"Point = collections.namedtuple(\"Point\", \"x, y\")\n"), "Point")
			assert.Length(t, st.Extends, 1, "a named tuple has one base")
			expect.Equal(t, st.Extends[0].Target,
				symbol.Identity{Lang: python.Lang, Package: "typing", Name: "NamedTuple", Kind: symbol.KindStruct},
				"the base is NamedTuple of typing")
			assert.Length(t, st.Fields, 2, "a field for each name of the string")
			expect.Nil(t, st.Fields[1].Type, "a field of namedtuple has no type")
			expect.Equal(t, st.Fields[1].Mutability, symbol.MutabilityImmutable, "a named tuple is immutable")
		})

		t.Run("lowers the defaults of namedtuple of collections onto the last fields", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from collections import namedtuple\n\n"+
				"Point = namedtuple(\"Point\", [\"x\", \"y\", \"z\"], defaults=[0, 1])\n"), "Point")
			assert.Length(t, st.Fields, 3, "a field for each string of the list")
			expect.Empty(t, st.Fields[0].Value, "the first field has no default")
			expect.Equal(t, st.Fields[2].Value, "1", "the last field takes the last default")
		})

		t.Run("sets no default for more defaults than fields", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from collections import namedtuple\n\n"+
				"Point = namedtuple(\"Point\", field_names=\"x\", defaults=(0, 1))\n"), "Point")
			assert.Length(t, st.Fields, 1, "a field for the name of the keyword")
			expect.Empty(t, st.Fields[0].Value, "namedtuple refuses more defaults than fields")
		})

		t.Run("reports DuplicateName for a name that namedtuple of collections repeats", func(t *testing.T) {
			t.Parallel()

			src := "from collections import namedtuple\n\nPoint = namedtuple(\"Point\", \"x x\")\n"
			gb, sink := parsedSource(t, src)
			st := named[*node.Struct](t, fileOf(t, gb).Decls, "Point")
			assert.Length(t, st.Fields, 1, "one field of the name")
			expect.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the second name reports")
		})

		t.Run("renames the fields of namedtuple of collections under rename", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from collections import namedtuple\n\n"+
				"Row = namedtuple(\"Row\", [\"abc\", \"def\", \"ghi\", \"abc\", \"_x\", \"1st\"], rename=True)\n"), "Row")
			names := make([]string, 0, len(st.Fields))
			for _, f := range st.Fields {
				names = append(names, f.Name)
			}
			expect.Equal(t, names, []string{"abc", "_1", "ghi", "_3", "_4", "_5"},
				"a keyword, a repeated name, a leading underscore and a name that is no identifier take their index")
		})
	})

	t.Run("member", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the first field of a name that a functional form repeats", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "from typing import NamedTuple\n\n"+
				"P = NamedTuple(\"P\", [(\"x\", int), (\"x\", str)])\n")
			st := named[*node.Struct](t, fileOf(t, gb).Decls, "P")
			assert.Length(t, st.Fields, 1, "one field of the name")
			expect.Equal(t, st.Fields[0].Type.Spelling, "int", "the type of the first field")
			expect.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the second field reports")
		})
	})

	t.Run("selfFields", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the assignments to the receiver in __init__ to fields", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class C:\n    x: int\n\n    def __init__(self, a):\n"+
				"        self.x = a\n        self.y = a\n        if a:\n            self.z: int = a\n"), "C")
			names := make([]string, 0, len(st.Fields))
			for _, f := range st.Fields {
				names = append(names, f.Name)
			}
			assert.Equal(t, names, []string{"x", "y", "z"}, "each name that the class body does not declare")
			expect.Equal(t, st.Fields[2].Type.Spelling, "int", "the annotation of the assignment")
		})

		t.Run("lowers the annotated assignments to the receiver of another method alone", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class C:\n    def load(this):\n        this.a = 1\n"+
				"        this.b: str = \"\"\n"), "C")
			assert.Length(t, st.Fields, 1, "the annotated assignment declares a field")
			expect.Equal(t, st.Fields[0].Name, "b", "the name of the annotated assignment")
		})
	})
}
