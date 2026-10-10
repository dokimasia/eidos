---
rfc: 0009
title: The render pass and the backend kit
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Review
created: 2026-08-30
updated: 2026-09-30
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0009: The render pass and the backend kit

## Summary

This RFC makes a backend invokable. The service provider interface
gains the render seam: a `Renderer` takes one plan's emit store and
returns files as values, bytes against names, with nothing arriving
on disk. The kernel package `render` owns the pass every backend
runs: group units into files, render declarations through the
language's kind templates in canonical order, splice slot
contributions through the same machinery, resolve each template
reference in its emitting plugin's tree, collect imports, and
finalise through the language formatter, continuing past a format
failure. The kernel gains `backend.New`, the kit that builds a
renderer from the handful of things a language genuinely varies in.
Template lint checks every declared template statically, and the
conformance suite gains `backendtest` with the checks a valueless
render can prove: byte-stable output, every kind rendered or
refused, slots spliced, format failures survived. Headers, trailers and sinks are
the output contract's seams, consumed here and proposed elsewhere.

## Motivation

The emit model now says everything about what to render, bodies
included, and nothing renders it. Three rules have to be settled
before the first language arrives, because retrofitting them means
rewriting every backend that came first:

- **One pass, owned once.** Grouping, splice order, reference
  resolution, the format-failure rule and the merge order are the
  same work in every language. Written per satellite they drift
  apart, and the drift is exactly the kind byte-identity cannot
  tolerate. The kit owns the procedure; the author supplies what the
  language varies in.
- **No plugin renders another plugin's values.** Slot contributions
  render through the backend's kind machinery whichever mode the
  host uses, and a template reference resolves in its emitting
  plugin's tree alone. Both rules need the pass to enforce them,
  because no template can.
- **text/template fails late, so the lint must run early.** The
  engine reports at execute time and checks nothing statically.
  Both weaknesses are handled by the conformance checks: the lint
  check parses every declared template against each target's merged
  funcmap before any run exists, and an execute-time error maps to
  a template file and line with the emitting plugin named.

## Detailed design

### The render seam on the service provider interface

A backend is composed and validated where plans are; rendering is
what it does when a caller hands it a plan's emit. The contract
returns values, because what arrives on disk, and whether anything
does, belongs to the sink:

```go
package plugin

// RenderedFile is one rendered output as a value: the derived
// filename under its owning package, and the finished bytes.
// Nothing here touches disk; paths, staging and commit belong to
// the sink that consumes it.
type RenderedFile struct {
    // Name is the target-spelled filename the unit's routing key
    // derives: word, tag and cardinality key joined the way the
    // language joins them.
    Name string
    // Pkg is the owning package's identity, zero for a plan file:
    // the namespace half of the file's address, because two
    // packages spell the same filename and stay two files,
    // whatever directory layout places them in.
    Pkg symbol.Identity
    // Body is the finished text: formatted, in the language's own
    // spelling. The output contract stamps and stages it.
    Body []byte
}

// Renderer renders one plan's emit into files as values. A
// problem with one file attaches to the context's sink and the
// pass continues with the remaining files; a returned error is
// fatal to the pass. Two calls over one store return the same
// bytes, which the conformance suite holds every renderer to.
type Renderer interface {
    Render(ctx *RenderContext) ([]RenderedFile, error)
}

// RenderContext carries what one render call may touch.
type RenderContext struct {
    // Emit is the plan's store, complete: every generator ran and
    // every accumulator flushed before rendering starts.
    Emit *Emit
    // Schedule is the plan's generators in bucket order: what
    // declared template overrides resolve against, carried as
    // data so the pass decides nothing.
    Schedule []ID
    // Trees holds each plugin's declared template tree for this
    // target, keyed by plugin: what a template reference resolves
    // in. The composition reads them off the TemplateProvider
    // surface; a fixture hands them over directly.
    Trees map[ID]fs.FS
    // Funcs holds each plugin's template helpers for this target,
    // and Overrides the shared names each declares it replaces,
    // both read off the same surface. The merge is the pass's:
    // schedule order, latest wins, and a shared name shadowed
    // without a declaration is refused and reported.
    Funcs     map[ID]template.FuncMap
    Overrides map[ID][]string
    // Sink takes the pass's findings: an unresolved reference, a
    // dropped slot marker, a format failure.
    Sink *diag.Sink
    // Plugin is the backend's own identity, the origin its
    // findings carry.
    Plugin ID
}

// TemplateProvider declares a plugin's presentation for each
// target: the tree its references resolve in, the helpers its
// templates call, and the shared vocabulary names it replaces.
//
// Templates returns the tree that serves a target and reports
// false where none does. TemplateTargets returns the targets the
// plugin declares a tree of its own for, sorted, so a composition
// refuses a plan whose target a plugin with such trees does not
// serve, at Build and not at render. TemplateFuncs returns the
// helpers for a target, the replacements included, and Overrides
// the replaced names for that target, which is the replace verb: a
// shared name shadowed without a declaration is a lint finding,
// and where two plugins override one name, the one at the latest
// schedule position takes effect, because a plugin that changes
// how a construct renders runs after what it changes.
type TemplateProvider interface {
    Templates(t Target) (fs.FS, bool)
    TemplateTargets() []Target
    TemplateFuncs(t Target) template.FuncMap
    Overrides(t Target) []string
}
```

