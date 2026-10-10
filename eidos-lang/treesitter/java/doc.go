// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package java pins tree-sitter-java and loads its grammar as
// [Grammar], which parses .java files.
//
// Linking the package compiles the grammar's parser from 2.6 MB of C,
// so a binary that imports it builds with cgo and a C11 compiler.
//
// # Dependency position
//
// The package imports github.com/tree-sitter/tree-sitter-java and
// go.dokimi.dev/eidos/lang/treesitter.
package java
