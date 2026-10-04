// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// conversionAllocs is a name the convention converts: the converted
// spelling.
const conversionAllocs = 1

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
				name: "returns the Pascal-case spelling of a public name",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "httpRow", want: "HTTPRow",
			},
			{
				name: "returns the Pascal-case spelling of a name of an unstated scope",
				kind: symbol.KindFunction, vis: symbol.VisibilityUnknown, give: "fetchRow", want: "FetchRow",
			},
			{
				name: "returns the camel-case spelling of a package-scoped name",
				kind: symbol.KindConstant, vis: symbol.VisibilityPackage, give: "MaxRows", want: "maxRows",
			},
			{
				name: "returns the camel-case spelling of a parameter of an unstated scope",
				host: symbol.KindFunction, kind: symbol.KindParam, vis: symbol.VisibilityUnknown,
				give: "RowCount", want: "rowCount",
			},
			{
				name: "returns the camel-case spelling of a named result",
				host: symbol.KindFunction, kind: symbol.KindReturn, vis: symbol.VisibilityUnknown,
				give: "errOut", want: "errOut",
			},
			{
				name: "returns a type parameter's spelling unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, vis: symbol.VisibilityUnknown,
				give: "T", want: "T",
			},
			{
				name: "returns the camel-case spelling of a package-scoped field",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPackage,
				give: "rowKey", want: "rowKey",
			},
			{
				name: "returns the Pascal-case spelling of a public method",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "fetch", want: "Fetch",
			},
			{
				name: "returns the blank identifier of a field unchanged",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "_", want: "_",
			},
			{
				name: "returns the blank identifier of a parameter unchanged",
				host: symbol.KindStruct, kind: symbol.KindParam, vis: symbol.VisibilityPublic, give: "_", want: "_",
			},
			{
				name: "returns the blank identifier of a type parameter unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, vis: symbol.VisibilityPublic, give: "_", want: "_",
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
				name: "returns an error for a protected scope",
				kind: symbol.KindStruct, vis: symbol.VisibilityProtected, give: "row",
			},
			{
				name: "returns an error for a name led by a digit",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "9lives",
			},
			{
				name: "returns an error for a package-scoped name that spells a keyword",
				kind: symbol.KindConstant, vis: symbol.VisibilityPackage, give: "type",
			},
			{
				name: "returns an error for a parameter that spells a keyword",
				kind: symbol.KindParam, vis: symbol.VisibilityUnknown, give: "func",
			},
			{
				name: "returns an error for a name of separators alone",
				kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "__",
			},
			{
				name: "returns an error for a type parameter that is a keyword",
				kind: symbol.KindTypeParam, vis: symbol.VisibilityUnknown, give: "range",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Name(symbol.KindStruct, tt.kind, tt.vis, tt.give)
				assert.HasError(t, err, "Go declares no such name")
				assert.Contains(t, err.Error(), tt.give, "the error names the name")
			})
		}
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
	assert.Equal(t, got, "HTTPRow", "Name converts the name")
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
			assert.Equal(b, got, "HTTPRow", "Name returns the exported spelling")
		})
	}
}
