// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import "go.dokimi.dev/eidos/sdk/plugin"

// The lowering policies of the protobuf target. protobuf spells an integer
// of one width and sign in two or three encodings, and each policy chooses
// the scalar type of one width and sign. A config file selects a choice
// for every plan or for one plan, and the param of the policy's name on
// the protobuf directive selects it for one declaration.
const (
	// Int32Encoding chooses the scalar type of a signed integer of 8, 16
	// or 32 bits.
	Int32Encoding plugin.PolicyKey = "protobuf.int32"
	// Int64Encoding chooses the scalar type of a signed integer of 64 bits
	// or of the platform's width.
	Int64Encoding plugin.PolicyKey = "protobuf.int64"
	// Uint32Encoding chooses the scalar type of an unsigned integer of 8,
	// 16 or 32 bits.
	Uint32Encoding plugin.PolicyKey = "protobuf.uint32"
	// Uint64Encoding chooses the scalar type of an unsigned integer of 64
	// bits or of the platform's width.
	Uint64Encoding plugin.PolicyKey = "protobuf.uint64"
)

// The choices of the encoding policies. Each choice is the scalar type
// that the spoke writes for it. The varint types are the defaults. The
// zigzag types encode a negative value in fewer bytes, and the fixed types
// encode a large value in fewer bytes.
const (
	Int32    plugin.Choice = "int32"
	Sint32   plugin.Choice = "sint32"
	Sfixed32 plugin.Choice = "sfixed32"
	Int64    plugin.Choice = "int64"
	Sint64   plugin.Choice = "sint64"
	Sfixed64 plugin.Choice = "sfixed64"
	Uint32   plugin.Choice = "uint32"
	Fixed32  plugin.Choice = "fixed32"
	Uint64   plugin.Choice = "uint64"
	Fixed64  plugin.Choice = "fixed64"
)

// The docs of the policies, which document each policy's key and the
// param of the protobuf directive that overrides it.
const (
	int32Doc  = "the encoding of a signed integer of 8, 16 or 32 bits"
	int64Doc  = "the encoding of a signed integer of 64 bits or of the platform's width"
	uint32Doc = "the encoding of an unsigned integer of 8, 16 or 32 bits"
	uint64Doc = "the encoding of an unsigned integer of 64 bits or of the platform's width"
)

// Policies returns the specs of the four encoding policies. The default
// of each policy is the varint type of its width and sign. The backend
// declares them through the backend kit.
//
// # Allocation contract
//
// Policies allocates the list of specs and the list of choices of each
// spec: five allocations.
func Policies() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{Key: Int32Encoding, Choices: []plugin.Choice{Int32, Sint32, Sfixed32}, Default: Int32, Doc: int32Doc},
		{Key: Int64Encoding, Choices: []plugin.Choice{Int64, Sint64, Sfixed64}, Default: Int64, Doc: int64Doc},
		{Key: Uint32Encoding, Choices: []plugin.Choice{Uint32, Fixed32}, Default: Uint32, Doc: uint32Doc},
		{Key: Uint64Encoding, Choices: []plugin.Choice{Uint64, Fixed64}, Default: Uint64, Doc: uint64Doc},
	}
}
