// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package meta carries typed facts between plugins.
//
// Metadata is the only channel between plugins: a fact that has to
// move between plugins goes on the graph, never through an import. [Key] is the
// typed handle a registration returns, [Registry] refuses a key or
// a namespace claimed twice naming both claimants, and [Facts]
// holds the run's stamped facts, one bag per subject.
//
// # Arbitration
//
// Every write carries a [Claim] and rank decides the winner, never
// arrival order: higher [Authority] first, then the earlier
// capability bucket, then the plugin name alphabetically, then the
// first claim in canonical match order. A later claim carrying a
// different value does nothing — that is precedence rather than
// conflict — and every claim is kept, so [Facts.Claims] shows a
// losing write instead of losing it.
//
// A drop is a claim of absence at directive authority: it outranks
// a plugin stamp whenever the stamp arrives, and loses to a manual
// write. [Facts.DropGroup] covers every member of a fact group,
// stamps that arrive after it included.
//
// # Reading
//
// [Get] returns the winning value untracked, which is the kernel's
// own path. [Fact] records the read at (subject, key) into a
// [Recorder] — a miss records too — and is the read every plugin
// makes. [Facts.ByKey] enumerates the subjects a key presently
// reads present on, in identity order, maintained at stamp time.
//
// # Failure semantics
//
// A write is refused with an error for a key nothing registered, a
// subject kind the key does not admit, and a false boolean: absence
// is the negative, so false is never stamped. Nothing here panics.
//
// # Dependency position
//
// core/meta imports core/symbol, core/position, core/diag and the
// Go stdlib. It never imports core/store: the store's read set
// implements [Recorder], so the two meet there, on the store's
// side.
package meta
