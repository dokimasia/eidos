// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// int64Wrapper is the well-known message that wraps an int64.
const int64Wrapper = "google.protobuf.Int64Value"

// TestTypes pins the wrapper of each scalar type, which the backend writes
// for a scalar that a list or a map cannot hold without one.
func TestTypes(t *testing.T) {
	t.Parallel()

	t.Run("Wrapper", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "returns DoubleValue for double", give: protobuf.TypeDouble, want: "google.protobuf.DoubleValue"},
			{name: "returns FloatValue for float", give: protobuf.TypeFloat, want: "google.protobuf.FloatValue"},
			{name: "returns Int64Value for int64", give: string(protobuf.Int64), want: int64Wrapper},
			{name: "returns Int64Value for sint64", give: string(protobuf.Sint64), want: int64Wrapper},
			{name: "returns Int64Value for sfixed64", give: string(protobuf.Sfixed64), want: int64Wrapper},
			{
				name: "returns UInt64Value for uint64",
				give: string(protobuf.Uint64), want: "google.protobuf.UInt64Value",
			},
			{
				name: "returns UInt64Value for fixed64",
				give: string(protobuf.Fixed64), want: "google.protobuf.UInt64Value",
			},
			{name: "returns Int32Value for int32", give: string(protobuf.Int32), want: "google.protobuf.Int32Value"},
			{name: "returns Int32Value for sint32", give: string(protobuf.Sint32), want: "google.protobuf.Int32Value"},
			{
				name: "returns Int32Value for sfixed32",
				give: string(protobuf.Sfixed32), want: "google.protobuf.Int32Value",
			},
			{
				name: "returns UInt32Value for uint32",
				give: string(protobuf.Uint32), want: "google.protobuf.UInt32Value",
			},
			{
				name: "returns UInt32Value for fixed32",
				give: string(protobuf.Fixed32), want: "google.protobuf.UInt32Value",
			},
			{name: "returns BoolValue for bool", give: protobuf.TypeBool, want: "google.protobuf.BoolValue"},
			{name: "returns StringValue for string", give: protobuf.TypeString, want: "google.protobuf.StringValue"},
			{name: "returns BytesValue for bytes", give: protobuf.TypeBytes, want: "google.protobuf.BytesValue"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, wraps := protobuf.Wrapper(tt.give)
				assert.True(t, wraps, tt.give+" has a wrapper")
				assert.Equal(t, got, tt.want, "the wrapper of the scalar type")
			})
		}

		t.Run("returns a type that WellKnownImport resolves to the file of the wrappers", func(t *testing.T) {
			t.Parallel()

			got, _ := protobuf.Wrapper(protobuf.TypeString)
			file, known := protobuf.WellKnownImport(got)
			assert.True(t, known, "the wrapper is a well-known type")
			assert.Equal(t, file, protobuf.WrappersFile, "the file of the wrappers declares it")
		})

		t.Run("reports false for a message", func(t *testing.T) {
			t.Parallel()

			got, wraps := protobuf.Wrapper("Row")
			assert.False(t, wraps, "a message has no wrapper")
			assert.Empty(t, got, "the name is empty")
		})
	})
}

// Wrapper reads the table without allocating. The ordinary run, which
// runs no benchmark, checks that ceiling here.
func TestTypesAllocs(t *testing.T) {
	var got string
	assert.MaxAllocs(t, func() { got, _ = protobuf.Wrapper(string(protobuf.Sint64)) }, 0,
		"Wrapper allocates nothing")
	assert.Equal(t, got, int64Wrapper, "Wrapper returns the wrapper of sint64")
}

// BenchmarkTypes measures the lookup of a wrapper, which the lowering makes
// for each optional scalar that a list or a map holds.
func BenchmarkTypes(b *testing.B) {
	b.Run("Wrapper", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got, _ = protobuf.Wrapper(string(protobuf.Sint64))
		}
		assert.Equal(b, got, int64Wrapper, "Wrapper returns the wrapper of sint64")
	})
}
