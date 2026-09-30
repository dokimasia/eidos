// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The value fixture: the modules the values name and a function of
// one of them.
const (
	unitsModule = "./units"
	timeModule  = "./time"
	unixName    = "unix"
	weightName  = "Weight"
)

// lineSeparator is U+2028, which the quoting escapes.
var lineSeparator = string(rune(0x2028))

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
				give: emit.Composite(valueRef(rowName, storeModule, rowName),
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
				give: emit.Composite(valueRef(rowName, storeModule, rowName),
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
				var refused *render.ValueError
				assert.True(t, errors.As(err, &refused), "the render reports the value's own code")
				assert.Contains(t, err.Error(), tt.want, "naming what TypeScript cannot spell")
				assert.Equal(t, refused.Lang, string(typescript.Lang), "naming the target")
			})
		}

		t.Run("imports what a conversion's type names for the type checker alone", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Conversion(
				valueRef(weightName, unitsModule, weightName), emit.Literal(emit.LiteralInt, "1")))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: unitsModule, Name: weightName, TypeOnly: true}},
				"a type-only import")
		})

		t.Run("imports a callee as a value", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Call(valueFn(timeModule, unixName)))
			assert.NoError(t, err, "the value spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: timeModule, Name: unixName}},
				"a call needs the function at run time")
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
}
