// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rust declares Rust's conformance entry, and its tests grade
// Rust against the shared feature inventory.
//
// [Corpus] is Rust's entry: the Rust frontend and the Rust rules over one
// crate that spells every inventory feature but method overloads, which
// Rust does not have. Each feature states the level at which the rules
// project it.
//
// # Dependency position
//
// rust imports the conformance package, core/frontend/frontendtest and
// the frontend and the rules of the Rust satellite. No package imports
// rust but its own tests.
package rust
