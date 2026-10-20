// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"slices"
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
		// The kinds of member whose name TypeScript writes as a key, and the
		// words that TypeScript reserves in some contexts alone.
		memberKinds := []symbol.Kind{symbol.KindField, symbol.KindMethod, symbol.KindEnumVariant, symbol.KindParam}
		contextual := []string{"await", "eval", "arguments"}
		tests := slices.Grow([]spelling{
			{
				name: "returns the Pascal-case spelling of a class name",
				kind: symbol.KindStruct, vis: symbol.VisibilityUnknown, give: "httpRow", want: "HttpRow",
			},
			{
				name: "returns the name of a method that the source declares in camel case unchanged",
				host: symbol.KindInterface, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "getById", want: "getById",
			},
			{
				name: "returns the acronym run of a method unchanged",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "toJSON", want: "toJSON",
			},
			{
				name: "returns the camel-case spelling of a snake-case parameter without an initialism",
				host: symbol.KindMethod, kind: symbol.KindParam, vis: symbol.VisibilityUnknown,
				give: "user_id", want: "userId",
			},
			{
				name: "returns a class name with a letter outside ASCII",
				kind: symbol.KindStruct, vis: symbol.VisibilityUnknown, give: "Caf\U000000e9", want: "Caf\U000000e9",
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
			{
				name: "returns a method named after a reserved word unchanged",
				host: symbol.KindInterface, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "delete", want: "delete",
			},
			{
				name: "returns a field named after a reserved word unchanged",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "default", want: "default",
			},
		}, len(memberKinds)+len(contextual))
		for _, kind := range memberKinds {
			tests = append(tests, spelling{
				name: "returns a non-identifier name of a " + kind.String() + " unchanged",
				host: symbol.KindStruct, kind: kind, give: wireName, want: wireName,
			})
		}
		for _, word := range contextual {
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
		// The kinds of declaration for which Name refuses a non-identifier
		// name.
		declarationKinds := []symbol.Kind{
			symbol.KindStruct, symbol.KindInterface, symbol.KindFunction,
			symbol.KindAlias, symbol.KindConstant, symbol.KindVariable,
		}
		refusals := slices.Grow([]refusal{
			{
				name: "returns an error for a function named after a reserved word",
				kind: symbol.KindFunction,
				give: "delete",
			},
			{name: "returns an error for a variable named yield", kind: symbol.KindVariable, give: "yield"},
			{name: "returns an error for a parameter named after a reserved word", kind: symbol.KindParam, give: "new"},
			{
				name: "returns an error for a type parameter named after a predefined type",
				kind: symbol.KindTypeParam, give: "string",
			},
		}, len(declarationKinds)+len(contextual))
		for _, kind := range declarationKinds {
			refusals = append(refusals, refusal{
				name: "returns an error for a non-identifier name of a " + kind.String(), kind: kind, give: wireName,
			})
		}
		for _, word := range contextual {
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

		identifiers := []struct {
			name string
			give string
		}{
			{name: "reports true for a letter", give: "a"},
			{name: "reports true for an underscore before a letter", give: "_x"},
			{name: "reports true for a dollar", give: "$"},
			{name: "reports true for a digit after a letter", give: "row1"},
			{name: "reports true for a dollar and an underscore after letters", give: "Row_$2"},
			{name: "reports true for a letter outside ASCII", give: "\U000000e9"},
			{name: "reports true for a letter number", give: "\U0000216b"},
			{name: "reports true for a zero-width non-joiner between letters", give: "a\U0000200cb"},
		}
		for _, tt := range identifiers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, spell.IsIdentifier(tt.give), "the name spells bare")
			})
		}

		others := []struct {
			name string
			give string
		}{
			{name: "reports false for a digit first", give: "1row"},
			{name: "reports false for a hyphen", give: wireName},
			{name: "reports false for a space", give: "a b"},
			{name: "reports false for a combining mark first", give: "\U00000301a"},
			{name: "reports false for a line separator", give: "a\U00002028"},
			{name: "reports false for the empty name", give: ""},
		}
		for _, tt := range others {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.False(t, spell.IsIdentifier(tt.give), "the name does not spell bare")
			})
		}
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
	assert.Equal(t, got, "HttpRow", "Name converts the name")
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
		{name: "a name in another style", give: "httpRow", allocs: conversionAllocs},
		{name: "a name in its style", give: "HttpRow"},
	}
	b.Run("Name", func(b *testing.B) {
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
				assert.Equal(b, got, "HttpRow", "Name returns the Pascal-case spelling")
			})
		}
	})

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
