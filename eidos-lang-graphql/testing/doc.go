// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the toolchain harness of GraphQL. It provides the
// adapter that the assertion set of the kernel drives over generated
// GraphQL files.
//
// [New] returns the adapter. It lays generated output out as a source tree
// in a scratch directory. It parses each file through the split of the
// frontend, which parses the type system and the executable definitions of
// a file as two documents of gqlparser.
//
// The type check loads one schema from the type system of every file of the
// tree, over the prelude of gqlparser with the built-in scalars and
// directives of the specification. It validates the executable definitions
// of every file against the schema under the rules of a client, as the
// frontend validates them: the files together, so a spread can name a
// fragment of another file, and an anonymous operation alone in its file.
// The validator of gqlparser accepts an extension without a base, a default
// of the wrong type, a non-null field of a @oneOf input object and
// @deprecated on a required argument, so the type check does not refuse
// them.
//
// The parse and the validation run in process, so the toolchain is on every
// machine. A GraphQL document declares no test, so the adapter refuses to
// run tests. A type satisfies a contract where the schema lists the contract
// among the interfaces and the unions of the type.
//
// # Dependency position
//
// lang/graphql/testing imports the language root, the document package of
// the satellite, the ast, gqlerror and validator packages of gqlparser, the
// symbol and toolchain facades of the SDK, and the Go stdlib.
package testing
