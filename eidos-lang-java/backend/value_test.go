// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The value fixture: the packages the values name, and the class a
// static callee belongs to.
const (
	unitsPkg     = "example/units"
	utilPkg      = "example/util"
	utilPkgDots  = "example.util"
	collections  = "java/util"
	rowsClass    = "Rows"
	makeName     = "make"
	weightName   = "Weight"
	listClass    = "List"
	listFactory  = "List<Long>"
	mapFactory   = "Map<Long, Long>"
	pairsMap     = "Map<Integer, Integer>"
	qualifiedOut = "java.util.List"
)

// valueRef returns a resolved reference to a class of one package.
func valueRef(spelling, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target:   symbol.Identity{Lang: java.Lang, Package: pkg, Name: name, Kind: symbol.KindStruct},
	}
}

// valueFn returns a callee identity whose Owner names its class where
// owner is set.
func valueFn(pkg, owner, name string) symbol.Identity {
	return symbol.Identity{Lang: java.Lang, Package: pkg, Owner: owner, Name: name, Kind: symbol.KindMethod}
}

// formed returns a structural reference of one form, spelled s.
func formed(s string, form symbol.TypeForm) *emit.TypeRef {
	return &emit.TypeRef{Spelling: s, Form: form}
}

// integer returns an integer literal.
func integer(text string) emit.Value { return emit.Literal(emit.LiteralInt, text) }

// pairs returns n map entries, each keying an integer to itself.
func pairs(n int) []emit.ValueField {
	out := make([]emit.ValueField, 0, n)
	for i := range n {
		text := strconv.Itoa(i)
		out = append(out, emit.KeyedEntry(integer(text), integer(text)))
	}
	return out
}

