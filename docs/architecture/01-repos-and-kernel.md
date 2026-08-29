# Module topology and the kernel

*Builds on: the [README](README.md) ground rules. Feeds: every
document — this is the map their machinery lives in.*

## Topology

One kernel, satellites around it — one repository, one Go module
per component, each tagged independently per Go's multi-module
convention (`eidos-core/v1.2.3`). The boundary is drawn by change
rate: declaration models and plugin contracts move slowly and their
breaks are expensive, while catalogs and language support churn as
their vocabularies settle. The kernel is exactly the slow-moving set.

The module inventory — every directory, its import path, and what it
holds — is the [repository README](../../README.md). This document owns
the boundary that inventory reflects.

The root module (`go.dokimi.dev/eidos`) anchors the import-path
prefix and pins the toolchain for CI; it holds no packages and is
not a workspace member. The kernel module's root package is named
`eidos`, so the authoring surface reads `eidos.NewPlugin(…)` at the
import site.

Go and TypeScript are first-class and protobuf read-only; the Java,
Kotlin, PHP, and Rust satellites hold the anticipated languages'
places in the same anatomy, and further languages
(`eidos-lang-python`, …) join as new modules
([11-languages.md](11-languages.md)). Bridge modules do not exist:
cross-language conversion is hub-and-spoke through the kernel
([10-cross-language.md](10-cross-language.md)), never pairwise.

`eidos-lang` is neither kernel nor satellite: it is the shared
tree-sitter binding layer and grammar set the tree-sitter
satellites parse through, registering no language of its own. It
depends only on the kernel; satellites that use it (TypeScript,
Java, Kotlin, PHP, Rust) depend on it beside the kernel, and
satellites with first-class pure-Go parsers (Go, protobuf) do not
depend on it at all. The satellite-never-imports-satellite law is
untouched — `eidos-lang` sits below the satellites, not beside
them. It is also where the tree-sitter cgo dependency is
concentrated: the kernel stays zero-dependency, and a binding or
grammar upgrade lands in one module.

The kernel is the only module whose tags gate anyone else.
Satellites release on their own cadence against a declared kernel
version range ([15-compatibility.md](15-compatibility.md)).

## The kernel package tree

```
eidos-core/
  symbol/        the declaration vocabulary: one Kind enum, the walk
                 interfaces (Symbol, Membered, Typed, Documented),
                 schema/ — the hand-written source of truth for kinds
  node/  emit/   the two concrete models, GENERATED from symbol/schema:
                 kind structs, Walk, RewireOwners, JSON, mirror guards.
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
  conformance/   every rung of the test ladder, the toolchain-adapter
                 skeleton, the completeness rung, the warm≡cold rung,
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

Nothing in the kernel names anything to its left; a satellite never
imports a satellite (cross-language needs go through the kernel's
hub, [10-cross-language.md](10-cross-language.md)).

Constraints the tree encodes:

- The kernel knows **no language**. No satellite name, no language
  string, no per-language table appears anywhere in it.
- The kernel carries **zero third-party dependencies**, as a hard
  property — a public framework's kernel is on every consumer's
  supply-chain audit.
- `symbol/schema` is the only hand-written definition of the
  declaration kinds. `node/` and `emit/` are generated from it and
  committed ([02-symbol-model.md](02-symbol-model.md)). The generator
  is `internal/gen`, a plain `go/ast`-plus-template tool with zero
  eidos dependencies — a kernel that used a language satellite to
  build itself would invert the layering and could not bootstrap.

## What is deliberately not in the kernel

- Any binary (`cmd/`). eidos ships no executable
  ([14-distribution-and-cli.md](14-distribution-and-cli.md)).
- Any language satellite content, including "just this once" helper
  tables. The degradation ladder and metadata namespaces exist so the
  kernel never needs a language exception.
- A daemon or client-server protocol
  ([09-incrementality.md](09-incrementality.md), decision D9).
