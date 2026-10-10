// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules implements protobuf's projection rules.
//
// [New] returns the value a composition registers. It satisfies the
// source-rules contract, the enum capability and the presence
// capability, and no other.
//
// # Decisions
//
//   - Nothing contributes members: protobuf has no embedding, no
//     supertypes and no implemented interfaces.
//   - An rpc's parameter is input and its return a value under no
//     error model. A streaming return is a stream.
//   - Numbers classify from the wire table: a width is the wire's,
//     so int32 is thirty-two bits in every target, and every derived
//     number states it.
//   - An enum value is its declared number converted to the enum
//     type, and an enum's zero is its first declared value. A closed
//     enum has no out-of-range value.
//   - A singular field of a message type has presence in every
//     version, which a translation adds where the field's type does
//     not contain it.
//   - A directive's spelling resolves through the frontend's
//     candidates, and a oneof member resolves as a field of its
//     message.
//   - A literal reads in protoc's grammar and types against the
//     reference: an integer type's width, a float type's precision,
//     an enum's value names.
//
// # Well-known types
//
// Policy defaults, keyed by fully-qualified name.
// [go.dokimi.dev/eidos/lang/protobuf.WellKnown] reports which
// spellings the table contains, with or without the leading dot.
//
//   - Timestamp, Duration and Empty map onto the kernel's registry.
//   - A wrapper is its scalar under the optional form: its samples
//     are the scalar's, and its zero is absence.
//   - Value and Any are the top type, because their content is
//     decided at run time. ListValue is a list of the top type, Struct
//     a map from strings onto the top type, and FieldMask a list of
//     strings.
//   - NullValue, an enum of one value, is opaque.
//
// # Dependency position
//
// lang/protobuf/rules imports the sdk facade, lang/naming,
// lang/numeric, lang/protobuf and the Go stdlib, go/constant among
// it.
package rules
