# Plugins

*Builds on: [04](04-metadata.md), [05](05-directives.md). Feeds:
[06b](06b-authoring.md) (the authoring surface over this SPI),
[07](07-rendering.md) (what plugins render),
[08](08-workspace-and-plans.md) (what plans compose),
[13](13-testing-and-conformance.md) (what conformance holds plugins
to).*

A plugin is behaviour under contract: a stable `Name()` plus one or
more role interfaces, which the conformance suite can test on their
own ([13-testing-and-conformance.md](13-testing-and-conformance.md)).

**Plugins compose at compile time**, as ordinary Go imports in the
consumer's binary. Typed metadata keys and a shared mutable graph
only work in one process. Serialize them across a boundary and the
type parameter carrying the safety becomes a string, while every
call ships the working set. A run's plugin set also has to be fixed
when the binary is built, because generated output is committed
source: two machines running the same binary over the same input
must produce the same bytes. Splitting the project across modules
changes none of this, because modules are packaging boundaries
rather than process boundaries.

## Roles

| Role | Phase | Reads | Writes |
|---|---|---|---|
| `Frontend` | Load | source | node symbols |
| `Annotator` | Annotate | the node graph, tracked | metadata only |
| `Generator` | Generate, per plan | the scoped node graph and its own plan's emit, tracked | emit and slots |
| `Backend` | Render, per plan | its own plan's emit | files, through the sink |
| `WorkspaceCheck` | Close | the records: manifests, exports, graph and facts | diagnostics only |

Every role is `Plugin` plus one method taking a context struct, and
every context carries the tracked reader and the diagnostic sink:

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

Failure works the same way in every role. A problem with one item
attaches to the diagnostic sink and the run continues. A returned
error is fatal to the phase.

Four structural rules bind the roles:

- Annotators never add or remove symbols. The node graph freezes
  after Annotate, and the store enforces that rather than a
  docblock.
- Generators are blind to languages. They read projections and emit
  neutral values.
- No state travels on the plugin struct between phases. The graph is
  the only memory. A field on the plugin skips the tracked read set,
  which produces output that is stale and looks current.
- `WorkspaceCheck`
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)) is the
  only place from which you can ask a cross-plan question such as
  "does every proto service method have a Go handler and a
  TypeScript client", and it produces diagnostics and nothing else.

## Capabilities

Capabilities are asserted through interfaces, and each has a name
the conformance suite can hold it to: `CapabilityProvider` for
priority plus provides and requires, where capability topology
orders execution inside priority buckets; `OptionsProvider` for
typed options schemas; `TemplateProvider` for template trees, funcs
and overrides; `FilenameProvider` for declared outputs;
`DirectiveProvider` for owned schemas; and `Versioned` for the
contribution to the cache key.

Capability labels follow the same rule as every other registered
name ([README](README.md)). The plugin that provides a capability
exports it as a constant, and requirers reference that constant. A
declarative manifest's `requires: [audit]` is the human spelling,
validated at Build against the declared providers.

**Priorities are per role.** A plugin declares a priority for each
role it implements, because one shared number cannot place a
dual-role plugin's annotator half and generator half independently,
and the two phases' bucket ladders have no reason to share integers.

## The concurrency contract

What a plugin may assume is stated here rather than discovered:

- Frontends run concurrently across patterns, so one frontend
  instance must tolerate concurrent `Load` calls. The store
  serializes graph writes per package.
- Annotators and generators run sequentially by default.
  Parallelism inside a bucket is a workspace opt-in, and a plugin
  that holds state across calls has to be safe under it. The
  conformance suite races exactly that.
- Plans always run concurrently with each other, which a plugin
  never observes, because plans are read-isolated by construction.
- A plugin does not leave goroutines running after its phase call
  returns. Whatever work it forks, it joins before returning.
  Otherwise the read tracking that feeds incrementality has already
  closed underneath it.

## Options

A plugin that takes configuration declares a typed options schema.
The workspace populates it at Build from the plugin's config
section, and a validation failure is a Build error naming the plugin
and the field.

Options are part of the plugin's fingerprint, so changing an option
invalidates what the plugin produced before
([09-incrementality.md](09-incrementality.md)). The schema also
appears in the published config JSON Schema, so a consumer's editor
completes plugin options the way it completes workspace ones
([14-distribution-and-cli.md](14-distribution-and-cli.md)).

