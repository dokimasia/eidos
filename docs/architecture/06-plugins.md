<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

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
| `Generator` | Generate, per plan | the scoped node graph, its own plan's emit, and the exports of the plans its plan depends on, tracked | emit and slots |
| `Backend` | Render, per plan | its own plan's emit | files, through the sink |
| `WorkspaceCheck` | Close | the records of the plans it names: manifest entries and exports, with the graph and facts | diagnostics only |

Every role is `Plugin` plus one method taking a context struct, and
every context carries the tracked reader and the diagnostic sink:

```go
type Frontend interface {
    Versioned                              // the version that every unit key includes
    Name() ID
    Lang() symbol.Lang
    Syntax() CommentSyntax
    Overloads() bool
    Selection() []string                   // the file claim
    Partition(ctx context.Context, files []SourceRef, r FileReader) ([][]SourceRef, error)
    Parse(ctx context.Context, u *SourceUnit) error
    Resolve(scope ImportScope, spelling string) Candidates
}
type Annotator interface {
    Plugin
    Annotate(ctx *AnnotatorContext) error  // ctx: tracked reader, diag
}
type Generator interface {
    Plugin
    Generate(ctx *GeneratorContext) error  // ctx: scoped tracked reader, plan emit, exports, diag
}
type Backend interface {
    Plugin
    Target() Target
    Renderer                               // Render(ctx *RenderContext) ([]RenderedFile, error)
    FileSpeller                            // the name of each file
    SyntaxProvider                         // the comment forms of the target
    TypeSpeller                            // types of other languages
    Packager                               // the package of each file
    PolicyProvider                         // the lowering policies
    Lowerer                                // Lower(symbol.Symbol) ([]symbol.Symbol, error)
    Respeller                              // Respell(host, kind, v, name) (string, error)
}
type WorkspaceCheck interface {
    Plugin
    Reads() []string                       // the plans whose records it reads; nil reads every plan
    Check(ctx *CheckContext) error         // ctx: their records, index, reader, facts, diag
}
```

Failure works the same way in every role. A problem with one item
attaches to the diagnostic sink and the run continues. A returned
error is fatal to the phase.

Four structural rules bind the roles:

- Annotators never add or remove symbols. The node graph freezes
  after Annotate, and the store enforces that rather than a
  docblock.
- Generators are never seeing languages. They read projections and emit
  neutral values.
- No state is carried on the plugin struct between phases. The graph is
  the only memory. A field on the plugin skips the tracked read set,
  which produces output that is stale and looks current.
- `WorkspaceCheck`
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)) is the
  only place from which you can ask a cross-plan question such as
  "does every proto service method have a Go handler and a
  TypeScript client", and it produces diagnostics and nothing else.

## Capabilities

Capabilities are asserted through interfaces, and each has a name
the conformance suite can hold it to:

- `CapabilityProvider` states the priority, the capabilities that the
  plugin provides and requires, and the capabilities that it runs after
  where the composition contains them. Capability topology orders the
  plugins of one priority. Build refuses a plugin that implements part
  of the interface, as it refuses a partial `TemplateProvider`.
- `OptionsProvider` states typed options schemas.
- `TemplateProvider` states template trees, funcs and overrides.
- `FilenameProvider` states the declared outputs.
- `DirectiveProvider` states the directive schemas of the plugin.
- `Versioned` states the contribution to the cache key.

Capability labels follow the same rule as every other registered
name ([README](README.md)). The plugin that provides a capability
exports it as a constant, and requirers reference that constant. A
declarative manifest's `requires: [audit]` is the human spelling,
validated at Build against the declared providers.

**Priorities are per role.** A plugin declares a priority for each
role it implements, because one shared number cannot place a
dual-role plugin's annotator half and generator half independently,
and the turn sequences of the two phases have no reason to share integers.

## The concurrency contract

What a plugin may assume is stated here rather than discovered:

- Frontends run concurrently across patterns, so one frontend
  instance must tolerate concurrent `Load` calls. The store
  serializes graph writes per package.
- Annotators and generators run sequentially by default.
  Parallelism inside a turn is a workspace opt-in,
  `Builder.Parallel`, under which one plugin's phase call runs its
  matches on up to the worker count. The dispatcher buffers the
  placements, slot appends and findings of every match and applies the
  buffers in canonical match order, so a plugin's output does not
  depend on the count ([06b-authoring.md](06b-authoring.md)). A handler that writes
  state outside its effects, a field of the plugin or an emit value
  it appends to directly, has to be safe under the opt-in. The
  conformance suite races exactly that.
- Plans run concurrently with each other, which a plugin never
  observes, because plans are read-isolated by construction. A plan
  that depends on others generates after each of them has rendered,
  and its generators read their exports as frozen values.
- A plugin does not leave goroutines running after its phase call
  returns. Whatever work it forks, it joins before returning.
  Otherwise the read tracking that feeds incrementality has already
  closed underneath it.
- Workspace checks run one after another, on one goroutine.

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
rather than methods returned per call.

