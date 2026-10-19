// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import "go.dokimi.dev/eidos/sdk/plugin"

// The lowering policies of the GraphQL target. GraphQL has five built-in
// scalars. A shape without a built-in scalar takes a built-in type that
// represents its value in JSON, or a custom scalar of the table of
// [Scalar]. Each policy chooses between them for one shape. A config file
// selects a choice for every plan or for one plan. The param of the name of
// the policy on the graphql directive selects a choice for one declaration.
const (
	// Int64Type chooses the type of a signed integer of 64 bits or of the
	// platform's width.
	Int64Type plugin.PolicyKey = "graphql.int64"
	// Uint32Type chooses the type of an unsigned integer of 32 bits.
	Uint32Type plugin.PolicyKey = "graphql.uint32"
	// Uint64Type chooses the type of an unsigned integer of 64 bits or of
	// the platform's width.
	Uint64Type plugin.PolicyKey = "graphql.uint64"
	// BytesType chooses the type of bytes.
	BytesType plugin.PolicyKey = "graphql.bytes"
	// TimestampType chooses the type of the well-known timestamp.
	TimestampType plugin.PolicyKey = "graphql.timestamp"
	// DurationType chooses the type of the well-known duration.
	DurationType plugin.PolicyKey = "graphql.duration"
	// DynamicType chooses the type of a dynamic value.
	DynamicType plugin.PolicyKey = "graphql.dynamic"
	// MapType chooses the type of a map.
	MapType plugin.PolicyKey = "graphql.map"
)

// The choices of the policies. A built-in choice is the built-in scalar
// that the spoke writes, a choice of the table is the custom scalar of the
// table of the same name, and Entries writes a map as a list of a
// synthesized entry type.
const (
	String       plugin.Choice = "String"
	Float        plugin.Choice = "Float"
	Long         plugin.Choice = "Long"
	UnsignedInt  plugin.Choice = "UnsignedInt"
	UnsignedLong plugin.Choice = "UnsignedLong"
	Base64String plugin.Choice = "Base64String"
	DateTime     plugin.Choice = "DateTime"
	Duration     plugin.Choice = "Duration"
	Any          plugin.Choice = "Any"
	Entries      plugin.Choice = "entries"
)

// The docs of the policies, which document each policy's key and the param
// of the graphql directive that overrides it.
const (
	int64Doc     = "the type of a signed integer of 64 bits or of the platform's width"
	uint32Doc    = "the type of an unsigned integer of 32 bits"
	uint64Doc    = "the type of an unsigned integer of 64 bits or of the platform's width"
	bytesDoc     = "the type of bytes"
	timestampDoc = "the type of the well-known timestamp"
	durationDoc  = "the type of the well-known duration"
	dynamicDoc   = "the type of a dynamic value"
	mapDoc       = "the type of a map"
)

// Policies returns the specs of the eight policies of the spoke. The
// default of each policy is a built-in type, so a written schema validates
// without a declaration that the plan lacks. A Float represents every
// integer of 32 bits exactly, so it is the default of an unsigned integer
// of 32 bits. The backend declares the policies through the backend kit.
//
// # Allocation contract
//
// Policies allocates the list of specs and the list of choices of each
// spec: nine allocations.
func Policies() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{Key: Int64Type, Choices: []plugin.Choice{String, Float, Long}, Default: String, Doc: int64Doc},
		{Key: Uint32Type, Choices: []plugin.Choice{Float, String, UnsignedInt}, Default: Float, Doc: uint32Doc},
		{Key: Uint64Type, Choices: []plugin.Choice{String, Float, UnsignedLong}, Default: String, Doc: uint64Doc},
		{Key: BytesType, Choices: []plugin.Choice{String, Base64String}, Default: String, Doc: bytesDoc},
		{Key: TimestampType, Choices: []plugin.Choice{String, DateTime}, Default: String, Doc: timestampDoc},
		{Key: DurationType, Choices: []plugin.Choice{String, Duration}, Default: String, Doc: durationDoc},
		{Key: DynamicType, Choices: []plugin.Choice{String, Any}, Default: String, Doc: dynamicDoc},
		{Key: MapType, Choices: []plugin.Choice{Entries, Any}, Default: Entries, Doc: mapDoc},
	}
}