## Plugin values: declarations as data

Most of what a plugin declares is data: templates, outputs, words,
rules bindings. So declarations are values, built once and frozen,
rather than methods answered per call.

The same idea scales up. **Plans are values too**
([08-workspace-and-plans.md](08-workspace-and-plans.md)), so a
satellite or a consumer may export a preset plan such as
`golang.ServerPlan(...)` as plain data the workspace instantiates.

Plans are not plugins. A plan owns an emit graph, a manifest slice,
a sweep scope and a position in the schedule, which are the things
the surrounding frame enforces between plugins. Make a plan a plugin
and the first question becomes "who isolates two plans", answered by
a workspace one level up that you have just reinvented. You compose
plugins the way you compose code, and plans the way you compose
config.

## The declarative host

The second authoring tier is a **declarative plugin**: a plugin
written as files, with no Go. It is a directory:

```
acme-audit/
  plugin.yaml
  templates/
    golang/audit.impl.tmpl
    typescript/audit.impl.tmpl
```

The manifest carries the complete field set:

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
manifest and presents it through the ordinary role interfaces:
Generator, TemplateProvider, DirectiveProvider. The loader is not
itself the plugin. The workspace sees N ordinary plugins, so
priorities, capability topology, options and conformance all apply
per declarative plugin rather than to the loader as a lump. Nothing
else in the kernel knows declarative plugins exist.

The tier's guarantees and its limits are both structural. Templates
are data, so compile-time composition still holds. Manifests hash
into the run fingerprint, so determinism still holds. The limits are
stated rather than discovered: no typed metadata, no projections,
and no logic beyond template conditionals. Needing any of those
means writing the plugin in Go. The manifest schema is public API,
versioned under the compatibility policy.

What this buys a public ecosystem: a declarative plugin is inert
files, which is safe to adopt from a stranger, while a typed plugin
is code you have to vet. A consumer whose binary compiles in the
loader gives its own users extension without any toolchain
([14-distribution-and-cli.md](14-distribution-and-cli.md)).

## The authoring surface

The role interfaces above are the SPI: what the workspace invokes,
what the declarative host adapts onto, and the floor everything
lowers to.

Authors write against a second layer, the kernel module's root
package, specified in [06b-authoring.md](06b-authoring.md):
declarations as values, kind-indexed triggers, effects chosen by the
handler's signature, declarative gates, and a guarantee that every
rule set compiles down to the SPI roles on this page. In one line:
subscribers get rules, edges get kits, everything is a value, and
everything lowers to the SPI.

## There is no Transformer role

Three separate reasons rule it out.

Rewriting user source breaks whole-file ownership, and eidos has no
determinism, sweep or drift story for a file it does not wholly own
([17-output-and-determinism.md](17-output-and-determinism.md)).

Transforming the node graph forks the one graph, after which
explain, invalidation and cross-plan checks all have to ask "which
variant" before they can answer anything
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

Rewriting another plugin's emit is the wrap verb, which the render
level refuses ([07-rendering.md](07-rendering.md)).

The graph is input truth, and a run never manufactures fictions into
its own inputs. Every use case people bring to a transformer has a
home that composes:

| "I want to transform…" | How eidos spells it |
|---|---|
| how a construct renders | Replace: a template override, or lowering policy |
| another plugin's output, by adding behaviour | wrap at the model level, emitting a wrapper that calls the original, or append into a body slot through `OnEmit` |
| source constructs into a normalized view | a projection ([03-projection.md](03-projection.md)) or stamped facts, which gives you the transformation as a view, with authority and provenance |
| nothing into something, meaning synthetic declarations | a frontend, since producing graph is its role |
| the user's own source files | consumer tooling at `manual` authority ([04-metadata.md](04-metadata.md)), outside the run |

## What plugins exchange

Facts, through metadata ([04-metadata.md](04-metadata.md)). Output,
through slots ([07-rendering.md](07-rendering.md)). Nothing else.

The alternatives fail on inspection. Direct plugin-to-plugin
interfaces turn the plugin graph into a compile-time dependency
graph, with nowhere to put a human override. A shared context struct
makes the framework a bottleneck for work that belongs in plugins,
and records no provenance per fact. An event bus, once you remove
the delivery semantics the workspace already provides by ordering
plugins, is a keyed store that is harder to use.
