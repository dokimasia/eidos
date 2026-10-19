// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// declared is the name every fixture declaration declares.
const declared = "Store"

// TestName pins the name that a file-level declaration declares in the
// scope of its file, and the type that a declaration attaches to from
// outside its type.
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
			{
				name: "returns the empty string for a field that another type receives",
				give: &emit.Field{Name: declared, Receives: &emit.TypeRef{Spelling: declared}}, want: "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, emit.DeclaredName(tt.give), tt.want, "the file-level name")
			})
		}
	})

	t.Run("ReceivesOf", func(t *testing.T) {
		t.Parallel()

		receiver := &emit.TypeRef{Spelling: declared}
		tests := []struct {
			name string
			give symbol.Symbol
			want *emit.TypeRef
		}{
			{
				name: "returns the type that a method attaches to",
				give: &emit.Method{Receives: receiver}, want: receiver,
			},
			{
				name: "returns the type that a field extends",
				give: &emit.Field{Receives: receiver}, want: receiver,
			},
			{name: "returns nil for a method inside its type", give: &emit.Method{Name: declared}, want: nil},
			{name: "returns nil for a field inside its type", give: &emit.Field{Name: declared}, want: nil},
			{name: "returns nil for a struct", give: &emit.Struct{Name: declared}, want: nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, emit.ReceivesOf(tt.give), tt.want, "ReceivesOf returns the receiver of the declaration",
					assert.ByIdentity())
			})
		}
	})
}

// TestNameAllocs checks that a name and a receiver read without
// allocating, in the ordinary run, which runs no benchmark.
func TestNameAllocs(t *testing.T) {
	var d symbol.Symbol = &emit.Variable{Name: declared}
	var got string
	assert.MaxAllocs(t, func() { got = emit.DeclaredName(d) }, 0, "DeclaredName allocates nothing")
	assert.Equal(t, got, declared, "DeclaredName returns the variable's name")

	receiver := &emit.TypeRef{Spelling: declared}
	var extension symbol.Symbol = &emit.Field{Receives: receiver}
	var receives *emit.TypeRef
	assert.MaxAllocs(t, func() { receives = emit.ReceivesOf(extension) }, 0, "ReceivesOf allocates nothing")
	assert.Equal(t, receives, receiver, "ReceivesOf returns the type that the field extends", assert.ByIdentity())
}

// BenchmarkName measures the name a variable declares, the last kind
// the switch tests, and the type that a field receives, the last kind
// with a receiver.
func BenchmarkName(b *testing.B) {
	b.Run("DeclaredName", func(b *testing.B) {
		var d symbol.Symbol = &emit.Variable{Name: declared}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = emit.DeclaredName(d)
		}
		assert.Equal(b, got, declared, "DeclaredName returns the variable's name")
	})

	b.Run("ReceivesOf", func(b *testing.B) {
		receiver := &emit.TypeRef{Spelling: declared}
		var d symbol.Symbol = &emit.Field{Receives: receiver}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got *emit.TypeRef
		for c.Loop() {
			got = emit.ReceivesOf(d)
		}
		assert.Equal(b, got, receiver, "ReceivesOf returns the type that the field extends", assert.ByIdentity())
	})
}
