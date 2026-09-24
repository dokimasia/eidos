// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package php is the eidos module for PHP as a source and target
// language.
//
// # Scope
//
// The satellite's parts are built against PHP's projection
// decisions:
//
//   - A nullable type projects as Optional. A native union has no
//     tagged counterpart, so it is a type shape and not a
//     declaration kind.
//   - An attribute is read statically and never executed.
//   - A backed or pure enum is the Enum kind. A trait projects
//     through Embeds, with its spelling kept in language metadata.
//   - The error model is Thrown.
//
// PHP's frontend reads a tree-sitter grammar pinned as a library, not
// a machine-supplied toolchain, so one workspace resolves one parser
// everywhere.
//
// The module contains this statement of scope and no code.
//
// # Dependency position
//
// The package imports nothing. Its test imports the assert module.
package php
