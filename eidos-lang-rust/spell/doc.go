// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes Rust's spellings of naming facts: [Filename]
// joins a unit's file-key stem, family word and tag into a filename,
// [Package] names the module a file at a routed path declares, and
// [Name] spells a declared name in Rust's convention. [Type] spells the
// canonical shape of a type of another language in Rust, as the spoke of
// the cross-language hub. The kernel consumes them through the backend
// kit's naming, package, respelling and type steps.
//
// # Dependency position
//
// lang/rust/spell imports the sdk's emit, plugin, rules and symbol
// facades, the module root for the language identity, the case
// conversion, the reference spelling and the spoke helpers of
// eidos-lang, and the Go stdlib.
package spell
