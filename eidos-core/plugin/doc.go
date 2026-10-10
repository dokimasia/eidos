// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package plugin is the service provider interface: the contracts
// the run invokes and the data it reads about a plugin, which is
// the base contract every authoring layer lowers to.
//
// [Plugin] is the base contract, a stable name spelled as [ID].
// The roles are Plugin plus one method taking a context struct:
// [Annotator] stamps facts, [Generator] produces emit values into
// one plan, [WorkspaceCheck] reports what the records of several
// plans break at Close, and [Backend] returns the [Target] its plan
// resolves at composition. [Subscribed] adds the gate tuples as data, so
// the engine knows what a plugin watches without executing a
// handler. A plugin that skips it reads as one implicit
// subscription to everything in scope. The provider interfaces,
// [KeyProvider] among them, are what the composition asserts to
// learn the rest of the declaration. [KeyBinder] binds a plugin's gates
// on named key handles once the key registry seals. [ValidateOptions] is the
// one check an options struct passes, at composition and in the
// conformance suite alike.
//
// # Two read surfaces
//
// Every context has two, split by rule. [Index] is the dispatcher's
// routing surface. It enumerates the declarations under the scope
// without tracking the reads, and it reads the [Validated] directive
// table and the skip rulings that the table states. It also returns the
// tracked readers. The [store.Reader] is the plugin's own path, recording
// every read. The index wraps the graph and does not expose it, so nothing
// reachable from a context can make a structural write or read another
// plugin's raw directives.
//
// # Invocations
//
// A [MatchKey] names one invocation of a phase call across runs: the
// plugin, the rule, the subject, the gating instance and, for an
// emit-phase rule, the [EmitRef] of the value it matched. The order of
// the key is the canonical match order. A phase context's
// [Selection] restricts the call to the listed matches and the
// candidates' matches, and its [Journal] receives one [Invocation] for
// each invocation the call ran: what it read, the exports it read, the
// [UnitRef] of each unit it touched, the values whose slots it
// appended into, the facts it stamped and the findings it reported. A
// plugin that implements its role directly may ignore both, and the
// run records its call as one invocation under [WholeCall].
//
// # The read side
//
// [Frontend] loads one language into the node graph through a
// [SourceUnit], the one door bytes enter a parse by. A frontend attaches
// the carriers of a comment through [SourceUnit.AttachCarriers]. A
// [Sugar] is a marker of metadata in the language's own syntax, and a
// frontend attaches it through [SourceUnit.AttachSugar]. Three optional
// roles widen what a frontend does: [Importer] names a file's import
// of another file, [Exporter] names what a file publishes and does not
// declare, and [Dependent] returns the dependency units the load
// parses signature-only. A dependency unit's members are qualified
// paths into the named stores a [StoreFS] provides, spelled by
// [StorePath], or workspace paths the selection does not claim.
//
// # The emit store
//
// [Emit] contains one plan's accumulated [Unit] values and a per-kind
// index over their declarations, maintained as units arrive, which
// is what makes an emit-triggered rule cost only its matches. A [Unit]
// records its full routing key, so no consumer re-derives any part
// of it from the declarations. [Emit.Ref] returns the [EmitRef] of a
// declaration in the store, from an index it builds on first use.
//
// # Routed files
//
// A plan's layout routes its settled units into [File] values: a
// workspace-relative path, the package the file declares, and the
// units it assembles. Two target surfaces state the language's part
// of the routing. [FileSpeller] splits a unit into the files the
// target writes and spells each filename, and [Packager] names the
// package a file at a routed path declares, from a [Placement] the
// run derives from the graph and the fact store. The render pass
// renders the routed files it receives in [RenderContext.Files].
//
// # The hub
//
// A backend that spells the types of other languages in its target
// implements [TypeSpeller], the spoke of the cross-language hub. The
// spoke spells a canonical [rules.TypeShape] under a resolved [Policy].
// The backend declares the lowering policies that its spoke reads
// through [PolicyProvider]. Each policy is a [PolicySpec] of a
// [PolicyKey] and its [Choice] values. [NewPolicy] resolves the policy
// of one plan, and the result is total. Every declared key has a
// choice, and [Policy.Choice] panics for a key that the backend does
// not declare. [NameParam] is the param of a target's directive that
// overrides the name of one declaration.
//
// # Exports
//
// [NewExport] lists what a plan rendered as an [ExportDoc]: each
// declaration under an [ExportKey] a dependent builds before the run,
// from the declaration's origin, its plugin, its family and its emitted
// names, with the spelling the settle chose and the file and package the
// layout chose. A dependent plan reads the exports of the plans it
// depends on from [GeneratorContext.Exports], and a workspace check reads
// every plan's export and manifest entries from [CheckContext.Plans].
//
// # Failure semantics
//
// A phase call attaches per-subject problems to its context's sink
// and continues. A returned error is fatal to the phase. A defect
// returns a plain error: a unit flushed twice, a unit with a nil
// declaration, a routing surface built over a moving graph.
//
// One defect class panics instead: a record on a nil subject through
// [GraphBuilder.Scope], [GraphBuilder.Attach], [GraphBuilder.Stamp] or
// [GraphBuilder.Rehome]. Nothing could ever join such a record to a
// declaration, and a frontend that records one is broken in its own
// constructor, not on its input.
//
// # Dependency position
//
// core/plugin imports core/diag, core/directive, core/emit,
// core/manifest, core/meta, core/node, core/position, core/rules,
// core/store, core/symbol and the Go stdlib.
package plugin
