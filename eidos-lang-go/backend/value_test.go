// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The value fixture: the package the spelled file belongs to, and the
// packages and functions its values name.
const (
	// svcPkg is the package of the file the fixture values render
	// into, and unitsPkg another workspace package.
	svcPkg   = "example.test/svc"
	unitsPkg = "example.test/units"
	// unixName and nowName are functions of timePkg.
	unixName = "Unix"
	nowName  = "Now"
)

// The allocations of a value.
const (
	// literalAllocs is an integer literal, which Go writes as its text.
	literalAllocs = 0
	// callAllocs is a call of a function of another package with two
	// arguments, in a file that binds the package already: the qualified
	// name of the callee and the text of the call.
	callAllocs = 2
)

// unixCall is the call time.Unix(1, 0). The specs only read it.
var unixCall = emit.Call(valueFn(timePkg, unixName),
	emit.Literal(emit.LiteralInt, "1"), emit.Literal(emit.LiteralInt, "0"))

// Every value spelling is pinned byte for byte, for the reason the
// statement spellings are: the text is spliced into generated bodies,
// and a drift rewrites files.
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
			{name: "writes an integer as its digits", give: emit.Literal(emit.LiteralInt, "42"), want: "\treturn 42\n"},
			{
				name: "writes a float as its digits",
				give: emit.Literal(emit.LiteralFloat, "1.5"),
				want: "\treturn 1.5\n",
			},
			{
				name: "writes a boolean as its keyword",
				give: emit.Literal(emit.LiteralBool, "true"),
				want: "\treturn true\n",
			},
			{name: "writes the absent value as nil", give: emit.Literal(emit.LiteralNil, ""), want: "\treturn nil\n"},
			{
				name: "writes a string quoted the way Go quotes",
				give: emit.Literal(emit.LiteralString, `a"b`),
				want: "\treturn \"a\\\"b\"\n",
			},
			{
				name: "writes an author's own Go text verbatim",
				give: emit.Raw(golang.Lang, "Row{ID: 1}"),
				want: "\treturn Row{ID: 1}\n",
			},
			{
				name: "writes a conversion as a call of the type",
				give: emit.Conversion(valueRef("Weight", svcPkg, "Weight"), emit.Literal(emit.LiteralFloat, "1.5")),
				want: "\treturn Weight(1.5)\n",
			},
			{
				name: "writes a conversion to a pointer type with the type in parentheses",
				give: emit.Conversion(&emit.TypeRef{
					Spelling: "*Row", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{valueRef(rowName, svcPkg, rowName)},
				}, emit.Literal(emit.LiteralNil, "")),
				want: "\treturn (*Row)(nil)\n",
			},
			{
				name: "writes a conversion to a receive-only channel with the type in parentheses",
				give: emit.Conversion(&emit.TypeRef{
					Spelling: "<-chan int", Form: symbol.FormStream, Elems: []*emit.TypeRef{ref("int")},
				}, emit.Literal(emit.LiteralNil, "")),
				want: "\treturn (<-chan int)(nil)\n",
			},
			{
				name: "writes a conversion to a function type with the type in parentheses",
				give: emit.Conversion(&emit.TypeRef{Spelling: "func()", Form: symbol.FormFunc},
					emit.Literal(emit.LiteralNil, "")),
				want: "\treturn (func())(nil)\n",
			},
			{
				name: "writes a struct composite with its named fields",
				give: emit.Composite(valueRef(rowName, svcPkg, rowName),
					emit.NamedField("ID", emit.Literal(emit.LiteralInt, "1")),
					emit.NamedField("Tag", emit.Literal(emit.LiteralString, "a"))),
				want: "\treturn Row{ID: 1, Tag: \"a\"}\n",
			},
			{
				name: "writes a map composite with its keyed entry",
				give: emit.Composite(ref("map[string]int"),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), emit.Literal(emit.LiteralInt, "2"))),
				want: "\treturn map[string]int{\"k\": 2}\n",
			},
			{
				name: "writes a slice composite with its bare element",
				give: emit.Composite(ref("[]int"), emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				want: "\treturn []int{2}\n",
			},
			{
				name: "writes a call of another package's function qualified",
				give: emit.Call(valueFn(timePkg, unixName),
					emit.Literal(emit.LiteralInt, "1"), emit.Literal(emit.LiteralInt, "0")),
				want: "\treturn time.Unix(1, 0)\n",
			},
			{
				name: "writes a call of a function in no package bare",
				give: emit.Call(symbol.Identity{Lang: golang.Lang, Name: "make"}),
				want: "\treturn make()\n",
			},
			{
				name: "writes the address of a composite",
				give: emit.Address(emit.Composite(valueRef(rowName, svcPkg, rowName))),
				want: "\treturn &Row{}\n",
			},
			{
				name: "writes a composite of a slice type that the substitution left without a spelling",
				give: emit.Composite(&emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{ref("int")}},
					emit.Element(emit.Literal(emit.LiteralInt, "1"))),
				want: "\treturn []int{1}\n",
			},
			{
				name: "writes the name of a constant of another package qualified",
				give: emit.Name(symbol.Identity{
					Lang: golang.Lang, Package: timePkg, Name: "Second", Kind: symbol.KindConstant,
				}),
				want: "\treturn time.Second\n",
			},
			{
				name: "writes the name of a variable of the file's own package bare",
				give: emit.Name(symbol.Identity{
					Lang: golang.Lang, Package: svcPkg, Name: "Registry", Kind: symbol.KindVariable,
				}),
				want: "\treturn Registry\n",
			},
			{
				name:    "returns a value error for another language's raw text",
				give:    emit.Raw("typescript", "{id: 1}"),
				want:    "written in typescript",
				wantErr: true,
			},
			{
				name:    "returns a value error for a boolean outside its two spellings",
				give:    emit.Literal(emit.LiteralBool, "yes"),
				want:    "spells true or false",
				wantErr: true,
			},
			{
				name:    "returns a value error for a number without text",
				give:    emit.Literal(emit.LiteralInt, ""),
				want:    "has no text",
				wantErr: true,
			},
			{
				name:    "returns a value error for a literal kind nothing declares",
				give:    emit.Value{Kind: emit.ValueLiteral},
				want:    "no spelling for the",
				wantErr: true,
			},
			{
				name:    "returns a value error for a composite of a type without a spelling",
				give:    emit.Composite(&emit.TypeRef{}),
				want:    "has no spelling",
				wantErr: true,
			},
			{
				name:    "returns a value error for a call of a function without a name",
				give:    emit.Call(symbol.Identity{Lang: golang.Lang, Package: timePkg}),
				want:    "has no name",
				wantErr: true,
			},
			{
				name:    "returns a value error for the address of anything but a composite literal",
				give:    emit.Address(emit.Literal(emit.LiteralInt, "1")),
				want:    "composite literal alone",
				wantErr: true,
			},
			{
				name:    "returns a value error for a construction",
				give:    emit.Construction(valueRef(rowName, svcPkg, rowName)),
				want:    "has no construction of Row",
				wantErr: true,
			},
			{
				name: "writes a function that returns its results",
				give: emit.Func(fnType("func(context.Context) error", 1, imported(contextPkg, contextPkg, "Context"),
					ref("error")), emit.Literal(emit.LiteralNil, "")),
				want: "\treturn func(context.Context) error { return nil }\n",
			},
			{
				name: "writes a function with two results",
				give: emit.Func(fnType("func() (int, error)", 0, ref("int"), ref("error")),
					emit.Literal(emit.LiteralInt, "0"), emit.Literal(emit.LiteralNil, "")),
				want: "\treturn func() (int, error) { return 0, nil }\n",
			},
			{
				name: "writes a function without results with an empty body",
				give: emit.Func(fnType("func(string)", 1, ref("string"))),
				want: "\treturn func(string) {}\n",
			},
			{
				name: "writes a make of a channel",
				give: emit.Make(composite(symbol.FormStream, "chan string", ref("string"))),
				want: "\treturn make(chan string)\n",
			},
			{
				name: "writes a make of a receive-only channel",
				give: emit.Make(composite(symbol.FormStream, "<-chan string", ref("string"))),
				want: "\treturn make(<-chan string)\n",
			},
			{
				name:    "returns a value error for a function of a type that is not a function type",
				give:    emit.Func(valueRef(rowName, svcPkg, rowName)),
				want:    "Row is not one",
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, _, err := returned(t, tt.give)
				if !tt.wantErr {
					assert.NoError(t, err, "the value spells")
					assert.Equal(t, got, tt.want, "the Go spelling")
					return
				}
				refused := assert.ErrorAs[*render.ValueError](t, err, "the render reports the value's own code")
				assert.Contains(t, err.Error(), tt.want, "naming what Go cannot spell")
				assert.Equal(t, refused.Lang, string(golang.Lang), "naming the target")
			})
		}

		t.Run("binds the import of every reference and callee of another package", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Composite(valueRef(rowName, storePkg, rowName),
				emit.NamedField("When", emit.Call(valueFn(timePkg, unixName))),
				emit.NamedField("W", emit.Conversion(
					valueRef("Weight", unitsPkg, "Weight"),
					emit.Literal(emit.LiteralInt, "1"),
				)),
				emit.NamedField("Old", emit.Name(symbol.Identity{
					Lang: golang.Lang, Package: legacyPkg, Name: "Default", Kind: symbol.KindVariable,
				}))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, set.Paths(), []string{legacyPkg, storePkg, unitsPkg, timePkg},
				"every package the tree names, in path order")
		})

		t.Run("binds the import of each type of the signature of a function", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Func(fnType("func(context.Context) error", 1,
				imported(contextPkg, contextPkg, "Context"), ref("error")), emit.Literal(emit.LiteralNil, "")))
			assert.NoError(t, err, "the function spells")
			assert.Equal(t, set.Paths(), []string{contextPkg}, "the file imports the package of the parameter")
		})

		t.Run("writes the names of the file's own package bare", func(t *testing.T) {
			t.Parallel()

			got, set, err := returned(t, emit.Composite(valueRef(rowName, svcPkg, rowName),
				emit.NamedField("ID", emit.Call(valueFn(svcPkg, "NextID"))),
				emit.NamedField("When", emit.Call(valueFn(timePkg, nowName)))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, got, "\treturn Row{ID: NextID(), When: time.Now()}\n",
				"a package never qualifies its own names")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the other package alone is imported")
		})

		t.Run("qualifies a callee through the name its path assumes", func(t *testing.T) {
			t.Parallel()

			got, _, err := returned(t, emit.Call(valueFn(yamlPkg, "Marshal")))
			assert.NoError(t, err, "the call spells")
			assert.Equal(t, got, "\treturn "+yamlName+".Marshal()\n", "the major version is dropped")
		})

		t.Run("qualifies a callee of a package whose path ends in a keyword through the escaped name",
			func(t *testing.T) {
				t.Parallel()

				got, set, err := returned(t, emit.Call(valueFn(keywordPkg, "Open")))
				assert.NoError(t, err, "the call spells")
				assert.Equal(t, got, "\treturn "+keywordName+".Open()\n", "go is a keyword, so the qualifier is go_")
				assert.Equal(t, set.Entries(), []render.Entry{{Path: keywordPkg, Name: keywordName}},
					"the import binds the escaped name")
			})

		t.Run("suffixes a callee's qualifier another package binds", func(t *testing.T) {
			t.Parallel()

			got, _, err := returned(t, emit.Composite(valueRef(rowName, storePkg, rowName),
				emit.NamedField("Old", emit.Call(valueFn(legacyPkg, "Open")))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, got, "\treturn store.Row{Old: "+storeName2+".Open()}\n",
				"the reference's package binds the name first")
		})

		t.Run("returns an error for a reference that does not spell", func(t *testing.T) {
			t.Parallel()

			_, _, err := returned(t, emit.Composite(ref("x.Y")))
			assert.HasError(t, err, "a qualified reference without a package is refused")
		})

		callees := []struct {
			name string
			give emit.Value
			want string
		}{
			{
				name: "returns an error for a callee of an internal tree that does not contain the file",
				give: emit.Call(valueFn("example.test/core/internal/clock", nowName)),
				want: "internal",
			},
			{
				name: "returns an error for a name whose package is not an import path",
				give: emit.Name(symbol.Identity{
					Lang: golang.Lang, Package: "example.test/bad units", Name: "Max", Kind: symbol.KindConstant,
				}),
				want: "malformed import path",
			},
		}
		for _, tt := range callees {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, set, err := returned(t, tt.give)
				assert.HasError(t, err, "the go command refuses the import")
				expect.Contains(t, err.Error(), tt.want, "naming the rule")
				expect.Equal(t, set.Len(), 0, "the refused import is not bound")
			})
		}

		t.Run("places a value in a call the scaffold writes", func(t *testing.T) {
			t.Parallel()

			fn := nameOf("want")
			out, err := backend.Scaffold(emit.Stmt{
				Kind: emit.StmtExpr,
				Value: emit.Expr{Kind: emit.ExprCall, Fn: &fn, Args: []emit.Expr{
					nameOf("got"), emit.ValueExpr(emit.Literal(emit.LiteralInt, "42")),
				}},
			}, &render.ImportSet{})
			assert.NoError(t, err, "the statement spells")
			assert.Equal(t, string(out), "\twant(got, 42)\n", "a value is an argument like any other")
		})
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a value as the scaffold writes it", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Value(unixCall, homeSet())
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, got, "time.Unix(1, 0)", "the Go spelling")
		})

		t.Run("binds the import of a callee of another package", func(t *testing.T) {
			t.Parallel()

			set := homeSet()
			_, err := backend.Value(unixCall, set)
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the file imports the package of the callee")
		})

		t.Run("returns a value error for a value that Go cannot spell", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Value(emit.Construction(valueRef(rowName, svcPkg, rowName)), homeSet())
			refused := assert.ErrorAs[*render.ValueError](t, err, "the render reports the value's own code")
			assert.Equal(t, refused.Lang, string(golang.Lang), "naming the target")
		})
	})
}