A backend that renders implements `Renderer` beside `Backend`. The
composition keeps validating the backend contract exactly as it
does; nothing at composition invokes rendering, so the seam is
additive and the plan value does not change. The authoring surface
passes the template declaration through the way it passes keys:

```go
package eidos

// Templates and Funcs declare the plugin-level tree and helpers,
// which serve every target without a declaration of its own.
func (b *Builder) Templates(tree fs.FS) *Builder
func (b *Builder) Funcs(fm template.FuncMap) *Builder

// For layers one target's tree, helpers and overrides over the
// plugin-level declarations.
func (b *Builder) For(t plugin.Target, opts ...TargetOption) *Builder
func Templates(tree fs.FS) TargetOption
func Funcs(fm template.FuncMap) TargetOption
func Overrides(fm template.FuncMap) TargetOption
```

### What a language is to the kit

Two values carry everything the pass cannot know:

```go
// CommentSyntax is a language's comment forms, declared once per
// satellite and shared by the kits: the frontend strips comments
// with it, and the output contract writes the generated-file
// header through it.
type CommentSyntax struct {
    // Line holds the line-comment openers, first one canonical.
    Line []string
    // Blocks holds the block forms.
    Blocks []CommentBlock
}

// CommentBlock is one block-comment form, gutter included, so a
// doc block's continuation lines strip and render the same way.
type CommentBlock struct {
    Open   string
    Close  string
    Gutter string
}
```

Filename spelling is a language fact the plugin never writes: one
declaration serves `store_stub.go` and `store.stub.ts` alike, so
the join, the case rule and the extension come from the target.
The kit takes it as a function of the unit's routing key, declared
beside the pass that calls it:

```go
package render

// Naming spells a unit's filename for one target: the join and
// extension of the family's word, the tag's treatment, and the
// cardinality key's stem. It is total: every unit the plan admits
// returns a name, and units returning one name assemble one file,
// which is how two plugins share it.
type Naming func(u plugin.Unit) string
```

### Splitting and clustering

Two language facts force two optional seams. A Java file holds one
public type, so a unit holding two must become two files, named
after the types they hold; and a Rust method renders only inside
an impl block grouped by the type it attaches to, which no
per-declaration template can write. Both are the target's own
facts, so both are declared beside Naming and both default to off.

```go
// Split reshapes one unit into the units the target files
// separately: Java returns one unit per file-level type, and its
// Naming reads the lone type's name, so the filename the language
// demands spells while the routing key keeps carrying the source
// derivation. A nil Split keeps every unit whole. The pass
// applies it before naming, preserves order, and calls it once
// per unit, so a pure function keeps the render deterministic.
type Split func(u plugin.Unit) []plugin.Unit

// GroupName names a declaration cluster a group template spells.
type GroupName string

// Cluster assigns one unit's declarations to named groups: Rust
// gathers methods under its impl group by the type they attach
// to. A declaration the function leaves unassigned renders
// through its kind template; a cluster renders through the group
// template its name selects, in place of its members' kind
// templates, at the position of its first member. A group
// template receives the cluster and spells its members itself,
// through the shared vocabulary and the body builtin. A nil
// Cluster leaves every declaration a singleton. Clusters stay
// inside one unit, so plugin attribution and canonical order
// survive.
type Cluster func(decls []symbol.Symbol) []Clustered

// Clustered is one cluster: the group template that spells it
// and the declarations it holds, in unit order.
type Clustered struct {
    Group GroupName
    Decls []symbol.Symbol
}
```

A target with no file-level callable kind declares the callable
kinds refused, and the render reports a standalone callable under
the refused-kind finding. The pass does not adopt orphans into
hosts: manufacturing a wrapper declaration is a generator's move
through slots, on the model, where provenance and composition
apply, and a render-level adoption would be the transform the
model level already refuses.

