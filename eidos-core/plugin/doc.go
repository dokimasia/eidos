// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package plugin is the service provider interface: the contracts
// the run invokes and the data it reads about a plugin, which is
// the base contract every authoring layer lowers to.
//
// [Plugin] is the base contract, a stable name spelled as [ID].
// The roles are Plugin plus one method taking a context struct:
// [Annotator] stamps facts, [Generator] produces emit values into
// one plan, and [Backend] carries the [Target] its plan resolves
// at composition. [Subscribed] adds the gate tuples as data, so
// the engine knows what a plugin watches without executing a
// handler; a plugin that skips it reads as one implicit
// subscription to everything in scope. The provider interfaces,
// [KeyProvider] among them, are what the composition asserts to
// learn the rest of the declaration, and [ValidateOptions] is the
// one check an options struct passes, at composition and in the
// conformance suite alike.
//
// # Two read surfaces
//
// Every context carries two, split by rule. [Index] is the
// dispatcher's routing surface: untracked, scope-filtered, holding
// the validated directive table and the skip table, and minting
// the tracked readers. The [store.Reader] is the plugin's own
// path, recording every read. The index wraps the graph rather
// than exposing it, so nothing reachable from a context can make a
// structural write or read another plugin's raw directives.
//
// # The emit store
//
// [Emit] holds one plan's accumulated [Unit] values and a per-kind
// index over their declarations, maintained as units arrive, which
// is what makes an emit-triggered rule cost only its matches. A [Unit]
// carries its full routing key, so no consumer re-derives any part
// of it from the declarations.
//
// # Failure semantics
//
// A phase call attaches per-subject problems to its context's sink
// and continues; a returned error is fatal to the phase. A defect
// returns a plain error: a unit flushed twice or unroutable, a
// routing surface built over a moving graph. Nothing here panics.
//
// # Dependency position
//
// core/plugin imports core/diag, core/directive, core/emit,
// core/meta, core/node, core/store, core/symbol and the Go
// stdlib.
package plugin
