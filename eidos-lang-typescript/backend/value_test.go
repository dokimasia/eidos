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

// lineSeparator is U+2028, which the quoting escapes.
var lineSeparator = string(rune(0x2028))

// valueRef returns a reference to a declaration in one package.
func valueRef(spelling, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target: symbol.Identity{
			Lang:    typescript.Lang,
			Package: pkg,
			Name:    name,
			Kind:    symbol.KindStruct,
		},
	}
}

// valueFn returns a callee identity.
func valueFn(pkg, name string) symbol.Identity {
	return symbol.Identity{
		Lang:    typescript.Lang,
		Package: pkg,
		Name:    name,
		Kind:    symbol.KindFunction,
	}
}

// spelled runs one value through the scaffold as a bare return and
// hands back the text it wrote and the imports it recorded.
func spelled(tb assert.TB, v emit.Value) (string, []string, error) {
	tb.Helper()

	var set render.ImportSet
	out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(v)}, &set)
	if err != nil {
		return "", nil, err
	}
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(string(out)), ";"))
	return strings.TrimPrefix(text, "return "), set.Paths(), nil
}

// Every value spelling is pinned, for the reason the statement
// spellings are: the text is spliced into generated bodies and a
// drift rewrites files.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("spells the forms TypeScript states", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			value emit.Value
			want  string
		}{
			{
				"an integer",
				emit.Literal(emit.LiteralInt, "42"),
				"42",
			},
			{
				"the absent value",
				emit.Literal(emit.LiteralNil, ""),
				"null",
			},
			{
				"a string quotes single, TypeScript's canon",
				emit.Literal(emit.LiteralString, "hi"),
				`'hi'`,
			},
			{
				"a string escapes in TypeScript's grammar",
				emit.Literal(emit.LiteralString, "it's\a \\ "+lineSeparator+"\U0001F600"),
				`'it\'s` + `\` + `u0007 \\ ` + `\` + `u2028` + "\U0001F600'",
			},
			{
				"control characters take their named escapes",
				emit.Literal(emit.LiteralString, "\b\f\n\r\t\v\x7f"+string(rune(0x2029))),
				`'\b\f\n\r\t\v` + `\` + `u007f` + `\` + `u2029'`,
			},
			{
				"a conversion asserts, because TypeScript erases its types",
				emit.Conversion(
					valueRef("Weight", "units", "Weight"),
					emit.Literal(emit.LiteralInt, "1"),
				),
				"1 as Weight",
			},
			{
				"a record spells an object literal",
				emit.Composite(
					valueRef("Row", "svc", "Row"),
					emit.NamedField("id", emit.Literal(emit.LiteralInt, "1")),
				),
				"{id: 1}",
			},
			{
				"a map computes its key",
				emit.Composite(
					&emit.TypeRef{Spelling: "Record<string, number>", Form: symbol.FormMap},
					emit.KeyedEntry(
						emit.Literal(emit.LiteralString, "k"),
						emit.Literal(emit.LiteralInt, "2"),
					),
				),
				`{['k']: 2}`,
			},
			{
				"a list spells an array literal",
				emit.Composite(&emit.TypeRef{Spelling: "number[]", Form: symbol.FormList},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				"[2]",
			},
			{
				"a call spells bare, because an import binds the name",
				emit.Call(valueFn("./time", "unix"), emit.Literal(emit.LiteralInt, "1")),
				"unix(1)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, _, err := spelled(t, tt.value)
				assert.NoError(t, err, "the value spells")
				assert.Equal(t, got, tt.want, tt.name)
			})
		}
	})

	t.Run("records the import every reference and callee needs", func(t *testing.T) {
		t.Parallel()

		_, paths, err := spelled(t, emit.Conversion(
			valueRef("Weight", "example/units", "Weight"), emit.Literal(emit.LiteralInt, "1"),
		))
		assert.NoError(t, err, "the value spells")
		assert.Equal(t, paths, []string{"example/units"}, "the reference's package is imported")

		_, paths, err = spelled(t, emit.Literal(emit.LiteralInt, "1"))
		assert.NoError(t, err, "a literal spells")
		assert.Empty(t, paths, "and imports nothing, because it names no package")
	})

	t.Run("refuses a value TypeScript has no form for, under its own code", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			value   emit.Value
			mention string
		}{
			{
				"another language's raw text",
				emit.Raw("golang", "Row{}"), "written in golang",
			},
			{
				"an address, which TypeScript has no operator for",
				emit.Address(emit.Literal(emit.LiteralInt, "1")), "no address",
			},
			{
				"a positional element in an object literal",
				emit.Composite(
					valueRef("Row", "svc", "Row"),
					emit.Element(emit.Literal(emit.LiteralInt, "1")),
				), "names every entry",
			},
			{
				"a named entry in an array literal",
				emit.Composite(&emit.TypeRef{Spelling: "number[]", Form: symbol.FormList},
					emit.NamedField("f", emit.Literal(emit.LiteralInt, "1"))), "takes none",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, _, err := spelled(t, tt.value)
				assert.HasError(t, err, tt.name)
				assert.Contains(
					t,
					err.Error(),
					tt.mention,
					"the refusal names what it cannot spell",
				)
				var refused *render.ValueError
				assert.True(t, errors.As(err, &refused),
					"a value refusal classifies as one, so the render reports its own code")
				assert.Equal(
					t,
					refused.Lang,
					string(typescript.Lang),
					"naming the target that refused",
				)
			})
		}
	})
}