The same idea scales up. **Plans are values too**
([08-workspace-and-plans.md](08-workspace-and-plans.md)), so a
satellite or a consumer may export a preset plan such as
`golang.ServerPlan(...)` as plain data the workspace instantiates.

Plans are not plugins. A plan owns an emit graph, a manifest slice,
a sweep scope and a position in the schedule, which are the things
the surrounding frame enforces between plugins. Make a plan a plugin
and the first question becomes "who isolates two plans", returned by
a workspace one level up that you have just reinvented. You compose
plugins the way you compose code, and plans the way you compose
config.

## The declarative host

The second authoring tier is a **declarative plugin**: a plugin
written as files, with no Go. It is a directory with a manifest and
one template tree for each target:

```
acme-audit/
  plugin.yaml
  templates/
    golang/
      function.tmpl
      method.tmpl
      test.tmpl
    typescript/
      function.tmpl
      method.tmpl
      test.tmpl
```

The manifest declares the plugin's directives, output families,
capabilities, options and rules:

```yaml
format: 1
name: acme-audit
version: 0.3.0
directives:
    - name: audit
      doc: Generates code that records each call of the declaration.
      params:
          - key: level
            type: string
            choices: [debug, info, warn]
            required: true
            doc: The level of each record.
          - key: sink
            type: reference
            resolve: callable-in-scope
            required: true
            doc: The function that receives each record.
outputs:
    - per: source
      word: audit
    - tag: test
      per: source
      word: audit
      template: test.tmpl
provides: [audit]
options:
    - key: redact
      type: bool
      default: true
      doc: When true, a record leaves out the arguments of the call.
rules:
    - on: interface
      directive: audit
      emit: implementation
      template: method.tmpl
    - on: function
      directive: audit
      emit: mirror
      template: function.tmpl
    - on: interface
      directive: audit
      output: test
      emit: mirror
```

A rule emits one declaration into one family for each subject of its
kind that it matches. A rule with a directive runs for each subject
with an instance of the directive. A rule without one runs for every
subject of its kind in the plan's sources. The loader accepts four
pairs of kind and emission:

| `on` | `emit` | The declaration |
|---|---|---|
| `function` | `mirror` | a function with the subject's signature, whose body the rule's template renders |
| `interface` | `mirror` | an interface with the subject's effective methods and properties |
| `interface` | `implementation` | a struct with the field `next` of the subject's type and a method for each effective method, whose bodies the rule's template renders. The rule reports `UndelegatedProperty` for an interface with a property, and emits nothing for it |
| `struct` | `mirror` | a struct with the subject's effective fields |

A template reads the declaration as `.Decl` and the payload as
`.Data`. The payload has the subject's name, the params of the gating
directive by key, and the plugin's options by key. The render executes
the template with `missingkey=error`, so a key that the payload does
not contain fails the template under `RefusedTemplate`.

A family with a `template` claims its files. Its rules emit records,
and no kind template renders them. The family's file template writes
each file from the records ([07-rendering.md](07-rendering.md)). The
family `test` of acme-audit writes a Go test of each implementation
this way, such as `svc/ledger_audit_test.go`.

The module `eidos-declarative` contains the **loader**, outside the
kernel ([01-repos-and-kernel.md](01-repos-and-kernel.md)).
`declarative.Load` reads one plugin directory and builds an ordinary
generator through the authoring surface. So priorities, capability
topology, options and conformance apply to each declarative plugin as
they apply to a typed one. The loader reports each fault of the
directory as an `Error` with its file and line, and it refuses every
declaration that the authoring surface refuses with a panic. A manifest
never makes Build panic.

The guarantees and the limits of the tier are both structural:

- Templates are data, so plugins still compose at compile time.
- The version of a declarative plugin is the manifest's `version`, a
  plus sign and the SHA-256 of the manifest and of every template
  file. The fingerprint folds the version, so an edit of either makes
  the next run a cold run.
- A declarative plugin has no typed metadata, no projections beyond
  the members and the types that its rules read, and no logic beyond
  template conditionals. A plugin that needs any of those is written
  in Go.
- The manifest format is public API, versioned under the
  compatibility policy. `declarative.Schema` returns its JSON Schema,
  and the module publishes the schema as `plugin.schema.json`.

A binary admits declarative plugins into the plans that it marks
`Extensible`. Its users list plugin directories under
`plans.<name>.plugins` in the config file
([14-distribution-and-cli.md](14-distribution-and-cli.md)). A
declarative plugin consists of inert files, so a team can adopt one
from another team without a review of its code. A typed plugin is code
that the team has to vet.

## The authoring surface

The role interfaces above are the SPI: what the workspace invokes,
what the declarative host adapts onto, and the base contract everything
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
determinism, sweep or drift guarantee for a file it does not wholly own
([17-output-and-determinism.md](17-output-and-determinism.md)).

Transforming the node graph forks the one graph, after which
explain, invalidation and cross-plan checks all have to ask "which
variant" before they can return anything
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
