// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package typescript makes TypeScript a source and target language
// for eidos workspaces.
//
// The package registers the typed language identity that the
// boundary spelling "typescript" resolves to, and the comment
// syntax the frontend and backend share. The module provides the
// TypeScript frontend, the projection rules, the lowering from
// canonical type shapes to TypeScript spellings, the rendering
// backend, and the TypeScript-only sdk importable from binding
// files alone.
//
// # Projection facts
//
//   - Unions are untagged and project onto the Union shape,
//     distinct from the tagged Sum shape.
//   - Async is signature-visible: Promise returns project as
//     Async, AsyncIterator as an async Stream.
//   - The error model is Thrown.
//   - Optionality lowers as the undefined-union spelling;
//     interfaces are property-majority and carry fields under
//     admit-with-empties.
//   - Decorators read statically through the annotation rules;
//     namespaces map into hierarchical package paths.
//
// # Parsing
//
// The frontend parses with a pinned tree-sitter grammar — a
// version-pinned library, never a machine-supplied toolchain — and
// resolves dependencies signature-only through declaration files
// (.d.ts), parsed and never executed.
//
// # Dependency position
//
// The module imports the kernel's root authoring package and the
// SPI packages beneath it, the shared helpers of eidos-lang, and
// the Go stdlib.
package typescript
