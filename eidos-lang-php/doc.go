// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package php makes PHP a source and target language for eidos
// workspaces.
//
// The package registers the typed language identity that the
// boundary spelling "php" resolves to, and the comment syntax the
// frontend and backend share. The module provides the PHP
// frontend, the projection rules, the lowering from canonical type
// shapes to PHP spellings, the rendering backend, and the PHP-only
// sdk importable from binding files alone.
//
// # Projection facts
//
//   - Native union types project onto the untagged Union shape;
//     nullable types project as Optional.
//   - Attributes read statically through the annotation rules —
//     never executed.
//   - Backed and pure enums land on the Enum kind; traits ride
//     Embeds with the trait spelling kept in language metadata.
//   - The error model is Thrown; constructors are read through
//     the construct rules.
//
// # Parsing
//
// The frontend parses with the pinned tree-sitter grammar — a
// version-pinned library, never a machine-supplied toolchain — so
// the same workspace resolves the same parser everywhere.
package php
