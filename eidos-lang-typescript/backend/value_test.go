// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The values of the cases refer to the modules units and time, which a
// file at the root imports as ./units and ./time. unix is a function of
// time, epoch is a constant of time, and Weight is a type of units.
const (
	unitsModule    = "units"
	timeModule     = "time"
	unitsSpecifier = "./units"
	timeSpecifier  = "./time"
	unixName       = "unix"
	epochName      = "epoch"
	weightName     = "Weight"
)

// The global array types, whose values are array literals, and the global
// set type, whose value is a construction.
const (
	arrayType         = "Array"
	readonlyArrayType = "ReadonlyArray"
	setType           = "Set"
)

// The allocations of a value.
const (
	// literalAllocs is an integer literal, which TypeScript writes as its
	// text.
	literalAllocs = 0
	// callAllocs is a call of a function of another module with one
	// argument, in a file that imports the function already: the relative
	// specifier of the module, and the text of the call. The specifier is
	// below 16 bytes and has no pointers, so a heap profile misses it.
	callAllocs = 2
	// funcAllocs is a function with one result: the list of the spelled
	// results, and the text of the arrow function.
	funcAllocs = 2
)

// lineSeparator is U+2028, which the quoting escapes.
var lineSeparator = string(rune(0x2028))

// unixCall is the call unix(1) of the function unix of the module time.
// The specs only read it.
var unixCall = emit.Call(valueFn(timeModule, unixName), emit.Literal(emit.LiteralInt, "1"))

