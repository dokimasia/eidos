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

Language support is uneven by design, because a satellite arrives
one side at a time. Go reads and writes, through a frontend, its
rules, a backend and an annotator. TypeScript, Java and Rust have
backends only, so a Go workspace renders into them before they can
be read from. protobuf is read-only by design, with a frontend and
its rules. The Kotlin and PHP modules contain a package comment
alone, and further languages such as `eidos-lang-python` join as
new modules
([11-languages.md](11-languages.md)). There are no bridge modules:
cross-language conversion goes through the kernel's hub, never
pairwise ([10-cross-language.md](10-cross-language.md)).

`eidos-lang` is neither the kernel nor a satellite. It holds the
tree-sitter bindings and the pinned grammars that the tree-sitter
satellites parse through, and it registers no language of its own.
It depends only on the kernel. Every satellite depends on it for
the pure helper packages it shares, case conversion first among
them; the tree-sitter satellites parse through it as well, while
Go and protobuf have fully supported pure-Go parsers and never
import the grammar packages. This does not weaken the rule that a
satellite never imports a satellite, because `eidos-lang` sits
below the satellites rather than beside them. It is also where the
tree-sitter cgo dependency stays, so the kernel keeps its zero
dependencies and a binding or grammar upgrade touches one module.

`eidos-cli` contains the seven command kernels, the config file and its
discovery, and `acceptancetest` ([20-cli.md](20-cli.md)). It depends on
the kernel and on one YAML library. No satellite or plugin module
depends on it, so the kernel keeps its zero dependencies and a
satellite never builds the YAML library.

The kernel is the only module whose tags gate anyone else.
Satellites release on their own schedule against a declared kernel
version range ([15-compatibility.md](15-compatibility.md)).

## The kernel package tree

Read and write are separate subtrees. Everything a run needs to
turn source into the graph sits under `frontend/`, everything that
turns a settled graph into bytes sits under `backend/`, and the
vocabularies both sides share sit beside them at the root.

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
  directive/     one grammar, schema machinery, the canonical parsed form
  diag/          positioned diagnostics with stable codes
  position/      the leaf
  plugin/        roles + capabilities, per-role priorities, plugin
                 values, the declarative host
                 plugintest/ — the kit holding a plugin to its roles
  authored/      the sample and witness annotators, which stamp an
                 author's values onto the kernel's own keys
  rules/         the projection vocabulary and the rules seam: the
                 questions every generator asks of a declaration, and
                 the contract each language returns them through
                 rulestest/ — the checks a language's rules run under
  store/         the graphs, per-plugin read tracking, scope predicates,
                 freeze enforcement
  frontend/      the read side's authoring kit: a language's claim,
                 partition, parse, resolve and classifiers, lowered to
                 the frontend role
    load/        the read pipeline: selection, partition, parse, splice,
                 identity assignment, Link, seal, and the unit keys
    frontendtest/ the frontend conformance suite and its scripted language
  backend/       the write side's authoring kit: what a language varies
                 in, lowered to the backend and renderer roles
    render/      the render pass: templates, import sets, template lint,
                 the fact-coverage guard
    backendtest/ the backend conformance suite, its fixtures and the
                 settle benchmark
  toolchain/     the skeleton of every language's toolchain harness: the
                 generated-output fixture, the assertion set, and the
                 adapter a satellite implements over its own compiler
  output/        the output contract: rendered files to sink, headers,
                 provenance trailers, manifests
  workspace/     workspaces, plans, plan exports, cross-plan checks,
                 the audit mode
  internal/
    gen/         the generators: model/ builds node and emit from
                 symbol/schema, facade/ builds the SDK re-exports.
                 Plain go/ast + text/template, zero eidos dependencies
                 (see 02); never imported, only run
    genfile/     formats, writes and guards generated Go sources
    gosource/    loads Go packages from source, without a build cache
    coretest/    the assertions and fixtures the kernel's own tests share
```

Planned kernel packages, and the milestone that builds each:

- `engine/` — incrementality: fingerprints, red-green invalidation,
  the sealed-graph persistence format. Milestone 0007.

Two names the map used to carry live elsewhere. Conformance is its
own module, `eidos-conformance`, because it drives the satellites
and nothing may depend on it. Case conversion is `eidos-lang`'s
`naming`, because only satellites ask for it and the kernel takes
nothing its satellites alone consume.

The SDK is a generated re-export facade over the kernel's
plugin-facing packages, laid flat under one namespace whatever
shape the kernel takes: `sdk/render` re-exports
`core/backend/render`, and regrouping the kernel moves no facade
import path ([RFC-0012](../rfc/0012-the-sdk-contract-module.md)).

Dependencies point one way and only one way:

```
consumers (dokimi, org binaries) ──► satellites (eidos-lang-go, …) ──► sdk ──► kernel
consumers (dokimi, org binaries) ──► eidos-cli (command kernels) ───────────► kernel
                                     satellites ──► eidos-lang (helpers) ──► kernel
                                     eidos-plugin-shape ─────────────► sdk ──► kernel
                                     tree-sitter satellites ──► eidos-lang ──► kernel
