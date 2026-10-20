// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The value fixture: the modules the values name and a function of
// one of them.
const (
	unitsModule = "example/units"
	utilModule  = "example/util"
	makeName    = "make"
	weightName  = "Weight"
)

// The allocations of a value.
const (
	// literalAllocs is an integer literal, which Rust writes as its text.
	literalAllocs = 0
	// callAllocs is a call of a function of another module with one
	// argument, in a file that uses the function already: the text of the
	// call.
	callAllocs = 1
	// funcAllocs is a function with one parameter and one result: the list
	// of the spelled results, and the text of the closure.
	funcAllocs = 2
)

// makeCall is the call make(1) of the function make of the module util.
// The specs only read it.
var makeCall = emit.Call(valueFn(utilModule, makeName), emit.Literal(emit.LiteralInt, "1"))

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
			{name: "writes an integer as its digits", give: integer("42"), want: "42"},
			{name: "writes a float without a fraction with one added", give: float("0"), want: "0.0"},
			{name: "writes a float with a fraction as written", give: float("1.5"), want: "1.5"},
			{name: "writes a float with an exponent as written", give: float("1e5"), want: "1e5"},
			{name: "writes the absent value as None", give: emit.Literal(emit.LiteralNil, ""), want: "None"},
			{
				name: "writes a string escaped in Rust's grammar",
				give: emit.Literal(emit.LiteralString, "bell\a nul\x00 \\ \"q\" é"),
				want: `"bell\` + `u{7} nul\0 \\ \"q\" é"`,
			},
			{
				name: "writes control characters under their named escapes",
				give: emit.Literal(emit.LiteralString, "\n\r\t\x7f"),
				want: `"\n\r\t\` + `u{7f}"`,
			},
			{
				name: "writes a conversion as the tuple struct a defined type is",
				give: emit.Conversion(valueRef(weightName, unitsModule, weightName), float("1.5")),
				want: "Weight(1.5)",
			},
			{
				name: "writes a generic tuple struct behind the turbofish",
				give: emit.Conversion(generic("Wrapper", ref("i32")), integer("1")),
				want: "Wrapper::<i32>(1)",
			},
			{
				name: "writes a conversion to a numeric primitive as a cast",
				give: emit.Conversion(ref("u8"), integer("5")),
				want: "5 as u8",
			},
			{
				name: "writes a borrowed cast in parentheses",
				give: emit.Address(emit.Conversion(ref("f64"), integer("5"))),
				want: "&(5 as f64)",
			},
			{
				name: "writes a struct literal with named fields",
				give: emit.Composite(valueRef(rowName, storeModule, rowName), emit.NamedField("id", integer("1"))),
				want: "Row { id: 1 }",
			},
			{
				name: "writes a generic struct literal behind the turbofish",
				give: emit.Composite(generic("Pair", generic("Vec", ref("i32"))),
					emit.NamedField("left", integer("1"))),
				want: "Pair::<Vec<i32>> { left: 1 }",
			},
			{
				name: "writes a struct literal without fields on empty braces",
				give: emit.Composite(valueRef(rowName, storeModule, rowName)),
				want: "Row {}",
			},
			{
				name: "writes a list as the vector macro",
				give: emit.Composite(&emit.TypeRef{Spelling: "Vec<i64>", Form: symbol.FormList},
					emit.Element(integer("2"))),
				want: "vec![2]",
			},
			{
				name: "writes an array in brackets",
				give: emit.Composite(&emit.TypeRef{Spelling: "[i64; 1]", Form: symbol.FormArray},
					emit.Element(integer("2"))),
				want: "[2]",
			},
			{name: "writes an address as a borrow", give: emit.Address(integer("1")), want: "&1"},
			{
				name: "writes a call under the name its use binds",
				give: emit.Call(valueFn(utilModule, makeName), integer("1")),
				want: "make(1)",
			},
			{
				name: "writes a call of a function in no module bare",
				give: emit.Call(symbol.Identity{Lang: rust.Lang, Name: makeName}),
				want: "make()",
			},
			{
				name: "returns a value error for another language's raw text",
				give: emit.Raw("golang", "Row{}"), want: "written in golang", wantErr: true,
			},
			{
				name: "returns a value error for a map",
				give: emit.Composite(&emit.TypeRef{Spelling: "HashMap<String, i64>", Form: symbol.FormMap},
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), integer("2"))),
				want: "no map literal", wantErr: true,
			},
			{
				name: "returns a value error for a positional element in a struct literal",
				give: emit.Composite(valueRef(rowName, storeModule, rowName), emit.Element(integer("1"))),
				want: "names every field", wantErr: true,
			},
			{
				name: "returns a value error for a conversion to String",
				give: emit.Conversion(ref("String"), emit.Literal(emit.LiteralString, "a")),
				want: "through a trait or a method", wantErr: true,
			},
			{
				name: "returns a value error for a type argument that states no type",
				give: emit.Composite(&emit.TypeRef{Spelling: "Pair", Args: []*emit.TypeRef{nil}},
					emit.NamedField("left", integer("1"))),
				want: "states no type", wantErr: true,
			},
			{
				name: "returns a value error for a list type without a spelling",
				give: emit.Composite(&emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{ref("i64")}},
					emit.Element(integer("2"))),
				want: "states no type", wantErr: true,
			},
			{
				name: "returns a value error for a construction",
				give: emit.Construction(valueRef(rowName, storeModule, rowName)),
				want: "has no construction of Row", wantErr: true,
			},
			{
				name: "writes a function as a closure that returns its result",
				give: emit.Func(&emit.TypeRef{
					Spelling: "fn(i64) -> i64", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{ref("i64"), ref("i64")},
				}, integer("0")),
				want: "|_| 0",
			},
			{
				name: "writes a function without parameters as a closure without parameters",
				give: emit.Func(&emit.TypeRef{
					Spelling: "fn() -> i64", Form: symbol.FormFunc, Elems: []*emit.TypeRef{ref("i64")},
				}, integer("0")),
				want: "|| 0",
			},
			{
				name: "writes a function without results as a closure with an empty block",
				give: emit.Func(&emit.TypeRef{
					Spelling: "fn(i64, i64)", Form: symbol.FormFunc, Split: 2,
					Elems: []*emit.TypeRef{ref("i64"), ref("i64")},
				}),
				want: "|_, _| {}",
			},
			{
				name: "writes the results of a function of two results as a tuple",
				give: emit.Func(&emit.TypeRef{
					Spelling: "dyn Fn() -> (i64, i64)", Form: symbol.FormFunc,
					Elems: []*emit.TypeRef{ref("i64"), ref("i64")},
				}, integer("0"), integer("1")),
				want: "|| (0, 1)",
			},
			{
				name: "returns a value error for a function of a type that is not a function type",
				give: emit.Func(generic("Box", &emit.TypeRef{Spelling: "dyn Fn(i64) -> i64", Form: symbol.FormFunc})),
				want: "Box is not one", wantErr: true,
			},
			{
				name: "returns a value error for a make",
				give: emit.Make(&emit.TypeRef{Spelling: "Receiver<i64>", Form: symbol.FormStream}),
				want: "no make of Receiver<i64>", wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, _, err := returned(t, tt.give)
				if !tt.wantErr {
					assert.NoError(t, err, "the value spells")
					assert.Equal(t, got, tt.want, "the Rust spelling")
					return
				}
				refused := assert.ErrorAs[*render.ValueError](t, err, "the render reports the value's own code")
				assert.Contains(t, err.Error(), tt.want, "naming what Rust cannot spell")
				assert.Equal(t, refused.Lang, string(rust.Lang), "naming the target")
			})
		}

		t.Run("uses the item a reference names", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Conversion(valueRef(weightName, unitsModule, weightName), integer("1")))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: unitsModule, Name: weightName}}, "the item's use")
		})

		t.Run("uses the function a call names", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Call(valueFn(utilModule, makeName)))
			assert.NoError(t, err, "the call spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: utilModule, Name: makeName}}, "the function's use")
		})

		t.Run("writes a constant under the name that its use binds", func(t *testing.T) {
			t.Parallel()

			got, set, err := returned(t, emit.Name(symbol.Identity{
				Lang: rust.Lang, Package: utilModule, Name: limitName, Kind: symbol.KindConstant,
			}))
			assert.NoError(t, err, "the name spells")
			assert.Equal(t, got, limitName, "the use binds the bare name")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: utilModule, Name: limitName}}, "the constant's use")
		})

		t.Run("uses the generic item a struct literal names", func(t *testing.T) {
			t.Parallel()

			weights := valueRef(weightName, unitsModule, weightName)
			weights.Args = []*emit.TypeRef{ref("f64")}
			_, set, err := returned(t, emit.Composite(weights, emit.NamedField("unit", float("1.5"))))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: unitsModule, Name: weightName}}, "the item's use")
		})

		t.Run("renames a callee another use binds", func(t *testing.T) {
			t.Parallel()

			got, _, err := returned(t, emit.Conversion(
				valueRef(weightName, unitsModule, weightName), emit.Call(valueFn(utilModule, weightName))))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, got, weightName+"("+weightName+"2())", "the type binds the name first")
		})

		t.Run("returns an error for a type argument that does not spell", func(t *testing.T) {
			t.Parallel()

			borrow := &emit.TypeRef{
				Form: symbol.FormBorrow, Spelling: "&Row", Elems: []*emit.TypeRef{imported(legacyModule, rowName)},
			}
			_, _, err := returned(t, emit.Composite(valueRef(rowName, storeModule, rowName),
				emit.NamedField("inner", emit.Conversion(generic("Wrapper", borrow), integer("1")))))
			assert.HasError(t, err, "the argument's item binds under another name")
		})

		t.Run("uses nothing for a literal", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, integer("1"))
			assert.NoError(t, err, "a literal spells")
			assert.Equal(t, set.Len(), 0, "a literal names no module")
		})
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a value as the scaffold writes it", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Value(makeCall, &render.ImportSet{})
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, got, makeName+"(1)", "the Rust spelling")
		})

		t.Run("uses the function a call names", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			_, err := backend.Value(makeCall, &set)
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: utilModule, Name: makeName}}, "the function's use")
		})

		t.Run("returns a value error for a value that Rust cannot spell", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Value(emit.Construction(valueRef(rowName, storeModule, rowName)), &render.ImportSet{})
			refused := assert.ErrorAs[*render.ValueError](t, err, "the render reports the value's own code")
			assert.Equal(t, refused.Lang, string(rust.Lang), "naming the target")
		})
	})
}

// A literal allocates nothing, a call allocates its text, and a function
// allocates the list of its results and its text. The ordinary run, which
// runs no benchmark, checks those ceilings here.
func TestValueAllocs(t *testing.T) {
	checkAllocs(t, valueCalls())
}

// BenchmarkValue measures the spelling of one value that a template
// writes.
func BenchmarkValue(b *testing.B) {
	benchCalls(b, valueCalls())
}

// valueCalls returns a call of Value over an integer literal, over a call
// of a function that the file uses already, and over a function with one
// parameter and one result.
func valueCalls() []allocCall {
	literal := integer("42")
	fn := emit.Func(&emit.TypeRef{
		Spelling: "fn(i64) -> i64", Form: symbol.FormFunc, Split: 1,
		Elems: []*emit.TypeRef{ref("i64"), ref("i64")},
	}, literal)
	set := &render.ImportSet{}
	var (
		out string
		err error
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "Value spells the value")
			assert.Equal(tb, out, want, "Value writes the Rust spelling")
		}
	}
	return []allocCall{
		{
			name: "Value", caseName: "an integer", allocs: literalAllocs,
			call: func() { out, err = backend.Value(literal, set) }, check: spells("42"),
		},
		{
			name: "Value", caseName: "a call of another module", allocs: callAllocs,
			call: func() { out, err = backend.Value(makeCall, set) }, check: spells(makeName + "(1)"),
		},
		{
			name: "Value", caseName: "a function with one result", allocs: funcAllocs,
			call: func() { out, err = backend.Value(fn, set) }, check: spells("|_| 42"),
		},
	}
}

// valueRef returns a resolved reference to an item of one module.
func valueRef(spelling, module, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target:   symbol.Identity{Lang: rust.Lang, Package: module, Name: name, Kind: symbol.KindStruct},
	}
}

// valueFn returns a callee identity.
func valueFn(module, name string) symbol.Identity {
	return symbol.Identity{Lang: rust.Lang, Package: module, Name: name, Kind: symbol.KindFunction}
}

// integer returns an integer literal.
func integer(text string) emit.Value { return emit.Literal(emit.LiteralInt, text) }

// float returns a float literal.
func float(text string) emit.Value { return emit.Literal(emit.LiteralFloat, text) }

// returned runs one value through the scaffold as a return and
// returns the value's text beside the file's import set.
func returned(tb assert.TB, v emit.Value) (string, *render.ImportSet, error) {
	tb.Helper()

	set := &render.ImportSet{}
	out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(v)}, set)
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(string(out)), ";"))
	return strings.TrimPrefix(text, "return "), set, err
}