// Every value spelling is pinned, for the reason the statement
// spellings are: the text is spliced into generated bodies, and a
// drift rewrites files.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    emit.Value
			want    string
			wantErr bool
		}{
			{name: "writes an integer as its digits", give: emit.Literal(emit.LiteralInt, "42"), want: "42"},
			{name: "writes the absent value as null", give: emit.Literal(emit.LiteralNil, ""), want: "null"},
			{name: "writes a string in single quotes", give: emit.Literal(emit.LiteralString, "hi"), want: `'hi'`},
			{
				name: "writes a string escaped in TypeScript's grammar",
				give: emit.Literal(emit.LiteralString, "it's\a \\ "+lineSeparator+"\U0001F600"),
				want: `'it\'s` + `\` + `u0007 \\ ` + `\` + `u2028` + "\U0001F600'",
			},
			{
				name: "writes control characters under their named escapes",
				give: emit.Literal(emit.LiteralString, "\b\f\n\r\t\v\x7f"+string(rune(0x2029))),
				want: `'\b\f\n\r\t\v` + `\` + `u007f` + `\` + `u2029'`,
			},
			{
				name: "writes a conversion as an assertion",
				give: emit.Conversion(valueRef(weightName, unitsModule, weightName),
					emit.Literal(emit.LiteralInt, "1")),
				want: "1 as Weight",
			},
			{
				name: "writes a record as an object literal",
				give: emit.Composite(valueRef(rowName, storePath, rowName),
					emit.NamedField("id", emit.Literal(emit.LiteralInt, "1"))),
				want: "{id: 1}",
			},
			{
				name: "writes a declared type named Array as an object literal",
				give: emit.Composite(valueRef(arrayType, storePath, arrayType),
					emit.NamedField("id", emit.Literal(emit.LiteralInt, "1"))),
				want: "{id: 1}",
			},
			{
				name: "writes a map with a computed key",
				give: emit.Composite(&emit.TypeRef{Spelling: "Record<string, number>", Form: symbol.FormMap},
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), emit.Literal(emit.LiteralInt, "2"))),
				want: `{['k']: 2}`,
			},
			{
				name: "writes a list as an array literal",
				give: emit.Composite(&emit.TypeRef{Spelling: "number[]", Form: symbol.FormList},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				want: "[2]",
			},
			{
				name: "writes a tuple as an array literal",
				give: emit.Composite(&emit.TypeRef{Spelling: "[string, number]", Form: symbol.FormTuple},
					emit.Element(emit.Literal(emit.LiteralString, "a")),
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				want: "['a', 2]",
			},
			{
				name: "writes an Array as an array literal",
				give: emit.Composite(&emit.TypeRef{Spelling: arrayType, Args: []*emit.TypeRef{{Spelling: "number"}}},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				want: "[2]",
			},
			{
				name: "writes a ReadonlyArray as an array literal",
				give: emit.Composite(&emit.TypeRef{Spelling: readonlyArrayType},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				want: "[2]",
			},
			{
				name: "writes a call under the name its import binds",
				give: emit.Call(valueFn(timeModule, unixName), emit.Literal(emit.LiteralInt, "1")),
				want: "unix(1)",
			},
			{
				name: "writes a call of a function in no module bare",
				give: emit.Call(symbol.Identity{Lang: typescript.Lang, Name: unixName}),
				want: "unix()",
			},
			{
				name: "writes a list that the substitution left without a spelling as an array literal",
				give: emit.Composite(&emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "number"}}},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				want: "[2]",
			},
			{
				name: "writes a construction as a new expression",
				give: emit.Construction(&emit.TypeRef{Spelling: setType, Args: []*emit.TypeRef{{Spelling: "string"}}},
					emit.Composite(&emit.TypeRef{Spelling: "string[]", Form: symbol.FormList},
						emit.Element(emit.Literal(emit.LiteralString, "a")))),
				want: "new Set<string>(['a'])",
			},
			{
				name: "writes a construction without arguments",
				give: emit.Construction(&emit.TypeRef{Spelling: setType}),
				want: "new Set()",
			},
			{
				name:    "returns a value error for another language's raw text",
				give:    emit.Raw("golang", "Row{}"),
				want:    "written in golang",
				wantErr: true,
			},
			{
				name:    "returns a value error for an address",
				give:    emit.Address(emit.Literal(emit.LiteralInt, "1")),
				want:    "no address",
				wantErr: true,
			},
			{
				name: "returns a value error for a positional element in an object literal",
				give: emit.Composite(valueRef(rowName, storePath, rowName),
					emit.Element(emit.Literal(emit.LiteralInt, "1"))),
				want:    "names every entry",
				wantErr: true,
			},
			{
				name: "returns a value error for a named entry in an array literal",
				give: emit.Composite(&emit.TypeRef{Spelling: "number[]", Form: symbol.FormList},
					emit.NamedField("f", emit.Literal(emit.LiteralInt, "1"))),
				want:    "takes none",
				wantErr: true,
			},
			{
				name: "writes a function without results as an arrow function with an empty body",
				give: emit.Func(&emit.TypeRef{
					Spelling: "(p0: string) => void", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{{Spelling: "string"}},
				}),
				want: "() => {}",
			},
			{
				name: "writes a function as an arrow function that returns its result",
				give: emit.Func(&emit.TypeRef{
					Spelling: "(p0: string) => number", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "number"}},
				}, emit.Literal(emit.LiteralInt, "0")),
				want: "() => 0",
			},
			{
				name: "writes an object literal that a function returns in parentheses",
				give: emit.Func(&emit.TypeRef{
					Spelling: "() => Row", Form: symbol.FormFunc,
					Elems: []*emit.TypeRef{valueRef(rowName, storePath, rowName)},
				}, emit.Composite(valueRef(rowName, storePath, rowName),
					emit.NamedField("id", emit.Literal(emit.LiteralInt, "1")))),
				want: "() => ({id: 1})",
			},
			{
				name: "writes the results of a function of two results as a tuple",
				give: emit.Func(&emit.TypeRef{
					Spelling: "() => [number, string]", Form: symbol.FormFunc,
					Elems: []*emit.TypeRef{{Spelling: "number"}, {Spelling: "string"}},
				}, emit.Literal(emit.LiteralInt, "0"), emit.Literal(emit.LiteralString, "a")),
				want: "() => [0, 'a']",
			},
			{
				name:    "returns a value error for a function of a type that is not a function type",
				give:    emit.Func(valueRef(weightName, unitsModule, weightName)),
				want:    "Weight is not one",
				wantErr: true,
			},
			{
				name: "returns a value error for a function of a constructor type",
				give: emit.Func(&emit.TypeRef{
					Spelling: "new () => Row", Form: symbol.FormFunc,
					Elems: []*emit.TypeRef{valueRef(rowName, storePath, rowName)},
				}),
				want:    "new () => Row is a constructor type",
				wantErr: true,
			},
			{
				name: "returns a value error for a make",
				give: emit.Make(&emit.TypeRef{Spelling: "AsyncIterable<string>", Form: symbol.FormStream}),
				want: "no make of AsyncIterable<string>", wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, _, err := returned(t, tt.give)
				if !tt.wantErr {
					assert.NoError(t, err, "the value spells")
					assert.Equal(t, got, tt.want, "the TypeScript spelling")
					return
				}
				refused := assert.ErrorAs[*render.ValueError](t, err, "the render reports the value's own code")
				assert.Contains(t, err.Error(), tt.want, "naming what TypeScript cannot spell")
				assert.Equal(t, refused.Lang, string(typescript.Lang), "naming the target")
			})
		}

		t.Run("imports what a conversion's type names for the type checker alone", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Conversion(
				valueRef(weightName, unitsModule, weightName), emit.Literal(emit.LiteralInt, "1")))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: unitsSpecifier, Name: weightName, TypeOnly: true}},
				"a type-only import")
		})

		t.Run("imports the class of a construction as a value", func(t *testing.T) {
			t.Parallel()

			got, set, err := returned(t, emit.Construction(valueRef(weightName, unitsModule, weightName)))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, got, "new Weight()", "the construction names the class")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: unitsSpecifier, Name: weightName}},
				"a construction needs the class at run time")
		})

		t.Run("imports a callee as a value", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Call(valueFn(timeModule, unixName)))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: timeSpecifier, Name: unixName}},
				"a call needs the function at run time")
		})

		t.Run("writes a constant under the name that its value import binds", func(t *testing.T) {
			t.Parallel()

			got, set, err := returned(t, emit.Name(symbol.Identity{
				Lang: typescript.Lang, Package: timeModule, Name: epochName, Kind: symbol.KindConstant,
			}))
			assert.NoError(t, err, "the name spells")
			assert.Equal(t, got, epochName, "the import binds the bare name")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: timeSpecifier, Name: epochName}},
				"a name needs the constant at run time")
		})

		t.Run("calls a function of the file's own module without an import", func(t *testing.T) {
			t.Parallel()

			set := &render.ImportSet{}
			set.SetHome(timeModule)
			out, err := backend.Scaffold(emit.Stmt{
				Kind: emit.StmtReturn, Value: emit.ValueExpr(emit.Call(valueFn(timeModule, unixName))),
			}, set)
			assert.NoError(t, err, "the value spells")
			assert.Contains(t, string(out), unixName+"()", "the call has the function's own name")
			assert.Equal(t, set.Len(), 0, "the file imports nothing of its own")
		})

		t.Run("renames a callee another import binds", func(t *testing.T) {
			t.Parallel()

			got, _, err := returned(t, emit.Conversion(
				valueRef(weightName, unitsModule, weightName),
				emit.Call(valueFn(timeModule, weightName))))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, got, weightName+"2() as "+weightName, "the type binds the name first")
		})

		t.Run("imports nothing for a literal", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Literal(emit.LiteralInt, "1"))
			assert.NoError(t, err, "a literal spells")
			assert.Equal(t, set.Len(), 0, "a literal names no module")
		})
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a value as the scaffold writes it", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Value(unixCall, &render.ImportSet{})
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, got, "unix(1)", "the TypeScript spelling")
		})

		t.Run("imports a callee as a value", func(t *testing.T) {
			t.Parallel()

			set := &render.ImportSet{}
			_, err := backend.Value(unixCall, set)
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: timeSpecifier, Name: unixName}},
				"a call needs the function at run time")
		})

		t.Run("returns a value error for a value that TypeScript cannot spell", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Value(emit.Address(emit.Literal(emit.LiteralInt, "1")), &render.ImportSet{})
			refused := assert.ErrorAs[*render.ValueError](t, err, "the render reports the value's own code")
			assert.Equal(t, refused.Lang, string(typescript.Lang), "naming the target")
		})
	})
}

// A literal allocates nothing, a call allocates the specifier of its
// module and its text, and a function allocates the list of its results
// and its text. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestValueAllocs(t *testing.T) {
	checkAllocs(t, valueCalls())
}

// BenchmarkValue measures the spelling of one value that a template
// writes.
func BenchmarkValue(b *testing.B) {
	benchCalls(b, valueCalls())
}

// valueCalls returns a call of Value over an integer literal, over a call
// of a function that the file imports already, and over a function with
// one result.
func valueCalls() []allocCall {
	literal := emit.Literal(emit.LiteralInt, "42")
	fn := emit.Func(&emit.TypeRef{
		Spelling: "() => number", Form: symbol.FormFunc, Elems: []*emit.TypeRef{{Spelling: "number"}},
	}, literal)
	set := &render.ImportSet{}
	var (
		out string
		err error
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "Value spells the value")
			assert.Equal(tb, out, want, "Value writes the TypeScript spelling")
		}
	}
	return []allocCall{
		{
			name: "Value", caseName: "an integer", allocs: literalAllocs,
			call: func() { out, err = backend.Value(literal, set) }, check: spells("42"),
		},
		{
			name: "Value", caseName: "a call of another module", allocs: callAllocs,
			call: func() { out, err = backend.Value(unixCall, set) }, check: spells("unix(1)"),
		},
		{
			name: "Value", caseName: "a function with one result", allocs: funcAllocs,
			call: func() { out, err = backend.Value(fn, set) }, check: spells("() => 42"),
		},
	}
}

// valueRef returns a resolved reference to a declaration of one
// module.
func valueRef(spelling, module, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target:   symbol.Identity{Lang: typescript.Lang, Package: module, Name: name, Kind: symbol.KindStruct},
	}
}

// valueFn returns a callee identity.
func valueFn(module, name string) symbol.Identity {
	return symbol.Identity{Lang: typescript.Lang, Package: module, Name: name, Kind: symbol.KindFunction}
}

// returned runs one value through the scaffold as a return and
// returns the value's text beside the file's import set.
func returned(tb assert.TB, v emit.Value) (string, *render.ImportSet, error) {
	tb.Helper()

	set := &render.ImportSet{}
	out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(v)}, set)
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(string(out)), ";"))
	return strings.TrimPrefix(text, "return "), set, err
}
