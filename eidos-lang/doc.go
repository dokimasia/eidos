// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package lang is the tree-sitter platform shared by the
// tree-sitter-based language satellites: one binding layer and the
// pinned grammar set, so every satellite parses through the same
// machinery and a grammar or binding upgrade happens once.
//
// The seam exists so satellites never import tree-sitter directly.
// Grammars and bindings are version-pinned here, which keeps
// parsing hermetic — the same workspace resolves the same parser
// everywhere — and keeps the binding implementation private, so it
// can change without any satellite change.
//
// # Consumers
//
// The TypeScript, Java, Kotlin, PHP, and Rust satellites parse
// through this module. Go and protobuf do not: their frontends use
// established pure-Go parsers (the standard library's go/parser
// and bufbuild/protocompile) and never import the grammar
// packages, so no consumer of theirs is taxed with the binding
// toolchain.
//
// # Shared helpers
//
// Beside the grammars, the module holds the pure packages every
// satellite shares, [go.dokimi.dev/eidos/lang/naming] first among
// them. They import the Go stdlib alone, so a satellite reaching
// for a helper pulls no grammar machinery with it. Language
// helpers live here rather than in the kernel, because the kernel
// calls none of them: what only satellites consume belongs below
// the satellites, not inside the kernel.
//
// # Grammars
//
// One pinned grammar per language, exposed through per-language
// constructors. The grammar version folds into the frontend's
// version and therefore into every unit fingerprint: upgrading a
// grammar invalidates exactly the graphs parsed under the old
// version.
//
// # Bindings
//
// The official tree-sitter Go bindings are cgo; a consumer binary
// that embeds a tree-sitter satellite therefore needs a C
// toolchain. The binding choice is private to this module — the
// parsing surface it exports is pure Go — so a binding change is
// invisible to satellites and their consumers.
package lang
