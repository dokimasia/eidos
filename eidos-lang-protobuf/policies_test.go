// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// policiesAllocs is the list of specs and the list of choices of each of
// the four specs.
const policiesAllocs = 1 + 4

// TestPolicies pins the keys, the choices and the defaults of the
// encoding policies, because a config file and a directive name them.
func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one policy for each width and sign of an integer", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protobuf.Policies(), []plugin.PolicySpec{
				{
					Key: protobuf.Int32Encoding, Default: protobuf.Int32,
					Choices: []plugin.Choice{protobuf.Int32, protobuf.Sint32, protobuf.Sfixed32},
					Doc:     "the encoding of a signed integer of 8, 16 or 32 bits",
				},
				{
					Key: protobuf.Int64Encoding, Default: protobuf.Int64,
					Choices: []plugin.Choice{protobuf.Int64, protobuf.Sint64, protobuf.Sfixed64},
					Doc:     "the encoding of a signed integer of 64 bits or of the platform's width",
				},
				{
					Key: protobuf.Uint32Encoding, Default: protobuf.Uint32,
					Choices: []plugin.Choice{protobuf.Uint32, protobuf.Fixed32},
					Doc:     "the encoding of an unsigned integer of 8, 16 or 32 bits",
				},
				{
					Key: protobuf.Uint64Encoding, Default: protobuf.Uint64,
					Choices: []plugin.Choice{protobuf.Uint64, protobuf.Fixed64},
					Doc:     "the encoding of an unsigned integer of 64 bits or of the platform's width",
				},
			}, "the varint encoding is the default of each policy")
		})

		t.Run("returns specs that a policy resolves", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewPolicy(protobuf.Target, protobuf.Policies(), nil)
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
			{
				name: "spells the policy of a signed integer of 32 bits",
				give: protobuf.Int32Encoding,
				want: "protobuf.int32",
			},
			{
				name: "spells the policy of a signed integer of 64 bits",
				give: protobuf.Int64Encoding,
				want: "protobuf.int64",
			},
			{
				name: "spells the policy of an unsigned integer of 32 bits", give: protobuf.Uint32Encoding,
				want: "protobuf.uint32",
			},
			{
				name: "spells the policy of an unsigned integer of 64 bits", give: protobuf.Uint64Encoding,
				want: "protobuf.uint64",
			},
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
			{name: "spells int32", give: protobuf.Int32, want: "int32"},
			{name: "spells sint32", give: protobuf.Sint32, want: "sint32"},
			{name: "spells sfixed32", give: protobuf.Sfixed32, want: "sfixed32"},
			{name: "spells int64", give: protobuf.Int64, want: "int64"},
			{name: "spells sint64", give: protobuf.Sint64, want: "sint64"},
			{name: "spells sfixed64", give: protobuf.Sfixed64, want: "sfixed64"},
			{name: "spells uint32", give: protobuf.Uint32, want: "uint32"},
			{name: "spells fixed32", give: protobuf.Fixed32, want: "fixed32"},
			{name: "spells uint64", give: protobuf.Uint64, want: "uint64"},
			{name: "spells fixed64", give: protobuf.Fixed64, want: "fixed64"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the scalar type that the spoke writes")
			})
		}
	})
}

// TestPoliciesAllocs checks the ceiling of the specs in the ordinary run,
// which runs no benchmark.
func TestPoliciesAllocs(t *testing.T) {
	var got []plugin.PolicySpec
	assert.MaxAllocs(t, func() { got = protobuf.Policies() }, policiesAllocs,
		"Policies allocates its list and the choices of each spec")
	assert.Length(t, got, 4, "Policies returns four specs")
}

// BenchmarkPolicies measures the specs that a backend declares once per
// build.
func BenchmarkPolicies(b *testing.B) {
	b.Run("Policies", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(policiesAllocs)
		defer c.End()
		var got []plugin.PolicySpec
		for c.Loop() {
			got = protobuf.Policies()
		}
		assert.Length(b, got, 4, "Policies returns four specs")
	})
}
