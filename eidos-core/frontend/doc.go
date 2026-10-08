// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend declares frontends: [New] accumulates one
// language's read-side declaration, which is its identity, comment
// syntax, overloading, file claim, partition, parse, resolve and
// classifiers, and [Builder.Build] lowers it to the
// [plugin.Frontend] role. A declaration can state four optional roles
// beside it:
//
//   - its options, through [Builder.Options]
//   - its dependency rounds, through [Builder.Dependencies]
//   - what its files re-export, through [Builder.Exports]
//   - the stores that its dependency rounds read, through
//     [Builder.Stores]
//
// The built frontend implements exactly the roles the declaration states. The kit adds nothing the
// roles do not state, so a kit-built frontend and a hand-rolled one
// meet the same conformance checks.
//
// # Failure semantics
//
// A declaration defect panics at [Builder.Build], before
// any run exists. A [Classifier] error is fatal to the load the
// way a parse error is.
//
// # Dependency position
//
// core/frontend imports core/plugin, core/symbol and the Go
// stdlib. Nothing beneath it imports it back.
package frontend
