// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package lang is the tree-sitter platform shared by the
// tree-sitter-based language satellites: one binding layer and the
// pinned grammar set, so every satellite parses through the same
// machinery and a grammar or binding upgrade lands once.
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
// first-class pure-Go parsers (the standard library's go/parser
// and bufbuild/protocompile) and take no dependency on this
// module.
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
