// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// valueRef returns a reference to a declaration in one package.
func valueRef(spelling, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target: symbol.Identity{
			Lang:    java.Lang,
			Package: pkg,
			Name:    name,
			Kind:    symbol.KindStruct,
		},
	}
}

// valueFn returns a callee identity, owned by a class where owner
// names one.
func valueFn(pkg, owner, name string) symbol.Identity {
	return symbol.Identity{Lang: java.Lang, Package: pkg, Owner: owner, Name: name, Kind: symbol.KindMethod}
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

	t.Run("spells the forms Java states", func(t *testing.T) {
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
				"a conversion casts",
				emit.Conversion(
					valueRef("Weight", "units", "Weight"),
					emit.Literal(emit.LiteralInt, "1"),
				),
				"(Weight) 1",
			},
			{
				"a record with no components constructs",
				emit.Composite(valueRef("Row", "svc", "Row")),
				"new Row()",
			},
			{
				"a list spells the collection factory",
				emit.Composite(&emit.TypeRef{Spelling: "List<Long>", Form: symbol.FormList},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				"List.of(2)",
			},
			{
				"a map spells its factory, key then value",
				emit.Composite(
					&emit.TypeRef{Spelling: "Map<String, Long>", Form: symbol.FormMap},
					emit.KeyedEntry(
						emit.Literal(emit.LiteralString, "k"),
						emit.Literal(emit.LiteralInt, "2"),
					),
				),
				`Map.of("k", 2)`,
			},
			{
				"a call spells the static method of the class its owner names",
				emit.Call(valueFn("example/util", "Rows", "make"), emit.Literal(emit.LiteralInt, "1")),
				"Rows.make(1)",
			},
			{
				"a string escapes in Java's grammar",
				emit.Literal(emit.LiteralString, "tab\tbell\x07 \\ \"q\" é"),
				`"tab\tbell\007 \\ \"q\" é"`,
			},
			{
				"control characters take their named escapes",
				emit.Literal(emit.LiteralString, "\b\f\n\r\x7f"),
				`"\b\f\n\r\177"`,
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

	t.Run("imports the class a factory or a callee names", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		_, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(
			emit.Call(valueFn("example/util", "Rows.Inner", "make"),
				emit.Composite(&emit.TypeRef{Spelling: "List<Long>", Form: symbol.FormList},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				emit.Composite(&emit.TypeRef{Spelling: "Map<Long, Long>", Form: symbol.FormMap},
					emit.KeyedEntry(emit.Literal(emit.LiteralInt, "1"), emit.Literal(emit.LiteralInt, "2")))),
		)}, &set)
		assert.NoError(t, err, "the value spells")
		assert.Equal(t, set.Entries(), []render.Entry{
			{Path: "example/util", Name: "Rows"},
			{Path: "java/util", Name: "List"},
			{Path: "java/util", Name: "Map"},
		}, "each class imports by name, a nested owner through its file-level class")
	})

	t.Run("refuses a value Java has no form for, under its own code", func(t *testing.T) {
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
				"an address, which Java has no operator for",
				emit.Address(emit.Literal(emit.LiteralInt, "1")), "no address",
			},
			{
				"a record whose value names its fields",
				emit.Composite(
					valueRef("Row", "svc", "Row"),
					emit.NamedField("id", emit.Literal(emit.LiteralInt, "1")),
				), "positionally",
			},
			{
				"a function whose identity names no owner",
				emit.Call(valueFn("example/util", "", "make")), "owned by no class",
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
				assert.Equal(t, refused.Lang, string(java.Lang), "naming the target that refused")
			})
		}
	})
}
