// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package toolchain provides the skeleton of the toolchain harness of each
// language: the fixture of generated output, the assertion set, the
// [Adapter] that a satellite implements over its own compiler, and the
// mechanics that every adapter shares.
//
// The package declares the assertions and their failure messages once. A
// satellite states how to lay out its language, how to parse it, how to
// type-check it, how to run its tests and how to ask whether a type
// satisfies a contract. It then runs the whole set.
//
// [RunToolchainSuite] is the minimum that a satellite runs: the generated
// output parses, it type-checks, and its own tests pass. A satellite adds
// its own assertions beside the suite, exported from its own harness under
// the same naming scheme, for a check that has a meaning under one
// toolchain alone.
//
// A toolchain that the machine does not have is a skip locally and a
// failure in CI, so a missing compiler cannot hide a regression.
// [RequiredInCI] decides which. The assertions never skip, because a caller
// of one has already decided that the toolchain is present.
//
// # The surface and the documentation
//
// Two checks read generated output through the frontend of its language
// and need no toolchain, so they never skip:
//
//   - [Surface] renders the public declarations of the output, one line
//     for each feature, without comments, bodies, the names of
//     parameters or the order of declarations. [AssertSurface] matches the
//     surface against a golden file, which -update writes.
//   - [AssertDocumented] requires a doc comment on every public declaration
//     at file level of the output.
//
// Both load the output with its sources under the brand toolchain, as a
// toolchain run lays the project out, and read the declarations of the
// generated files alone.
//
// # Harness mechanics
//
// An adapter keeps its binaries, its project files, its probe texts and the
// parse of the output of its tools. The package provides the rest:
//
//   - [Run] runs a binary under a context, bounded by [RunTimeout];
//   - [LookPaths] reports whether the binaries of a toolchain are on PATH;
//   - [Scratch] writes a fixture and the project files of a language into
//     a new temporary directory, and [WriteFile] writes one file under a
//     directory;
//   - [Sources] lists the files of a laid-out project;
//   - [Probe] adds one file to a project while a check runs.
//
// Each call of the mechanics and the checks starts a process, loads files
// or writes files on the cold path of a test, so the package states no
// allocation contract.
//
// # Failure semantics
//
// Every check records each contract that it finds broken as the assertion
// of go.dokimi.dev/assert/expect that states it, and returns. The contract
// of the record contains the language and the broken contract, and its
// detail contains the value that broke it, so a satellite's own test reads
// the failures of a check through [go.dokimi.dev/assert.Rejects]. The
// contract of a failed toolchain run ends with the listing of the
// generated files, each line behind its number, because the error of the
// toolchain names lines of a scratch project that [Prepare] removes when
// the check returns.
//
// # Dependency position
//
// core/toolchain imports core/diag, core/frontend/load, core/node,
// core/output, core/plugin and core/symbol, the assert module with its
// expect and golden packages, and the Go stdlib, the testing package and
// os/exec among it. It runs the binary that an adapter names and knows no
// compiler itself. The SDK facade does not re-export the load, so the
// package uses it as an implementation dependency alone.
package toolchain
