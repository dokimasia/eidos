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

// TestCallable pins the functions and the methods: their signatures, their
// accessors and the marks of their decorators and bodies.
func TestCallable(t *testing.T) {
	t.Parallel()

	t.Run("function", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a function with its parameters, its result and its docstring", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def total(count: int, rate=1.5) -> float:\n"+
				"    \"\"\"The total.\"\"\"\n    return count * rate\n"), "total")
			assert.Length(t, fn.Params, 2, "the parameters of the signature")
			expect.Equal(t, fn.Params[0].Type.Spelling, "int", "the annotation of the parameter")
			expect.Equal(t, fn.Params[1].Default, "1.5", "the default as written")
			assert.Length(t, fn.Returns, 1, "one result")
			expect.Equal(t, fn.Returns[0].Type.Spelling, "float", "the return annotation")
			expect.Equal(t, fn.Doc, []string{"The total."}, "the docstring of the function")
		})

		t.Run("lowers an async function as asynchronous", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "async def load() -> int: ...\n"), "load")
			expect.True(t, fn.Async, "the result arrives asynchronously")
		})

		t.Run("lowers an async generator as a generator that is not asynchronous", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "async def stream():\n    yield 1\n")
			fn := named[*node.Function](t, fileOf(t, gb).Decls, "stream")
			expect.False(t, fn.Async, "a call returns the iterator at once")
			expect.Equal(t, stampsOn(gb, fn, python.GeneratorKey), []any{true}, "the body yields")
		})

		t.Run("stamps python.generator on a function whose yield is inside a statement", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "def pump():\n    received = yield 1\n")
			fn := named[*node.Function](t, fileOf(t, gb).Decls, "pump")
			expect.Equal(t, stampsOn(gb, fn, python.GeneratorKey), []any{true}, "the assignment yields")
		})

		t.Run("stamps nothing on a function whose nested function yields", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "def outer():\n    def inner():\n        yield 1\n    return inner\n")
			fn := named[*node.Function](t, fileOf(t, gb).Decls, "outer")
			expect.Empty(t, stampsOn(gb, fn, python.GeneratorKey), "the body of outer does not yield")
		})

		t.Run("marks a function under the decorator overload as an overload signature", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "import typing\n\n@typing.overload\ndef f(x: int) -> int: ...\ndef f(x): return x\n")
			assert.Length(t, decls, 2, "the overload and the implementation")
			overload, _ := decls[0].(*node.Function)
			implementation, _ := decls[1].(*node.Function)
			assert.NotNil(t, overload, "the overload is a function")
			assert.NotNil(t, implementation, "the implementation is a function")
			expect.True(t, overload.Overload, "the overload is a signature")
			expect.False(t, implementation.Overload, "the implementation is no signature")
			expect.Equal(t, overload.Annotations, symbol.Annotations{{Name: "typing.overload"}},
				"the decorator as written")
		})

		t.Run("takes the trailing comment on the line of the name", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "def f():  # the head\n    pass\ndef g(): ...  # the line\n")
			f, g := named[*node.Function](t, decls, "f"), named[*node.Function](t, decls, "g")
			expect.Equal(t, f.Comment, "the head", "the comment before the body")
			expect.Equal(t, g.Comment, "the line", "the comment after a body on one line")
		})
	})

	t.Run("method", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the first parameter of a method as its receiver", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "class C:\n    def get(self, key: str) -> int: ...\n", "get")
			assert.NotNil(t, m.Receiver, "the method has a receiver")
			expect.Equal(t, m.Receiver.Name, "self", "the receiver")
			assert.Length(t, m.Params, 1, "the parameters after the receiver")
		})

		t.Run("lowers a static method at type level without a receiver", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "class C:\n    @staticmethod\n    def make(x: int) -> \"C\": ...\n", "make")
			expect.Nil(t, m.Receiver, "a static method has no receiver")
			expect.Equal(t, m.Level, symbol.LevelType, "a static method belongs to the class")
			assert.Length(t, m.Params, 1, "every parameter is a parameter")
		})

		t.Run("lowers a class method at type level with its receiver", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "class C:\n    @classmethod\n    def load(cls) -> None: ...\n", "load")
			expect.Equal(t, m.Level, symbol.LevelType, "a class method belongs to the class")
			expect.Equal(t, m.Receiver.Name, "cls", "the receiver is the class")
		})

		t.Run("lowers __init__ and __new__ as methods that construct", func(t *testing.T) {
			t.Parallel()

			src := "class C:\n    def __new__(cls): ...\n    def __init__(self): ...\n"
			st := named[*node.Struct](t, declsOf(t, src), "C")
			assert.Length(t, st.Methods, 2, "both methods")
			expect.True(t, st.Methods[0].Constructs, "__new__ constructs")
			expect.Equal(t, st.Methods[0].Level, symbol.LevelType, "__new__ is static")
			expect.True(t, st.Methods[1].Constructs, "__init__ constructs")
			expect.Empty(t, st.Methods[1].Returns, "__init__ returns no result")
		})

		t.Run("lowers a property, its setter and its deleter to three accessors of one name", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class C:\n    @property\n    def name(self) -> str: ...\n"+
				"    @name.setter\n    def name(self, value: str) -> None: ...\n"+
				"    @name.deleter\n    def name(self) -> None: ...\n"), "C")
			assert.Length(t, st.Methods, 3, "each accessor is a method")
			expect.Equal(t, st.Methods[0].Accessor, symbol.AccessorGet, "the getter")
			expect.Equal(t, st.Methods[1].Accessor, symbol.AccessorSet, "the setter")
			expect.Equal(t, st.Methods[2].Accessor, symbol.AccessorDelete, "the deleter")
		})

		t.Run("lowers a getter under the decorator getter of a property", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "class C:\n    @property\n    def name(self) -> str: ...\n"+
				"    @name.getter\n    def name(self) -> str: ...\n"), "C")
			assert.Length(t, st.Methods, 2, "both accessors are methods")
			expect.Equal(t, st.Methods[1].Accessor, symbol.AccessorGet, "the getter replaces the getter")
		})

		t.Run("lowers a cached property as a getter", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "from functools import cached_property\n\n\nclass C:\n"+
				"    @cached_property\n    def size(self) -> int: ...\n", "size")
			expect.Equal(t, m.Accessor, symbol.AccessorGet, "the cached property reads the value")
		})

		t.Run("lowers the marks of the decorators of typing and abc", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "import abc\nfrom typing_extensions import final, override\n\n\n"+
				"class C(Base):\n    @abc.abstractmethod\n    def a(self) -> None: ...\n"+
				"    @final\n    def b(self) -> None: ...\n    @override\n    def c(self) -> None: ...\n"), "C")
			expect.True(t, st.Methods[0].Abstract, "abstractmethod makes the method abstract")
			expect.True(t, st.Methods[1].Final, "final forbids an override")
			expect.True(t, st.Methods[2].Override, "override replaces a method of a base")
		})

		t.Run("lowers a method of a Protocol with a body as a default", func(t *testing.T) {
			t.Parallel()

			it := named[*node.Interface](t, declsOf(t, "from typing import Protocol\n\n\nclass P(Protocol):\n"+
				"    def doc(self) -> int:\n        \"\"\"Only a docstring.\"\"\"\n"+
				"    def body(self) -> int:\n        return 1\n"), "P")
			expect.True(t, it.Methods[0].Abstract, "a docstring is no body")
			expect.False(t, it.Methods[0].HasDefault, "an abstract method has no default")
			expect.True(t, it.Methods[1].HasDefault, "a body is the default of a class that names the Protocol")
		})

		t.Run("marks a method under the decorator overload as an overload signature", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "from typing import overload\n\n\nclass C:\n"+
				"    @overload\n    def get(self, key: int) -> int: ...\n    def get(self, key): ...\n"), "C")
			assert.Length(t, st.Methods, 2, "the overload and the implementation")
			expect.True(t, st.Methods[0].Overload, "the overload is a signature")
			expect.False(t, st.Methods[1].Overload, "the implementation is no signature")
		})

		t.Run("stamps python.generator on a method whose body yields", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "class C:\n    def rows(self):\n        yield 1\n")
			st := named[*node.Struct](t, fileOf(t, gb).Decls, "C")
			assert.Length(t, st.Methods, 1, "the generator method")
			expect.Equal(t, stampsOn(gb, st.Methods[0], python.GeneratorKey), []any{true}, "the body yields")
		})

		t.Run("lowers a mangled method as private at run time", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "class C:\n    def __secret(self) -> None: ...\n", "__secret")
			expect.Equal(t, m.Visibility, symbol.VisibilityPrivate, "Python mangles the name")
			expect.True(t, m.Hard, "the mangling is a mechanism of run time")
		})

		t.Run("reports DuplicateName for a method after the accessors of a property", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "class C:\n    @property\n    def name(self) -> str: ...\n"+
				"    def name(self) -> str: ...\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "an ordinary method repeats the name")
		})
	})

	t.Run("signature", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the parameters before / by position and after * by keyword", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f(a, /, b, *, c, d=1): ...\n"), "f")
			passing := make([]symbol.Passing, 0, len(fn.Params))
			for _, p := range fn.Params {
				passing = append(passing, p.Passing)
			}
			assert.Equal(t, passing, []symbol.Passing{
				symbol.PassingPositional, symbol.PassingDefault, symbol.PassingKeyword, symbol.PassingKeyword,
			}, "the passing of each parameter")
		})

		t.Run("passes the parameters after *args by keyword", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f(*args: int, key: str, **kwargs: object): ...\n"), "f")
			assert.Length(t, fn.Params, 3, "the three parameters")
			expect.Equal(t, fn.Params[0].Variadic, symbol.VariadicPositional, "*args takes the positional arguments")
			expect.Equal(t, fn.Params[0].Type.Spelling, "int", "the type of one argument")
			expect.Equal(t, fn.Params[1].Passing, symbol.PassingKeyword, "a parameter after *args")
			expect.Equal(t, fn.Params[2].Variadic, symbol.VariadicKeyword, "**kwargs takes the keyword arguments")
		})

		t.Run("passes the leading parameters of two underscores by position", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "class C:\n    def f(self, __a, __b__, c): ...\n", "f")
			expect.Equal(t, m.Params[0].Passing, symbol.PassingPositional, "the historical form of PEP 484")
			expect.Equal(t, m.Params[1].Passing, symbol.PassingDefault, "a dunder name ends the run")
		})

		t.Run("lowers a function without a return annotation to one result without a type", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f(): pass\n"), "f")
			assert.Length(t, fn.Returns, 1, "one result")
			expect.Nil(t, fn.Returns[0].Type, "the source states no type")
		})

		t.Run("lowers -> None to no result", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f() -> None: ...\n"), "f")
			expect.Empty(t, fn.Returns, "None is no result")
		})

		t.Run("takes the types of a type comment of the signature", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "class C:\n    def f(self, a, *b):\n        # type: (int, *str) -> bool\n"+
				"        pass\n", "f")
			expect.Equal(t, spellings([]*node.TypeRef{m.Params[0].Type, m.Params[1].Type}), []string{"int", "str"},
				"the types of the parameters without the receiver")
			expect.Equal(t, m.Returns[0].Type.Spelling, "bool", "the result of the comment")
		})

		t.Run("takes the result None of a type comment as no result", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f(a):  # type: (...) -> None\n    pass\n"), "f")
			expect.Empty(t, fn.Returns, "None is no result")
			expect.Nil(t, fn.Params[0].Type, "... states no type of a parameter")
		})

		t.Run("gives a function the type variables that its signature names", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "from typing import ParamSpec, TypeVar\n\nT = TypeVar(\"T\")\n"+
				"P = ParamSpec(\"P\")\n\ndef call(f: Callable[P, T], *args: P.args) -> T: ...\n"), "call")
			names := make([]string, 0, len(fn.TypeParams))
			for _, tp := range fn.TypeParams {
				names = append(names, tp.Name)
			}
			assert.Equal(t, names, []string{"**P", "T"}, "the variables in the order of their first use")
		})

		t.Run("gives a method no type variable of its class", func(t *testing.T) {
			t.Parallel()

			m := classMethod(t, "from typing import Generic, TypeVar\n\nT = TypeVar(\"T\")\nS = TypeVar(\"S\")\n\n\n"+
				"class C(Generic[T]):\n    def f(self, a: T, b: S) -> T: ...\n", "f")
			assert.Length(t, m.TypeParams, 1, "the variable that the class does not bind")
			expect.Equal(t, m.TypeParams[0].Name, "S", "the variable of the method")
		})
	})

	t.Run("params", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnmodeledConstruct for a tuple parameter of Python 2", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "def py2((a, b), c):\n    pass\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnmodeledConstruct}, "the model has no tuple parameter")
			fn := named[*node.Function](t, fileOf(t, gb).Decls, "py2")
			assert.Length(t, fn.Params, 1, "the declaration loads without the tuple")
		})

		t.Run("reports UnmodeledConstruct for a tuple parameter of Python 2 with a default", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "def py2((a, b)=(1, 2)):\n    pass\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnmodeledConstruct}, "the model has no tuple parameter")
		})

		t.Run("stamps the metadata of an Annotated on a parameter", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from typing import Annotated\n\ndef f(x: Annotated[int, \"m\"]): ...\n")
			fn := named[*node.Function](t, fileOf(t, gb).Decls, "f")
			expect.Equal(t, stampsOn(gb, fn.Params[0], python.AnnotatedKey), []any{"\"m\""}, "the metadata as written")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a parameter", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "def f(\n    "+carrier+"    a: int,\n): ...\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnaddressedCarrier}, "no rule takes a parameter")
		})
	})

	t.Run("returns", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the metadata of an Annotated on a result", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from typing import Annotated\n\ndef f() -> Annotated[int, \"m\"]: ...\n")
			fn := named[*node.Function](t, fileOf(t, gb).Decls, "f")
			expect.Equal(t, stampsOn(gb, fn.Returns[0], python.AnnotatedKey), []any{"\"m\""}, "the metadata as written")
		})
	})
}

// classMethod parses one source at appFile and returns the method of a
// name of the class C.
func classMethod(tb testing.TB, src, name string) *node.Method {
	tb.Helper()

	st := named[*node.Struct](tb, declsOf(tb, src), "C")
	for _, m := range st.Methods {
		if m.Name == name {
			return m
		}
	}
	tb.Fatalf("the class C has no method %s", name)
	return nil
}
