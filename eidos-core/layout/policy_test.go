// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"

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
