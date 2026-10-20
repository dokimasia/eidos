// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of a value.
const (
	// numberAllocs is an integer literal, which Python writes as its text.
	numberAllocs = 0
	// callAllocs is a call of a function of another module with one
	// argument, in a file that imports the function already: the text of
	// the call.
	callAllocs = 1
	// lambdaAllocs is a function with one parameter and one result: the
	// list of the spelled results, and the text of the lambda.
	lambdaAllocs = 2
)

// The module of the function that the value specs call, and the name of the
// function.
const (
	utilModule = "shop/util"
	makeName   = "make"
)

// makeCall is the call make(1) of the function make of the module
// shop.util. The specs only read it.
var makeCall = emit.Call(symbol.Identity{
	Lang: python.Lang, Package: utilModule, Name: makeName, Kind: symbol.KindFunction,
}, emit.Literal(emit.LiteralInt, "1"))

// TestValue pins the spellings of the values of the neutral vocabulary in
// Python, each the value that the annotation of its type admits.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		one := emit.Literal(emit.LiteralInt, "1")
		two := emit.Literal(emit.LiteralInt, "2")
		tests := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "writes a number as its text", give: emit.Literal(emit.LiteralInt, "8"), want: "8"},
			{
				name: "writes a string in double quotes", give: emit.Literal(emit.LiteralString, "row"),
				want: `"row"`,
			},
			{
				name: "writes a string with a double quote and no single quote in single quotes",
				give: emit.Literal(emit.LiteralString, `a"b`), want: `'a"b'`,
			},
			{
				name: "writes a string with both quotes in double quotes with an escaped double quote",
				give: emit.Literal(emit.LiteralString, `a"b'c`), want: `"a\"b'c"`,
			},
			{
				name: "writes the escapes of the line breaks, the backslash, the tab and the control characters",
				give: emit.Literal(emit.LiteralString, "a\nb\\c\x01\x7f\t\r"), want: `"a\nb\\c\x01\x7f\t\r"`,
			},
			{name: "writes true as True", give: emit.Literal(emit.LiteralBool, "true"), want: "True"},
			{name: "writes false as False", give: emit.Literal(emit.LiteralBool, "false"), want: "False"},
			{name: "writes the absent value as None", give: emit.Literal(emit.LiteralNil, ""), want: "None"},
			{
				name: "writes raw Python as it is",
				give: emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralRaw, Text: "1 << 4", Lang: python.Lang},
				want: "1 << 4",
			},
			{
				name: "writes a conversion as a call of its type",
				give: emit.Value{
					Kind: emit.ValueConversion, Type: &emit.TypeRef{Spelling: "UserId"},
					Inner: &emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralInt, Text: "0"},
				},
				want: "UserId(0)",
			},
			{
				name: "writes a composite of a list as a list literal",
				give: emit.Value{
					Kind:   emit.ValueComposite,
					Type:   &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "int"}}},
					Fields: []emit.ValueField{{Value: one}, {Value: two}},
				},
				want: "[1, 2]",
			},
			{
				name: "writes a composite of an immutable list as a tuple literal",
				give: emit.Value{
					Kind: emit.ValueComposite,
					Type: &emit.TypeRef{
						Form:       symbol.FormList,
						Mutability: symbol.MutabilityImmutable,
						Elems:      []*emit.TypeRef{{Spelling: "int"}},
					},
					Fields: []emit.ValueField{{Value: one}, {Value: two}},
				},
				want: "(1, 2)",
			},
			{
				name: "writes a composite of a tuple of one element with its comma",
				give: emit.Value{
					Kind:   emit.ValueComposite,
					Type:   &emit.TypeRef{Form: symbol.FormTuple, Elems: []*emit.TypeRef{{Spelling: "int"}}},
					Fields: []emit.ValueField{{Value: one}},
				},
				want: "(1,)",
			},
			{
				name: "writes a composite of an array as a tuple literal",
				give: emit.Value{
					Kind:   emit.ValueComposite,
					Type:   &emit.TypeRef{Form: symbol.FormArray, Length: 2, Elems: []*emit.TypeRef{{Spelling: "int"}}},
					Fields: []emit.ValueField{{Value: one}, {Value: two}},
				},
				want: "(1, 2)",
			},
			{
				name: "writes a composite of a set as a set literal",
				give: emit.Value{
					Kind:   emit.ValueComposite,
					Type:   &emit.TypeRef{Spelling: "set"},
					Fields: []emit.ValueField{{Value: one}},
				},
				want: "{1}",
			},
			{
				name: "writes an empty set as a call of set",
				give: emit.Value{Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "set"}}, want: "set()",
			},
			{
				name: "writes a composite of a frozenset as a frozenset of a set literal",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "frozenset"},
					Fields: []emit.ValueField{{Value: one}},
				},
				want: "frozenset({1})",
			},
			{
				name: "writes a composite of a map as a dict literal",
				give: emit.Value{
					Kind: emit.ValueComposite,
					Type: &emit.TypeRef{
						Form:  symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: "str"}, {Spelling: "int"}},
					},
					Fields: []emit.ValueField{{
						Key: &emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralString, Text: "a"}, Value: one,
					}},
				},
				want: `{"a": 1}`,
			},
			{
				name: "writes a composite of the builtin list as a list literal",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "list"},
					Fields: []emit.ValueField{{Value: one}, {Value: two}},
				},
				want: "[1, 2]",
			},
			{
				name: "writes a composite of a Sequence as a list literal",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "Sequence"},
					Fields: []emit.ValueField{{Value: one}},
				},
				want: "[1]",
			},
			{
				name: "writes a composite of the builtin tuple as a tuple literal",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "tuple"},
					Fields: []emit.ValueField{{Value: one}, {Value: two}},
				},
				want: "(1, 2)",
			},
			{
				name: "writes a composite of a Mapping as a dict literal",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "Mapping"},
					Fields: []emit.ValueField{{
						Key: &emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralString, Text: "a"}, Value: one,
					}},
				},
				want: `{"a": 1}`,
			},
			{
				name: "writes a composite of another structural type as a call of its type",
				give: emit.Value{
					Kind: emit.ValueComposite,
					Type: &emit.TypeRef{Spelling: "&Point", Form: symbol.FormBorrow, Elems: []*emit.TypeRef{{
						Spelling: "Point",
					}}},
					Fields: []emit.ValueField{{Value: one}},
				},
				want: "Point(1)",
			},
			{
				name: "writes a composite of another type as a call with positional and keyword arguments",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "Point"},
					Fields: []emit.ValueField{{Value: one}, {Name: "y", Value: two}},
				},
				want: "Point(1, y=2)",
			},
			{
				name: "writes a call of a function",
				give: emit.Value{
					Kind: emit.ValueCall, Args: []emit.Value{one},
					Callee: symbol.Identity{Lang: python.Lang, Package: home, Name: "make", Kind: symbol.KindFunction},
				},
				want: "make(1)",
			},
			{
				name: "writes a member of a type after its type",
				give: emit.Value{Kind: emit.ValueName, Callee: symbol.Identity{
					Lang: python.Lang, Package: home, Owner: "Color", Name: "RED", Kind: symbol.KindEnumVariant,
				}},
				want: "Color.RED",
			},
			{
				name: "writes a member of a nested type after its types",
				give: emit.Value{Kind: emit.ValueName, Callee: symbol.Identity{
					Lang: python.Lang, Package: home, Owner: "Row" + symbol.OwnerSep + "Key", Name: "ID",
					Kind: symbol.KindEnumVariant,
				}},
				want: "Row.Key.ID",
			},
			{
				name: "writes a construction as a call of its type",
				give: emit.Value{
					Kind: emit.ValueConstruction, Type: &emit.TypeRef{Spelling: "Point"}, Args: []emit.Value{one, two},
				},
				want: "Point(1, 2)",
			},
			{
				name: "writes a function without results as a lambda that returns None",
				give: emit.Func(&emit.TypeRef{Spelling: "Callable[[], None]", Form: symbol.FormFunc}),
				want: "lambda: None",
			},
			{
				name: "writes a lambda with one parameter for each parameter of the function type",
				give: emit.Func(&emit.TypeRef{
					Spelling: "Callable[[int, str], bool]", Form: symbol.FormFunc, Split: 2,
					Elems: []*emit.TypeRef{{Spelling: "int"}, {Spelling: "str"}, {Spelling: "bool"}},
				}, emit.Literal(emit.LiteralBool, "true")),
				want: "lambda _0, _1: True",
			},
			{
				name: "writes the results of a function of two results as a tuple",
				give: emit.Func(&emit.TypeRef{
					Form: symbol.FormFunc, Elems: []*emit.TypeRef{{Spelling: "int"}, {Spelling: "str"}},
				}, one, emit.Literal(emit.LiteralString, "a")),
				want: `lambda: (1, "a")`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, diags := rendered(t, backend.New(), versionPolicy(t, python.Version312), &emit.Function{
					Name: "load",
					Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn, Value: emit.ValueExpr(tt.give)}}},
				})
				assert.Empty(t, diags, "the render reports nothing")
				assert.Equal(t, got, "def load() -> None:\n    return "+tt.want+"\n", "the file")
			})
		}

		t.Run(
			"writes a call of a declaration of another module under the name that its import binds",
			func(t *testing.T) {
				t.Parallel()

				got, _ := rendered(t, backend.New(), versionPolicy(t, python.Version312), &emit.Function{
					Name: "load",
					Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn, Value: emit.ValueExpr(emit.Value{
						Kind: emit.ValueConstruction, Type: &emit.TypeRef{Spelling: "datetime", Package: "datetime"},
						Args: []emit.Value{emit.Literal(emit.LiteralInt, "2023")},
					})}}},
				})
				assert.Equal(
					t,
					got,
					"from datetime import datetime\n\n\ndef load() -> None:\n    return datetime(2023)\n",
					"the file",
				)
			},
		)

		refusals := []struct {
			name string
			give emit.Value
			want string
		}{
			{
				name: "reports a composite of a type without a spelling",
				give: emit.Value{Kind: emit.ValueComposite, Type: &emit.TypeRef{}},
				want: "the type of a value has no spelling",
			},
			{
				name: "reports a number without text",
				give: emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralInt},
				want: "the int literal has no text",
			},
			{
				name: "reports a boolean that is neither true nor false", give: emit.Literal(emit.LiteralBool, "yes"),
				want: `a boolean literal is true or false, not "yes"`,
			},
			{
				name: "reports raw text of another language",
				give: emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralRaw, Text: "1 << 4", Lang: "go"},
				want: `"1 << 4" is written in go, not python`,
			},
			{
				name: "reports a literal of a kind that the vocabulary does not declare",
				give: emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralKind(200)},
				want: "no spelling for the 200 literal",
			},
			{
				name: "reports an entry of a map without a key",
				give: emit.Value{
					Kind: emit.ValueComposite,
					Type: &emit.TypeRef{
						Form:  symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: "str"}, {Spelling: "int"}},
					},
					Fields: []emit.ValueField{{Value: emit.Literal(emit.LiteralInt, "1")}},
				},
				want: "a dict[str, int] value has an entry without a key",
			},
			{
				name: "reports a keyed argument of a call",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "Point"},
					Fields: []emit.ValueField{{
						Key:   &emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralString, Text: "x"},
						Value: emit.Literal(emit.LiteralInt, "1"),
					}},
				},
				want: "a call of Point has no keyed argument",
			},
			{
				name: "reports a positional argument of a call after a keyword argument",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "Point"},
					Fields: []emit.ValueField{
						{Name: "x", Value: emit.Literal(emit.LiteralInt, "1")},
						{Value: emit.Literal(emit.LiteralInt, "2")},
					},
				},
				want: "a call of Point takes its positional arguments before its keyword arguments",
			},
			{
				name: "reports a named entry of a list",
				give: emit.Value{
					Kind:   emit.ValueComposite,
					Type:   &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "int"}}},
					Fields: []emit.ValueField{{Name: "x", Value: emit.Literal(emit.LiteralInt, "1")}},
				},
				want: "a list[int] value has a named entry, and its literal takes elements alone",
			},
			{
				name: "reports a named entry of a set",
				give: emit.Value{
					Kind: emit.ValueComposite, Type: &emit.TypeRef{Spelling: "frozenset"},
					Fields: []emit.ValueField{{Name: "x", Value: emit.Literal(emit.LiteralInt, "1")}},
				},
				want: "a frozenset value has a named entry, and its literal takes elements alone",
			},
			{
				name: "reports an address, which Python has no form for",
				give: emit.Value{Kind: emit.ValueAddress, Inner: &emit.Value{
					Kind: emit.ValueLiteral, Literal: emit.LiteralInt, Text: "1",
				}},
				want: "Python spells no address of a value",
			},
			{
				name: "reports a construction of a type that Python does not spell",
				give: emit.Value{
					Kind: emit.ValueConstruction, Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
				},
				want: "A & B is an intersection of types, which Python declares as a Protocol",
			},
			{
				name: "reports a function of a type that states no parameters",
				give: emit.Func(&emit.TypeRef{Spelling: "Handler"}),
				want: "Python writes a lambda of a function type that states its parameters, and Handler states none",
			},
			{
				name: "reports a make, which Python has no form for",
				give: emit.Make(&emit.TypeRef{Spelling: "Queue"}),
				want: "Python has no make of Queue",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, diags := rendered(t, backend.New(), versionPolicy(t, python.Version312),
					&emit.Function{Name: "load", Body: emit.Body{Stmts: []emit.Stmt{{
						Kind: emit.StmtReturn, Value: emit.ValueExpr(tt.give),
					}}}},
					&emit.Constant{Name: "LIMIT", Value: "8"},
				)
				assert.Length(t, diags, 1, "the render reports the value")
				assert.Contains(t, diags[0].Msg, tt.want, "the report names the value")
			})
		}
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a value as the scaffold writes it", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Value(makeCall, &render.ImportSet{})
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, got, makeName+"(1)", "the Python spelling")
		})

		t.Run("imports the function that a call names", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			_, err := backend.Value(makeCall, &set)
			assert.NoError(t, err, "the value spells")
			want := []render.Entry{{Path: utilModule, Name: makeName}}
			assert.Equal(t, set.Entries(), want, "the import of the function")
		})

		t.Run("returns a value error for a value that Python has no form for", func(t *testing.T) {
			t.Parallel()

			address := emit.Value{Kind: emit.ValueAddress, Inner: &makeCall}
			_, err := backend.Value(address, &render.ImportSet{})
			refused := assert.ErrorAs[*render.ValueError](t, err, "the value refuses")
			assert.Equal(t, refused.Lang, string(python.Lang), "the refusal names the target")
		})
	})
}

