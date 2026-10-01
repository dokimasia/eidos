// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package load drives source into the node graph: the read side's
// pipeline from file selection to the sealed store.
//
// [Load] runs the phases in order:
//
//   - Selection matches each frontend's claim over the workspace tree
//     and refuses an overlap.
//   - Partition groups the selected files into units through a
//     recorded reader.
//   - Parse lowers every unit into a private builder.
//   - The dependency rounds of each [plugin.Dependent] frontend load the
//     units its sources import from outside the workspace.
//   - The splice merges the unit graphs in unit order, the units sorted
//     by their first member's path, so scheduling never orders the
//     graph.
//   - The resolution step assigns canonical identities, keeps the first
//     of two declarations spelling one, and resolves type references.
//   - The store seals, and the report records every unit's key.
//
// Parsing is the one parallel step: the workspace's units parse
// together, and so do the units of each dependency round. Every other
// step is sequential and ordered, which makes the load deterministic.
//
// The load validates no directive. The graph's consumer validates
// between the seal and its first handler, as the workspace run does.
//
// # Resolution
//
// A type reference resolves through the bindings of its file's
// language. A named reference gains its target, and a structural one
// has no target of its own, because its children resolve in its place.
// For a language whose frontend is a [plugin.Importer], a reference
// resolved to another file's declaration also gains the package the
// frontend names that file's import by, because such a language's
// import names a file and its parse cannot know which file declares a
// name. For a language whose frontend is a [plugin.Exporter], a
// candidate that names no declaration is followed through the
// re-exports of its package.
//
// # The brand
//
// A load runs under the composition's [Config.Brand], and refuses a
// brand outside [output.Brand.Valid]. Every unit reads its carriers
// under the brand's three marks: brand: and +brand: set a directive,
// and -brand: negates one. Another tool's marks are comment text.
//
// # Outputs are never inputs
//
// A claimed file with a provenance trailer under the load's own brand
// is the workspace's own output, and it does not load. The exclusion
// runs on the bytes, after selection and before anything partitions,
// and the report lists what it refused. A file another brand stamped is
// ordinary input: parsed, classified by the language as generated, and
// gated by consumers like any other fact. The proof is the trailer
// alone. The load reads each claimed file's last [output.TailSize]
// bytes through [io.Seeker], and the whole file only where they contain
// the trailer's key. A file that cannot seek is read whole.
//
// # Stores and dependency rounds
//
// [Config.Stores] names the read-only trees beside the workspace that
// dependency units read, such as a Go module cache or a JDK's ct.sym. A
// file of a store has a qualified path: the store's name, "://" and the
// path inside the store, as [plugin.StorePath] returns it. Every door of
// the load resolves a qualified path in its store, and a qualified path
// naming a store the load does not provide returns an error wrapping
// [plugin.ErrStoreAbsent].
//
// After the workspace's units parse, each [plugin.Dependent] frontend
// runs its rounds, in composition order. A round hands the frontend the
// import paths its files name that no loaded package of its language
// declares and no earlier round passed, and the shared inputs its
// partition declared on the workspace's units. The load parses every
// unit a round returns at [plugin.DepthSignatures], and the imports of
// those units are the next round's needs. The first round always runs,
// and a later round runs only when it has a need. A returned unit whose
// every member a loaded unit already has is dropped, so a frontend that
// returns its declared dependencies in every round loads them once. The
// report records each unit's round, and zero for a unit the partition
// returned.
//
// # Keys
//
// Every unit's key folds, in order: the unit's recorded reads, the
// recorded reads of the door that shaped the unit, the unit's depth,
// the frontend's name, language and declared version, the frontend's
// options in their canonical encoding, the composition's plugin-set
// fingerprint, the brand, and
// [go.dokimi.dev/eidos/core/node.ModelFingerprint]. The door is the
// partition for a unit of the workspace and the round for a dependency
// unit, and its record contains every file it read and every directory
// it listed. No key folds a store's location on disk. Each part is
// length-prefixed, so two parts cannot trade bytes and collide. The
// report records the keys.
//
// # Failure semantics
//
// A frontend's Partition, Parse or Dependencies error is fatal to the
// whole load: the resolution step runs over the union of every
// frontend's graph, and a partial union resolves wrong. So is a
// dependency unit that breaks the round's contract: an empty unit, a
// member some frontend's selection claims, a member the round returns
// twice, or a unit that shares only some of its members with loaded
// units. What a unit's source gets wrong reports through the sink and
// the load continues: a duplicate identity keeps the first declaration
// under [DuplicateDeclaration], an ambiguous reference keeps the first
// candidate under [AmbiguousReference], and an unresolved spelling
// remains a spelling. A structural defect in what a frontend built, such
// as an emit-side symbol, a named kind without a name or a nil entry in
// a signature, panics naming the frontend, because a malformed graph
// discovered at resolution points away from the frontend that built it.
//
// # Dependency position
//
// core/frontend/load imports core/plugin, core/store, core/node,
// core/directive, core/meta, core/output, core/diag, core/symbol,
// core/position and the Go stdlib. It drives frontends and writes the
// store, so the read side's pieces meet here, and no package beneath it
// imports it.
package load
