// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package load drives source into the node graph: the read side's
// pipeline from file selection to the sealed store.
//
// [Load] runs the phases in order:
//
//   - The fingerprint gate stats every file of the tree and records it
//     as a [FileRecord], hashing a file only where the last commit's
//     record cannot prove it unchanged.
//   - Selection matches each frontend's claim over the workspace tree
//     and refuses an overlap, and the gate judges each claimed file an
//     input or the brand's own output.
//   - Partition groups the selected files into units through a
//     recorded door, and the report keeps the door's [DoorRecord].
//   - Every unit's key folds the gate's digests of its members, before
//     the unit parses.
//   - A unit whose key the last load recorded is kept, one whose key
//     the memo contains is restored, and every other parses: its
//     frontend lowers it into a private builder.
//   - The dependency rounds of each [plugin.Dependent] frontend load the
//     units its sources import from outside the workspace.
//   - The splice merges the parsed units' graphs in unit order, the
//     units sorted by their first member's path, so scheduling never
//     orders the graph.
//   - The resolution step assigns the parsed units canonical
//     identities, keeps the first of two declarations spelling one, and
//     resolves type references, and selects the references of kept and
//     restored units again where a change can move their targets.
//   - Each parsed unit becomes a [store.Region], and the graph is the
//     one [store.Sealed] returns over every unit's region.
//
// Parsing is the one parallel step: the workspace's units parse
// together, and so do the units of each dependency round. Every other
// step is sequential and ordered, which makes the load deterministic.
//
// # Warm loads
//
// [Config.Prior] is the last committed load's record. A load with a
// prior leaves the graph, the findings and the units a load without one
// leaves over the same tree, and reads only what changed:
//
//   - The gate proves a file unchanged by its stat, and hashes the rest.
//   - A frontend's partition runs again only where its claimed paths
//     changed or a path the partition read reads otherwise, a path
//     nothing was at included. A frontend's dependency rounds run again
//     only where the first round's needs or shared inputs changed, a
//     path a round read reads otherwise, or a round's unit changed.
//   - A unit whose key the record states is kept: the graph decodes its
//     region from the record the first time a read needs it, and the
//     load reports the findings the record states for it.
//   - A unit the record cannot keep is restored from [Config.Memo] where
//     the memo contains its key, except in a language whose frontend is
//     a [plugin.Importer], and parses otherwise.
//   - A kept or restored unit parses where a parsed or restored unit
//     couples to it: a unit of the same package that declares an
//     identity the change added or removed, every unit of a package that
//     records a duplicate declaration, every unit of a package whose
//     units changed or were renamed where the package's units now spell
//     more than one name, and a kept unit of an importing language whose
//     references named an identity that appeared, disappeared or moved
//     to another file.
//   - The references of every restored unit, and of every kept unit
//     whose references named, or found through a re-export, a
//     declaration that appeared or disappeared, or followed a re-export
//     of a package a changed exporting unit contributes to, select their
//     targets again from the candidates the record names. A link that
//     follows a re-export through a kept unit's files parses that unit
//     for its bindings and keeps its region.
//
// [Report.Changes] states what changed by identity: the declarations
// that appeared, disappeared and changed, and their packages.
// [Report.Reparsed] counts the units the load parsed though the record
// kept them, and [Report.Decoded] the regions it and its graph decoded.
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
// alone, read from the bytes the gate hashes, so a claimed file whose
// record proves it unchanged keeps its recorded verdict without a read.
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
// A frontend reports each need it places nowhere through the round, with
// the reason. The load reports each such need once, under
// [UnplacedNeed], at its import in the first file in path order that
// imports it, naming how many files import it and quoting the reason.
// Every reference into the need keeps its spelling.
//
// # The gate
//
// A file whose record proves it unchanged keeps its digest and its
// verdict without a read: its size and modification time match, its
// change time and inode match where the stat reports them, and its
// modification time precedes the anchor of the run that recorded it. A
// matching file with a later modification time is racily clean, and the
// gate hashes it. Every other file a frontend claims, and every file a
// unit or a door reads, is read whole and hashed, and the same read
// proves whether a claimed file ends in the brand's provenance trailer.
// The walk skips the brand's state directory. [Report.Anchor] is the
// instant the sweep began, less two seconds, which the next load's gate
// compares modification times with.
//
// # Keys
//
// Every unit's key folds, in order: each member's path and digest, each
// with its shared inputs' paths and digests, in roster order; the fold
// of the record of the door that shaped the unit; the unit's depth; the
// frontend's name, language and declared version; the frontend's options
// in their canonical encoding; the brand; and
// [go.dokimi.dev/eidos/core/node.ModelFingerprint]. The door is the
// partition for a unit of the workspace and the round for a dependency
// unit, and its record contains every file it read and every directory
// it listed, each with its digest. A shared input no file is at folds
// as absent. No key folds a store's location on disk, and no key folds
// the composition, because a region depends on the frontend and the
// unit's inputs alone. Each part is length-prefixed and each list
// counted, so two parts cannot trade bytes and collide. The report
// records the keys.
//
// # Regions
//
// A unit's [store.Region] is its own package nodes after the splice,
// the assignment and the link, the directives and stamps its frontend
// attached, resolved to identities, the record of every reference its
// frontend resolved, and the findings its parse, the splice and the
// assignment reported about it. The report contains the region of each
// unit the load parsed, restored or selected references of again, and
// [Config.Memo] records each region a load parsed under the unit's key.
//
// # Failure semantics
//
// A frontend's Partition, Parse or Dependencies error is fatal to the
// whole load: the resolution step runs over the union of every
// frontend's graph, and a partial union resolves wrong. So is a
// dependency unit that breaks the round's contract: an empty unit, a
// member some frontend's selection claims, a member the round returns
// twice, or a unit that shares only some of its members with loaded
// units, and so is a report of a path that is no need of the round.
// What a unit's source gets wrong reports through the sink and
// the load continues: a duplicate identity keeps the first declaration
// under [DuplicateDeclaration], an ambiguous reference keeps the first
// candidate under [AmbiguousReference], and an unresolved spelling
// remains a spelling. A structural defect in what a frontend built, such
// as an emit-side symbol, a named kind without a name or a nil entry in
// a signature, panics naming the frontend, because a malformed graph
// discovered at resolution points away from the frontend that built it.
//
// A prior whose units, doors, probes or regions do not read whole fails
// the load with the prior's own error, which the caller reads as damage
// and loads again without the prior. A kept region that fails to decode
// after the load reads as absent, and the graph's
// [store.Graph.Damaged] returns the failure.
//
// # Dependency position
//
// core/frontend/load imports core/plugin, core/store, core/node,
// core/directive, core/meta, core/output, core/diag, core/symbol,
// core/position and the Go stdlib. It drives frontends and writes the
// store, so the read side's pieces meet here, and no package beneath it
// imports it.
package load
