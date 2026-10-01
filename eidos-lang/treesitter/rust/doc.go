// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rust pins tree-sitter-rust and loads its grammar as
// [Grammar], which parses .rs files.
//
// Linking the package compiles the grammar's parser and external
// scanner from 6.5 MB of C, so a binary that imports it builds with
// cgo and a C11 compiler.
//
// # Dependency position
//
// The package imports github.com/tree-sitter/tree-sitter-rust and
// go.dokimi.dev/eidos/lang/treesitter.
package rust
