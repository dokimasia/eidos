// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package php is the module PHP takes as a source and target
// language for eidos workspaces: the typed language identity the
// boundary spelling "php" resolves to, and the comment syntax a
// frontend and a backend share.
//
// # Scope
//
// PHP's projection decisions, which the satellite's parts are
// built against:
//
//   - A nullable type projects as Optional. A native union has no
//     tagged counterpart, so it stays a type shape rather than a
//     declaration kind.
//   - An attribute reads statically, never executed.
//   - A backed or pure enum is the Enum kind; a trait carries
//     through Embeds with its spelling kept in language metadata.
//   - The error model is Thrown.
//
// PHP's frontend reads a tree-sitter grammar pinned as a library
// rather than a machine-supplied toolchain, so one workspace
// resolves one parser everywhere.
//
// The module holds this statement of scope and no code.
package php
