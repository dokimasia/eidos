// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package eidos is the kernel of a code-generation framework:
// frontends parse source into a language-neutral symbol graph,
// plugins annotate it and generate output entities, and backends
// render those into target languages, deterministically and
// incrementally, across languages in one run.
//
// The kernel is a library. It ships no binary; consumers compose a
// workspace in their own main and hand it to the command kernels.
// It knows no language: language support is in satellite modules
// that implement the projection tiers, and no satellite name,
// language string, or per-language table appears in the kernel. Its
// runtime packages import the module's own packages and the
// standard library alone.
//
// # Composition model
//
// One workspace contains the read side: frontends, annotators and
// one frozen symbol graph. N plans contain the write side, each with
// its own generators, layout and exactly one backend, and the
// composition declares the one output a run writes through. Plans
// are values, run in parallel, and exchange nothing except declared,
// versioned exports. Plugins compose at compile time and
// communicate only through typed metadata on the graph: every write
// records its authority (plugin < directive < manual) and its
// provenance, so a human override at the declaration outranks any
// inference and every fact records what it was derived from.
//
// # Authoring model
//
// This root package is the authoring surface. A plugin is a value:
// [NewPlugin] starts the declaration, identity, outputs, priorities
// and schemas are data built once, and only the handlers, the key
// registration declared through [Builder.Keys] and the template
// helpers are functions. Handlers attach to kind-indexed triggers,
// one generated constructor per subject kind such as [OnInterface],
// plus [OnGraph] and [OnEmit]. Gates are declarative: [Directive]
// binds a schema and [Where] binds fact predicates, never a filter
// inside the handler. The handler's second parameter picks its
// effect, [Emitter] to generate or [Stamper] to annotate, and
// [Builder.Build] lowers every rule set to the SPI roles in
// [go.dokimi.dev/eidos/core/plugin], which remain public for what
// the facade does not fit.
//
// A frontend or backend is declared the same way, in
// [go.dokimi.dev/eidos/core/frontend] and
// [go.dokimi.dev/eidos/core/backend]: identity and language facts
// as data, the role's functions declared once, and a Build that
// lowers to the same SPI roles.
//
// # Presentation
//
// A plugin whose emit names templates declares where they resolve.
// [Builder.Templates] and [Builder.Funcs] declare the plugin-level
// tree and helpers, which serve every target, and [Builder.For]
// layers one target's [Templates], [Funcs] and [Overrides] over
// them. A plugin that declares trees for named targets alone serves
// those targets alone, and a composition refuses a plan of any
// other target at Build. [Emitter.JoinName] joins a family word onto
// a generated identifier in the neutral convention, and the plan's
// backend spells the result in the target's case when the plan
// settles.
//
// # Dispatch
//
// Dispatch is indexed: a directive-gated rule visits its carriers,
// a fact-gated rule visits its stamped subjects, and only a bare
// rule visits the whole graph. Every invocation has its own read
// set, minted on first read, so a fact write's derivation names
// what its match read; sequence numbers follow canonical match
// order, so arbitration never depends on scheduling. The kernel
// skip directive excludes a subject from bare and fact-gated rules,
// for every plugin or one named plugin, and directive-gated rules
// run regardless.
//
// # Failure semantics
//
// A declaration defect panics at [Builder.Build], before any run
// exists. A handler's per-subject problem goes to the sink through
// its match and the phase continues; a returned error is fatal to
// the phase, wrapped with the plugin and rule. A stamp the fact
// store refuses reports under [RefusedStamp] at the subject's
// position.
//
// # Determinism
//
// Byte-identity is the contract: the same workspace over the same
// input produces the same bytes on every machine, warm or cold.
// Every ordering is defined: units order by plugin, cardinality,
// key, package and tag, and contributions by origin, gating
// instance and insertion. Output contains no clock reading and
// nothing from the environment.
//
// # Dependency position
//
// The root package imports core/plugin, core/diag,
// core/directive, core/emit, core/meta, core/node, core/position,
// core/rules, core/store, core/symbol and the Go stdlib. Nothing
// in the module imports the root package back.
package eidos
