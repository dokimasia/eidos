<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Rendering

*Builds on: [02](02-symbol-model.md) (slot declarations),
[06](06-plugins.md), [06b](06b-authoring.md). Feeds:
[13](13-testing-and-conformance.md) (the lint check),
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

**Replace**, through declared template overrides of the funcmap's
shared helpers. This serves the plugin whose whole purpose is
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
knowledge. `Walk` and the lint check know every slot, and "what can I
append to a Method" has a generated answer rather than a tribal one.

Generation includes **typed accessors** for member slots. Go code
appends through `s.FieldsSlot()` and `s.MethodsSlot()` instead of
through a slot-name string, so appending to a slot that does not
exist is a compile error. A body's standard slots are the fields
`Body.Prologue` and `Body.Epilogue`. `Body.Slot` looks up an owner's
named body slot by its name and reports false for a name the owner
never declared, so a contribution into an invented extension point
fails where it is made. Otherwise slot names appear as strings only
at the boundary: in the `{{slots}}` markers in templates, and in the
declarative manifest.

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

Every plugin author asks this first. The answer is a fixed order of
claims, smallest first: scaffolding statements, then a
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
and never executes one, which keeps generators never seeing languages
while the same emit graph renders through
`templates/golang/method1.tpl` in one plan and
`templates/typescript/method1.tpl` in another.

**A plugin may claim its own files.** With a file template in
`Output.Template` ([18-routing-and-layout.md](18-routing-and-layout.md)),
a family claims every one of its files. Its handlers place records
through `Out.Record`. A
record is a declaration with the data of its match. No name table,
collision check or export reads a record. The render executes the
template from the plugin's tree for the plan's target. The template
reads the name and the package of the file, and the declaration and the
data of each record. It writes the declarations of the file through the
language's funcmap, and it places the body of a record through the
`body` builtin. The language's file skeleton then frames the output
with the header and the imports that the template recorded through
`use`. A file that mixes the units of a claimed family with other units
is withheld under `MixedClaim`. This is the presentation path, for
output whose shape belongs to the plugin rather than to the language.

**Slot contents always render through the backend's kind
machinery**, whichever mode the host uses. So a cross-cutting
plugin's contribution looks the same in every file it arrives in, and
no plugin's template ever renders another plugin's values. A
slot-appended value may carry a `TemplateRef` like any other, and it
resolves in its own emitter's tree.

**A generator may state the header of a file.** An `emit.File` in a
unit states the documentation and the annotations of its file, which
the layout merges into the header of the file
([18-routing-and-layout.md](18-routing-and-layout.md)). The render
passes the header to the file skeleton as `.Header`, and every template
of the file reads it through the `header` builtin. The declaration run
skips an `emit.File`. A file whose units contain an `emit.File` alone
renders its skeleton. The coverage verdict of the annotations of a file
applies to the header, and the guard reads it before the declarations
render. The Go, TypeScript, Java and Rust backends refuse the
annotations of a file, because their skeletons do not write a header. A
header in those targets reports `RefusedFact`.

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

An expression has one of four kinds:

- `ExprName` is a local spelling, such as a parameter, a receiver or a
  callable of the file's own package. The target writes it as it is
  and imports nothing for it.
- `ExprCall` applies its `Fn` to its `Args`.
- `ExprValue` holds a `Value`, the tree that a projection derives.
- `ExprRef` refers to a function, a constant or a variable of another
  package by its identity in `Ref`.

The target qualifies the name of a reference for the file and records
the import through `Callee`, as it does for the callee of a value call.
As the `Fn` of a call, a reference calls the function with the
arguments of the call. So a contributor appends `audit.Record(w, r)` to
a handler, with the parameters of the handler as names. The scaffold
refuses a reference whose identity has no name, and a reference
without a target. The settle leaves a reference as written, because the
target spells the identity. A name in one of the plan's own packages is
an `ExprName`, and the settle follows it.

A check's call takes a sample as an argument through `ExprValue`,
whose `Value` contains the data of the sample rather than its spelling
([03-projection.md](03-projection.md)). Each target's scaffold spells the tree
through the shared walk, recording the imports its references and
callees need into the file's import set, and refuses a leaf it has
no form for under `UnspeltValue`: raw text written in another
language, an address in a language with no operator for one, a map
literal in a language that states none. The declaration is skipped
and the file renders without it, the way any refused spelling is.

A construction builds a value of a class from argument values, as
`new Set<string>(['a'])`. The walk passes its type to the target
without a spelling, because a target that imports a declaration for
its type checker alone imports the class of a construction for its
value. TypeScript and Java write a construction. Go and Rust refuse it
under `UnspeltValue`. A structural type without a spelling, which the
substitution of a type parameter leaves, also passes to the target.
The Go and TypeScript targets restate it from its children. The Java
and Rust targets refuse it under `UnspeltValue`, because the spelling
of their composites does not follow from the structure.

