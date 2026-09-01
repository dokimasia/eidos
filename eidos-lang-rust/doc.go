// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rust makes Rust a source and target language for eidos
// workspaces.
//
// The package registers the typed language identity that the
// boundary spelling "rust" resolves to, and the comment syntax the
// frontend and backend share. The module provides the Rust
// frontend, the projection rules, the lowering from canonical type
// shapes to Rust spellings, the rendering backend, and the
// Rust-only sdk importable from binding files alone.
//
// # Projection facts
//
//   - Data enums project onto the tagged Sum shape — variants
//     with payloads — and payload-free variant sets onto Enum.
//   - The error model is ResultType; async functions and Streams
//     are signature-visible.
//   - Ownership is read through the ownership rules: by-value,
//     borrow, or mutable borrow per parameter.
//   - Default trait method bodies carry on the method's
//     has-default flag; mod nesting maps into hierarchical
//     package paths; pub(crate) visibility normalizes with the
//     raw spelling kept in language metadata.
//   - Lifetimes are representable, not projectable: they carry as
//     language metadata on an opaque shape.
//
// # Parsing
//
// The frontend parses with the pinned tree-sitter grammar — a
// version-pinned library, never a machine-supplied toolchain — and
// resolves dependencies signature-only from crate source.
//
// # Dependency position
//
// The module imports the kernel's root authoring package and the
// SPI packages beneath it, the shared helpers of eidos-lang, and
// the Go stdlib.
package rust
