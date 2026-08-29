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
// inference and every fact can answer "derived from what".
//
// # Authoring model
//
// This root package is the authoring surface. A plugin is a value:
// identity, outputs, priorities, and schemas are data built once;
// only handlers are functions. Handlers attach to kind-indexed
// triggers, gates are declarative — a directive or a stamped fact,
// never a filter inside the handler — and the effect, generate or
// annotate, is chosen by the handler's signature. Every rule set
// lowers to the SPI role interfaces, which remain public for what
// the facade does not fit.
//
// # Determinism
//
// Byte-identity is the contract: the same workspace over the same
// input produces the same bytes on every machine, warm or cold.
// Every ordering is defined, output carries no clocks or
// environment, sinks write atomically and only on change, and
// incremental runs are licensed by a conformance rung that proves
// warm and cold runs byte-equal — a cache that cannot prove
// equivalence is a determinism bug wearing a speedup.
package eidos
