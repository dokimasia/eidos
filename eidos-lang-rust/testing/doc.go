// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the Rust language's toolchain harness: the
// adapter the kernel's assertion set drives over generated Rust.
//
// [New] returns the adapter. It lays generated output out as a crate
// in a scratch directory with a Cargo.toml, parses every source with
// rustfmt, type-checks every target with cargo check, runs the tests
// with cargo test, and asks rustc whether a type implements a trait.
// cargo runs offline and builds inside the scratch directory.
//
// Rust has no parser in Go, so the parse runs the toolchain too.
// Every check needs it, and a caller passes
// [go.dokimi.dev/eidos/sdk/toolchain.Require] before any: absent
// locally, the check skips, and absent in CI, it fails.
//
// # Dependency position
//
// lang/rust/testing imports the language root, the sdk's toolchain
// and symbol facades, and the Go stdlib, os/exec among it. It is a
// harness and runs cargo and rustfmt on purpose.
package testing