```

Nothing in the kernel names anything to its left, and no satellite
imports another satellite. Cross-language needs go through the
kernel's hub ([10-cross-language.md](10-cross-language.md)).

## The read/write seam

The read side never renders and the write side never loads. In
imports: nothing under `frontend/` reaches `emit` or any `backend/`
package, and nothing under `backend/` reaches `node`, `store` or any
`frontend/` package. The write side receives its subjects through
the render context, not through the graph the read side sealed.

This is a lint, not a convention. The `frontend-seam` and
`backend-seam` depguard rules in
[`.golangci.yml`](../../.golangci.yml) deny each direction by import
prefix with the reason attached, so a crossing fails CI at the
import line rather than in review.

## Where each invariant is pinned

| Package | Governing RFC | Invariant | The check that gates it |
| --- | --- | --- | --- |
| `symbol/schema`, `node/`, `emit/` | [0001](../rfc/0001-symbol-model-contract.md), [0002](../rfc/0002-model-generator.md) | The schema is the only hand-written definition; adding a field is a schema edit plus a regeneration | The mirror guard: a committed model that differs from regeneration fails the kernel's tests |
| `store/` | [0003](../rfc/0003-diagnostics-and-store.md) | Every plugin read goes through a tracked reader, so a fact's derivation names what its match read | The store's own suite over the tracked handles, and freeze enforcement after the seal |
| root package (authoring, dispatch) | [0006](../rfc/0006-authoring-surface-and-dispatch.md) | A declaration defect panics at Build, before any run exists; dispatch is indexed, never a filter inside a handler | The root package's builder and dispatch suites, and `plugin/plugintest`, which holds a facade-built plugin and a hand-rolled one to one set of checks |
| `frontend/`, `frontend/load` | [0013](../rfc/0013-the-frontend-kit.md), [0007](../rfc/0007-build-steps.md) | Reads are hermetic through the fold: every unit key folds the unit's and the partition's recorded reads, the frontend's version and its options | `frontend/frontendtest` (same fixture, identical graph twice, keys observed by a probe cache) plus `BenchmarkLoad` |
| `backend/`, `backend/render` | [0009](../rfc/0009-render-pass-and-backend-kit.md), [0011](../rfc/0011-neutral-emit-and-the-target-lowering-seams.md) | A kit backend and a hand-rolled pass over one language return the same bytes; a backend declaring a settle seam refuses an unsettled store | `backend/backendtest`, including the every-kind coverage check and the settle benchmark |
| `output/` | [0010](../rfc/0010-output-contract.md) | Output carries no clocks or environment; the same workspace over the same input writes the same bytes | The output suite over headers, trailers and manifests |
| `eidos-sdk` | [0012](../rfc/0012-the-sdk-contract-module.md) | Plugin modules speak the facade alone; the kernel arrives transitively and stays the single home of every definition | The `plugin-modules` depguard rule, and the facade's own mirror guard |

## What is deliberately not in the kernel

- Any binary. eidos ships no executable
  ([14-distribution-and-cli.md](14-distribution-and-cli.md)).
- The command line. The command kernels read a YAML config file, so
  they are the module `eidos-cli`, beside the kernel.
- Any language satellite content, including a helper table added
  "just this once". The degradation scale and the metadata
  namespaces exist so the kernel never needs a language exception.
- A daemon or a client-server protocol
  ([09-incrementality.md](09-incrementality.md), decision D9).

The tree encodes three constraints:

- **The kernel knows no language.** No satellite name, no language
  string and no per-language table appears anywhere in it.
- **The kernel's runtime takes no third-party dependencies.** Every
  package a run executes imports the standard library alone, because
  a public framework's kernel appears in every consumer's
  supply-chain audit. The conformance kits `plugin/plugintest`,
  `frontend/frontendtest`, `backend/backendtest` and
  `rules/rulestest`, and `internal/coretest`, import
  `go.dokimi.dev/assert`, which requires `github.com/google/go-cmp`.
  A binary built from the runtime packages links neither module, and
  a module whose tests run the kits compiles both.
- **`symbol/schema` is the only hand-written definition of the
  declaration kinds.** `node/` and `emit/` are generated from it and
  committed ([02-symbol-model.md](02-symbol-model.md)). The
  generator is `internal/gen`, a plain `go/ast` and template tool
  with no eidos dependencies. A kernel that used a language
  satellite to build itself would invert the layering and could
  never bootstrap.
