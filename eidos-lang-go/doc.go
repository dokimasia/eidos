// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package golang makes Go a source and target language for eidos
// workspaces.
//
// The package registers the typed language identity that the
// boundary spelling "golang" resolves to, and the comment syntax
// the frontend and backend share. The module provides the Go
// frontend, the projection rules, the lowering from canonical type
// shapes to Go spellings, the rendering backend, and the Go-only
// sdk importable from binding files alone.
//
// # Projection facts
//
// What Go says to the neutral vocabulary is fixed by the
// language, not by configuration:
//
//   - Callables are Sync always: Go concurrency is caller-side
//     and invisible in signatures.
//   - The error model is LastReturn; sentinel conventions are
//     read through the error-value rules as paired
//     name/predicate inverses.
//   - Composition is Embeds, never Extends: promotion is read
//     through the members projection and the promotion rules.
//   - Optionality projects from pointers; comparability is read
//     through the equality rules, naming the members that poison
//     it (slices, maps, funcs).
//   - Struct tags are read through the tag rules, const-group
//     enums through the enum rules, iota arithmetic included.
//
// # Parsing
//
// The frontend parses with the standard library's go/parser — a
// pinned, pure-Go library, never a machine-supplied toolchain —
// so positions and comment attachment are exact and the same
// workspace resolves the same parser everywhere. Dependencies
// load signature-only from module source.
//
// # Dependency position
//
// The module imports the kernel's root authoring package and the
// SPI packages beneath it, the shared helpers of eidos-lang, and
// the Go stdlib. It never imports eidos-lang's grammar packages:
// the frontend parses with the standard library, and the backend
// renders through the kernel's own pass.
package golang