// A literal allocates nothing, and a call allocates the qualified name of
// its callee and the text that it joins. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestValueAllocs(t *testing.T) {
	checkAllocs(t, valueCalls())
}

// BenchmarkValue measures the spelling of one value that a template
// writes.
func BenchmarkValue(b *testing.B) {
	benchCalls(b, valueCalls())
}

// valueCalls returns a call of Value over an integer literal, and over a
// call of a function of a package that the file binds already.
func valueCalls() []allocCall {
	literal := emit.Literal(emit.LiteralInt, "42")
	set := homeSet()
	var (
		out string
		err error
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "Value spells the value")
			assert.Equal(tb, out, want, "Value writes the Go spelling")
		}
	}
	return []allocCall{
		{
			name: "Value", caseName: "an integer", allocs: literalAllocs,
			call: func() { out, err = backend.Value(literal, set) }, check: spells("42"),
		},
		{
			name: "Value", caseName: "a call of another package", allocs: callAllocs,
			call: func() { out, err = backend.Value(unixCall, set) }, check: spells("time.Unix(1, 0)"),
		},
	}
}

// homeSet returns the import set of a file of svcPkg.
func homeSet() *render.ImportSet {
	set := &render.ImportSet{}
	set.SetHome(svcPkg)
	return set
}

// valueRef returns a resolved reference to a struct of one package,
// spelled the way Go source spells it.
func valueRef(spelling, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target:   symbol.Identity{Lang: golang.Lang, Package: pkg, Name: name, Kind: symbol.KindStruct},
	}
}

// valueFn returns a callee identity.
func valueFn(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: pkg, Name: name, Kind: symbol.KindFunction}
}

// returned runs one value through the Go scaffold as a return in a
// file of svcPkg, and returns the statement's text beside the file's
// import set.
func returned(tb assert.TB, v emit.Value) (string, *render.ImportSet, error) {
	tb.Helper()

	set := &render.ImportSet{}
	set.SetHome(svcPkg)
	out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(v)}, set)
	return string(out), set, err
}
