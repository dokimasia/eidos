// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package kotlin is the module Kotlin takes as a source and target
// language for eidos workspaces: the typed language identity the
// boundary spelling "kotlin" resolves to, and the comment syntax a
// frontend and a backend share.
//
// # Scope
//
// Kotlin's projection decisions, which the satellite's parts are
// built against:
//
//   - A sealed class is the tagged Sum shape; variance is
//     declaration-site and carries on type parameters.
//   - suspend is signature-visible and projects as Async.
//   - A property is a computed view rather than a model mutation;
//     a companion is a type-level member.
//   - internal visibility normalizes, its raw spelling kept in
//     language metadata; a nullable type projects as Optional.
//   - Generics are erased, and a reified type parameter carries as
//     language metadata.
//
// Kotlin's frontend reads a tree-sitter grammar pinned as a
// library rather than a machine-supplied toolchain, and its
// dependency types resolve signature-only from JVM class files
// inside JARs, parsed and never executed.
//
// The module holds this statement of scope and no code.
package kotlin