A function is a value of a function type that returns its result
values, and a make is the empty value of a type that `make` returns.
The walk passes the type of a function to the target without a
spelling, beside the spellings of its results. It passes the type of a
make with its spelling. Each target writes the function literal of its
language:

| Target | A function of one result | A function without results |
|---|---|---|
| Go | `func(context.Context) error { return nil }` | `func(string) {}` |
| TypeScript | `() => 0` | `() => {}` |
| Java | `(unused0) -> 0L` | `() -> {}` |
| Rust | `\|_\| 0` | `\|_, _\| {}` |
| Python | `lambda _0: 0` | `lambda _0, _1: None` |

The targets follow these rules:

- The Go target writes the signature through its speller, so the file
  imports each type of the signature. The other targets do not write a
  type, and a function there does not import anything.
- The Python target writes one parameter for each parameter of the
  `Callable`, because a lambda with fewer parameters is not a value of
  it.
- TypeScript, Rust and Python return the results of a function of two
  or more results as a tuple, as their function types do. Java refuses
  such a function, because a lambda returns one value.
- Every target refuses a function of a type that is not a function type
  under `UnspeltValue`, and TypeScript refuses a constructor type.
- The Go target writes a make as `make(chan string)`. `make` is the
  built-in of Go, so the TypeScript, Java, Rust and Python targets
  refuse a make under `UnspeltValue`.

`Sample` is the rendered form of a Values-projection answer
([03-projection.md](03-projection.md)): a literal that a template or
the builder places into a body, gated on `OK()` like every refusable
answer.

A template writes a value inside an expression through the `value`
builtin, such as a field of a composite literal or the initializer of a
variable. The builtin passes the `emit.Value` to the language's `Value`,
which writes it through the scaffold target of the file and records its
imports. So a template and a body of one file write a value with one
spelling. A kind template, a body template and a file template all
resolve the builtin. The backends of Go, TypeScript, Java, Rust and
Python declare a writer. The protobuf and GraphQL backends do not write
bodies or declare a writer, so a call there fails under
`RefusedTemplate` with the name of the language. A value that the
target cannot spell reports `UnspeltValue`, as it does in a body.

A body's own content is one of four things, with the body slots
around it:

- **Nothing.** The kind template's default, where the standard slots
  render automatically.
- **Scaffolding statements**, from the vocabulary above.
- **A `TemplateRef`.** The template must place the slots, through a
  `{{slots}}` marker or named markers. The template lint checks
  statically that every body-claiming template contains the marker,
  and at run time a contribution into a body whose template dropped
  it is an Error naming the emitting plugin and counting what went
  unplaced, because a slot contribution carries no attribution to
  name its contributor by.
- **`Verbatim`**, which is literal text. It is for the genuine
  one-liner only, and it costs three things: the template-lint
  check cannot see into it, it contributes no imports, and nothing
  can compose with it. A `TemplateRef` avoids all three and is
  almost always the better tool. `Verbatim` exists only inside
  bodies, never as a declaration-level value.

The marker rule binds body-claiming templates alone. A kind template
that never calls the body builtin renders its declarations
signature-only, and no rule applies: what a language's own templates
choose to place is the template author's responsibility.

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
that language's backend**. A plugin
registers only the helpers it wrote, under the names it declared,
and replaces a shared name through a declared override, which is the
replace verb.

There are no per-plugin prefixes. A helper has one documented name
everywhere, so templates move between plugins and the documentation
can name what authors actually type.

The backends of Go, TypeScript, Java, Rust and Python declare the eight
shared names of the render package. The protobuf and GraphQL backends
declare `docs` and `wrapdocs`:

| Helper | Writes |
|---|---|
| `docs` | the documentation of a declaration in the comment form of the target |
| `wrapdocs` | the lines of a text wrapped at 72 columns, which a template passes to `docs` or to the comment helper of a member |
| `spell` | a type reference in a type position |
| `spellvalue` | a type reference in a value position, where the declaration must exist at run time, such as the class of a construction |
| `typeparams` | the type parameter list of a declaration |
| `typeargs` | a list of type parameters or of type references as the type arguments of a reference, and nothing for an empty list |
| `params` | the parameter list of a callable |
| `results` | the results of a callable |

A TypeScript `spellvalue` imports the declaration for its value and
the type arguments for the type checker alone. A Rust `spellvalue`
writes the type arguments behind the turbofish of an expression's
path.

`wrapdocs` keeps a word that the documentation syntax of the target
reads as the start of a block off the start of a line: a block tag of
TSDoc and Javadoc, and a block opener of CommonMark in TypeScript, Rust,
GraphQL and a Python docstring. A Go and a protobuf comment read an
unindented line as text. The wrap never breaks a word, so a word longer than the width is
a line of its own.

## The engine is text/template

