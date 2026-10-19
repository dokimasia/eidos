// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// policiesAllocs is the list of specs and the list of choices of each of
// the eight specs.
const policiesAllocs = 1 + 8

// TestPolicies pins the keys, the choices and the defaults of the policies,
// because a config file and a directive name them.
func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one policy for each shape without a built-in scalar", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.Policies(), []plugin.PolicySpec{
				{
					Key: graphql.Int64Type, Default: graphql.String,
					Choices: []plugin.Choice{graphql.String, graphql.Float, graphql.Long},
					Doc:     "the type of a signed integer of 64 bits or of the platform's width",
				},
				{
					Key: graphql.Uint32Type, Default: graphql.Float,
					Choices: []plugin.Choice{graphql.Float, graphql.String, graphql.UnsignedInt},
					Doc:     "the type of an unsigned integer of 32 bits",
				},
				{
					Key: graphql.Uint64Type, Default: graphql.String,
					Choices: []plugin.Choice{graphql.String, graphql.Float, graphql.UnsignedLong},
					Doc:     "the type of an unsigned integer of 64 bits or of the platform's width",
				},
				{
					Key: graphql.BytesType, Default: graphql.String,
					Choices: []plugin.Choice{graphql.String, graphql.Base64String},
					Doc:     "the type of bytes",
				},
				{
					Key: graphql.TimestampType, Default: graphql.String,
					Choices: []plugin.Choice{graphql.String, graphql.DateTime},
					Doc:     "the type of the well-known timestamp",
				},
				{
					Key: graphql.DurationType, Default: graphql.String,
					Choices: []plugin.Choice{graphql.String, graphql.Duration},
					Doc:     "the type of the well-known duration",
				},
				{
					Key: graphql.DynamicType, Default: graphql.String,
					Choices: []plugin.Choice{graphql.String, graphql.Any},
					Doc:     "the type of a dynamic value",
				},
				{
					Key: graphql.MapType, Default: graphql.Entries,
					Choices: []plugin.Choice{graphql.Entries, graphql.Any},
					Doc:     "the type of a map",
				},
			}, "a built-in type is the default of each policy")
		})

		t.Run("returns specs that a policy resolves", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewPolicy(graphql.Target, graphql.Policies(), nil)
			assert.NoError(t, err, "the specs are well formed")
		})
	})

	t.Run("PolicyKey", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.PolicyKey
			want string
		}{
			{name: "spells the policy of a signed integer of 64 bits", give: graphql.Int64Type, want: "graphql.int64"},
			{
				name: "spells the policy of an unsigned integer of 32 bits", give: graphql.Uint32Type,
				want: "graphql.uint32",
			},
			{
				name: "spells the policy of an unsigned integer of 64 bits", give: graphql.Uint64Type,
				want: "graphql.uint64",
			},
			{name: "spells the policy of bytes", give: graphql.BytesType, want: "graphql.bytes"},
			{name: "spells the policy of the timestamp", give: graphql.TimestampType, want: "graphql.timestamp"},
			{name: "spells the policy of the duration", give: graphql.DurationType, want: "graphql.duration"},
			{name: "spells the policy of a dynamic value", give: graphql.DynamicType, want: "graphql.dynamic"},
			{name: "spells the policy of a map", give: graphql.MapType, want: "graphql.map"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the key that a config file names")
			})
		}
	})

	t.Run("Choice", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Choice
			want string
		}{
			{name: "spells String", give: graphql.String, want: "String"},
			{name: "spells Float", give: graphql.Float, want: "Float"},
			{name: "spells Long", give: graphql.Long, want: "Long"},
			{name: "spells UnsignedInt", give: graphql.UnsignedInt, want: "UnsignedInt"},
			{name: "spells UnsignedLong", give: graphql.UnsignedLong, want: "UnsignedLong"},
			{name: "spells Base64String", give: graphql.Base64String, want: "Base64String"},
			{name: "spells DateTime", give: graphql.DateTime, want: "DateTime"},
			{name: "spells Duration", give: graphql.Duration, want: "Duration"},
			{name: "spells Any", give: graphql.Any, want: "Any"},
			{name: "spells entries", give: graphql.Entries, want: "entries"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the choice that a config file names")
			})
		}
	})
}

// TestPoliciesAllocs checks the ceiling of the specs in the ordinary run,
// which does not run benchmarks.
func TestPoliciesAllocs(t *testing.T) {
	var got []plugin.PolicySpec
	assert.MaxAllocs(t, func() { got = graphql.Policies() }, policiesAllocs,
		"Policies allocates its list and the choices of each spec")
	assert.Length(t, got, 8, "Policies returns eight specs")
}

// BenchmarkPolicies measures the specs that a backend declares once per
// build.
func BenchmarkPolicies(b *testing.B) {
	b.Run("Policies", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(policiesAllocs)
		defer c.End()
		var got []plugin.PolicySpec
		for c.Loop() {
			got = graphql.Policies()
		}
		assert.Length(b, got, 8, "Policies returns eight specs")
	})
}
