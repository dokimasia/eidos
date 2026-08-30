// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package store holds the node graph and the tracked reads over it.
//
// [Graph] holds the declarations a run loaded, seals them at
// [Graph.Freeze], and hands out the [Reader] that is the only path a
// plugin's read takes. [ReadSet] is what one derived artifact read,
// and [Scope] is the packages one reader may see.
//
// # Phases
//
// Loading and annotating are different phases, and the difference is
// enforced rather than documented. Before the seal the graph admits
// packages and answers nothing: both indexes build at Freeze, where
// they are free, because nothing may add a declaration afterwards.
// After the seal a structural write is refused under [FrozenWrite]
// and a read answers from the indexes.
//
// # Reading
//
// Reads come at two grains, and the grain is what keeps a read from
// costing more than it observed.
//
//   - [Reader.Lookup] and [Reader.PackageOf] record a per-identity
//     edge: the artifact runs again when that declaration changes.
//   - [Reader.ByKind] records a set-membership edge: the artifact
//     runs again when a declaration of that kind enters or leaves
//     the set, plus a per-identity edge for each declaration the
//     caller actually reached while iterating.
//
// [Graph.ByKind] and [Graph.Lookup] answer the same questions
// untracked. They are the kernel's own path, for the dispatcher
// deciding which rules a phase runs: dispatch is not a plugin's read
// and must not record one. A plugin holds a [Reader] and never the
// graph.
//
// # Failure semantics
//
// A condition a run can meet answers a [RefusedError], which carries the
// code a consumer scripts against. A condition only a defect
// reaches, such as adding no package at all, answers a plain error.
// Nothing here panics.
//
// # Dependency position
//
// core/store imports core/node, core/symbol, core/diag, core/meta
// and the Go stdlib; the read set satisfies the fact recorder, so
// the two stores meet here, on this side.
package store
