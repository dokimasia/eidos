// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package directive holds the one grammar every carrier lowers to,
// the schemas that close it, and the validation that types it.
//
// A carrier — a comment marker in some language's source — hands
// over [Raw]: the grammar parsed by [Parse], values untyped, joined
// from continuations by [Join]. A [Schema] declares what a
// directive accepts: positional params in order, typed keyed
// params, a closed role set, repeatability, and the constraints
// between directives. [Registry] holds every schema, refuses what
// the contract refuses, and resolves spellings: bare while
// unambiguous, plugin-prefixed once two plugins claim one name.
// [Validate] runs over one subject's full list and returns
// [Directive] instances typed per schema, reporting every
// violation as a positioned Error under its own registered code
// before any handler runs.
//
// # The kernel's names
//
// Four names belong to the kernel and register through [Kernel]:
// meta, out, diag and skip. Their semantics stay with their
// owners; what lives here is the spelling and its validation. The
// reserved keys [ReservedOut] and [ReservedTag] are admitted on
// every directive and no plugin schema may claim them.
//
// # Failure semantics
//
// [Parse] and [Registry.Register] return errors: their callers are
// composition and carriers, which collect faults. [Validate]
// reports diagnostics: its findings are the author's to fix, and
// every one carries the carrier's position. Nothing here panics.
//
// # Dependency position
//
// core/directive imports core/symbol, core/position, core/diag,
// core/meta and the Go stdlib. It never imports core/store: the
// store attaches and indexes raw instances, so the dependency
// points from the store to here.
package directive
