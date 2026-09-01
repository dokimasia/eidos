// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package kotlin makes Kotlin a source and target language for
// eidos workspaces.
//
// The package registers the typed language identity that the
// boundary spelling "kotlin" resolves to, and the comment syntax
// the frontend and backend share. The module provides the Kotlin
// frontend, the projection rules, the lowering from canonical type
// shapes to Kotlin spellings, the rendering backend, and the
// Kotlin-only sdk importable from binding files alone.
//
// # Projection facts
//
//   - Sealed classes project onto the tagged Sum shape; variance
//     is declaration-site and carries on type parameters.
//   - suspend is signature-visible and projects as Async; Flow
//     projects as an async Stream.
//   - Properties are read through the property rules as a computed
//     view, never a model mutation; companions are Type-level
//     members.
//   - internal visibility normalizes with the raw spelling kept
//     in language metadata; nullable types project as Optional.
//   - Generics are erased; reified type parameters carry as
//     language metadata.
//
// # Parsing
//
// The frontend parses with the pinned tree-sitter grammar — a
// version-pinned library, never a machine-supplied toolchain — and
// resolves dependencies signature-only from JVM class files inside
// JARs, parsed and never executed.
package kotlin
