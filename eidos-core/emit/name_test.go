// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// declared is the name every fixture declaration declares.
const declared = "Store"

// A file-level declaration's name is what the file's scope binds, so
// the render reserves it and the layout resolves references by it.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("DeclaredName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{name: "returns a struct's name", give: &emit.Struct{Name: declared}, want: declared},
			{name: "returns an interface's name", give: &emit.Interface{Name: declared}, want: declared},
			{name: "returns an enum's name", give: &emit.Enum{Name: declared}, want: declared},
			{name: "returns a sum's name", give: &emit.Sum{Name: declared}, want: declared},
			{name: "returns a function's name", give: &emit.Function{Name: declared}, want: declared},
			{name: "returns an alias's name", give: &emit.Alias{Name: declared}, want: declared},
			{name: "returns a constant's name", give: &emit.Constant{Name: declared}, want: declared},
			{name: "returns a variable's name", give: &emit.Variable{Name: declared}, want: declared},
			{name: "returns the empty string for a method", give: &emit.Method{Name: declared}, want: ""},
			{name: "returns the empty string for a field", give: &emit.Field{Name: declared}, want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, emit.DeclaredName(tt.give), tt.want, "the file-level name")
			})
		}
	})
}
