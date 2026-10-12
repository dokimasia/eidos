// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the toolchain harness of Rust and the spelling adapter
// of its fixtures. It provides the adapter that the assertion set of the
// kernel drives over generated Rust, and the Language of the fixture
// builder of nodetest.
//
// [New] returns the adapter. It lays generated output out as a crate in a
// scratch directory with a Cargo.toml, and parses every source with
// rustfmt. It type-checks every target with cargo check, runs the tests
// with cargo test, and asks rustc whether a type implements a trait. cargo
// runs offline and builds inside the scratch directory.
//
// Rust has no parser in Go, so the parse runs the toolchain too. Every
// check needs it, and a caller passes
// [go.dokimi.dev/eidos/sdk/toolchain.Require] before any. The check skips
// where the toolchain is absent locally, and fails where it is absent in
// CI.
//
// [Language] writes the compact spelling of a structural reference of a
// fixture, as the Rust frontend records the same type. Its test pins each
// form to the frontend.
//
// # Dependency position
//
// lang/rust/testing imports the language root, the node, nodetest,
// toolchain and symbol facades of the SDK, and the Go stdlib. It runs cargo
// and rustfmt through the toolchain facade.
package testing