### The import entries

An import set entry records the path a spelling qualified with
and, where the language's import form binds one, the name it binds
in the file. TypeScript writes the bound names in braces before the
path, Java binds one qualified name per statement, Rust one use
path, and Go one package name. A bare path is the side-effect or
whole-namespace form everywhere.

```go
// Entry is one collected import: the path, the name it binds where
// the language's import form binds one, and the declaration's own
// name where the bound name renames it, as TypeScript's
// { Row as Row2 } does. TypeOnly marks a binding the language
// erases at run time, such as TypeScript's import type. A bare
// path leaves Name empty.
type Entry struct {
    Path     string
    Name     string
    Item     string
    TypeOnly bool
}

// Add records a bare path, AddNamed a bound name under a path, and
// AddType a type-only one, each outside the name assignment. Paths
// returns the distinct paths sorted, for a renderer that binds no
// names; Entries returns the full set sorted by path, name and
// item, a value binding before a type-only one.
```

The set assigns the local names, so two imports never bind one
name in a file:

```go
// Bind imports a whole package and returns the name the file
// refers to it by: name where it is free, and otherwise name with
// the lowest free numeric suffix. BindItem imports one declaration
// of a path under its own name the same way. Claim imports one
// declaration under its simple name and reports false where
// another binding takes it, so the language writes the qualified
// name instead. Reserve takes the names the file declares before
// the first declaration renders, so no import binds one of them.
// Every method leaves the file's own package out of the set, and
// its declarations spell unqualified.
```

The first claimant of a name keeps it, and the pass renders
declarations in canonical order, so two runs bind the same names.
A declaration the pass skips withdraws every entry and name it
recorded.

### The backend kit

The author supplies what the language varies in; the kit owns the
procedure and lowers to the SPI, the same boundary the plugin builder
holds:

```go
// New starts a backend declaration for one target.
func New(name plugin.ID, target plugin.Target, syntax plugin.CommentSyntax) *Builder

// FileTemplate sets the file skeleton; a kit default renders the
// package clause, the import block and the declarations in that
// order. The header is not the skeleton's: the output contract
// prepends it when it stamps, after the formatter ran.
func (b *Builder) FileTemplate(t string) *Builder

// KindTemplates declares how the language spells each emit kind,
// keyed by kind. Build refuses an empty set. Which kinds render
// standalone and which render inside their hosts is the language's
// own split.
func (b *Builder) KindTemplates(ts map[symbol.Kind]string) *Builder

// RefusedKinds declares the emit kinds the language cannot spell,
// each with its reason. The render skips a declaration of a refused
// kind under the refused-kind finding and states the reason. Build
// refuses a kind both spelt and refused, and a refusal without a
// reason. The conformance suite's every-kind check renders every
// file-level kind and fails on a kind the backend neither spells,
// lowers nor refuses.
func (b *Builder) RefusedKinds(rs map[symbol.Kind]string) *Builder

// Scaffold sets the language's statement printer: how each kind
// of the neutral scaffolding vocabulary spells, recording into
// the file's import set whatever it qualifies with. The body
// builtin calls it for slot contributions and scaffold content
// alike.
func (b *Builder) Scaffold(f func(s emit.Stmt, set *render.ImportSet) ([]byte, error)) *Builder

// Funcs registers the language's shared template vocabulary, once,
// into the overrideable bucket: a function returning the helpers
// bound to one file's import set, so a helper that spells a type
// records the import the spelling needs. The pass binds it once
// per worker.
func (b *Builder) Funcs(part func(set *render.ImportSet) template.FuncMap) *Builder

// Naming sets the target's filename spelling.
func (b *Builder) Naming(n Naming) *Builder

// Split sets the target's unit reshaping; undeclared, every unit
// files whole.
func (b *Builder) Split(s render.Split) *Builder

// Cluster sets the target's declaration clustering, and Groups
// the templates its group names select. A cluster without group
// templates, a group name declared twice and a group template
// that does not parse are defects at Build.
func (b *Builder) Cluster(c render.Cluster) *Builder
func (b *Builder) Groups(gs map[render.GroupName]string) *Builder

// Imports sets the renderer for a file's collected import set:
// grouping and sorting are language facts.
func (b *Builder) Imports(r func(set *render.ImportSet) string) *Builder

// Finalise sets the language formatter, run last per file. A
// failure is a positioned Error carrying the file, and the pass
// continues with the remaining files; the sink never receives an
// unformatted file.
func (b *Builder) Finalise(f func(src []byte) ([]byte, error)) *Builder

// Build freezes the declaration and returns the backend, which
// implements plugin.Backend and plugin.Renderer both. It panics
// on a declaration defect, the same rule the plugin builder
// follows: an empty name, a zero target, an empty kind-template
// set, a kind both spelt and refused, no naming, a template that
// does not parse.
func (b *Builder) Build() plugin.Backend
```

