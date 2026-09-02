// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend declares frontends: [New] accumulates one
// language's read-side declaration — identity, comment syntax,
// file claim, partition, parse, resolve, classifiers — and
// [Builder.Build] lowers it to the [plugin.Frontend] role. The
// kit adds nothing the role does not state, so a kit-built
// frontend and a hand-rolled one meet the same conformance
// checks.
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
