// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes Rust's spellings of naming facts: [Filename]
// joins a unit's routing-key stem, family word and tag into a
// filename, and [Name] spells a declared name in Rust's convention.
// The kernel consumes both through the backend kit's naming and
// respelling steps.
//
// # Dependency position
//
// lang/rust/spell imports the sdk's plugin and symbol facades, the
// module root for the language identity, the case conversion of
// eidos-lang, and the Go stdlib.
package spell