### The pass, in order

The kit's renderer runs the same pass for every language. Files
are independent after grouping, so the kit parallelises the
per-file loop; determinism survives because every ordering below
is defined and no file reads another.

```mermaid
sequenceDiagram
    participant C as caller
    participant R as kit renderer
    participant T as kind templates
    participant P as plugin template tree
    participant F as Finalise

    C->>R: Render(ctx)
    Note over R: group units into files,<br/>one Naming call per unit
    loop per file, in parallel
        R->>T: each declaration, in canonical order
        T-->>R: rendered declarations
        R->>T: each slot's contents, in insertion order
        T-->>R: rendered slot contents
        R->>P: resolve each TemplateRef in its emitter's tree
        P-->>R: the executed body, slot markers placed
        Note over R: collect the imports and render the block
        R->>F: the assembled file
        F-->>R: formatted bytes, or a positioned Error
    end
    R-->>C: files as values, findings on the sink
```

1. **Group.** Every unit passes the target's Split, and every
   unit that leaves it maps to one file through the target's
   Naming; units sharing a name merge into one file in unit
   order, and a name collision across plans is the output
   contract's to refuse, because only it sees every plan.
2. **Render declarations** through the kind templates, in
   canonical order: origin identity, then the unit's declaration
   order. Declarations the target's Cluster gathers render
   together through their group template instead, at the position
   of the cluster's first member. A declaration of a kind the
   language declares refused reports under the refused-kind
   finding with the language's reason. Any other declaration
   without a template is the unspelt-kind finding. The pass skips
   both, and withholds a file whose every declaration it skipped,
   because the file has no content to stamp.
3. **Splice slots.** Slot contents render through the same kind
   machinery, in the order they were appended, and the pass adds
   no sort of its own, because a slot item carries no attribution
   to sort by. The ordering rule holds by construction: a
   contribution arrives only through a handler the run's bucket
   schedule already sequenced, so insertion order is the lowered
   order, priority, capability topology, then name.
4. **Resolve references.** A body whose form is a template claim
   resolves its name in the emitting plugin's tree for this
   target, never the backend's or another plugin's. The template must
   place the slot markers: a pending contribution into a body
   whose template dropped them is an Error naming the emitting
   plugin and counting what went unplaced, because a slot
   statement carries no attribution to name its contributor by.
5. **Collect imports.** Spelling a type feeds the file's one
   ImportSet as a side effect, and the Imports renderer groups and
   sorts the block the way the language's own formatter leaves it.
   Three constraints meet here: the funcmap registers once,
   templates parse once, and files render in parallel. The pass
   satisfies all three by cloning the parsed templates per worker
   and binding the spelling helpers to a worker-local file, which
   the loop swaps between files. That mechanism is contract, not an
   implementation detail: the clone count is bounded by the
   parallelism rather than the file count, and no two workers share
   a parse tree.
6. **Finalise.** The formatter runs per file. A failure is a
   positioned Error carrying the file it could not format, and the
   pass continues with the remaining files.

The pass ends at bytes. Stamping the header and trailer and
writing through a staged sink are the output contract's steps,
consumed by whoever holds both a renderer and a sink.

### Template lint, the static half

The lint is a kernel function beside the pass, and the conformance
suite is its caller. Every template of every plugin parses against
each declared target's merged funcmap. Three things are findings:
an undefined function, a colliding override, and a body template
without its slot markers.

Field references check wherever the bound type is known, which is
always for kind templates and for the emit-value half of body
templates, and never for a reference's payload. The verbatim form
is the one thing the lint cannot see into at all, which is the cost
verbatim carries.

### The conformance checks this opens

`backendtest` holds a renderer to what a valueless render can
prove, over a hand-built emit fixture:

