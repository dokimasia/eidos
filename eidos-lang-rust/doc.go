// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rust is the root of the Rust satellite: the language's
// identity and comment forms, which the satellite's packages share.
//
// [Lang] is the language of every Rust declaration. [Target] and
// [Name] are the spellings a plan resolves to reach the backend,
// [Extension] is the suffix of every Rust file, and [Version] is the
// backend's behavior version. [Syntax] returns Rust's comment forms,
// which the output contract writes the generated-file header
// through.
//
// # Packages
//
// The module covers Rust as a target:
//
//   - spell spells filenames and declared names.
//   - backend renders emit values as Rust source.
//
// # Dependency position
//
// The root package imports the sdk's plugin and symbol facades. The
// module's other packages import the kernel's SPI through the sdk
// facade, the shared helpers of eidos-lang, and the Go stdlib.
package rust
