// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// conversionAllocs is a name that the convention converts: the converted
// spelling.
const conversionAllocs = 1

// directiveAllocs is the name of a directive in another style: the
// converted name and the name with its sigil.
const directiveAllocs = 2

// The field name of the allocation cases in the convention of GraphQL and
// in another style.
const (
	createdAt      = "createdAt"
	createdAtSnake = "created_at"
)

// TestName pins the spelling of every kind of declared name by the guide
// to naming of graphql.org, byte for byte.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			host symbol.Kind
			kind symbol.Kind
			vis  symbol.Visibility
			give string
			want string
		}{
			{
				name: "returns the Pascal-case spelling of an object type",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "catalog_item", want: "CatalogItem",
			},
			{
				name: "returns the Pascal-case spelling of an interface",
				kind: symbol.KindInterface, vis: symbol.VisibilityPublic, give: "node", want: "Node",
			},
			{
				name: "returns the Pascal-case spelling of an enum",
				kind: symbol.KindEnum, vis: symbol.VisibilityPublic, give: "status", want: "Status",
			},
			{
				name: "returns the Pascal-case spelling of a union",
				kind: symbol.KindAlias, vis: symbol.VisibilityPublic, give: "searchResult", want: "SearchResult",
			},
			{
				name: "returns the Pascal-case spelling of a sum",
				kind: symbol.KindSum, vis: symbol.VisibilityPublic, give: "shape", want: "Shape",
			},
			{
				name: "returns the Pascal-case spelling of an operation",
				kind: symbol.KindFunction, vis: symbol.VisibilityPublic, give: "getUser", want: "GetUser",
			},
			{
				name: "returns the spelling of an operation that a built-in scalar spells",
				kind: symbol.KindFunction, vis: symbol.VisibilityPublic, give: "id", want: "ID",
			},
			{
				name: "returns the camel-case spelling of a field",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "CreatedAt", want: createdAt,
			},
			{
				name: "returns the camel-case spelling of a field with arguments",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "Search", want: "search",
			},
			{
				name: "returns the camel-case spelling of an argument",
				host: symbol.KindMethod, kind: symbol.KindParam, vis: symbol.VisibilityUnknown,
				give: "first_count", want: "firstCount",
			},
			{
				name: "returns the camel-case spelling of a named result",
				host: symbol.KindMethod, kind: symbol.KindReturn, vis: symbol.VisibilityUnknown,
				give: "NextToken", want: "nextToken",
			},
			{
				name: "returns the camel-case spelling of a sum variant",
				host: symbol.KindSum, kind: symbol.KindSumVariant, vis: symbol.VisibilityUnknown,
				give: "TextBody", want: "textBody",
			},
			{
				name: "returns the screaming-snake-case spelling of an enum value",
				host: symbol.KindEnum, kind: symbol.KindEnumVariant, vis: symbol.VisibilityUnknown,
				give: "inProgress", want: "IN_PROGRESS",
			},
			{
				name: "returns the camel-case spelling of a directive with its sigil",
				kind: symbol.KindInterface, vis: symbol.VisibilityPublic, give: "@requires_role", want: "@requiresRole",
			},
			{
				name: "returns the spelling of a type parameter unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, vis: symbol.VisibilityUnknown,
				give: "T", want: "T",
			},
			{
				name: "returns the spelling of a name of an unstated scope",
				kind: symbol.KindStruct, vis: symbol.VisibilityUnknown, give: "row", want: "Row",
			},
			{
				name: "returns a field name in its style unchanged",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: createdAt, want: createdAt,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Name(tt.host, tt.kind, tt.vis, tt.give)
				assert.NoError(t, err, "the name spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		refusals := []struct {
			name string
			kind symbol.Kind
			vis  symbol.Visibility
			give string
		}{
			{
				name: "returns an error for a private name",
				kind: symbol.KindStruct, vis: symbol.VisibilityPrivate, give: "row",
			},
			{
				name: "returns an error for a package-scoped name",
				kind: symbol.KindField, vis: symbol.VisibilityPackage, give: "rowKey",
			},
			{
				name: "returns an error for a name that is not a GraphQL name",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "content-type",
			},
			{
				name: "returns an error for a name led by a digit",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "9lives",
			},
			{
				name: "returns an error for a name with a letter outside ASCII",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "größe",
			},
			{
				name: "returns an error for a name that introspection reserves",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "__typename",
			},
			{
				name: "returns an error for a directive name that introspection reserves",
				kind: symbol.KindInterface, vis: symbol.VisibilityPublic, give: "@__auth",
			},
			{
				name: "returns an error for a sigil without a name",
				kind: symbol.KindInterface, vis: symbol.VisibilityPublic, give: "@",
			},
			{
				name: "returns an error for a sigil before the name of a type",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "@row",
			},
			{
				name: "returns an error for a name of a separator alone",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "_",
			},
			{
				name: "returns an error for an object type that spells a built-in scalar",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "id",
			},
			{
				name: "returns an error for a scalar that spells a built-in scalar",
				kind: symbol.KindAlias, vis: symbol.VisibilityPublic, give: "string",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Name(symbol.KindInvalid, tt.kind, tt.vis, tt.give)
				assert.HasError(t, err, "GraphQL declares no such name")
				assert.Contains(t, err.Error(), tt.give, "the error contains the name")
			})
		}
	})
}

// TestNameAllocs checks that Name allocates a converted spelling, the
// sigil of a directive, and nothing for a name in its style, in the
// ordinary run, which does not run benchmarks.
func TestNameAllocs(t *testing.T) {
	var (
		got string
		err error
	)
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, createdAtSnake)
	}, conversionAllocs, "Name allocates the converted spelling")
	assert.NoError(t, err, "Name spells the name")
	assert.Equal(t, got, createdAt, "Name converts the name")
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, createdAt)
	}, 0, "Name does not allocate for a name in its style")
	assert.NoError(t, err, "Name spells the name in its style")
	assert.Equal(t, got, createdAt, "Name returns the name in its style")
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindInvalid, symbol.KindInterface, symbol.VisibilityPublic, "@requires_role")
	}, directiveAllocs, "Name allocates the converted name of a directive and its sigil")
	assert.NoError(t, err, "Name spells the directive")
	assert.Equal(t, got, "@requiresRole", "Name converts the directive")
}

// BenchmarkName measures the spelling that the settle makes for every
// declared name.
func BenchmarkName(b *testing.B) {
	names := []struct {
		name   string
		give   string
		allocs uint64
	}{
		{name: "a name in another style", give: createdAtSnake, allocs: conversionAllocs},
		{name: "a name in its style", give: createdAt},
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
					got, err = spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, tt.give)
				}
				assert.NoError(b, err, "Name spells the name")
				assert.Equal(b, got, createdAt, "Name returns the camel-case spelling")
			})
		}
	})
}
