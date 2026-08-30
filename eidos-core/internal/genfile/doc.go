// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package genfile formats, writes and guards generated Go sources.
//
// A generator renders its output into a [Set], a map from
// module-relative slash path to bytes. [Format] canonicalizes each
// file, [Write] puts the set on disk, and [Verify] is the mirror
// guard: it reports whether the tree matches what the generator
// would produce right now.
//
// # The guard
//
// [Verify] fails on three conditions, and each one is a real
// defect. A file whose bytes differ means somebody edited generated
// output or changed the source without regenerating. A missing file
// means the tree is incomplete. A generated file on disk that the
// set does not name is a stray left behind by a rename or a
// deletion, which would otherwise compile forever.
//
// # Dependency position
//
// core/internal/genfile imports only the Go stdlib.
package genfile
