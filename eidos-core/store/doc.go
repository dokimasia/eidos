// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package store contains the node graph and the tracked reads over it.
//
// [Graph] contains the declarations a run loaded and hands out the
// [Reader] that is the only path a plugin's read takes. [ReadSet] is
// what one derived artifact read, and [Scope] is the packages one
// reader may see.
//
// # Phases
//
// Loading and annotating are different phases, and the types enforce
// the difference. A graph [New] returns admits packages and returns
// nothing before the seal: both indexes build at [Graph.Freeze],
// because nothing may add a declaration afterwards. After the seal a
// structural write is refused under [FrozenWrite] and a read returns
// from the indexes.
//
// # Sealed graphs
//
// [Sealed] returns a frozen graph over a [Source]: one [Region] for
// each frontend unit, each with a [RegionInfo] summary that the source
// states without decoding the region. A read decodes the regions it
// needs on first use, so a warm run that reads one package decodes that
// package's regions and no other. A region that fails to decode reads
// as absent, and [Graph.Damaged] returns the failure.
//
// # Reading
//
// Reads record four kinds of edge, and the kind keeps a read from
// costing more than it observed.
//
//   - [Reader.Lookup] records a declaration edge: the artifact runs
//     again when that declaration appears, disappears or changes.
//   - [Reader.PackageOf], and [Reader.Lookup] of a package's own
//     identity, record a package edge: the caller can walk every member
//     of the package it took, so the artifact runs again when any
//     member changes.
//   - [Reader.ByKind] and [Reader.ByDirective] record a membership
//     edge: the artifact runs again when a declaration of that kind, or
//     a carrier of that spelling, enters or leaves the set. Each also
//     records a declaration edge for each declaration the caller
//     met while iterating.
//   - A fact read records a fact edge at (subject, key) into the same
//     [ReadSet], which satisfies [meta.Recorder].
//
// A [ReadLog] keeps the edges of many read sets in flat storage, one
// entry for each, and loads an entry back into a set: what a dispatcher
// keeps of each invocation's reads while it reuses one set across a
// rule's invocations.
//
// [Graph.ByKind], [Graph.Lookup] and [Graph.PackageOf] return the same
// questions untracked. They are the kernel's own path, for the
// dispatcher deciding which rules a phase runs: dispatch is not a
// plugin's read and records none. A plugin has a [Reader] and never the
// graph.
//
// # Failure semantics
//
// A condition a run can meet returns a [RefusedError], which contains
// the code a consumer scripts against. A condition only a defect
// produces, such as adding no package at all, returns a plain error.
// Nothing here panics.
//
// # Dependency position
//
// core/store imports core/node, core/symbol, core/diag, core/meta,
// core/directive, core/internal/grow and the Go stdlib; the read set
// satisfies the fact recorder and the graph attaches and indexes raw
// directive instances, so those packages meet here, on this side.
package store
