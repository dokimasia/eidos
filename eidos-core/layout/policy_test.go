// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/layout"
)

// A policy is a closed vocabulary a configuration spells.
func TestPolicy(t *testing.T) {
	t.Parallel()

	t.Run("ParsePolicy", func(t *testing.T) {
		t.Parallel()

		policies := []struct {
			name string
			give layout.Policy
		}{
			{name: "returns PolicyInherit for its spelling", give: layout.PolicyInherit},
			{name: "returns PolicyAlongside for its spelling", give: layout.PolicyAlongside},
			{name: "returns PolicyCentralised for its spelling", give: layout.PolicyCentralised},
		}
		for _, tt := range policies {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.RoundTrip(t, func(p layout.Policy) (string, error) { return p.String(), nil },
					layout.ParsePolicy, tt.give, "ParsePolicy reads the spelling that String writes")
			})
		}

		refusals := []struct {
			name string
			give string
		}{
			{name: "returns an error naming a spelling in another case", give: "Centralised"},
			{name: "returns an error naming the empty spelling", give: ""},
			{name: "returns an error naming the spelling of an undeclared policy", give: layout.Policy(3).String()},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := layout.ParsePolicy(tt.give)
				assert.HasError(t, err, "the spelling is not a policy")
				expect.That(t, err.Error()).
					Contains(strconv.Quote(tt.give), "the error names the spelling").
					Contains(layout.PolicyInherit.String(), "the error lists the spelling of PolicyInherit").
					Contains(layout.PolicyAlongside.String(), "the error lists the spelling of PolicyAlongside").
					Contains(layout.PolicyCentralised.String(), "the error lists the spelling of PolicyCentralised")
			})
		}
	})

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

// The parse of a policy's spelling and a declared policy's checks
// allocate nothing in the ordinary run, which runs no benchmark.
func TestPolicyAllocs(t *testing.T) {
	spelling := layout.PolicyCentralised.String()
	var (
		parsed layout.Policy
		err    error
	)
	assert.MaxAllocs(t, func() { parsed, err = layout.ParsePolicy(spelling) }, 0,
		"ParsePolicy allocates nothing for the spelling of a policy")
	assert.NoError(t, err, "ParsePolicy reads the spelling of a policy")
	assert.Equal(t, parsed, layout.PolicyCentralised, "ParsePolicy returns the spelled policy")
	var valid bool
	assert.MaxAllocs(t, func() { valid = layout.PolicyCentralised.Valid() }, 0, "Valid allocates nothing")
	assert.True(t, valid, "Valid admits a declared policy")
	var spelt string
	assert.MaxAllocs(t, func() { spelt = layout.PolicyCentralised.String() }, 0,
		"String allocates nothing for a declared policy")
	assert.Equal(t, spelt, "centralised", "String spells the policy")
}

// BenchmarkPolicy measures the parse of a policy's spelling and a
// declared policy's checks.
func BenchmarkPolicy(b *testing.B) {
	b.Run("ParsePolicy", func(b *testing.B) {
		spelling := layout.PolicyCentralised.String()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got layout.Policy
			err error
		)
		for c.Loop() {
			got, err = layout.ParsePolicy(spelling)
		}
		assert.NoError(b, err, "ParsePolicy reads the spelling of a policy")
		assert.Equal(b, got, layout.PolicyCentralised, "ParsePolicy returns the spelled policy")
	})

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
