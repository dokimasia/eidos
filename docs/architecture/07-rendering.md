# Rendering

*Builds on: [02](02-symbol-model.md) (slot declarations),
[06](06-plugins.md), [06b](06b-authoring.md). Feeds:
[13](13-testing-and-conformance.md)
(the lint rung), [17](17-output-and-determinism.md) (the backend's
byte contract), [18](18-routing-and-layout.md) (declared outputs).*

How emit values become committed source text. Read this document
through its first section: everything after it is one of three verbs
applied at some granularity.

## The composition verbs

Rendering composes by exactly three verbs, each with one home:

1. **Append** — slots, member-level and body-level. The default
   verb: two plugins compose without knowing each other.
2. **Replace** — declared template overrides through the funcmap's
   overrideable bucket, for the plugin whose purpose is changing how
   a construct renders. Visible, declared, resolved by capability
   topology.
3. **Wrap** — refused at the render level. Around-advice spliced
   into another plugin's rendered body is order-sensitive semantics
   no machinery can defend. Wrapping is a model-level pattern: emit
   a wrapper that calls the original — an ordinary emit value with
   provenance, imports, and lint coverage, composed by the target
   language's own means.

## Slots — the append verb

Generated output is rarely owned by one plugin: a repository
generator emits a struct and its constructor; a tracing plugin wants
a field on that struct; a metrics plugin wants the same,
independently, without either knowing the other exists. The
primitive for that is accumulation, not override: generators emit
entities carrying named slots typed by content kind; plugins append
into slots; contents order by capability topology with an
alphabetical tie-break. A method appended into a field slot fails at
the append with the slot named — not in the diff, not at the
compiler.

Slot declarations live in `symbol/schema` and generate with the
models ([02-symbol-model.md](02-symbol-model.md)): which slots exist
on which emit kind is generated documentation and static knowledge —
`Walk` and the lint rung know every slot, and "what can I append to
a Method?" has a generated answer, not a tribal one. The generation
includes **typed accessors** — Go code appends through
`s.FieldsSlot()` and `m.PrologueSlot()`, never through a slot-name
string, so appending to a slot that doesn't exist is a compile
error. Slot names appear as strings only at the boundary: the
`{{slots}}` markers in templates and the declarative manifest.

**Bodies are slot sequences too.** Every body carries two standard
slots by construction — `prologue` and `epilogue` — and the owner
may declare named ones between; contributions are scaffolding
statements, ordered like any slot and rendered through the kind
machinery. Standard body slots close the classic slot weakness (an
owner who must anticipate where extension is wanted): for bodies,
extension points exist whether or not the owner thought of them. A
cross-cutting plugin appends an audit call into another plugin's
handler body without either knowing the other exists — and without
ever touching the other plugin's template.

## Render dispatch: whose template renders what

The question every plugin author asks first, answered as a
granularity ladder — smallest claim first: scaffolding statements →
a body (`TemplateRef`) → a file (claimed output) → nothing larger.

- **The backend's kind templates render every emit value by
  default.** A plugin that only assembles model values — structs,
  methods, fields into slots — ships no templates at all and its
  output still renders idiomatically, because the backend owns how
  the language spells its kinds.
- **A plugin may claim a single body.** Any emit value with a body
  (Method, Function) may carry a `TemplateRef{name, data}` instead
  of scaffolding statements. The reference resolves in the emitting
  plugin's template tree for the plan's language and the backend
  executes it at render time inside the kind template's body slot —
  the declaration half (signature, docs, slot placement, imports)
  stays structured and machine-checked while the body renders the
  plugin's way. The generator *names* the template; it never
  executes one — which keeps generators language-blind while the
  same emit graph renders through `templates/golang/method1.tpl` in
  one plan and `templates/typescript/method1.tpl` in another.
- **A plugin may claim its own files.** Per declared output
  ([18-routing-and-layout.md](18-routing-and-layout.md)) a plugin
  may ship a file template per language: the backend hands it that
  file's emit values and the merged funcmap, and the template drives
  the file's body. This is the presentation path — for output whose
  shape is the plugin's signature, not the language's default.
- **Slot contents always render through the backend's kind
  machinery**, whichever mode the host uses — so a cross-cutting
  plugin's contribution looks the same in every file it lands in,
  and no plugin's template ever renders another plugin's values. A
  slot-appended value may carry a `TemplateRef` like any other; it
  resolves in *its* emitter's tree.

There is deliberately no way to inject raw text *between*
declarations: a free-floating text value among typed declarations is
content the slot machinery cannot check, the collision logic cannot
merge, and the import resolver cannot see. Wanting that much control
of a file is what claiming the file is for.

## Bodies: content and the scaffolding vocabulary

