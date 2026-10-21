// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import "go.dokimi.dev/eidos/sdk/plugin"

// The lowering policies of the TypeScript target. Each policy but [GRPC]
// and [QuoteStyle] chooses between TypeScript's own type and the type that
// a JSON payload delivers. Config selects a choice for every plan or for
// one plan. The param of the policy's name on the typescript directive
// overrides the choice for one declaration, except for [QuoteStyle], which
// applies to every file of a plan.
const (
	// Int64 chooses the spelling of an integer of 64 bits or of the
	// platform's width: bigint, a decimal string, or number, which
	// represents an integer exactly up to 2^53.
	Int64 plugin.PolicyKey = "typescript.int64"
	// Absent chooses the spelling of an optional: T | undefined, or
	// T | null, which a JSON payload delivers for an absent value.
	Absent plugin.PolicyKey = "typescript.absent"
	// Timestamp chooses the spelling of the well-known timestamp: Date,
	// or an ISO 8601 string.
	Timestamp plugin.PolicyKey = "typescript.timestamp"
	// Bytes chooses the spelling of bytes: Uint8Array, or a base64
	// string.
	Bytes plugin.PolicyKey = "typescript.bytes"
	// Duration chooses the spelling of the well-known duration: a string,
	// as ProtoJSON writes a duration such as "1.5s", or a number of
	// milliseconds, the unit of JavaScript's timers. TypeScript does not
	// declare a duration type of its own, so the default is the string
	// that a JSON payload delivers.
	Duration plugin.PolicyKey = "typescript.duration"
	// GRPC chooses the gRPC binding of the code that a plan writes against
	// a protobuf service: none, or Connect-ES, whose code uses the messages
	// of protobuf-es.
	GRPC plugin.PolicyKey = "typescript.grpc"
	// QuoteStyle chooses the quote of each string that the backend writes:
	// the double quote, which Biome and Prettier write by default, or the
	// single quote. [AppendQuote] states the strings that take the other
	// quote.
	QuoteStyle plugin.PolicyKey = "typescript.quote"
)

// The choices of [QuoteStyle].
const (
	DoubleQuote plugin.Choice = "double"
	SingleQuote plugin.Choice = "single"
)

// The choices of [GRPC]. Under [GRPCNone] a reference to a protobuf
// declaration spells as eidos translates the declaration. Under
// [GRPCConnect] it spells as protobuf-es names the declaration.
const (
	GRPCNone    plugin.Choice = "none"
	GRPCConnect plugin.Choice = "connect-es"
)

// The choices of the TypeScript policies. Each choice is the TypeScript
// spelling that the spoke writes for it.
const (
	BigInt     plugin.Choice = "bigint"
	String     plugin.Choice = "string"
	Number     plugin.Choice = "number"
	Undefined  plugin.Choice = "undefined"
	Null       plugin.Choice = "null"
	Date       plugin.Choice = "Date"
	Uint8Array plugin.Choice = "Uint8Array"
)

// The docs of the policies, which document each policy's key and the
// param of the typescript directive that overrides it.
const (
	int64Doc     = "the spelling of an integer of 64 bits or of the platform's width"
	absentDoc    = "the spelling of an absent value of an optional"
	timestampDoc = "the spelling of a point in time"
	bytesDoc     = "the spelling of bytes"
	durationDoc  = "the spelling of a span of time"
	grpcDoc      = "the gRPC binding of the code that a plan writes against a protobuf service"
	quoteDoc     = "the quote of each string that the written files contain"
)

// Policies returns the specs of the seven TypeScript policies. The
// defaults of four are TypeScript's own types: bigint, undefined, Date and
// Uint8Array. The default of the duration is string, because TypeScript
// does not declare a duration type, and the default of the binding of gRPC
// is none. The default quote is the double quote, the default of Biome and
// Prettier. The quote is a policy of the plan, because every file of a
// plan has one style. The backend declares the policies through the
// backend kit.
//
// # Allocation contract
//
// Policies allocates the list of specs and the list of choices of each
// spec: eight allocations.
func Policies() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{Key: Int64, Choices: []plugin.Choice{BigInt, String, Number}, Default: BigInt, Doc: int64Doc},
		{Key: Absent, Choices: []plugin.Choice{Undefined, Null}, Default: Undefined, Doc: absentDoc},
		{Key: Timestamp, Choices: []plugin.Choice{Date, String}, Default: Date, Doc: timestampDoc},
		{Key: Bytes, Choices: []plugin.Choice{Uint8Array, String}, Default: Uint8Array, Doc: bytesDoc},
		{Key: Duration, Choices: []plugin.Choice{String, Number}, Default: String, Doc: durationDoc},
		{Key: GRPC, Choices: []plugin.Choice{GRPCNone, GRPCConnect}, Default: GRPCNone, Doc: grpcDoc},
		{
			Key: QuoteStyle, Choices: []plugin.Choice{DoubleQuote, SingleQuote}, Default: DoubleQuote,
			Doc: quoteDoc, Plan: true,
		},
	}
}
