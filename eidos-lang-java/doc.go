// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package java makes Java a source and target language for eidos
// workspaces.
//
// The package registers the typed language identity that the
// boundary spelling "java" resolves to, and the comment syntax the
// frontend and backend share. The module provides the Java
// frontend, the projection rules, the lowering from canonical type
// shapes to Java spellings, the rendering backend, and the
// Java-only sdk importable from binding files alone.
//
// # Projection facts
//
//   - Annotations read statically through the annotation rules as
//     structured, queryable, overridable metadata — never
//     executed.
//   - Generics are erased; the generics rules report erasure and
//     supply witnesses and substitution.
//   - Overloads are legal: member lists are slices, and identity
//     carries a signature discriminator.
//   - Checked exceptions are read through the throws rules; enums
//     are classes and carry members.
//   - Unannotated references are nullability-unknown: a metadata
//     fact plus a workspace lowering policy, never a third
//     Optional state.
//
// # Parsing
//
// The frontend parses with the pinned tree-sitter grammar and
// resolves dependencies signature-only from JVM class files inside
// JARs — declarative artifacts, parsed and never executed.
//
// # Dependency position
//
// The module imports the kernel's root authoring package and the
// SPI packages beneath it, the shared helpers of eidos-lang, and
// the Go stdlib.
package java
