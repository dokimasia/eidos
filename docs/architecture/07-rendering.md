# Rendering

*Builds on: [02](02-symbol-model.md) (slot declarations),
[06](06-plugins.md), [06b](06b-authoring.md). Feeds:
[13](13-testing-and-conformance.md) (the lint rung),
[17](17-output-and-determinism.md) (the backend's byte contract),
[18](18-routing-and-layout.md) (declared outputs).*

How emit values become committed source text. Read the first section
first: everything after it is one of three verbs applied at some
granularity.

## The composition verbs

Rendering composes through exactly three verbs, and each has one
home.

**Append**, through slots, at member level and body level. This is
the default verb, and it lets two plugins compose without knowing
about each other.

**Replace**, through declared template overrides in the funcmap's
overrideable bucket. This serves the plugin whose whole purpose is
changing how a construct renders. It is visible, declared, and
resolved by capability topology.

**Wrap**, which the render level refuses. Splicing around-advice
into another plugin's rendered body creates order-sensitive
semantics that no machinery can defend. Wrapping belongs at the
model level: emit a wrapper that calls the original. That gives you
an ordinary emit value with provenance, imports and lint coverage,
composed by the target language's own means.

## Slots, the append verb

One plugin rarely owns a generated file. A repository generator
emits a struct and its constructor. A tracing plugin wants a field
on that struct. A metrics plugin wants the same thing,
independently, without either knowing the other exists.

The primitive for that is accumulation rather than override.
Generators emit entities carrying named slots, each typed by what it
can contain. Plugins append into slots. Contents order by capability
topology, with an alphabetical tie-break. Append a method into a
field slot and it fails at the append, naming the slot, rather than
in the diff or at the compiler.

Slot declarations live in `symbol/schema` and generate with the
models ([02-symbol-model.md](02-symbol-model.md)), so which slots
exist on which emit kind is generated documentation and static
knowledge. `Walk` and the lint rung know every slot, and "what can I
append to a Method" has a generated answer rather than a tribal one.

Generation includes **typed accessors**. Go code appends through
`s.FieldsSlot()` and `m.PrologueSlot()` rather than through a
slot-name string, so appending to a slot that does not exist is a
compile error. Slot names appear as strings only at the boundary: in
the `{{slots}}` markers in templates, and in the declarative
manifest.

**A body is a slot sequence too.** Every body carries two standard
slots by construction, `prologue` and `epilogue`, and its owner may
declare named slots between them. Contributions are scaffolding
statements, ordered like any slot and rendered through the kind
machinery.

Standard body slots fix the classic weakness of slots, which is that
the owner has to anticipate where somebody will want to extend. For
bodies, the extension points exist whether the owner thought of them
or not. A cross-cutting plugin appends an audit call into another
plugin's handler body without either knowing the other exists, and
without touching the other plugin's template.

## Render dispatch: whose template renders what

Every plugin author asks this first. The answer is a ladder of
granularity, smallest claim first: scaffolding statements, then a
body through `TemplateRef`, then a file through a claimed output,
and nothing larger.

**The backend's kind templates render every emit value by default.**
A plugin that only assembles model values, meaning structs, methods
and fields into slots, ships no templates at all, and its output
still renders idiomatically, because the backend owns how the
language spells its kinds.

**A plugin may claim a single body.** Any emit value with a body,
meaning a Method or a Function, may carry a `TemplateRef{name,
data}` instead of scaffolding statements. The reference resolves in
the emitting plugin's template tree for the plan's language, and the
backend executes it at render time inside the kind template's body
slot. The declaration half stays structured and machine-checked,
covering the signature, the docs, slot placement and imports, while
the body renders the plugin's way. The generator names the template
and never executes one, which keeps generators blind to languages
while the same emit graph renders through
`templates/golang/method1.tpl` in one plan and
`templates/typescript/method1.tpl` in another.

**A plugin may claim its own files.** For each declared output
([18-routing-and-layout.md](18-routing-and-layout.md)) a plugin may
ship a file template per language. The backend hands it that file's
emit values and the merged funcmap, and the template drives the
file's body. This is the presentation path, for output whose shape
belongs to the plugin rather than to the language.

**Slot contents always render through the backend's kind
machinery**, whichever mode the host uses. So a cross-cutting
plugin's contribution looks the same in every file it lands in, and
no plugin's template ever renders another plugin's values. A
slot-appended value may carry a `TemplateRef` like any other, and it
resolves in its own emitter's tree.

There is deliberately no way to inject raw text between
declarations. A free-floating text value among typed declarations is
content the slot machinery cannot check, the collision logic cannot
merge, and the import resolver cannot see. If you want that much
control over a file, claim the file.

## Bodies: content and the scaffolding vocabulary

"Interfaces, never implementations"
([10-cross-language.md](10-cross-language.md)) has a precise edge,
and this is where it sits.

`emit` carries a deliberately small neutral statement vocabulary,
`Expr` and `Stmt`, covering a delegate call, a return, an assignment
and a guard. That is enough for delegating scaffolding: a handler
calling the user's function, a stub recording its arguments, a check
comparing a sample. It is deliberately too small to write logic in.
Anything beyond it belongs in a hand-written file that the generated
one calls.

`Sample` is the rendered form of a Values-projection answer
([03-projection.md](03-projection.md)): a literal that a template or
the builder places into a body, gated on `OK()` like every refusable
answer.

A body's own content is one of four things, with the body slots
around it:

- **Nothing.** The kind template's default, where the standard slots
  render automatically.
- **Scaffolding statements**, from the vocabulary above.
- **A `TemplateRef`.** The template must place the slots, through a
  `{{slots}}` marker or named markers. The template-lint rung checks
  statically that every body-claiming template contains the marker,
  and at run time a contribution into a body whose template dropped
  it is an Error naming both plugins.
- **`Verbatim`**, which is literal text. It is the sharp knife for a
  genuine one-liner, and it costs three things: the template-lint
  rung cannot see into it, it contributes no imports, and nothing
  can compose with it. A `TemplateRef` avoids all three and is
  almost always the better tool. `Verbatim` exists only inside
  bodies, never as a declaration-level value.

## Template ownership

Each backend owns its language's core templates, and each plugin
ships its own trees for the languages it supports.

The framework owns no templates. It has no view on how a Go method
renders, and any view it held would be re-argued for every language
added afterwards. Languages differ in structure rather than in
substitution, so a shared template parameterized per language just
pushes another conditional into a file every language has to share.

## The funcmap

A language's shared template vocabulary is registered **once, by
that language's backend**, into the overrideable bucket. A plugin
registers only the helpers it wrote, under the names it declared,
and replaces a shared name through a declared override, which is the
replace verb.

There are no per-plugin prefixes. A helper has one documented name
everywhere, so templates move between plugins and the documentation
can name what authors actually type.

## The engine is text/template

It is native to the ecosystem that writes typed plugins, and it adds
no dependency, which the kernel needs. CUE is rejected for
rendering, because it is a configuration language and does not
render text, and rejected for config too
([21-decisions.md](21-decisions.md), D12).

text/template has two real weaknesses: errors surface when the
template executes, and nothing is checked statically. Both are
handled where they belong, in the conformance ladder.

**The template-lint rung** parses every template of every plugin
against the merged funcmap of each language it declares, at CI time.
Undefined functions, colliding overrides and missing body-slot
markers become build failures. So do undefined field references
wherever the bound type is known, which means always for kind
templates, and for the emit-value half of body and file templates
but not for plugin-supplied payloads.

**Execute-time errors map to a template file and line**, carried on
the diagnostic, with the emitting plugin named.

## The builder API, an equal second path

A typed, fluent Go API constructs emit values directly, and it is
documented as a peer of templates rather than a fallback.

Templates suit presentation-shaped plugins, and they are the only
path in the declarative tier. The builder suits logic-shaped
plugins, and it is type-checked end to end. Both paths produce
identical emit values and compose in the same slots, so a plugin may
use either or both.

## Backends

A backend renders one plan's emit graph. There is one backend per
plan ([08-workspace-and-plans.md](08-workspace-and-plans.md)), so
import resolution, formatting and layout each have exactly one
language to answer for.

The kit owns the pass itself ([11-languages.md](11-languages.md)),
and it runs the same way in every satellite:

1. **Group** the plan's emit values by `Target`. Each group is one
   file, and the files are independent from here, so the kit
   parallelises the per-file loop.
2. **Render declarations** through the language's kind templates, in
   canonical order: subject identity, then slot order. A plugin's
   claimed file template takes the whole group instead, per the
   ladder above.
3. **Splice slot contributions** through the same kind machinery, so
   a contribution renders identically in every file it lands in.
4. **Resolve each `TemplateRef`** in its emitting plugin's tree for
   the plan's target, and execute it inside the kind template's body
   slot. The template must place the slot marker. A pending
   contribution into a body whose template dropped it is an Error
   naming both plugins.
5. **Collect imports** as a side effect of spelling types into the
   file's one `ImportSet`. The language's `Imports` renderer groups
   and sorts them.
6. **Finalise** through the language formatter. A format failure is
   a positioned Error carrying the file it could not format, and the
   pass continues with the remaining files. The sink never receives
   an unformatted file.
7. **Stamp** the generated-file header and the provenance trailer
   ([17-output-and-determinism.md](17-output-and-determinism.md)).
8. **Write** through the plan's staged sink. Write-if-changed and
   atomicity belong to the sink rather than the backend.

The reference a generator hands the backend, pinned:

```go
type TemplateRef struct {
    Name string // resolves in the EMITTING plugin's tree, per target
    Data any    // plugin-supplied payload; the lint rung checks the
}               // emit-value half of a body template, never Data
```

Byte-determinism is the backend's contract: the same emit graph
produces the same bytes on every machine. The conformance ladder
asserts it, and everything downstream assumes it, including
manifests, sweeps and warm≡cold.
