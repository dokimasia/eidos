// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules implements the projection rules of GraphQL.
//
// [New] returns the value that a composition registers. It satisfies the
// source-rules contract, the enum capability and the shape capability, and
// no other.
//
// # Decisions
//
//   - Nothing contributes members. An object type and an interface restate
//     the fields of their interfaces, and the frontend merges an extension
//     into its type.
//   - Every argument and every variable is an input. A field and an
//     operation have one result under no error model, and the result of a
//     subscription is a stream.
//   - Int is a signed integer of 32 bits, Float a float of 64 bits, String
//     and ID text, and Boolean a bool. A custom scalar whose specifiedBy URL
//     the table of the satellite contains has the shape of its entry.
//   - An enum value is its name converted to the enum type, and an enum has
//     no zero value.
//   - The spelling of a directive resolves in the packages of the schema of
//     its subject, and a built-in scalar resolves to a stand-in.
//   - A literal reads in the grammar of a constant value of GraphQL, and
//     types as GraphQL coerces an input.
//
// # Dependency position
//
// lang/graphql/rules imports the sdk facade, lang/graphql and its internal
// value package, lang/naming, lang/numeric, the ast package of gqlparser and
// the Go stdlib, go/constant among it.
package rules
