# eidos architecture

These documents specify eidos, a code-generation framework built as
several Go modules. Frontends parse source into a symbol graph that
belongs to no language. Plugins annotate that graph and produce
output entities. Backends render those into target languages. One
run regenerates only what changed, produces the same bytes every
time, and can target several languages at once.

Each document covers one area of the system, and between them they
cover all of it: every component, contract and policy appears in
exactly one document. Cross-references are relative links. Each
document argues its own decisions where it defines them, and
[21-decisions.md](21-decisions.md) indexes every decision in one
table.

**Read [00-one-declarations-journey.md](00-one-declarations-journey.md)
first.** It follows a single declaration through the whole system
and links into the specification at every step. Keep the
[glossary](GLOSSARY.md) open beside the rest.

## Ground rules

- **eidos is a library.** It ships no binary. Consumers build their
  own binary on the kernel's command kernels
  ([20-cli.md](20-cli.md)).
- **Outsiders use this.** So the kernel's API stability is checked
  by machinery rather than promised, the documentation ships as
  part of the release, and the conformance suite is what proves a
  compatibility claim.
- **One repository, one module per component.** The kernel is
  `go.dokimi.dev/eidos/core`. The satellites are
  `go.dokimi.dev/eidos/lang-go`, `/lang-typescript`,
  `/lang-protobuf`, `/plugin-shape` and `/reference`, and the
  tree-sitter satellites share `/lang`. Each module is tagged and
  released on its own. Everything is written in Go: a satellite is
  a packaging boundary, not a rewrite in that language.
- **The system has to work on a monorepo.** Ten thousand packages
  or more, warm regeneration under a second, the graph resident in
  memory with scoped loading. There is no separate full-rebuild
  path: regenerating only what changed is how every run works.
- **Languages.** Go and TypeScript are first-class. protobuf is
  read-only by design. The vocabulary already holds Rust, Python,
  Java, Kotlin, PHP, C# and Swift. C and C++ are out of scope,
  because there is no declaration model worth projecting without a
  real compiler frontend.
- **The shape catalog reads no language's syntax**, and every entry
  in it starts as a written spec.
- **One directive grammar**, with a carrier per language. No CUE.
  text/template renders. Config is YAML with a published JSON
  Schema.
- **eidos maps interfaces, never implementations.** It generates
  declarations and delegating scaffolding. Function bodies stay out
  of scope permanently.
- **Strings only where humans type them**, which means directives,
  YAML and command-line arguments. Inside the type system every
  registered name is a typed value or a generated constant: language
  identities, metadata keys, diagnostic codes, slots, tags,
  capability labels, policy keys and directive names. Every registry
  generates its own constants, and every human-typed string resolves
  against its registry when the workspace builds. A typo is a Build
  error that names the candidates, not a silent miss at the point of
  use.

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

| Doc | What it covers |
|---|---|
| [01-repos-and-kernel.md](01-repos-and-kernel.md) | Module topology, kernel package tree, import paths |
| [02-symbol-model.md](02-symbol-model.md) | The symbol vocabulary; node and emit as generated mirrors |
| [03-projection.md](03-projection.md) | The rules tiers: what every language answers |
| [04-metadata.md](04-metadata.md) | Typed metadata, authority, provenance, namespaces |
| [05-directives.md](05-directives.md) | Grammar, carriers, schemas |
| [06-plugins.md](06-plugins.md) | Roles, capabilities, plugin values, the declarative host |
| [06b-authoring.md](06b-authoring.md) | The authoring surface: triggers, effects, gates, the lowering guarantee |
| [07-rendering.md](07-rendering.md) | Templates, funcmaps, the builder API, backends |
| [08-workspace-and-plans.md](08-workspace-and-plans.md) | Workspaces, plans, exports, multi-workspace repositories |
| [09-incrementality.md](09-incrementality.md) | The engine, persistence, the performance model |
| [10-cross-language.md](10-cross-language.md) | Canonical types, lowering, policy, refusal |
| [11-languages.md](11-languages.md) | Satellite anatomy, the language landscape, completeness |
| [12-shape-catalog.md](12-shape-catalog.md) | eidos-plugin-shape: classification, written as specs first |
| [13-testing-and-conformance.md](13-testing-and-conformance.md) | The rung ladder, harness skeletons, fixtures |
| [14-distribution-and-cli.md](14-distribution-and-cli.md) | Library-only delivery, command kernels, consumer binaries |
| [15-compatibility.md](15-compatibility.md) | Versioning, the canary ring, deprecation |
| [16-diagnostics.md](16-diagnostics.md) | Severities, failure semantics, suppression, codes |
| [17-output-and-determinism.md](17-output-and-determinism.md) | The sink contract, determinism rules, headers, manifest |
| [18-routing-and-layout.md](18-routing-and-layout.md) | Layouts, tags, overrides, collision semantics |
| [19-benchmarking.md](19-benchmarking.md) | The corpus generator, scenario matrix, budget gates |
| [20-cli.md](20-cli.md) | The command kernels: per-command contracts, flags, machine output |
| [21-decisions.md](21-decisions.md) | The decision table |
