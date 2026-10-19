// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the toolchain harness of protobuf. It provides the
// adapter that the assertion set of the kernel drives over generated proto
// files.
//
// [New] returns the adapter. It lays generated output out as a source tree
// in a scratch directory, and parses each file through the parser of the
// protobuf frontend. It type-checks the tree with the experimental compiler
// of protocompile, which links the references of the files and interprets
// their options. The compiler reads the well-known types from its own copy
// of their sources.
//
// The parser and the compiler know the editions up to 2024. A file of
// edition 2026 has the grammar of edition 2024, so the adapter reads it
// from a copy that states 2024. The check covers the grammar, the
// references and the options of such a file, and not the defaults that
// 2026 changes.
//
// The parse and the compile run in process, so the toolchain is on every
// machine. A protobuf file declares no test and implements no contract, so
// the adapter refuses to run tests and to answer whether a type satisfies a
// contract.
//
// # Dependency position
//
// lang/protobuf/testing imports the language root, the parse package of the
// satellite, the experimental incremental, ir, report and source packages
// of protocompile, the symbol and toolchain facades of the SDK, and the Go
// stdlib.
package testing
