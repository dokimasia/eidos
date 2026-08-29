# Plugins

*Builds on: [04](04-metadata.md), [05](05-directives.md). Feeds:
[06b](06b-authoring.md) (the authoring surface over this SPI),
[07](07-rendering.md) (what plugins render),
[08](08-workspace-and-plans.md) (what plans compose),
[13](13-testing-and-conformance.md) (what conformance holds
plugins to).*

A plugin is behavior under contract: a stable `Name()` plus one or
more role interfaces, conformance-testable in isolation
([13-testing-and-conformance.md](13-testing-and-conformance.md)).

Composition is **compile-time**: plugins are ordinary Go imports in
the consumer's binary. Typed metadata keys and a shared mutable
graph exist only in-process — serialized across any boundary, the
type parameter that carries the safety degrades to a string, and
every call would ship the working set. And a run's plugin set must
be fixed at build time: generated output is committed source, so two
machines running the same binary over the same input must produce
the same bytes. The multi-module split changes nothing here —
modules are packaging boundaries, not process boundaries.

## Roles

| Role | Phase | Reads | Writes |
|---|---|---|---|
| `Frontend` | Load | source | node symbols |
| `Annotator` | Annotate | node graph (tracked) | metadata only |
| `Generator` | per-plan Generate | scoped node graph + own plan's emit (tracked) | emit + slots |
| `Backend` | per-plan Render | own plan's emit | files via sink |
| `WorkspaceCheck` | Close | the records: manifests, exports, graph + facts | diagnostics only |

Every role is `Plugin` plus one method taking a context struct, and
every context carries the tracked reader and the diag sink:

```go
type Frontend interface {
    Plugin
    Load(ctx *FrontendContext) error       // ctx: store, pattern, parser, cache, fingerprint
}
type Annotator interface {
    Plugin
    Annotate(ctx *AnnotatorContext) error  // ctx: tracked reader, diag
}
type Generator interface {
    Plugin
    Generate(ctx *GeneratorContext) error  // ctx: scoped tracked reader, plan emit, diag
}
type Backend interface {
    Plugin
    Language() rules.Target
    Render(ctx *BackendContext) error      // ctx: plan emit, sink, diag
}
type WorkspaceCheck interface {
    Plugin
    Check(ctx *CheckContext) error         // ctx: manifests, exports, graph, facts, diag
}
```

The failure split is uniform across roles: per-item issues attach to
the diag sink and the run continues; a returned error is fatal to
the phase.

Four structural rules bind the roles:

- Annotators never add or remove symbols; the node graph freezes
  after Annotate, enforced by the store, not by docblock.
- Generators are language-blind: they read projections and emit
  neutral values.
- State never travels on the plugin struct between phases — the
  graph is the only memory. A field on the plugin bypasses the
  tracked read set and produces output that is stale but looks
  current.
