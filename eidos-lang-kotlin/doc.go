// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package kotlin is the eidos module for Kotlin as a source and
// target language.
//
// # Scope
//
// The satellite's parts are built against Kotlin's projection
// decisions:
//
//   - A sealed class is the tagged Sum shape. Variance is
//     declaration-site and is stated on type parameters.
//   - suspend is signature-visible and projects as Async.
//   - A property is a computed view, not a model mutation. A
//     companion is a type-level member.
//   - internal visibility normalizes, and its raw spelling is kept in
//     language metadata. A nullable type projects as Optional.
//   - Generics are erased, and a reified type parameter is recorded
//     as language metadata.
//
// Kotlin's frontend reads a tree-sitter grammar pinned as a library,
// not a machine-supplied toolchain. Its dependency types resolve
// signature-only from JVM class files inside JARs, which are parsed
// and never executed.
//
// The module contains this statement of scope and no code.
//
// # Dependency position
//
// The package imports nothing. Its test imports the assert module.
package kotlin
