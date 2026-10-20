// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package typescript pins the fork dokimasia/tree-sitter-typescript and
// loads its two grammars: [TypeScript] parses .ts, .mts and .cts files,
// and [TSX] parses .tsx files.
//
// The binding of the fork compiles both grammars in one package, so
// linking the package compiles 17.5 MB of parser C, and a binary that
// imports it builds with cgo and a C11 compiler.
//
// # Dependency position
//
// The package imports github.com/dokimasia/tree-sitter-typescript and
// go.dokimi.dev/eidos/lang/treesitter.
package typescript
