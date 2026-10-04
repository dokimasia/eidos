// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// wireName is a name that is no identifier, such as a header a field
// maps.
const wireName = "content-type"

// conversionAllocs is a name the convention converts: the converted
// spelling.
const conversionAllocs = 1

// The settle spells every declared name through the convention. Each
// mapping, and each name TypeScript refuses, is pinned byte for byte.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		type spelling struct {
			name string
			host symbol.Kind
			kind symbol.Kind
			vis  symbol.Visibility
			give string
			want string
		}
		tests := []spelling{
			{
				name: "returns the Pascal-case spelling of a class name",
				kind: symbol.KindStruct, vis: symbol.VisibilityUnknown, give: "httpRow", want: "HTTPRow",
			},
			{
				name: "returns the camel-case spelling of a method",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "FetchRow", want: "fetchRow",
			},
			{
				name: "returns the camel-case spelling of a private field",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPrivate,
				give: "RowKey", want: "rowKey",
			},
			{
				name: "returns the screaming-snake spelling of a constant",
				kind: symbol.KindConstant, vis: symbol.VisibilityUnknown, give: "maxRows", want: "MAX_ROWS",
			},
			{
				name: "returns a type parameter's spelling unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, vis: symbol.VisibilityUnknown,
				give: "T", want: "T",
			},
			{
				name: "returns the Pascal-case spelling of an enum member",
				kind: symbol.KindEnumVariant, vis: symbol.VisibilityUnknown, give: "rowOpen", want: "RowOpen",
			},
			{
				name: "returns a field named after a predefined type unchanged",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityUnknown,
				give: "string", want: "string",
			},
		}
		for _, kind := range []symbol.Kind{
			symbol.KindField, symbol.KindMethod, symbol.KindEnumVariant, symbol.KindParam,
		} {
			tests = append(tests, spelling{
				name: "returns a non-identifier name of a " + kind.String() + " unchanged",
				host: symbol.KindStruct, kind: kind, give: wireName, want: wireName,
			})
		}
		for _, word := range []string{"await", "eval", "arguments"} {
			tests = append(tests, spelling{
				name: "returns the member key " + word + " unchanged",
				host: symbol.KindStruct, kind: symbol.KindField, give: word, want: word,
			})
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Name(tt.host, tt.kind, tt.vis, tt.give)
				assert.NoError(t, err, "the name spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		type refusal struct {
			name string
			kind symbol.Kind
			give string
		}
		refusals := []refusal{
			{
				name: "returns an error for a function named after a reserved word",
				kind: symbol.KindFunction,
				give: "delete",
			},
			{name: "returns an error for a variable named yield", kind: symbol.KindVariable, give: "yield"},
			{
				name: "returns an error for a type parameter named after a predefined type",
				kind: symbol.KindTypeParam, give: "string",
			},
		}
		for _, kind := range []symbol.Kind{
			symbol.KindStruct, symbol.KindInterface, symbol.KindFunction,
			symbol.KindAlias, symbol.KindConstant, symbol.KindVariable,
		} {
			refusals = append(refusals, refusal{
				name: "returns an error for a non-identifier name of a " + kind.String(), kind: kind, give: wireName,
			})
		}
		for _, word := range []string{"await", "eval", "arguments"} {
			refusals = append(refusals, refusal{
				name: "returns an error for a variable named " + word, kind: symbol.KindVariable, give: word,
			})
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Name(symbol.KindInvalid, tt.kind, symbol.VisibilityUnknown, tt.give)
				assert.HasError(t, err, "TypeScript declares no such name")
				assert.Contains(t, err.Error(), tt.give, "the error names the name")
			})
		}
	})

	t.Run("IsIdentifier", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"a", "_x", "$", "row1", "Row_$2"} {
			t.Run("reports true for "+name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, spell.IsIdentifier(name), "the name spells bare")
			})
		}
		for _, name := range []string{"1row", wireName, "é", "a b"} {
			t.Run("reports false for "+name, func(t *testing.T) {
				t.Parallel()

				assert.False(t, spell.IsIdentifier(name), "the name does not spell bare")
			})
		}

		t.Run("reports false for the empty name", func(t *testing.T) {
			t.Parallel()

			assert.False(t, spell.IsIdentifier(""), "an empty name spells nothing")
		})
	})
}

// Name allocates a converted spelling, and nothing for a name the
// convention spells as it is. IsIdentifier allocates nothing. The
// ordinary run, which runs no benchmark, checks those ceilings here.
func TestNameAllocs(t *testing.T) {
	var (
		got   string
		err   error
		valid bool
	)
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindInvalid, symbol.KindStruct, symbol.VisibilityPublic, "httpRow")
	}, conversionAllocs, "Name allocates the converted spelling")
	assert.NoError(t, err, "Name spells the name")
	assert.Equal(t, got, "HTTPRow", "Name converts the name")
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindInvalid, symbol.KindStruct, symbol.VisibilityPublic, "Row")
	}, 0, "Name allocates nothing for a name in its style")
	assert.NoError(t, err, "Name spells the styled name")
	assert.Equal(t, got, "Row", "Name returns the styled name")
	assert.MaxAllocs(t, func() { valid = spell.IsIdentifier("rowCount") }, 0, "IsIdentifier allocates nothing")
	assert.True(t, valid, "IsIdentifier reports an identifier")
}

// BenchmarkName measures the spelling the settle makes for every
// declared name, and the identifier check the backend makes for every
// member key.
func BenchmarkName(b *testing.B) {
	names := []struct {
		name   string
		give   string
		allocs uint64
	}{
		{name: "Name", give: "httpRow", allocs: conversionAllocs},
		{name: "Name/a name in its style", give: "HTTPRow"},
	}
	for _, tt := range names {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			var (
				got string
				err error
			)
			for c.Loop() {
				got, err = spell.Name(symbol.KindInvalid, symbol.KindStruct, symbol.VisibilityPublic, tt.give)
			}
			assert.NoError(b, err, "Name spells the name")
			assert.Equal(b, got, "HTTPRow", "Name returns the Pascal-case spelling")
		})
	}

	b.Run("IsIdentifier", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var valid bool
		for c.Loop() {
			valid = spell.IsIdentifier("rowCount")
		}
		assert.True(b, valid, "IsIdentifier reports an identifier")
	})
}