// A number allocates nothing, a call allocates its text, and a function
// allocates the list of its results and its text. The ordinary run, which
// runs no benchmark, checks those ceilings here.
func TestValueAllocs(t *testing.T) {
	checkAllocs(t, valueCalls())
}

// BenchmarkValue measures the spelling of one value that a template writes.
func BenchmarkValue(b *testing.B) {
	benchCalls(b, valueCalls())
}

// valueCalls returns a call of Value over an integer literal, over a call of
// a function that the file imports already, and over a function with one
// parameter and one result.
func valueCalls() []allocCall {
	number := emit.Literal(emit.LiteralInt, "42")
	fn := emit.Func(&emit.TypeRef{
		Spelling: "Callable[[int], int]", Form: symbol.FormFunc, Split: 1,
		Elems: []*emit.TypeRef{{Spelling: "int"}, {Spelling: "int"}},
	}, number)
	set := &render.ImportSet{}
	var (
		out string
		err error
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "Value spells the value")
			assert.Equal(tb, out, want, "Value writes the Python spelling")
		}
	}
	return []allocCall{
		{
			name: "Value", caseName: "an integer", allocs: numberAllocs,
			call: func() { out, err = backend.Value(number, set) }, check: spells("42"),
		},
		{
			name: "Value", caseName: "a call of another module", allocs: callAllocs,
			call: func() { out, err = backend.Value(makeCall, set) }, check: spells(makeName + "(1)"),
		},
		{
			name: "Value", caseName: "a function with one result", allocs: lambdaAllocs,
			call: func() { out, err = backend.Value(fn, set) }, check: spells("lambda _0: 42"),
		},
	}
}
