// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package plugin is the service provider interface: the contracts
// the run invokes and the data it reads about a plugin, which is
// the floor every authoring layer lowers to.
//
// [Plugin] is the base contract, a stable name. The roles are
// Plugin plus one method taking a context struct: [Annotator]
// stamps facts, [Generator] produces emit values into one plan.
// [Subscribed] adds the gate tuples as data, so the engine knows
// what a plugin watches without executing a handler; a plugin that
// skips it reads as one implicit subscription to everything in
// scope.
//
// # Two read surfaces
//
// Every context carries two, split by law. [Index] is the
// dispatcher's routing surface: untracked, scope-filtered, holding
// the validated directive table and the skip table, and minting
// the tracked readers. The [store.Reader] is the plugin's own
// path, recording every read. The index wraps the graph rather
// than exposing it, so nothing reachable from a context can make a
// structural write or read a stranger's raw directives.
//
// # The emit store
//
// [Emit] holds one plan's accumulated [Unit] values and a per-kind
// index over their declarations, maintained as units land, which
// is what prices an emit-triggered rule at its matches. A [Unit]
// carries its full routing key, so no consumer re-derives any part
// of it from the declarations.
//
// # Failure semantics
//
// A phase call attaches per-subject problems to its context's sink
// and continues; a returned error is fatal to the phase. A defect
// answers a plain error: a unit flushed twice or unroutable, a
// routing surface built over a moving graph. Nothing here panics.
//
// # Dependency position
//
// core/plugin imports core/diag, core/directive, core/emit,
// core/meta, core/node, core/store, core/symbol and the Go
// stdlib.
package plugin
