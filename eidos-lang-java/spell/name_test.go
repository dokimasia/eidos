// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// conversionAllocs is a name the convention converts: the converted
// spelling.
const conversionAllocs = 1

// maxRows is the name of a constant of a class in Java's convention.
const maxRows = "MAX_ROWS"

// staticFinal is a static final field of a class.
var staticFinal = plugin.Member{
	Host: symbol.KindStruct, Kind: symbol.KindField, Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable,
}

// The settle spells every declared name through the convention. Each
// mapping is pinned byte for byte.
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
				name: "returns the camel-case spelling of a method of any scope",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityProtected,
				give: "FetchRow", want: "fetchRow",
			},
			{
				name: "returns the camel-case spelling of a class field",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "rowKey", want: "rowKey",
			},
			{
				name: "returns the screaming-snake spelling of an interface field",
				host: symbol.KindInterface, kind: symbol.KindField, vis: symbol.VisibilityUnknown,
				give: "maxRows", want: "MAX_ROWS",
			},
			{
				name: "returns the screaming-snake spelling of a constant",
				kind: symbol.KindConstant, vis: symbol.VisibilityPublic, give: "maxRows", want: "MAX_ROWS",
			},
			{
				name: "returns the screaming-snake spelling of an enum constant",
				kind: symbol.KindEnumVariant, vis: symbol.VisibilityUnknown, give: "rowOpen", want: "ROW_OPEN",
			},
			{
				name: "returns a type parameter's spelling unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, vis: symbol.VisibilityUnknown,
				give: "T", want: "T",
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
			give string
			want string
		}{
			{
				name: "returns an error for a wire name outside the identifier shape",
				give: "content-type",
				want: "content-type",
			},
			{name: "returns an error for a spelling that is a reserved word", give: "class", want: "reserved"},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, tt.give)
				assert.HasError(t, err, "Java declares no such name")
				assert.Contains(t, err.Error(), tt.want, "the error names the cause")
			})
		}
	})

	t.Run("Member", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Member
			want string
		}{
			{
				name: "returns the spelling of a static final field without a lower-case letter unchanged",
				give: staticFinal, want: maxRows,
			},
			{
				name: "returns the camel-case spelling of a static field that is mutable",
				give: plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindField, Level: symbol.LevelType},
				want: "maxRows",
			},
			{
				name: "returns the camel-case spelling of an immutable field of an instance",
				give: plugin.Member{
					Host: symbol.KindStruct, Kind: symbol.KindField, Mutability: symbol.MutabilityImmutable,
				},
				want: "maxRows",
			},
			{
				name: "returns the camel-case spelling of a method at the level of the type",
				give: plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindMethod, Level: symbol.LevelType},
				want: "maxRows",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Member(tt.give, maxRows)
				assert.NoError(t, err, "the name spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		t.Run("returns the camel-case spelling of a static final field with a lower-case letter", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Member(staticFinal, "row_logger")
			assert.NoError(t, err, "the name spells")
			assert.Equal(t, got, "rowLogger", "the spelling of a final reference")
		})

		t.Run("returns an error for a static final field whose name is no identifier", func(t *testing.T) {
			t.Parallel()

			_, err := spell.Member(staticFinal, "MAX-ROWS")
			assert.HasError(t, err, "Java declares no such name")
			assert.Contains(t, err.Error(), "MAX-ROWS", "the error names the name")
		})
	})
}

// Name allocates a converted spelling, and nothing for a name the
// convention spells as it is. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestNameAllocs(t *testing.T) {
	var (
		got string
		err error
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
	assert.MaxAllocs(t, func() {
		got, err = spell.Member(staticFinal, maxRows)
	}, 0, "Member allocates nothing for the name of a constant")
	assert.NoError(t, err, "Member spells the name of the constant")
	assert.Equal(t, got, maxRows, "Member keeps the name of the constant")
}

// BenchmarkName measures the spelling the settle makes for every
// declared name.
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
	b.Run("Member", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got string
			err error
		)
		for c.Loop() {
			got, err = spell.Member(staticFinal, maxRows)
		}
		assert.NoError(b, err, "Member spells the name of the constant")
		assert.Equal(b, got, maxRows, "Member keeps the name of the constant")
	})
}
