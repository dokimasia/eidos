// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package eidos is the kernel of a code-generation framework:
// frontends parse source into a language-neutral symbol graph,
// plugins annotate it and generate output entities, and backends
// render those to target languages — deterministically,
// incrementally, and across languages in one run.
//
// The kernel is a library. It ships no binary; consumers compose a
// workspace in their own main and hand it to the command kernels.
// It knows no language: language support lives in satellite modules
// that implement the projection tiers, and no satellite name,
// language string, or per-language table appears in the kernel. It
// carries zero third-party dependencies.
//
// # Composition model
//
// One workspace holds the read side — frontends, annotators, one
// frozen symbol graph — and N plans hold the write side, each with
// its own generators, layout, exactly one backend, and one sink.
// Plans are values, run in parallel, and exchange nothing except
// declared, versioned exports. Plugins compose at compile time and
// communicate only through typed metadata on the graph: every
// write carries authority (plugin < directive < manual) and
// provenance, so a human override at the declaration beats any
// inference and every fact can return "derived from what".
//
// # Authoring model
//
// This root package is the authoring surface. A plugin is a value:
// [NewPlugin] starts the declaration, identity, outputs, priorities
// and schemas are data built once, and only the handlers and the
// key registration declared through [Builder.Keys] are functions. Handlers attach to kind-indexed triggers — one
// generated constructor per subject kind, [OnInterface] and its
// siblings — plus [OnGraph] and [OnEmit]. Gates are declarative:
// [Directive] binds a schema and [Where] binds fact predicates,
// never a filter inside the handler. The handler's second parameter
// picks its effect, [Emitter] to generate or [Stamper] to annotate,
// and [Builder.Build] lowers every rule set to the SPI roles in
// [go.dokimi.dev/eidos/core/plugin], which remain public for what
// the facade does not fit.
//
// A backend is declared the same way: [NewBackend] takes the
// identity, target and comment syntax, the language pieces are
// data, and [BackendBuilder.Build] lowers them to the render pass,
// returning the Backend and Renderer roles both.
//
// # Dispatch
//
// Dispatch is indexed: a directive-gated rule visits its carriers,
// a fact-gated rule visits its stamped subjects, and only a bare
// rule visits the whole graph. Every invocation carries its own
// read set, minted on first read, so a fact write's derivation
// names what its match read; sequence numbers follow canonical
// match order, so arbitration never depends on scheduling. The
// kernel skip directive excludes a subject from bare and fact-gated
// rules, for every plugin or one named plugin, and directive-gated
// rules run regardless.
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
// Every ordering is defined — units by plugin, cardinality, key and
// tag; contributions by origin, gating instance and insertion — and
// output carries no clocks or environment.
//
// # Dependency position
//
// The root package imports core/plugin, core/diag, core/directive,
// core/emit, core/meta, core/node, core/position, core/render,
// core/store, core/symbol and the Go stdlib. Nothing in the module
// imports the root package back.
package eidos
