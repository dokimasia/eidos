// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import "go.dokimi.dev/eidos/sdk/plugin"

// The lowering policies of the TypeScript target. Each policy chooses
// between TypeScript's own type and the type that a JSON payload
// delivers. Config selects a choice for every plan or for one plan, and
// the param of the policy's name on the typescript directive overrides
// the choice for one declaration.
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
)

// Policies returns the specs of the five TypeScript policies. The
// defaults of four are TypeScript's own types: bigint, undefined, Date and
// Uint8Array. The default of the duration is string, because TypeScript
// does not declare a duration type. The backend declares them through the
// backend kit.
//
// # Allocation contract
//
// Policies allocates the list of specs and the list of choices of each
// spec: six allocations.
func Policies() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{Key: Int64, Choices: []plugin.Choice{BigInt, String, Number}, Default: BigInt, Doc: int64Doc},
		{Key: Absent, Choices: []plugin.Choice{Undefined, Null}, Default: Undefined, Doc: absentDoc},
		{Key: Timestamp, Choices: []plugin.Choice{Date, String}, Default: Date, Doc: timestampDoc},
		{Key: Bytes, Choices: []plugin.Choice{Uint8Array, String}, Default: Uint8Array, Doc: bytesDoc},
		{Key: Duration, Choices: []plugin.Choice{String, Number}, Default: String, Doc: durationDoc},
	}
}
