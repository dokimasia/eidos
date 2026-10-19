// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// conversionAllocs is a name the convention converts: the converted
// spelling.
const conversionAllocs = 1

// The settle spells every declared name through the convention of
// protobuf's style guide. Each mapping is pinned byte for byte.
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
				name: "returns the Pascal-case spelling of a message",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "session_state", want: "SessionState",
			},
			{
				name: "returns the Pascal-case spelling of a message with an abbreviation as one word",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "get_dns_request", want: "GetDnsRequest",
			},
			{
				name: "returns a message name in its style unchanged",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "HttpRule", want: "HttpRule",
			},
			{
				name: "returns the Pascal-case spelling of a nested message",
				host: symbol.KindStruct, kind: symbol.KindStruct, vis: symbol.VisibilityPublic,
				give: "entry", want: "Entry",
			},
			{
				name: "returns the Pascal-case spelling of an enum",
				kind: symbol.KindEnum, vis: symbol.VisibilityPublic, give: "status", want: "Status",
			},
			{
				name: "returns the Pascal-case spelling of a service",
				kind: symbol.KindInterface, vis: symbol.VisibilityPublic, give: "rowStore", want: "RowStore",
			},
			{
				name: "returns the Pascal-case spelling of an rpc",
				host: symbol.KindInterface, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "getRow", want: "GetRow",
			},
			{
				name: "returns the Pascal-case spelling of a sum at file level",
				kind: symbol.KindSum, vis: symbol.VisibilityPublic, give: "result", want: "Result",
			},
			{
				name: "returns the snake-case spelling of a field",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "ExpiresAt", want: "expires_at",
			},
			{
				name: "returns the snake-case spelling of an extension",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "LegacyID", want: "legacy_id",
			},
			{
				name: "returns the snake-case spelling of a oneof",
				host: symbol.KindStruct, kind: symbol.KindSum, vis: symbol.VisibilityPublic,
				give: "Payload", want: "payload",
			},
			{
				name: "returns the snake-case spelling of a member of a oneof",
				host: symbol.KindSum, kind: symbol.KindSumVariant, vis: symbol.VisibilityUnknown,
				give: "TextBody", want: "text_body",
			},
			{
				name: "returns the screaming-snake-case spelling of an enum value",
				host: symbol.KindEnum, kind: symbol.KindEnumVariant, vis: symbol.VisibilityUnknown,
				give: "stateActive", want: "STATE_ACTIVE",
			},
			{
				name: "returns the snake-case spelling of a parameter",
				host: symbol.KindMethod, kind: symbol.KindParam, vis: symbol.VisibilityUnknown,
				give: "rowID", want: "row_id",
			},
			{
				name: "returns the snake-case spelling of a named result",
				host: symbol.KindMethod, kind: symbol.KindReturn, vis: symbol.VisibilityUnknown,
				give: "nextToken", want: "next_token",
			},
			{
				name: "returns a type parameter's spelling unchanged",
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
				give: "expires_at", want: "expires_at",
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
				name: "returns an error for a name that is not an identifier",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "content-type",
			},
			{
				name: "returns an error for a name led by a digit",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "9lives",
			},
			{
				name: "returns an error for a name of separators alone",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "__",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Name(symbol.KindStruct, tt.kind, tt.vis, tt.give)
				assert.HasError(t, err, "protobuf declares no such name")
				assert.Contains(t, err.Error(), tt.give, "the error names the name")
			})
		}
	})
}

// Name allocates a converted spelling, and nothing for a name the
// convention spells as it is. The ordinary run, which runs no benchmark,
// checks those ceilings here.
func TestNameAllocs(t *testing.T) {
	var (
		got string
		err error
	)
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, "expiresAt")
	}, conversionAllocs, "Name allocates the converted spelling")
	assert.NoError(t, err, "Name spells the name")
	assert.Equal(t, got, "expires_at", "Name converts the name")
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, "expires_at")
	}, 0, "Name allocates nothing for a name in its style")
	assert.NoError(t, err, "Name spells the styled name")
	assert.Equal(t, got, "expires_at", "Name returns the styled name")
}

// BenchmarkName measures the spelling the settle makes for every declared
// name.
func BenchmarkName(b *testing.B) {
	names := []struct {
		name   string
		give   string
		allocs uint64
	}{
		{name: "a name in another style", give: "expiresAt", allocs: conversionAllocs},
		{name: "a name in its style", give: "expires_at"},
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
				assert.Equal(b, got, "expires_at", "Name returns the snake-case spelling")
			})
		}
	})
}