// returned runs one value through the scaffold as a return and
// returns the value's text beside the file's import set.
func returned(tb assert.TB, set *render.ImportSet, v emit.Value) (string, error) {
	tb.Helper()

	out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(v)}, set)
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(string(out)), ";"))
	return strings.TrimPrefix(text, "return "), err
}

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
			{name: "writes the largest int without a suffix", give: integer("2147483647"), want: "2147483647"},
			{
				name: "writes an integer beyond the int range with the long suffix",
				give: integer("5000000000"), want: "5000000000L",
			},
			{
				name: "writes a negative integer beyond the int range with the long suffix",
				give: integer("-2147483649"), want: "-2147483649L",
			},
			{name: "writes a float as its digits", give: emit.Literal(emit.LiteralFloat, "1.5"), want: "1.5"},
			{
				name: "writes a float for a 32-bit type with the float suffix",
				give: emit.Number(emit.LiteralFloat, "1.5", 32), want: "1.5f",
			},
			{
				name: "writes a float for a 64-bit type without a suffix",
				give: emit.Number(emit.LiteralFloat, "1.5", 64), want: "1.5",
			},
			{
				name: "writes an integer for a 64-bit type with the long suffix",
				give: emit.Number(emit.LiteralInt, "2", 64), want: "2L",
			},
			{
				name: "writes an integer for a narrower type without a suffix",
				give: emit.Number(emit.LiteralInt, "2", 16), want: "2",
			},
			{name: "writes the absent value as null", give: emit.Literal(emit.LiteralNil, ""), want: "null"},
			{
				name: "writes a conversion as a cast",
				give: emit.Conversion(valueRef(weightName, unitsPkg, weightName), integer("1")),
				want: "(Weight) 1",
			},
			{
				name: "writes a record without components as a constructor call",
				give: emit.Composite(valueRef(rowName, storePkg, rowName)),
				want: "new Row()",
			},
			{
				name: "writes a list through the collection factory",
				give: emit.Composite(formed(listFactory, symbol.FormList),
					emit.Element(emit.Number(emit.LiteralInt, "2", 64))),
				want: "List.of(2L)",
			},
			{
				name: "writes a map through its factory",
				give: emit.Composite(formed("Map<String, Long>", symbol.FormMap),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), emit.Number(emit.LiteralInt, "2", 64))),
				want: `Map.of("k", 2L)`,
			},
			{
				name: "writes a map of ten pairs through the factory",
				give: emit.Composite(formed(pairsMap, symbol.FormMap), pairs(10)...),
				want: "Map.of(0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9)",
			},
			{
				name: "writes a map above ten pairs from entries",
				give: emit.Composite(formed(pairsMap, symbol.FormMap), pairs(11)...),
				want: "Map.ofEntries(Map.entry(0, 0), Map.entry(1, 1), Map.entry(2, 2), " +
					"Map.entry(3, 3), Map.entry(4, 4), Map.entry(5, 5), Map.entry(6, 6), " +
					"Map.entry(7, 7), Map.entry(8, 8), Map.entry(9, 9), Map.entry(10, 10))",
			},
			{
				name: "writes an array as an array creation",
				give: emit.Composite(formed("int[]", symbol.FormArray),
					emit.Element(integer("1")), emit.Element(integer("2"))),
				want: "new int[] {1, 2}",
			},
			{
				name: "writes an empty array with an empty initializer",
				give: emit.Composite(formed("String[]", symbol.FormArray)),
				want: "new String[] {}",
			},
			{
				name: "writes a call of the static method of its owner's class",
				give: emit.Call(valueFn(utilPkg, rowsClass, makeName), integer("1")),
				want: "Rows.make(1)",
			},
			{
				name: "writes a string escaped in Java's grammar",
				give: emit.Literal(emit.LiteralString, "tab\tbell\x07 \\ \"q\" é"),
				want: `"tab\tbell\007 \\ \"q\" é"`,
			},
			{
				name: "writes control characters under their named escapes",
				give: emit.Literal(emit.LiteralString, "\b\f\n\r\x7f"),
				want: `"\b\f\n\r\177"`,
			},
			{
				name: "returns a value error for another language's raw text",
				give: emit.Raw("golang", "Row{}"), want: "written in golang", wantErr: true,
			},
			{
				name: "returns a value error for an integer beyond the long range",
				give: integer("9223372036854775808"), want: "does not fit a Java long", wantErr: true,
			},
			{
				name: "returns a value error for an address",
				give: emit.Address(integer("1")), want: "no address", wantErr: true,
			},
			{
				name: "returns a value error for a record whose value names its fields",
				give: emit.Composite(valueRef(rowName, storePkg, rowName), emit.NamedField("id", integer("1"))),
				want: "positionally", wantErr: true,
			},
			{
				name: "returns a value error for a function of no class",
				give: emit.Call(valueFn(utilPkg, "", makeName)), want: "belongs to no class", wantErr: true,
			},
			{
				name: "returns a value error for a keyed entry in a list",
				give: emit.Composite(formed(listFactory, symbol.FormList),
					emit.KeyedEntry(integer("1"), integer("2"))),
				want: "elements alone", wantErr: true,
			},
			{
				name: "returns a value error for a named entry in an array",
				give: emit.Composite(formed("int[]", symbol.FormArray), emit.NamedField("id", integer("1"))),
				want: "elements alone", wantErr: true,
			},
			{
				name: "returns a value error for a map entry without a key",
				give: emit.Composite(formed(mapFactory, symbol.FormMap), emit.Element(integer("1"))),
				want: "no key", wantErr: true,
			},
			{
				name: "returns a value error for a keyed entry in a record",
				give: emit.Composite(valueRef(rowName, storePkg, rowName),
					emit.KeyedEntry(integer("1"), integer("2"))),
				want: "positionally", wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := returned(t, &render.ImportSet{}, tt.give)
				if !tt.wantErr {
					assert.NoError(t, err, "the value spells")
					assert.Equal(t, got, tt.want, "the Java spelling")
					return
				}
				var refused *render.ValueError
				assert.True(t, errors.As(err, &refused), "the render reports the value's own code")
				assert.Contains(t, err.Error(), tt.want, "naming what Java cannot spell")
				assert.Equal(t, refused.Lang, string(java.Lang), "naming the target")
			})
		}

		t.Run("imports the class a reference names", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			_, err := returned(t, &set, emit.Conversion(valueRef(weightName, unitsPkg, weightName), integer("1")))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Paths(), []string{unitsPkg}, "the reference's package")
		})

		t.Run("imports nothing for a literal", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			_, err := returned(t, &set, integer("1"))
			assert.NoError(t, err, "a literal spells")
			assert.Equal(t, set.Len(), 0, "a literal names no package")
		})

		t.Run("imports the classes of a callee's owner and a factory", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			_, err := returned(t, &set, emit.Call(valueFn(utilPkg, rowsClass+".Inner", makeName),
				emit.Composite(formed(listFactory, symbol.FormList), emit.Element(integer("2"))),
				emit.Composite(formed(mapFactory, symbol.FormMap), emit.KeyedEntry(integer("1"), integer("2")))))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{
				{Path: utilPkg, Name: rowsClass},
				{Path: collections, Name: listClass},
				{Path: collections, Name: "Map"},
			}, "a nested owner through its file-level class")
		})

		t.Run("writes the qualified call of a class whose simple name the file reserves", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Reserve(rowsClass)
			got, err := returned(t, &set, emit.Call(valueFn(utilPkg, rowsClass, makeName)))
			assert.NoError(t, err, "the call spells")
			assert.Equal(t, got, utilPkgDots+"."+rowsClass+"."+makeName+"()", "the way javac reads a clash")
		})

		t.Run("writes the qualified factory of a collection whose simple name the file reserves", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Reserve(listClass)
			got, err := returned(t, &set, emit.Composite(formed(listFactory, symbol.FormList),
				emit.Element(integer("2"))))
			assert.NoError(t, err, "the list spells")
			assert.Equal(t, got, qualifiedOut+".of(2)", "the collection written qualified")
		})
	})
}
