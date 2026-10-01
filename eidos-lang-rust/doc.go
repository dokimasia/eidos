// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rust is the root of the Rust satellite: the language's
// identity and comment forms, which the satellite's packages share.
//
// [Lang] is the language of every Rust declaration, and [CodePrefix]
// opens every diagnostic code the satellite registers. [Target] and
// [Name] are the spellings a plan resolves to select the backend,
// [Extension] is the suffix of every Rust file, [Version] is the
// backend's behavior version, and [FrontendVersion] the frontend's.
// [Syntax] returns Rust's comment forms: the frontend strips comments
// with them, and the output contract writes the generated-file header
// through them. [Keys] registers the rust.* keys the frontend stamps.
//
// # Packages
//
// The module covers Rust as a source and as a target:
//
//   - frontend loads Rust source into the node graph.
//   - spell spells filenames and declared names.
//   - backend renders emit values as Rust source.
//   - testing runs cargo and rustfmt over generated output.
//
// # Dependency position
//
// The root package imports the sdk's diag, meta, plugin and symbol
// facades and the Go stdlib. The module's other packages import the
// kernel's SPI through the sdk facade, the shared helpers of eidos-lang,
// the Rust grammar of lang/treesitter, github.com/BurntSushi/toml for
// the Cargo manifest, and the Go stdlib.
package rust
