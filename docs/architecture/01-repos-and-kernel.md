# Module topology and the kernel

*Builds on: the [README](README.md) ground rules. Feeds: every
document. This is the map their machinery lives in.*

## Topology

One kernel with satellites around it, as one repository holding one
Go module per component. Each module is tagged on its own, following
Go's multi-module convention: `eidos-core/v1.2.3`.

Change rate draws the boundary. Declaration models and plugin
contracts move slowly, and breaking them is expensive. Catalogs and
language support change often while their vocabularies settle. The
kernel is exactly the slow-moving set.

The [repository README](../../README.md) lists every module, its
import path and what it holds. This document explains the boundary
that list reflects.

The root module `go.dokimi.dev/eidos` owns the import-path prefix
and pins the toolchain for CI. It holds no packages and is not a
workspace member. The kernel module's root package is called
`eidos`, so plugin authors write `eidos.NewPlugin(…)`.

Go and TypeScript are fully supported, and protobuf is read-only. The
Java, Kotlin, PHP and Rust satellites hold the anticipated
languages' places in the same anatomy, and further languages such as
`eidos-lang-python` join as new modules
([11-languages.md](11-languages.md)). There are no bridge modules:
cross-language conversion goes through the kernel's hub, never
pairwise ([10-cross-language.md](10-cross-language.md)).

`eidos-lang` is neither the kernel nor a satellite. It holds the
tree-sitter bindings and the pinned grammars that the tree-sitter
satellites parse through, and it registers no language of its own.
It depends only on the kernel. TypeScript, Java, Kotlin, PHP and
Rust depend on it alongside the kernel; Go and protobuf have
fully supported pure-Go parsers and do not depend on it at all. This
does not weaken the rule that a satellite never imports a satellite,
because `eidos-lang` sits below the satellites rather than beside
them. It is also where the tree-sitter cgo dependency stays, so the
kernel keeps its zero dependencies and a binding or grammar upgrade
touches one module.

The kernel is the only module whose tags gate anyone else.
Satellites release on their own schedule against a declared kernel
version range ([15-compatibility.md](15-compatibility.md)).

## The kernel package tree

```
eidos-core/
  symbol/        the declaration vocabulary: one Kind enum, the walk
                 interfaces (Symbol, Membered, Typed, Documented),
                 schema/ — the hand-written source of truth for kinds
  node/  emit/   the two concrete models, GENERATED from symbol/schema:
                 kind structs, Walk, JSON, mirror guards.
                 Hand-written per side: node's resolver, queries, and
                 freeze; emit's slots, Target, Ref variants, Expr/Stmt,
                 samples, and the builder
  meta/          typed metadata: bags, authority, provenance, declared
                 keys, namespace ownership, completeness contracts
  rules/         the projection vocabulary (Tier 1/2), typed language
                 identity (rules.Source / rules.Target), the canonical
                 type system, the Lowering seam
  directive/     one grammar, schema machinery, the canonical parsed form
  diag/          positioned diagnostics with stable codes
  position/ naming/ opt/
                 the leaves
  plugin/        roles + capabilities, per-role priorities, plugin
                 values, the declarative host
  store/         the graphs, per-plugin read tracking, scope predicates,
                 freeze enforcement
  workspace/     workspaces, plans, plan exports, cross-plan checks,
                 the audit mode
  engine/        incrementality: fingerprints, red-green invalidation,
                 the sealed-graph persistence format
  sink/ writer/ manifest/
                 output: sinks, text emission + import sets, manifests
  conformance/   every conformance check, the toolchain-adapter
                 skeleton, the completeness check, the warm≡cold check,
                 the benchmark harness and corpus generator
  cli/           command kernels: run, plan, explain, prune, doctor,
                 version, watch; config loading; flag conventions
  internal/gen/  the model generator: plain go/ast + text/template,
                 zero eidos dependencies (see 02); never imported,
                 only run
```

Dependencies point one way and only one way:

```
consumers (dokimi, org binaries) ──► satellites (eidos-lang-go, …) ──► kernel
                                     eidos-plugin-shape ─────────────► kernel
                                     tree-sitter satellites ──► eidos-lang ──► kernel
```

Nothing in the kernel names anything to its left, and no satellite
imports another satellite. Cross-language needs go through the
kernel's hub ([10-cross-language.md](10-cross-language.md)).

The tree encodes three constraints:

- **The kernel knows no language.** No satellite name, no language
  string and no per-language table appears anywhere in it.
- **The kernel takes no third-party dependencies.** This is a hard
  property, because a public framework's kernel appears in every
  consumer's supply-chain audit.
- **`symbol/schema` is the only hand-written definition of the
  declaration kinds.** `node/` and `emit/` are generated from it and
  committed ([02-symbol-model.md](02-symbol-model.md)). The
  generator is `internal/gen`, a plain `go/ast` and template tool
  with no eidos dependencies. A kernel that used a language
  satellite to build itself would invert the layering and could
  never bootstrap.

## What is deliberately not in the kernel

- Any binary. eidos ships no executable
  ([14-distribution-and-cli.md](14-distribution-and-cli.md)).
- Any language satellite content, including a helper table added
  "just this once". The degradation scale and the metadata
  namespaces exist so the kernel never needs a language exception.
- A daemon or a client-server protocol
  ([09-incrementality.md](09-incrementality.md), decision D9).
