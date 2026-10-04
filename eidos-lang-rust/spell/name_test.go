// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of a name.
const (
	// conversionAllocs is a name the convention converts: the converted
	// spelling.
	conversionAllocs = 1
	// rawAllocs is a keyword's raw form: r# before the spelling.
	rawAllocs = 1
)

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
				name: "returns the Pascal-case spelling of a struct name",
				kind: symbol.KindStruct, vis: symbol.VisibilityUnknown, give: "httpRow", want: "HTTPRow",
			},
			{
				name: "returns the snake-case spelling of a method of any scope",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityInternal,
				give: "FetchRow", want: "fetch_row",
			},
			{
				name: "returns the snake-case spelling of a parameter",
				host: symbol.KindFunction, kind: symbol.KindParam, vis: symbol.VisibilityUnknown,
				give: "rowCount", want: "row_count",
			},
			{
				name: "returns the screaming-snake spelling of a constant",
				kind: symbol.KindConstant, vis: symbol.VisibilityUnknown, give: "maxRows", want: "MAX_ROWS",
			},
			{
				name: "returns the Pascal-case spelling of a variant",
				kind: symbol.KindSumVariant, vis: symbol.VisibilityUnknown, give: "rowOpen", want: "RowOpen",
			},
			{
				name: "returns a type parameter's spelling unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, vis: symbol.VisibilityUnknown,
				give: "N", want: "N",
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

		for _, keyword := range []string{"type", "box", "try", "gen", "yield", "abstract"} {
			t.Run("returns the raw form of the keyword "+keyword, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, keyword)
				assert.NoError(t, err, "a keyword takes the raw form")
				assert.Equal(t, got, "r#"+keyword, "the spelling behind r#")
			})
		}

		refusals := []struct {
			name string
			kind symbol.Kind
			give string
		}{
			{name: "returns an error for a keyword the raw form cannot express", kind: symbol.KindField, give: "self"},
			{name: "returns an error for a blank type parameter", kind: symbol.KindTypeParam, give: "_"},
			{
				name: "returns an error for a wire name outside the identifier shape",
				kind: symbol.KindField,
				give: "content-type",
			},
			{
				name: "returns an error for a name whose snake-case spelling is empty",
				kind: symbol.KindFunction,
				give: "__",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Name(symbol.KindStruct, tt.kind, symbol.VisibilityPublic, tt.give)
				assert.HasError(t, err, "Rust declares no such name")
				assert.Contains(t, err.Error(), tt.give, "the error names the name")
			})
		}
	})
}

// Name allocates a converted spelling and a raw form, and nothing for a
// name the convention spells as it is. The ordinary run, which runs no
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
	assert.Equal(t, got, "HTTPRow", "Name converts the name")
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindStruct, symbol.KindField, symbol.VisibilityPublic, "type")
	}, rawAllocs, "Name allocates the raw form")
	assert.NoError(t, err, "Name spells the keyword")
	assert.Equal(t, got, "r#type", "Name returns the raw form")
	assert.MaxAllocs(t, func() {
		got, err = spell.Name(symbol.KindInvalid, symbol.KindStruct, symbol.VisibilityPublic, "Row")
	}, 0, "Name allocates nothing for a name in its style")
	assert.NoError(t, err, "Name spells the styled name")
	assert.Equal(t, got, "Row", "Name returns the styled name")
}

// BenchmarkName measures the spelling the settle makes for every
// declared name.
func BenchmarkName(b *testing.B) {
	names := []struct {
		name   string
		kind   symbol.Kind
		give   string
		allocs uint64
		want   string
	}{
		{
			name:   "a name in another style",
			kind:   symbol.KindStruct,
			give:   "httpRow",
			allocs: conversionAllocs,
			want:   "HTTPRow",
		},
		{name: "a name in its style", kind: symbol.KindStruct, give: "HTTPRow", want: "HTTPRow"},
		{name: "a keyword", kind: symbol.KindField, give: "type", allocs: rawAllocs, want: "r#type"},
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
					got, err = spell.Name(symbol.KindStruct, tt.kind, symbol.VisibilityPublic, tt.give)
				}
				assert.NoError(b, err, "Name spells the name")
				assert.Equal(b, got, tt.want, "Name returns the spelling")
			})
		}
	})
}
