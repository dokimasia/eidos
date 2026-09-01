// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package load drives source into the node graph: the read side's
// pipeline from file selection to the sealed store.
//
// [Load] runs the phases in order. Selection matches each
// frontend's claim over the workspace tree and refuses an overlap.
// Partition groups the selected files into units through a
// recorded reader. Parse fans out over the units, each into a
// private builder. The splice merges the unit graphs in unit
// order — units sorted by their first file's path — so scheduling
// never orders the graph. The resolution step assigns canonical
// identities, keeps the first of two declarations spelling one,
// and resolves type references through each language's own
// bindings. The store then seals, and the report carries every
// unit's key. Parse is the one parallel phase; everything before
// and after it is sequential and ordered, which is where
// determinism lives.
//
// Directive validation is not this package's: the graph's consumer
// validates between the seal and its first handler, which is where
// the workspace run does it today.
//
// # Keys
//
// Every unit's key folds, in order: the unit's recorded reads, the
// partition's recorded reads, the unit's depth, the frontend's
// declared version, the frontend's options in their canonical
// encoding, the composition's plugin-set fingerprint, and
// [go.dokimi.dev/eidos/core/node.ModelFingerprint]. Each part is
// length-prefixed, so two parts cannot trade bytes and collide.
// The report records the keys; milestone 0007 consumes them.
//
// # Failure semantics
//
// A frontend's Partition or Parse error is fatal to the whole
// load: the resolution step runs over the union of every
// frontend's graph, and a partial union resolves wrong. What a
// unit's source gets wrong reports through the sink and the load
// continues: a duplicate identity keeps the first declaration
// under [DuplicateDeclaration], an ambiguous reference keeps the
// first candidate under [AmbiguousReference], and an unresolved
// spelling stays a spelling. A structural defect in what a
// frontend built — an emit-side symbol, a named kind without a
// name — panics naming the frontend, because a malformed graph
// discovered at resolution points away from the frontend that
// built it.
//
// # Dependency position
//
// core/load imports core/plugin, core/store, core/node,
// core/directive, core/diag, core/symbol, core/position and the Go
// stdlib; it drives frontends and writes the store, so the read
// side's pieces meet here and nothing beneath imports it back.
package load
