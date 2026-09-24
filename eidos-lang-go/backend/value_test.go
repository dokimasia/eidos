// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// valueRef returns a reference to a declaration in one package,
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

// spelled runs one value through the Go scaffold as a bare return
// and hands back the statement's text and the imports it recorded.
func spelled(tb assert.TB, v emit.Value) (string, []string, error) {
	tb.Helper()

	var set render.ImportSet
	out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(v)}, &set)
	if err != nil {
		return "", nil, err
	}
	return string(out), set.Paths(), nil
}

// Every value spelling is pinned byte for byte, for the reason the
// statement spellings are: the text is spliced into generated
// bodies and a drift rewrites files.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("spells every form as Go", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			value emit.Value
			want  string
		}{
			{"an integer", emit.Literal(emit.LiteralInt, "42"), "\treturn 42\n"},
			{"a float", emit.Literal(emit.LiteralFloat, "1.5"), "\treturn 1.5\n"},
			{"a boolean", emit.Literal(emit.LiteralBool, "true"), "\treturn true\n"},
			{"the absent value", emit.Literal(emit.LiteralNil, ""), "\treturn nil\n"},
			{
				"a string quotes the way Go quotes",
				emit.Literal(emit.LiteralString, `a"b`), "\treturn \"a\\\"b\"\n",
			},
			{
				"an author's own Go text spells verbatim",
				emit.Raw(golang.Lang, "Row{ID: 1}"), "\treturn Row{ID: 1}\n",
			},
			{
				"a conversion",
				emit.Conversion(valueRef("Weight", "units", "Weight"), emit.Literal(emit.LiteralFloat, "1.5")),
				"\treturn Weight(1.5)\n",
			},
			{
				"a struct composite names its fields",
				emit.Composite(valueRef("Row", "svc", "Row"),
					emit.NamedField("ID", emit.Literal(emit.LiteralInt, "1")),
					emit.NamedField("Tag", emit.Literal(emit.LiteralString, "a"))),
				"\treturn Row{ID: 1, Tag: \"a\"}\n",
			},
			{
				"a map composite keys its entry",
				emit.Composite(&emit.TypeRef{Spelling: "map[string]int"},
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), emit.Literal(emit.LiteralInt, "2"))),
				"\treturn map[string]int{\"k\": 2}\n",
			},
			{
				"a slice composite spells its element bare",
				emit.Composite(&emit.TypeRef{Spelling: "[]int"},
					emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				"\treturn []int{2}\n",
			},
			{
				"a call qualifies its callee",
				emit.Call(valueFn("time", "Unix"),
					emit.Literal(emit.LiteralInt, "1"), emit.Literal(emit.LiteralInt, "0")),
				"\treturn time.Unix(1, 0)\n",
			},
			{
				"a call in no package spells bare",
				emit.Call(symbol.Identity{Lang: golang.Lang, Name: "make"}), "\treturn make()\n",
			},
			{
				"an address takes a composite directly",
				emit.Address(emit.Composite(valueRef("Row", "svc", "Row"))), "\treturn &Row{}\n",
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

		_, paths, err := spelled(t, emit.Composite(valueRef("svc.Row", "example.test/svc", "Row"),
			emit.NamedField("When", emit.Call(valueFn("time", "Unix"))),
			emit.NamedField("W", emit.Conversion(
				valueRef("units.Weight", "example.test/units", "Weight"),
				emit.Literal(emit.LiteralInt, "1"),
			))))
		assert.NoError(t, err, "the tree spells")
		assert.Equal(t, paths, []string{"example.test/svc", "example.test/units", "time"},
			"every package the tree names is imported, sorted the way the set reports")

		_, paths, err = spelled(t, emit.Composite(&emit.TypeRef{Spelling: "[]int"},
			emit.Element(emit.Literal(emit.LiteralInt, "1"))))
		assert.NoError(t, err, "a builtin composite spells")
		assert.Empty(t, paths, "and imports nothing, because a builtin names no package")
	})

	t.Run("spells and imports nothing of the file's own package", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.SetHome("example.test/svc")
		out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(
			emit.Composite(valueRef("Row", "example.test/svc", "Row"),
				emit.NamedField("ID", emit.Call(valueFn("example.test/svc", "NextID"))),
				emit.NamedField("When", emit.Call(valueFn("time", "Now")))),
		)}, &set)
		assert.NoError(t, err, "the tree spells")
		assert.Equal(t, string(out), "\treturn Row{ID: NextID(), When: time.Now()}\n",
			"a callee in the file's own package spells bare")
		assert.Equal(t, set.Paths(), []string{"time"}, "and only the other package imports")
	})

	t.Run("refuses a value Go has no form for, under its own code", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			value   emit.Value
			mention string
		}{
			{
				"another language's raw text",
				emit.Raw("typescript", "{id: 1}"), "written in typescript",
			},
			{
				"a boolean outside its two spellings",
				emit.Literal(emit.LiteralBool, "yes"), "spells true or false",
			},
			{"a number with no text", emit.Literal(emit.LiteralInt, ""), "carries no text"},
			{
				"a literal kind nothing declares",
				emit.Value{Kind: emit.ValueLiteral},
				"no spelling for the",
			},
			{
				"a composite naming a type that spells nothing",
				emit.Composite(&emit.TypeRef{}), "spells nothing",
			},
			{
				"a call naming a function that spells nothing",
				emit.Call(symbol.Identity{Lang: golang.Lang, Package: "time"}), "spells nothing",
			},
			{
				"the address of anything but a composite literal",
				emit.Address(emit.Literal(emit.LiteralInt, "1")), "composite literal alone",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, _, err := spelled(t, tt.value)
				assert.HasError(t, err, tt.name)
				assert.Contains(t, err.Error(), tt.mention, "the refusal names what it cannot spell")
				var refused *render.ValueError
				assert.True(t, errors.As(err, &refused),
					"a value refusal classifies as one, so the render reports its own code")
				assert.Equal(t, refused.Lang, string(golang.Lang), "naming the target that refused")
			})
		}
	})

	t.Run("places a value in a call the scaffold writes", func(t *testing.T) {
		t.Parallel()

		fn := nameOf("want")
		var set render.ImportSet
		out, err := backend.Scaffold(emit.Stmt{
			Kind: emit.StmtExpr,
			Value: emit.Expr{Kind: emit.ExprCall, Fn: &fn, Args: []emit.Expr{
				nameOf("got"), emit.ValueExpr(emit.Literal(emit.LiteralInt, "42")),
			}},
		}, &set)
		assert.NoError(t, err, "the statement spells")
		assert.Equal(t, string(out), "\twant(got, 42)\n",
			"a sample is an argument like any other, which is what a check's call needs")
	})
}