- `WorkspaceCheck`
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)) is the
  only vantage point from which cross-plan claims ("every proto
  service method has a Go handler and a TS client") are askable, and
  it produces diagnostics and nothing else.

## Capabilities

Interface-asserted, each with a name a conformance suite can hold it
to: `CapabilityProvider` (priority + provides/requires — capability
topology orders execution within priority buckets),
`OptionsProvider` (typed options schemas), `TemplateProvider`
(template trees + funcs + overrides), `FilenameProvider` (declared
outputs), `DirectiveProvider` (owned schemas), and `Versioned` (the
cache-key contribution).

Capability labels follow the boundary-strings law
([README](README.md)): the plugin that *provides* a capability
exports it as a constant, and requirers reference the constant — a
declarative manifest's `requires: [audit]` is boundary spelling,
validated at Build against the declared providers.

**Priorities are per-role.** A plugin declares its priority for each
role it implements: one shared number cannot place a dual-role
plugin's annotator half and generator half independently, and the
two phases' bucket ladders have no reason to share integers.

## The concurrency contract

What a plugin may assume is stated, not discovered:

- Frontends run concurrently across patterns; one frontend instance
  must tolerate concurrent `Load` calls. The store serializes graph
  writes per package.
- Annotators and generators run sequentially by default;
  within-bucket parallelism is workspace opt-in, and a plugin that
  holds state across calls must be safe under it (the conformance
  suite races exactly this).
- Plans run concurrently with each other always — which a plugin
  never observes, because plans are read-isolated by construction.
- A plugin does not spawn goroutines that outlive its phase call;
  work it forks, it joins before returning, or the read-tracking
  that feeds incrementality has already closed under it.

## Options

A plugin that takes configuration declares a typed options schema;
the workspace populates it at Build from the plugin's config section
and validation failures are Build errors naming the plugin and the
field. Options are part of the plugin's fingerprint: a changed
option invalidates what the plugin previously produced
([09-incrementality.md](09-incrementality.md)). The schema surfaces
in the published config JSON Schema, so a consumer's editor
completes plugin options the same as workspace ones
([14-distribution-and-cli.md](14-distribution-and-cli.md)).

## Plugin values: declarations as data

Most of what a plugin declares is data — templates, outputs, words,
rules bindings — so declarations are values built once and frozen,
not methods answered per call. The same principle scales up: **plans
are values too**
([08-workspace-and-plans.md](08-workspace-and-plans.md)) — a
satellite or consumer may export a preset plan
(`golang.ServerPlan(...)`) as plain data the workspace instantiates.

Plans are not plugins. A plan owns an emit graph, a manifest slice,
a sweep scope, a schedule position — the things enforced *between*
plugins by the frame they run in. Make the plan a plugin and the
first question is "who isolates two plans?", answered by a workspace
one level up, reinvented. Plugins compose like code; plans compose
like config.

## The declarative host

The second authoring tier: **declarative plugins** — plugin-as-files,
no Go. A declarative plugin is a directory:

```
acme-audit/
  plugin.yaml
  templates/
    golang/audit.impl.tmpl
    typescript/audit.impl.tmpl
```

with a manifest carrying the complete field set:

```yaml
# acme-audit/plugin.yaml
name: acme-audit          # plugin identity; reserved-name rules apply
version: "0.3.0"          # folds into the run fingerprint
directives:               # schemas, same machinery as typed plugins
  - name: audit
    params:
      - {key: level, type: string, required: true}
      - {key: sink,  type: reference, resolve: callable-in-scope}
outputs:                  # tagged outputs with suffix derivations
  - {tag: "", suffix: "_audit"}
provides: [audit]         # capability topology, per plugin
requires: [handlers]
options:                  # typed options schema, config-populated
  - {key: redact, type: bool, default: true}
words: {marker: "Audit"}  # per-language vocabulary, under templates/<lang>
```

A kernel-provided **loader** instantiates one adapted plugin per
manifest, presenting it through the ordinary role interfaces
(Generator, TemplateProvider, DirectiveProvider). The loader is not
itself the plugin: the workspace sees N ordinary plugins, so
priorities, capability topology, options, and conformance all apply
per declarative plugin — never to the loader as a lump. Nothing else
in the kernel knows declarative plugins exist.

The tier's guarantees and limits are both structural. Templates are
data, so compile-time composition is intact; manifests hash into the
run fingerprint, so determinism is intact. And the limits are
stated, not discovered: no typed metadata, no projections, no logic
beyond template conditionals — needing any of those graduates the
plugin to Go. The manifest schema is public API, versioned under the
compatibility policy.

What this buys a public ecosystem: declarative plugins are inert
files, safe to adopt from strangers; typed plugins are code you vet.
A consumer's binary that compiles in the loader gives *its* users
extension without any toolchain
([14-distribution-and-cli.md](14-distribution-and-cli.md)).

## The authoring surface

The role interfaces above are the SPI: what the workspace invokes,
what the declarative host adapts onto, and the floor everything
lowers to. Authors write against a second layer — the kernel
module's root package — specified in
[06b-authoring.md](06b-authoring.md): declarations as values,
kind-indexed triggers, effects chosen by handler signature,
declarative gates, and the lowering guarantee that every rule set
compiles down to the SPI roles on this page. The frame in one
line: **subscribers get rules; edges get kits; everything is a
value; everything lowers to the SPI.**

## There is no Transformer role

Deliberately, three times over. Rewriting user source violates
whole-file ownership — no determinism, sweep, or drift story exists
for files eidos does not wholly own
([17-output-and-determinism.md](17-output-and-determinism.md));
transforming the node graph forks the one graph, and explain,
invalidation, and cross-plan checks would all have to answer "which
variant?" first
([08-workspace-and-plans.md](08-workspace-and-plans.md));
rewriting other plugins' emit is the wrap verb, refused at render
level ([07-rendering.md](07-rendering.md)). The graph is input
truth; a run never manufactures fictions into its own inputs.

Every transformer use case has a composing home instead:

| "I want to transform…" | The eidos spelling |
|---|---|
| how a construct renders | Replace — a template override, or lowering policy |
| another plugin's output, by adding behavior | wrap at the model level (emit a wrapper calling the original), or body-slot append via `OnEmit` |
| source constructs into a normalized consumer view | a projection ([03-projection.md](03-projection.md)) or stamped facts — transformation-as-view, with authority and provenance |
| nothing into something (synthetic declarations) | a frontend: graph production is its role |
| the user's own source files | consumer tooling at `manual` authority ([04-metadata.md](04-metadata.md)), outside the run |

## What plugins exchange

Facts through metadata ([04-metadata.md](04-metadata.md)). Output
through slots ([07-rendering.md](07-rendering.md)). Nothing else —
the alternatives fail on inspection: direct plugin-to-plugin
interfaces make the plugin graph a compile-time dependency graph
with nowhere to put a human override; a shared context struct makes
the framework a bottleneck for work that belongs in plugins, with no
per-fact provenance; an event bus, once its delivery semantics are
removed (the workspace already orders plugins), is a keyed store
with worse ergonomics.
