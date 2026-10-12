// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the toolchain harness of Go and the spelling adapter
// of its fixtures. It provides the adapter that the assertion set of the
// kernel drives, the assertions that have a meaning under the Go toolchain
// alone, and the Language of the fixture builder of nodetest.
//
// [New] returns the adapter. It lays generated output out as a module in a
// scratch directory and parses it with the standard library. It
// type-checks, tests and vets the output through the go tool, which it runs
// with [toolchain.Run]. The parse needs no toolchain, so a machine without
// one still checks the output against the grammar of Go. Every other check
// reports the toolchain absent, skips locally and fails in CI.
//
// [AssertVets] and [AssertParallel] are the assertions that Go adds. go
// vet finds code that compiles and is still wrong, which is the class of
// defect that a generator produces most easily. AssertParallel requires
// every generated test and subtest to call Parallel, because a generated
// suite runs in the CI of every consumer. Both record each failure as an
// assertion of the assert module, as the assertions of the kernel do, so
// [go.dokimi.dev/assert.Rejects] returns their records.
//
// [Language] writes the source spelling of a structural reference of a
// fixture through [golang.Restate], as the Go frontend records the same
// type in source that gofmt formats. Its test pins each form to the
// frontend.
//
// # Dependency position
//
// lang/go/testing imports the language root, the emit, node, nodetest,
// toolchain and symbol facades of the SDK, the assert module and the Go
// stdlib. It runs the go tool through the toolchain facade.
package testing
