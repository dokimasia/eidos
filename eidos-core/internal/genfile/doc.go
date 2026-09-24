// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package genfile formats, writes and guards generated Go sources.
//
// A generator renders its output into a [Set], a map from
// module-relative slash path to bytes. Every file opens with the
// preamble [Header] returns. [Render] formats a generator's rendered
// files into the set through [Format], and [Write] puts the set on
// disk. [Regenerate] renders a set and writes it, for a go:generate
// wrapper. [Verify] is the mirror guard: it reports whether the tree
// matches what the generator produces from the current sources.
//
// # The guard
//
// [Verify] fails on three conditions, and each one is a real
// defect. A file whose bytes differ means somebody edited generated
// output or changed the source without regenerating. A missing file
// means the tree is incomplete. A generated file on disk that the
// set does not name is a stray from a rename or a deletion, and it
// compiles until somebody deletes it.
//
// # Dependency position
//
// core/internal/genfile imports only the Go stdlib.
package genfile
