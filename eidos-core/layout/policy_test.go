// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/layout"
)

// A policy is a closed vocabulary a configuration spells.
func TestPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give layout.Policy
			want bool
		}{
			{name: "reports true for PolicyInherit", give: layout.PolicyInherit, want: true},
			{name: "reports true for PolicyAlongside", give: layout.PolicyAlongside, want: true},
			{name: "reports true for PolicyCentralised", give: layout.PolicyCentralised, want: true},
			{name: "reports false for the first undeclared policy", give: layout.Policy(3), want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Valid(), tt.want, "the policy's validity")
			})
		}
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give layout.Policy
			want string
		}{
			{name: "returns inherit for PolicyInherit", give: layout.PolicyInherit, want: "inherit"},
			{
				name: "returns alongside-source for PolicyAlongside",
				give: layout.PolicyAlongside,
				want: "alongside-source",
			},
			{name: "returns centralised for PolicyCentralised", give: layout.PolicyCentralised, want: "centralised"},
			{name: "returns the number of an undeclared policy", give: layout.Policy(3), want: "Policy(3)"},
			{name: "returns every digit of an undeclared policy's number", give: layout.Policy(12), want: "Policy(12)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}

// A declared policy's checks allocate nothing in the ordinary run,
// which runs no benchmark.
func TestPolicyZeroAlloc(t *testing.T) {
	var valid bool
	assert.MaxAllocs(t, func() { valid = layout.PolicyCentralised.Valid() }, 0, "Valid allocates nothing")
	assert.True(t, valid, "Valid admits a declared policy")
	var spelt string
	assert.MaxAllocs(t, func() { spelt = layout.PolicyCentralised.String() }, 0,
		"String allocates nothing for a declared policy")
	assert.Equal(t, spelt, "centralised", "String spells the policy")
}

// BenchmarkPolicy measures a declared policy's checks.
func BenchmarkPolicy(b *testing.B) {
	b.Run("Valid", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = layout.PolicyCentralised.Valid()
		}
		assert.True(b, got, "Valid admits a declared policy")
	})

	b.Run("String", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = layout.PolicyCentralised.String()
		}
		assert.Equal(b, got, "centralised", "String spells the policy")
	})
}
