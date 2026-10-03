// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package meta records the typed facts plugins exchange.
//
// Metadata is the only channel between plugins: a fact that has to
// move between plugins goes on the graph, never through an import.
// [Key] is the typed handle a registration returns, [Registry]
// records the registered namespaces and keys, and [Facts] stores the
// run's stamped facts, one bag per subject.
//
// # Registration
//
// A [Registry] handle is bound to one registrant: a plugin's name,
// or no name for the composition. [Registry.For] returns the handle
// for a registrant, and every handle shares one set of
// registrations. A namespace belongs to the registrant whose handle
// claimed it, and a key registers only into a namespace its own
// registrant claimed. A namespace claimed twice and a key registered
// twice are errors that name both claimants. [Kernel] claims
// [KernelNamespace] as [KernelOwner], so no plugin registers a key
// under it.
//
// # Arbitration
//
// Every write comes with a [Claim], and rank decides the winner,
// never arrival order: higher [Authority] first, then the earlier
// capability bucket, then the plugin name alphabetically, then the
// first claim in canonical match order, which the claim's [Order]
// states as the rule, the invocation's subject and the gating instance.
// A claim that loses on rank changes nothing, whenever it arrives.
// [Facts.Claims] lists every claim, the losing ones included.
//
// A drop is a claim of absence at directive authority: it outranks
// a plugin stamp whenever the stamp arrives, and loses to a manual
// write. [Facts.DropGroup] covers every member of a fact group,
// including the stamps that arrive after it.
//
// # Warm runs
//
// Every run computes the same [Order] for the same claim, so a claim a
// previous run recorded ranks against a claim the current run makes.
// [Restore] returns a store that loads each subject's recorded claims
// from a [BagSource] on first use, and each key's recorded presence on
// its first enumeration. [Facts.Withdraw] and [Facts.WithdrawGroup]
// remove the claim of one rank source before its match runs again, or
// where its match disappeared, and [Facts.ClaimedBy] lists the facts
// one plugin claimed in the current run. A source that fails leaves the
// bag empty, and [Facts.Damaged] returns the failure.
//
// # Reading
//
// [Get] returns the winning value untracked, which is the kernel's
// own path. [Fact] records the read at (subject, key) into a
// [Recorder], a miss included, and is the read every plugin makes.
// [Facts.ByKey] enumerates the subjects on which a key reads
// present, in identity order, maintained at stamp time.
//
// # Failure semantics
//
// A write returns an error for a key nothing registered, a subject
// kind the key does not admit, and a false boolean: absence is the
// negative, so false is never stamped. Nothing here panics.
//
// # Dependency position
//
// core/meta imports core/symbol, core/position, core/diag and the
// Go stdlib. It never imports core/store: the store's read set
// implements [Recorder].
package meta