"Interfaces, never implementations"
([10-cross-language.md](10-cross-language.md)) has a precise edge,
and it is here. `emit` carries a deliberately small neutral
statement vocabulary — `Expr` and `Stmt`: delegate call, return,
assignment, guard — enough for delegating scaffolding (a handler
calling the user's function, a stub recording its arguments, a check
comparing a sample), and deliberately too small to write logic in.
Anything beyond it belongs in a hand-written file the generated one
calls. `Sample` is the rendered form of a Values-projection answer
([03-projection.md](03-projection.md)): a literal a template or
builder places into a body, gated on `OK()` like every refusable
answer.

A body's own content is one of four things, with the body slots
around it:

- **Nothing** — the kind template's default; standard slots render
  automatically.
- **Scaffolding statements** — the vocabulary above.
- **`TemplateRef`** — the template must **place** the slots: a
  `{{slots}}` marker (or named markers). The template-lint rung
  checks statically that every body-claiming template contains the
  marker; at run time, contributions into a body whose template
  dropped the marker are an Error naming both plugins.
- **`Verbatim`** — literal text, the sharp knife for the genuine
  one-liner, with three stated costs: opaque to the template-lint
  rung, contributes no imports, closed to composition. A
  `TemplateRef` avoids all three and is almost always the better
  tool. Verbatim exists only inside bodies; it is not a
  declaration-level value.

## Template ownership

Each backend owns its language's core templates; each plugin ships
its own trees per language it supports. The framework owns no
templates: it has no view on how a Go method renders, and any
opinion it held would be re-litigated for every language added after
it — languages differ by structure, not by substitution, so a shared
template parameterized per language pushes another conditional into
a file every language shares.

## The funcmap

A language's shared template vocabulary is registered **once, by
that language's backend**, into the overrideable bucket. A plugin
registers only helpers it wrote, under names it declared, and
replaces a shared name through declared overrides — the replace
verb. No per-plugin prefixes: a helper has one documented name
everywhere, so templates are portable between plugins and
documentation can name what authors type.

## Engine: text/template (decided)

Ecosystem-native for the audience that writes typed plugins, and
zero-dependency — a kernel virtue. CUE is rejected for rendering —
it is a configuration language and does not render prose — and for
config ([21-decisions.md](21-decisions.md), D12).

text/template's real weaknesses are execute-time errors and no
static checking. Both are addressed where they belong, in the
conformance ladder:

- **The template-lint rung**: every template of every plugin parses
  against the merged funcmap of each language it declares, at CI
  time. Undefined functions, colliding overrides, missing body-slot
  markers, and — wherever the bound type is known (kind templates
  always; body and file templates for their emit-value half, not
  for plugin-supplied payloads) — undefined field references are
  build failures, not run-time surprises.
- **Execute-time errors map to template file:line**, carried on the
  diagnostic, with the emitting plugin named.

## The builder API — the equal second path

A typed, fluent Go API constructing emit values directly, documented
as a peer of templates. Templates suit presentation-shaped plugins
(and are the *only* path in the declarative tier); the builder suits
logic-shaped plugins, and is type-checked end to end. Both paths
produce identical emit values and compose in the same slots — a
plugin may use either or both.

## Backends

A backend renders one plan's emit graph. One backend per plan
([08-workspace-and-plans.md](08-workspace-and-plans.md)), so import
resolution, formatting, and layout each have exactly one language
to answer for. The pass itself is kit-owned
([11-languages.md](11-languages.md)) and runs the same way in
every satellite:

1. **Group** the plan's emit values by `Target`. Each group is one
   file, and files are independent from here — the kit
   parallelises the per-file loop.
2. **Render declarations** through the language's kind templates,
   in canonical order: subject identity, then slot order. A
   plugin's claimed file template takes the group instead, per the
   dispatch ladder above.
3. **Splice slot contributions** through the same kind machinery —
   a contribution renders identically in every host file.
4. **Resolve each `TemplateRef`** in its emitting plugin's tree
   for the plan's target and execute it inside the kind template's
   body slot. The template must place the slot marker: pending
   contributions into a body whose template dropped it are an
   Error naming both plugins.
5. **Collect imports** as a side effect of type spelling into the
   file's one `ImportSet`; the language's `Imports` renderer
   groups and sorts them.
6. **Finalise** through the language formatter. A format failure
   is a positioned Error carrying the offending rendered file, and
   the pass continues with the remaining files — the
   continue-on-failure flow; the sink never receives an
   unformatted file.
7. **Stamp** the generated-file header and the provenance trailer
   ([17-output-and-determinism.md](17-output-and-determinism.md)).
8. **Write** through the plan's staged sink; write-if-changed and
   atomicity are the sink's contract, not the backend's.

Execute-time template errors map to template file:line with the
emitting plugin named — the run-time half of the lint rung's
static promise.

The reference a generator hands the backend, pinned:

```go
type TemplateRef struct {
    Name string // resolves in the EMITTING plugin's tree, per target
    Data any    // plugin-supplied payload; the lint rung checks the
}               // emit-value half of a body template, never Data
```

Byte-determinism is the backend's contract: same emit graph, same
bytes, on every machine. It is asserted by the conformance ladder
and assumed by everything downstream (manifests, sweeps, warm≡cold).
