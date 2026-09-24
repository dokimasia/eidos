// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package lang is the root of the module of helper packages the
// language satellites build their frontends and backends from. The
// kernel calls none of them, so the helpers are in their own module
// below the satellites.
//
// # Packages
//
//   - [go.dokimi.dev/eidos/lang/lowering] copies emit-model shapes and
//     checks method names for the backends' Lower hooks.
//   - [go.dokimi.dev/eidos/lang/naming] converts identifiers between
//     case conventions and builds filenames from a unit's parts.
//   - [go.dokimi.dev/eidos/lang/scaffold] spells the statement and
//     value vocabulary for the C-family targets.
//   - [go.dokimi.dev/eidos/lang/spellref] spells emit-model type
//     references.
//   - [go.dokimi.dev/eidos/lang/textfmt] writes comments and import
//     statements and normalizes rendered source.
//
// # Dependency position
//
// The root package imports nothing. naming imports the Go stdlib
// alone. lowering and spellref import sdk/emit, textfmt imports
// sdk/render and sdk/symbol, and scaffold imports sdk/emit,
// sdk/render and sdk/symbol, each beside the Go stdlib. No package in
// the module parses source or binds a grammar.
package lang
