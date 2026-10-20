// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package matrix renders the support matrix of the satellites: a
// Markdown document of what each language reads, which optional
// capabilities of the projection vocabulary its rules implement, what
// each target renders, how each target spells each canonical form, and
// which languages lower markers in their own syntax.
//
// [Entries] returns the entry of each satellite, and [Markdown] renders
// the document of a list of entries. The document has five tables:
//
//   - The read side contains the verdict of each language's conformance
//     corpus on each feature of the inventory.
//   - The projection side contains the verdict of each language's rules
//     on each optional capability, which the matrix finds by asserting
//     it on the rules.
//   - The render side contains the verdict of each target's backend on
//     each fact of the model and on each kind of declaration.
//   - The hub contains the spelling of each canonical form in each
//     target, under the target's default policy and under each choice of
//     its lowering policies.
//   - The sugar contains the verdict of each language on a marker in its
//     own syntax for metadata, and the marker of the brand acme.
//
// CI uploads the document that the package's test writes into its
// artifact directory.
//
// # Dependency position
//
// matrix imports the conformance package and the language entries under
// conformance/lang, the backend packages of the Go, TypeScript, Java,
// Rust, protobuf, GraphQL and Python satellites, the root packages of the
// TypeScript, protobuf, GraphQL and Python satellites, lang/spellref and
// the SDK facade. Its tests also import the assert module and the root packages of
// the satellites. No package imports matrix.
package matrix
