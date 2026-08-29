# eidos — architecture

The architecture of eidos: a multi-module, public code-generation
framework. Frontends parse source into a language-neutral symbol
graph; plugins annotate it and generate output entities; backends
render those to target languages — deterministically, incrementally,
and across languages in one run.

These documents are the specification. Each covers one bounded
context; together they are closed — every component, contract, and
policy of the system appears in exactly one of them. Cross-references
are relative links. Settled decisions live inline in the context they
belong to, each with its reasoning and, where applicable, a recorded
revisit trigger; [21-decisions.md](21-decisions.md) is the index of
all of them.

## Ground rules

- eidos is a **library and nothing else**: no binary is shipped, ever.
  Consumers build binaries on the kernel's command kernels
  ([20-cli.md](20-cli.md)).
- **Public framework**: kernel API stability is a promise, docs are a
  product, conformance is the compatibility instrument.
- **Multi-module**: one repository, one kernel module
  (`go.dokimi.dev/eidos/core`) plus satellite modules
  (`go.dokimi.dev/eidos/lang-go`, `/lang-typescript`,
  `/lang-protobuf`, `/plugin-shape`, `/reference`, …) and the
  shared tree-sitter binding module (`/lang`), each tagged and
  released independently. Everything is implemented in Go; a
  satellite module per language is a packaging boundary, not a
  rewrite in that language.
- **Scale envelope: monorepo-grade.** 10k+ packages, sub-second warm
  regeneration, memory-resident graph with scoped loading.
  Incrementality is the core execution path, not an optimization.
- Languages: Go and TypeScript first-class, protobuf **read-only by
  design**. The vocabulary anticipates Rust, Python, Java, Kotlin,
  PHP, C#, Swift. C/C++ are out of scope, recorded, not half-done.
- The shape catalog is **language-neutral** and **spec-first**.
- One directive grammar, per-language carriers. No CUE anywhere;
  text/template for rendering; YAML + published JSON Schema for
  config.
- eidos maps **interfaces, never implementations**. Function bodies
  are a transpiler's problem and permanently out of scope.
- **Boundary strings, interior constants.** Strings exist where
  humans type — directives, YAML, CLI arguments. Inside the type
  system every registered name (language IDs, metadata keys,
  diagnostic codes, slots, tags, capability labels, policy keys,
  directive names) is a typed value or a generated constant, every
  registry generates its constants, and every boundary string
  resolves against its registry at workspace Build — a typo is a
  Build error naming candidates, never a silent miss at use.

**If you read one document, read
[00-one-declarations-journey.md](00-one-declarations-journey.md)** —
one declaration traced through the whole system, with a pointer into
the spec at every step. Keep the [GLOSSARY](GLOSSARY.md) beside the
rest.

## Where to start, by role

- **Building the kernel**: [02](02-symbol-model.md) →
  [03](03-projection.md) → [08](08-workspace-and-plans.md) →
  [09](09-incrementality.md).
- **Writing a language satellite**: [11](11-languages.md) →
  [03](03-projection.md) → [10](10-cross-language.md) →
  [13](13-testing-and-conformance.md).
- **Writing a plugin**: [06](06-plugins.md) →
  [06b](06b-authoring.md) → [07](07-rendering.md) →
  [05](05-directives.md) → [18](18-routing-and-layout.md).
- **Building a consumer binary**: [14](14-distribution-and-cli.md)
  → [20](20-cli.md) → [08](08-workspace-and-plans.md).

## Reading order

| Doc | Bounded context |
|---|---|
| [01-repos-and-kernel.md](01-repos-and-kernel.md) | Module topology, kernel package tree, import paths |
| [02-symbol-model.md](02-symbol-model.md) | The symbol vocabulary; node/emit as generated mirrors |
| [03-projection.md](03-projection.md) | The rules tiers: what every language answers |
| [04-metadata.md](04-metadata.md) | Typed metadata, authority, provenance, namespaces |
| [05-directives.md](05-directives.md) | Grammar, carriers, schemas |
| [06-plugins.md](06-plugins.md) | Roles, capabilities, plugin values, the declarative host |
| [06b-authoring.md](06b-authoring.md) | The authoring surface: triggers, effects, gates, the lowering guarantee |
| [07-rendering.md](07-rendering.md) | Templates, funcmaps, the builder API, backends |
| [08-workspace-and-plans.md](08-workspace-and-plans.md) | Workspaces, plans, exports, multi-workspace repos |
| [09-incrementality.md](09-incrementality.md) | The engine, persistence, the performance model |
| [10-cross-language.md](10-cross-language.md) | Canonical types, lowering, policy, refusal |
| [11-languages.md](11-languages.md) | Satellite anatomy, the language landscape, completeness |
| [12-shape-catalog.md](12-shape-catalog.md) | eidos-plugin-shape: spec-first classification |
| [13-testing-and-conformance.md](13-testing-and-conformance.md) | The rung ladder, harness skeletons, fixtures |
| [14-distribution-and-cli.md](14-distribution-and-cli.md) | Library-only delivery, command kernels, consumer binaries |
| [15-compatibility.md](15-compatibility.md) | Versioning, the canary ring, deprecation |
| [16-diagnostics.md](16-diagnostics.md) | Severities, failure semantics, suppression, codes |
| [17-output-and-determinism.md](17-output-and-determinism.md) | The sink contract, determinism laws, headers, manifest |
| [18-routing-and-layout.md](18-routing-and-layout.md) | Layouts, tags, overrides, collision semantics |
| [19-benchmarking.md](19-benchmarking.md) | The corpus generator, scenario matrix, budget gates |
| [20-cli.md](20-cli.md) | The command kernels: per-command contracts, flags, machine output |
| [21-decisions.md](21-decisions.md) | Decision log and revisit triggers |
