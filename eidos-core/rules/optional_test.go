// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/rules"
)

// The optional capabilities are interfaces a language satisfies, so the
// vocabularies they return are what this file pins.
func TestOptional(t *testing.T) {
	t.Parallel()

	t.Run("EnumForm.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.EnumForm
			want string
		}{
			{name: "returns identifier for EnumIdentifier", give: rules.EnumIdentifier, want: "identifier"},
			{name: "returns value for EnumValue", give: rules.EnumValue, want: "value"},
			{name: "returns the number of an undeclared form", give: rules.EnumForm(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("Ownership.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.Ownership
			want string
		}{
			{name: "returns by-value for OwnByValue", give: rules.OwnByValue, want: "by-value"},
			{name: "returns borrow for OwnBorrow", give: rules.OwnBorrow, want: "borrow"},
			{name: "returns borrow-mut for OwnBorrowMut", give: rules.OwnBorrowMut, want: "borrow-mut"},
			{name: "returns the number of an undeclared ownership", give: rules.Ownership(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}

// A declared form and a declared ownership spell without allocating in
// the ordinary run, which runs no benchmark.
func TestOptionalZeroAlloc(t *testing.T) {
	form, ownership := rules.EnumValue, rules.OwnBorrowMut
	var got string
	assert.MaxAllocs(t, func() { got = form.String() }, 0, "EnumForm.String allocates nothing for a declared form")
	assert.Equal(t, got, "value", "EnumForm.String spells EnumValue")
	assert.MaxAllocs(t, func() { got = ownership.String() }, 0,
		"Ownership.String allocates nothing for a declared ownership")
	assert.Equal(t, got, "borrow-mut", "Ownership.String spells OwnBorrowMut")
}

// BenchmarkOptional measures the spelling of an enumeration's form and
// of a parameter's ownership, which a backend writes once per
// declaration.
func BenchmarkOptional(b *testing.B) {
	b.Run("EnumForm.String", func(b *testing.B) {
		form := rules.EnumValue
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = form.String()
		}
		assert.Equal(b, got, "value", "EnumForm.String spells EnumValue")
	})

	b.Run("Ownership.String", func(b *testing.B) {
		ownership := rules.OwnBorrowMut
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = ownership.String()
		}
		assert.Equal(b, got, "borrow-mut", "Ownership.String spells OwnBorrowMut")
	})
}
