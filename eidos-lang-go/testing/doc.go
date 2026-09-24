// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the Go language's toolchain harness: the
// adapter the kernel's assertion set drives, and the assertions
// that mean something under the Go toolchain alone.
//
// [New] returns the adapter. It lays generated output out as a
// module in a scratch directory, parses with the standard library,
// and type-checks, tests and vets by running the go tool. Parsing
// needs no toolchain, so a machine without one still holds the
// output to Go's grammar; everything below it reports absent and
// skips locally, and fails in CI.
//
// [AssertVets] is the language-specific addition: go vet finds what
// compiles and is still wrong, which is the class of defect a
// generator produces most easily.
//
// # Dependency position
//
// lang/go/testing imports the language root, the sdk's toolchain
// and symbol facades, and the Go stdlib, os/exec among it. The
// frontend never imports os/exec. This package is a harness and
// runs the go tool on purpose.
package testing
