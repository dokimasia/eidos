// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rust declares Rust's conformance entry, and its tests grade
// Rust against the shared feature inventory.
//
// [Corpus] is Rust's entry: the Rust frontend over one crate that
// spells every inventory feature but method overloads, which Rust does
// not have.
//
// # Dependency position
//
// rust imports the conformance package, core/frontend/frontendtest and
// the Rust satellite's frontend. No package imports rust but its own
// tests.
package rust
