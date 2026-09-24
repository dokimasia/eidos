// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// valueRef returns a reference to a declaration in one package.
func valueRef(spelling, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target: symbol.Identity{
			Lang:    rust.Lang,
			Package: pkg,
			Name:    name,
			Kind:    symbol.KindStruct,
		},
	}
}

// valueFn returns a callee identity.
func valueFn(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: rust.Lang, Package: pkg, Name: name, Kind: symbol.KindFunction}
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

	t.Run("spells the forms Rust states", func(t *testing.T) {
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
				"None",
			},
			{
				"a string escapes in Rust's grammar",
				emit.Literal(emit.LiteralString, "bell\a nul\x00 \\ \"q\" é"),
				`"bell\` + `u{7} nul\0 \\ \"q\" é"`,
			},
			{
				"control characters take their named escapes",
				emit.Literal(emit.LiteralString, "\n\r\t\x7f"),
				`"\n\r\t\` + `u{7f}"`,
			},
			{
				"a conversion constructs the tuple struct a defined type is",
				emit.Conversion(
					valueRef("Weight", "units", "Weight"),
					emit.Literal(emit.LiteralFloat, "1.5"),
				),
				"Weight(1.5)",
			},
			{
				"a struct literal names its fields",
				emit.Composite(
					valueRef("Row", "svc", "Row"),
					emit.NamedField("id", emit.Literal(emit.LiteralInt, "1")),
				),
				"Row { id: 1 }",
			},
			{
				"a struct with no fields spells its name alone",
				emit.Composite(valueRef("Row", "svc", "Row")),
				"Row",
			},
			{
				"a list spells the vector macro",
				emit.Composite(&emit.TypeRef{Spelling: "Vec<i64>", Form: symbol.FormList},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				"vec![2]",
			},
			{
				"an array spells brackets",
				emit.Composite(&emit.TypeRef{Spelling: "[i64; 1]", Form: symbol.FormArray},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				"[2]",
			},
			{
				"an address borrows",
				emit.Address(emit.Literal(emit.LiteralInt, "1")),
				"&1",
			},
			{
				"a call spells bare, because the import binds the name",
				emit.Call(valueFn("example/util", "make"), emit.Literal(emit.LiteralInt, "1")),
				"make(1)",
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

	t.Run("refuses a value Rust has no form for, under its own code", func(t *testing.T) {
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
				"a map, which Rust states no literal for",
				emit.Composite(
					&emit.TypeRef{Spelling: "HashMap<String, i64>", Form: symbol.FormMap},
					emit.KeyedEntry(
						emit.Literal(emit.LiteralString, "k"),
						emit.Literal(emit.LiteralInt, "2"),
					),
				), "no map literal",
			},
			{
				"a positional element in a struct literal",
				emit.Composite(
					valueRef("Row", "svc", "Row"),
					emit.Element(emit.Literal(emit.LiteralInt, "1")),
				), "names every field",
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
				assert.Equal(t, refused.Lang, string(rust.Lang), "naming the target that refused")
			})
		}
	})
}
