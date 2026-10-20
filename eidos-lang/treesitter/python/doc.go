// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package python pins the fork dokimasia/tree-sitter-python and loads its
// grammar as [Grammar], which parses the .py and .pyi files of Python 3.15
// with its lazy imports.
//
// Linking the package compiles the grammar's parser and external scanner
// from 3.5 MB of C, so a binary that imports it builds with cgo and a C11
// compiler.
//
// # Dependency position
//
// The package imports github.com/dokimasia/tree-sitter-python and
// go.dokimi.dev/eidos/lang/treesitter.
package python