```go
package backendtest

// Fixture is a hand-built plan as the renderer sees it: the emit
// store, the schedule, and the template trees, helpers and
// override declarations the composition would have handed over.
type Fixture struct {
    Emit      *plugin.Emit
    Schedule  []plugin.ID
    Trees     map[plugin.ID]fs.FS
    Funcs     map[plugin.ID]template.FuncMap
    Overrides map[plugin.ID][]string
}

// Setup builds the backend under test with the emit fixture it
// renders, fresh per call, the way the plugin suite's Setup does.
type Setup func(tb assert.TB) (plugin.Renderer, *Fixture)

// RunBackendSuite checks a renderer against what a render returns
// as values: two runs produce byte-identical files, every emit
// kind renders or reports its declared refusal, slot contents
// render through the kind machinery, and a format failure reports
// positioned and does not stop the remaining files.
func RunBackendSuite(t *testing.T, setup Setup)
```

The header and trailer checks join the suite with the output
contract, which owns their shape.

## Alternatives considered

### Rendering on the Backend interface

Growing `Backend` with a Render method puts the seam where the plan
already points. It lost because every composed backend would break
at compile time the day the method appears, and because carrying
and invoking are different capabilities: the composition validates
one, a run exercises the other, and two interfaces let a test fake
carry without rendering.

### A template method set instead of a pass

Letting each backend own its loop and giving it helpers was
weighed: it is how most template engines are consumed. It lost
because the loop is where the three rules in Motivation hold, and
the helpers are not. A backend owning the loop can splice a
another plugin's values through its own templates, resolve a reference in
the wrong tree, or stop at the first format failure, and nothing
but review would notice.

### Rendering through the walk instead of unit order

Driving the pass off the emit walk would reuse generated
machinery. It lost because a unit already carries its declarations
in canonical order while the walk's order is the tree's, and
because rendering produces files, which are unit-shaped.

### A richer file value

Answering staged files with modes, directories and overwrite
intents was weighed. It lost at this seam: the renderer knows bytes
and names, the sink knows disk, and every field added here is one
the output contract has to validate twice.

## Drawbacks

- Two more SPI values and one interface: `Renderer`,
  `RenderContext`, `RenderedFile`, plus `CommentSyntax` carried
  for the output contract's benefit before anything stamps with
  it.
- The kit is the second builder on the root package, and its
  surface is seven methods. The precedent is the plugin builder;
  the cost is a wider root package either way.
- `Schedule` is carried on the context as data, so a fixture can hand a
  render pass an order the composition would never produce. The
  suite renders what it is given; only the composed path
  guarantees the order is the lowered one.
- Slot order is unverifiable at render: an item carries no
  attribution, so the pass renders insertion order and trusts the
  run to have sequenced the appends. A hand-built fixture that
  appends out of schedule order renders that order, honestly.
- The service provider interface grows by a fourth surface here,
  `TemplateProvider`, and it drags fs.FS and text/template into
  the SPI's vocabulary: both stdlib, both now contract.
- The lint parses templates it never executes, so a template that
  parses and still misbehaves at execute time, wrong data shape
  above all, reports at render with a file and line rather than at
  lint.
- Per-file parallelism makes render findings arrive in file order
  only after the sink sorts them; the pass itself reports in
  completion order.
- Split and Cluster widen Language by two functions and a template
  map, all optional; a target that needs none carries nil fields.
  A group template spells its members itself, so the lint parses
  it against the vocabulary but cannot check member completeness
  the way it checks a kind template's bound type.
- Entries widen the import set's surface by one type and two
  reads. A renderer that binds no names keeps reading Paths, and a
  bare Add and a named AddNamed under one path stay two entries,
  which is what a side-effect import beside a named one means.

## Open questions

- The set's home is settled: it stays with the pass, and its
  entries carry the bound name beside the path, so a named import
  form renders from the set alone while the spelling helpers that
  fill it stay the lowering's.

## Unresolved and future work

- The builder facade declares template trees and nothing else: a
  plugin whose reference templates need helpers or an override
  declaration implements the provider directly. Widening the
  facade with the two sibling declarations is a follow-up no
  consumer has needed.
- The generated-file header, the provenance trailer, the staged
  sinks and the write path are the output contract's, proposed
  separately and consumed by whoever holds both halves.
- The lowering seam that spells types and feeds the ImportSet, and
  the Go backend built on this kit, are their own proposals.
- Claimed file templates, the per-output presentation path, wait
  on declared outputs meeting layout.

## References

- [07-rendering.md](../architecture/07-rendering.md), the verbs,
  the dispatch order, the funcmap and the engine choice
- [11-languages.md](../architecture/11-languages.md), the kit
  surfaces and the comment syntax
- [18-routing-and-layout.md](../architecture/18-routing-and-layout.md),
  filename derivation as a language fact
- [17-output-and-determinism.md](../architecture/17-output-and-determinism.md),
  the header, trailer and sink contracts this pass hands off to
- [13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md),
  the backend checks and the template lint