It is native to the ecosystem that writes typed plugins, and it adds
no dependency, which the kernel needs. CUE is rejected for
rendering, because it is a configuration language and does not
render text, and rejected for config too
([21-decisions.md](21-decisions.md), D12).

text/template has two real weaknesses: errors surface when the
template executes, and nothing is checked statically. Both are
handled where they belong, in the conformance checks.

**The template-lint check** parses every template of every plugin
against the merged funcmap of each language it declares, at CI time.
Undefined functions, colliding overrides and missing body-slot
markers become build failures. So do undefined field references
wherever the bound type is known, which means always for kind
templates, and for the emit-value half of body and file templates
but not for plugin-supplied payloads. A file template places no
body-slot marker, so the check parses it against the file builtins
and does not ask for the marker.

**Execute-time errors map to a template file and line**, carried on
the diagnostic, with the emitting plugin named. The render executes
every body template and every file template with `missingkey=error`.
A key that a payload map does not contain fails the execution under
`RefusedTemplate`, so the generated source never contains the text
`<no value>`.

## The builder API, an equal second path

A plugin constructs emit values directly, as composite literals of
the generated kinds, which the compiler checks. This path is
documented as a peer of templates rather than a fallback.

Templates suit presentation-shaped plugins, and they are the only
path in the declarative tier. Composite literals suit logic-shaped
plugins. Both paths produce identical emit values and compose in the
same slots, so a plugin may use either or both.

## Backends

A backend renders one plan's emit graph. There is one backend per
plan ([08-workspace-and-plans.md](08-workspace-and-plans.md)), so
import resolution, formatting and layout each have exactly one
language to return for.

The kit owns the render pass ([11-languages.md](11-languages.md))
and the output contract finishes it
([17-output-and-determinism.md](17-output-and-determinism.md)); the
flow runs the same way in every satellite:

1. **Take the routed files** the plan's layout composed
   ([18-routing-and-layout.md](18-routing-and-layout.md)): each file's
   path, its package and its units, split through the target's `Split`
   and named through its `Naming`. The files are independent from here,
   so the kit parallelises the per-file loop, and each file's package
   is the import home its references qualify against.
2. **Render declarations** through the language's kind templates, in
   canonical order: the source position of each origin, then the
   unit's declaration order. A plugin's claimed file template takes the whole file
   instead, per the order above. A backend declares each kind its
   language cannot spell refused, with a reason, and the pass
   skips a declaration of that kind under an Error that states
   the reason. A file whose every declaration is skipped is
   withheld, because it has no content to stamp.
3. **Splice slot contributions** through the same kind machinery, so
   a contribution renders identically in every file it arrives in.
4. **Resolve each `TemplateRef`** in its emitting plugin's tree for
   the plan's target, and execute it inside the kind template's body
   slot. The template must place the slot marker. A pending
   contribution into a body whose template dropped it is an Error
   naming the emitting plugin and counting what went unplaced.
5. **Collect imports** as a side effect of spelling types into the
   file's one `ImportSet`. The language's `Imports` renderer groups
   and sorts them. The set has the names that the package clauses of
   the run's packages declare, and the Go backend binds an import under
   the declared name of its package. A rendered file lists the packages
   whose names it read, and a warm run renders the file again after an
   edit renames one of them. The set also has the package of the file
   under render, through `SetPackage`, and the packages of the files
   that the file refers into, through `SetReferents`, so the protobuf
   backend qualifies a reference by the package clause of its file.
   The set has the lowering policy of the plan, through `SetPolicy`, so
   the Python backend writes the syntax of the version of Python that
   the policy selects. `Enter` names the declaration under render, and
   `Pending` reports a name that it or a later declaration of the file
   takes, so the Python backend quotes a reference to a declaration that
   has not rendered yet. While the pass spells a body, `Body` returns its
   callable and the declaration that places it. `Has` reads back an
   entry that the vocabulary recorded for the file.
6. **Finalise** through the language formatter. A format failure is
   a positioned Error carrying the file it could not format, and the
   pass continues with the remaining files. The sink never receives
   an unformatted file.
7. **Stamp** the generated-file header and the provenance trailer
   ([17-output-and-determinism.md](17-output-and-determinism.md)).
8. **Write** through the plan's staged sink, at the file's routed
   path. Write-if-changed and atomicity belong to the sink, not the
   backend.

The reference a generator hands the backend, pinned:

```go
type TemplateRef struct {
    Name  string      // resolves in the owner's tree, per target
    Data  any         // plugin-supplied payload; the lint checks the
                      // emit-value half of a body template, never Data
    Owner diag.Origin // the emitting plugin, which Emitter.Ref sets;
}                     // empty resolves in the unit's plugin's tree
```

Byte-determinism is the backend's contract: the same emit graph
produces the same bytes on every machine. The conformance checks
assert it, and everything downstream assumes it, including
manifests, sweeps and warm≡cold.
