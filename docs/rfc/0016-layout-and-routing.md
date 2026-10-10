---
rfc: 0016
title: Layout, the routing of generated declarations to files
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-10-01
updated: 2026-10-01
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0016: Layout, the routing of generated declarations to files

## Summary

Each plan gets a Layout phase between the settle and the render. Layout
resolves every emitted declaration to a file path and to the package that
file declares. It reads four inputs in fixed precedence: the families the
plugin declares, the plan's layout policy, the plan's configured
refinements, and the overrides the author wrote on the declaration. The
target spells each filename and names each file's package through two
backend surfaces. The render pass renders the files Layout composed, so
every path, package and import home is fixed before a template runs. A
routing failure is a positioned Error under one of seven kernel codes, and
declarations of two packages routed to one path are an Error that names
both.

## Motivation

### A generated file was written under its import path

The run routed each rendered file through `Plan.Layout`, a
`func(pkg symbol.Identity, name string) string` that it applied after the
render. A nil function joined the package path to the filename. For Go the
package path is the import path, so the stub of `svc/store.go` in module
`example.com/acme` was written to `example.com/acme/svc/store_stub.go`, a
directory tree named after the module path. No layout policy placed it
beside its source.

The routing a source author wrote was validated and then ignored.
Validation admitted `out=` and `tag=` on every directive, and the kernel
`out` directive took `path` and `tag`. The run read neither when it wrote a
file. Outside the directive package, the witness annotator read the two
keys, and it read them to skip them.

The render pass grouped units by their origin package and their spelled
filename, and gave every file its origin package as the import home. A file
written into another package would have spelt its origin's declarations
without a qualifier, and a Go file in a `main` package declared the name
its import path assumes, not `main`.

The filename spelling also disagreed with the specification. A
per-package family's file is the family word plus the extension, `suite.go`.
`naming.FilenameParts` put the routing key's last segment in front of the
word for every key, and a per-package unit's key is its package path, so Go
spelt `svc_suite.go`.

### Routing in the kernel, spelling in the target

- One file merges units from more than one plugin, and the merge is legal
  only when every unit agrees on the path and the package. That check needs
  every plugin's routes at once, which only the plan has.
- Routing reads the plan's configuration and the authors' directives. Both
  are composition data the kernel validates, and no backend sees either.
- The manifest, the sweep and the export each record where a file was
  written. They need the route before the render and independent of the
  target.
- What varies by language is small and already declared in two places: how
  a filename is spelled, and which package a file at a path declares. The
  backend keeps both.

## Detailed design

### Components

| Component | Package | Responsibility |
|---|---|---|
| `layout.Config`, `layout.Refinement`, `layout.Policy` | `core/layout` | One plan's routing configuration, validated at Build |
| `layout.Route` | `core/layout` | The resolution pass: family, key, directory, filename, package, collisions and references, per declaration |
| `layout.Residents`, `layout.Modules` | `core/layout` | The source tree a package rule reads, derived once per run |
| `plugin.File` | `core/plugin` | One routed output file: its path, its package and its units |
| `plugin.FileSpeller`, `plugin.Unit.FileKey` | `core/plugin` | The target's filename half, and the key a filename takes its stem from |
| `plugin.Packager`, `plugin.Placement`, `plugin.Resident`, `plugin.Module` | `core/plugin` | The target's package half: the package a file at a routed path declares |
| `RenderContext.Files` | `core/plugin`, `core/backend/render` | The render pass renders the files Layout composed and groups nothing |
| `Builder.Packages` | `core/backend` | The kit's hook for a target's package rule |
| `pathset.Set` | `core/internal/pathset` | The clash rule the layout reports by and every output sink stages by |
| Seven routing codes | `core/layout` | Positioned Errors with stable codes |
| Package rules | each backend satellite | Go, TypeScript, Java and Rust each state where a file's package comes from |
| Crate facts | `lang/rust/frontend` | Every package of a crate takes `gen.module` and `gen.moduleRoot`, so `Modules` lists the crates |

### Where Layout runs

```mermaid
flowchart LR
    G[Generate] -->|units| S[Settle]
    S -->|units in the target's names and shapes| L[Layout]
    L -->|files: path, package, units| R[Render]
    R -->|formatted bodies| T[Stamp]
    T -->|stamped bytes at routed paths| K[Stage]
```

Layout runs once per plan, after the settle, because the target's filename
rules read settled names: Java names a file after the public type it
declares. It runs before the render, because the render needs each file's
package to decide which references to qualify. Plans run in parallel, and
each plan's Layout reads only its own store, the frozen graph and the
source tree the run read once. A composition that declares no output stops
after the settle and routes nothing.

### The configuration

```go
// Package layout routes a settled plan's emitted declarations to the
// files they are written in.
package layout

// Policy is where a plan places a generated file by default.
type Policy uint8

const (
    // PolicyInherit takes the policy of the enclosing scope: a
    // refinement takes its plan's, and a plan takes PolicyAlongside.
    PolicyInherit Policy = 0
    // PolicyAlongside places a file in the directory of the source it
    // derives from.
    PolicyAlongside Policy = 1
    // PolicyCentralised places a file under the configured output
    // directory, at its source package's workspace-relative directory.
    PolicyCentralised Policy = 2
)

// Valid reports whether p is one of the three policies.
func (p Policy) Valid() bool

// Config is one plan's routing configuration. The zero Config places
// every file beside its source.
type Config struct {
    // Policy is the plan's default placement.
    Policy Policy
    // Dir is the output directory, workspace-relative and
    // slash-separated: where a centralised policy writes, and where a
    // per-plan family's file is written.
    Dir string
    // ImportBase is the package path a target joins Dir's
    // subdirectories onto, for a directory no loaded module contains.
    ImportBase string
    // Plugins refines every family of one generator.
    Plugins map[plugin.ID]Refinement
    // Families refines one family of one generator, and takes
    // precedence over Plugins field by field.
    Families map[Family]Refinement
}

// Check validates the configuration against the families each
// generator of a plan declares, keyed by generator, and returns one
// fault per problem, each naming the plan. A generator that declares
// no family is a key with no families.
func (c Config) Check(plan string, outputs map[plugin.ID][]plugin.Output) []error

// Family names one declared output family: a generator and a tag,
// the empty tag naming the primary family.
type Family struct {
    Plugin plugin.ID
    Tag    string
}

// Refinement overrides a plan's routing for one generator or one
// family. A zero field inherits from the enclosing scope.
type Refinement struct {
    Policy Policy
    // Dir is the output directory a centralised family writes under,
    // and the directory a per-plan family's file is written in.
    Dir string
    // File is the filename every file of the family takes, in place of
    // the target's spelling.
    File string
}
```

`workspace.Plan.Layout` changes from a function to `layout.Config`. Build
calls `Config.Check` in the plans step, beside the other plan faults, with
the families each generator declares through `OutputProvider`. A misspelled
plugin or tag fails Build and never waits for a run. A per-plan file has no
source directory to be written beside, so Build refuses a plan that
declares a per-plan family and gives that family no directory.

### The resolution pass

```go
// Input is one plan's routing input.
type Input struct {
    // Emit is the plan's settled store.
    Emit *plugin.Emit
    // Config is the plan's configuration, which Config.Check accepted.
    Config Config
    // Outputs are the families each generator of the plan declares,
    // keyed by generator.
    Outputs map[plugin.ID][]plugin.Output
    // Speller is the backend's filename half.
    Speller plugin.FileSpeller
    // Packager is the backend's package half. A nil Packager gives
    // every file the package of its first unit.
    Packager plugin.Packager
    // Index resolves origins: their source files, their packages and
    // their validated directives.
    Index *plugin.Index
    // Directives names the plugin that registered each directive's
    // schema, which scopes the reserved routing keys. A nil registry
    // leaves the kernel out directive as the only override.
    Directives *directive.Registry
    // Residents are the source files the load placed in each directory,
    // keyed by the directory, each list sorted by file.
    Residents map[string][]plugin.Resident
    // Modules are the toolchain modules the load resolved, innermost
    // root first.
    Modules []plugin.Module
    // Sink takes the routing findings.
    Sink *diag.Sink
}

// Route resolves every declaration of one settled plan to the file it
// is written in, and returns the files sorted by path. Each file's
// units keep the store's unit order, and each unit keeps its
// declarations' order. A declaration that a finding refuses is absent
// from every file, and a file left with no declaration is absent too.
// Route reports routing problems to the sink and sets the package of
// every bare reference that crosses into a file of another package. It
// returns an error only for a defect in its inputs: a nil store, an
// unsettled store, a missing speller, index or sink, a speller whose
// parts do not contain the declarations of the unit it split, and a
// filename that is not one path element.
func Route(in Input) ([]plugin.File, error)

// Residents returns the source files of the workspace tree by
// directory, each with the package that declares it, Name set to the
// name its files declare it under. A file of a dependency store is
// absent.
func Residents(g *store.Graph) map[string][]plugin.Resident

// Modules returns the toolchain modules the load resolved, read off
// the module facts the frontends stamp on packages, innermost root
// first.
func Modules(g *store.Graph, facts *meta.Facts, k meta.KernelKeys) []plugin.Module
```

Route visits the declarations of every unit in the store's order and takes
six decisions per declaration. Each decision reads its inputs in precedence
order. The first input that states a value decides it.

1. **Family.** A declaration the emitter placed in its plugin's primary
   family moves to the family that a tag override names. A declaration
   the emitter placed in a tagged family remains in it, because the
   handler addressed that family on purpose. A tag that the plugin does
   not declare is `UnknownTag`, and a unit whose family its plugin does
   not declare is `UndeclaredFamily`.
2. **Key.** The family's cardinality picks the key. A per-source key is
   the origin's source file, a per-package key the origin's package, and
   a per-plan key empty. A declaration without an origin keeps its unit's
   key. The file's stem and its set of declarations follow from the key,
   so a moved declaration joins the declarations of its new family under
   the same key.
3. **Directory.** The source directory is the directory of the key's
   source file for a per-source family, and the directory of the
   package's files for a per-package family. Where a package's files span
   more than one directory, the first directory in path order is the
   package's. An author's path override resolves against the directory of
   the origin's source file, and against the source directory where the
   origin has no workspace source. Otherwise a centralised policy writes
   under `path.Join(Dir, sourceDirectory)`, an alongside policy writes in
   the source directory, and a per-plan family writes in its configured
   `Dir`. A declaration with no source directory, such as one whose
   origin is in a dependency store, is `NoDestination`.
4. **Filename.** An author's override that names a file takes
   precedence, then the family's configured `File`, then the target's
   spelling, `FileSpeller.FileName` of the unit after
   `FileSpeller.SplitUnit`. The speller reads the stem through
   `Unit.FileKey`, which returns the source path of a per-source unit and
   the empty string otherwise. A per-source file's name joins the key's
   stem, the family word and the tag. A per-package or per-plan file's
   name joins the word and the tag alone, so a per-package `suite` family
   in the test companion is `suite_test.go` under Go and `suite.test.ts`
   under TypeScript. A target reads the unit's package as well where its
   language spells a package's files apart. Only a Go test file declares
   an external test package, so Go spells every file of one as a test
   file: the per-package `stub` of `store_test` is `stub_test.go`, beside
   the `stub.go` of `store`.
5. **Package.** Route calls the target's `Packager.PackageAt` once per
   file for the package a file at that path declares. Without a
   `Packager`, Route gives every file the package of its first unit,
   which is its origin's package. A target that derives no package
   returns an error, and the file's package is the zero identity, which
   step 7 refuses only where a reference needs it.
6. **Grouping.** Declarations with one path form one file. Declarations
   of two packages routed to one path, two paths that differ only in
   case, and a path that another path needs as a directory are each
   `PathCollision`, under the clash rule every output sink stages by. The
   finding is at the second file's first declaration, with the first
   file's as related. The declarations of both files are refused, and the
   remaining files render. A plan unit derives from no package and joins
   a file of any one package.

A seventh step runs over the composed files. The settle resolves a bare
reference to a declaration of the same plan by the referenced
declaration's package and settled name. After grouping, the referenced
declaration can be in a file of another package: a centralised file, a
file an author redirected, or a TypeScript or Rust file, whose package is
the file itself. Route then sets the reference's package to the package of
the file the referenced declaration is written in, so the target's speller
qualifies it and records the import. Where that package is the zero
identity, the reference is `UnderivedPackage` at the origin of the
referencing declaration, and the referencing declaration is refused. A
reference within one package does not need a qualifier, so the step runs
only where some file declares a package other than a unit it contains, and
a centralised file that nothing references renders with no finding.

### The author's overrides

An author routes one declaration's output in two spellings. Both resolve
to the same override.

| Spelling | Applies to | Example |
|---|---|---|
| `out=` and `tag=` on a plugin's own directive | That plugin's output from the declaration | `//+acme:stub tag=test` |
| The kernel `out` directive, with `path` and `tag` | Every plugin's output from the declaration | `//+acme:out path=mocks/` |

The reserved keys apply to the output of the plugin that registered the
directive's schema and to no other plugin. A declaration with two plugins'
directives routes each plugin's output by that plugin's own directive. Where
both spellings apply to one plugin, the reserved keys on that plugin's own
directive take precedence, field by field. Within one spelling the first
instance in source order takes precedence, and a negated directive routes
nothing.

A path override is relative to the directory of the origin's source file.
A trailing slash, a final `.` and a final `..` name a directory, and the
target's spelling names the file. Otherwise the last element is the
filename. The joined path must be inside the workspace root: an absolute
path, a path with a backslash, and one whose `..` elements leave the root
are `EscapingPath`. A filename override on a declaration from which the
plugin emits into more than one family is `AmbiguousOverride`, because
every family would be written to one file. A directory override, or a
filename override together with `tag`, scopes to one family and is never
ambiguous.

The precedence per decision, most specific first:

| Decision | Precedence |
|---|---|
| Family | `tag=` on the plugin's directive, the kernel `out tag=`, the family the handler addressed |
| Directory | `out=` on the plugin's directive, the kernel `out path=`, the family's refinement, the generator's refinement, the plan |
| Filename | `out=`, the kernel `out path=`, the family's `File`, the generator's `File`, the target's spelling |
| Policy | The family's refinement, the generator's refinement, the plan, alongside-source |

### The target's two surfaces

```go
// File is one output file a plan writes: where it is written, the
// package it declares, and the units it assembles, in render order.
type File struct {
    // Path is workspace-relative and slash-separated.
    Path string
    // Pkg is the package the file declares, with Name set to the name
    // the target writes in its package clause. The zero identity means
    // the target derives no package for the path.
    Pkg symbol.Identity
    // Units are the file's units in the store's order. A unit that
    // routing split keeps its plugin, family and key, and contains only
    // the declarations written in this file.
    Units []Unit
}

// FileSpeller is a target's filename half. The layout calls it on one
// goroutine.
type FileSpeller interface {
    // SplitUnit reshapes one unit into the units the target writes as
    // separate files, such as one unit per public type. It returns the
    // unit whole where the target writes it as one file. A unit with
    // declarations splits into at least one unit, and the split keeps
    // every declaration.
    SplitUnit(u Unit) []Unit
    // FileName spells one unit's filename from its cardinality, its
    // FileKey, its family word and its tag, in the target's case and
    // join, with the target's extension. A target reads the unit's
    // package too where its language spells a package's files apart, as
    // Go names every file of an external test package _test.go.
    FileName(u Unit) string
}

// FileKey returns the routing key a unit's filename takes its stem
// from: the source path of a per-source unit, and the empty string for
// a per-package or per-plan unit.
func (u Unit) FileKey() string

// Packager is a target's package half: the package a file at a routed
// path declares, which is the package the target language's frontend
// names when it reads that file.
type Packager interface {
    // PackageAt returns the package a file declares, with Name set to
    // the name its package clause writes. It returns an error naming
    // what is missing where the target derives no package for the path.
    PackageAt(p Placement) (symbol.Identity, error)
}

// PackageRule is a target's package rule in the form a backend kit
// declares it.
type PackageRule func(p Placement) (symbol.Identity, error)

// Resident is one source file the load placed in a directory, with
// the package the file declares. The package's Name is the name its
// files declare it under.
type Resident struct {
    File string
    Pkg  symbol.Identity
}

// Placement is what a target reads to name the package of a routed
// file. The run derives it from the frozen graph and the fact store,
// so a target reads no source.
type Placement struct {
    // Path is the routed file, workspace-relative.
    Path string
    // Origin is the package of the file's first unit: the package the
    // file's declarations derive from, zero for a plan file.
    Origin symbol.Identity
    // Residents are the source files the load placed in the routed
    // file's directory, each with the package it declares, sorted by
    // file.
    Residents []Resident
    // Modules are the toolchain modules the load resolved, innermost
    // root first.
    Modules []Module
    // ImportBase is the package path of BaseDir, the plan's output
    // directory, for a directory no loaded module contains. Both are
    // empty where the plan states no import base.
    ImportBase, BaseDir string
}

// ModuleOf returns the innermost module of a language whose root
// contains a directory, and false where no module of the language
// contains it.
func (p Placement) ModuleOf(lang symbol.Lang, dir string) (Module, bool)

// Module is one toolchain module the load resolved, from the
// gen.module and gen.moduleRoot facts on its packages.
type Module struct {
    Lang symbol.Lang
    // Path is the module's identity: a Go module path, a Maven
    // artifact.
    Path string
    // Root is the workspace-relative directory the module is declared
    // in, "." for the tree's root.
    Root string
}

// Contains reports whether a workspace-relative directory is the
// module's root or below it.
func (m Module) Contains(dir string) bool

// Rel returns a directory's path relative to the module's root.
func (m Module) Rel(dir string) string

// Packages declares the target's package rule.
func (b *Builder) Packages(r plugin.PackageRule) *Builder
```

The run derives the residents and the modules once, through
`layout.Residents` and `layout.Modules`, before the plans run. The kit
implements `FileSpeller` on every backend from its declared `Naming` and
`Split`. It implements `Packager` on every backend too: through the rule
`Builder.Packages` declares, and with the origin's package where the
backend declares none.

Each satellite states its rule in its own `spell/` package, from the rule
its frontend applies when it reads the file:

| Target | The package of a file at a routed path |
|---|---|
| Go | For a `_test.go` file, the origin's package where a file of the directory declares it, which is how a file of an external test package declares `store_test`. Otherwise the package the directory's non-test files declare, so any other test file compiles into the package's test binary and sees its unexported names. In a directory with no package, the innermost module root that contains it, joined with the directory's path relative to that root. Otherwise `ImportBase` joined with the path relative to `BaseDir`. Otherwise none |
| TypeScript | The file's own module path: its path without the extension |
| Java | The origin's package, because a Java package is declared by the file's clause and one package may be written under several source roots. A plan file derives none |
| Rust | The module the directory's files belong to, joined with the file's stem: the module of a `mod.rs` or a crate root in the directory, and otherwise the parent of a file module's path. A crate root is a file whose module is a Rust crate in `Modules`. In a directory without a Rust file, none |

The Go rule takes the import path from `golang.ImportPath`, the derivation
the Go frontend loads a directory's package under, so the two cannot
disagree. The Go filename and the Go rule read the external test package
through `golang.TestSuffix`, the suffix the frontend appends to the import
path it loads that package under. The TypeScript rule takes the module path
from `typescript.ModulePath`, the function the frontend names a loaded
module by. A Go file written into a `main` package declares `main`, because
the clause name comes from the resident package.

The Rust frontend stamps every package of a crate with `gen.module`, the
crate's name, and `gen.moduleRoot`, the directory of the crate's
`Cargo.toml`. A crate of its own, a file without a manifest, takes its own
directory. `Modules` then lists every crate, so the Rust rule tells the
root of a crate of its own, which is named for its path, from a file
module.

Every Go file opens with a package clause. The Go backend's `package`
template function returns an error for the zero identity, so the render
withholds a Go file whose package derives none under `RefusedTemplate`,
even when nothing references the file. The message states the remedy, an
import base in the plan's layout. TypeScript, Rust and Java render such a
file: TypeScript and Rust name a file's module by its path, and a Java
file without a clause is in the unnamed package.

### The render renders files

`plugin.RenderContext` gains `Files []File`, and the render pass renders
those files. It no longer groups units and no longer calls `Naming` or
`Split`: the layout calls them through `Pass.FileName` and
`Pass.SplitUnit`. For each file the pass sets the import home to the file's
package path, gives the skeleton the file's base name and package, and
positions its findings at the file's path. `plugin.RenderedFile.Name`
becomes `Path`, the routed path, so the stamp and the staging read the
destination from the rendered value.

The backend suite renders a hand-built emit store without a graph to route
against. `backendtest.Files` routes it the way a plan routes a store whose
packages are at their package paths: each unit splits through the target's
speller, each part takes its spelled name under its package's path, and
parts that share a package and a name assemble one file.

### Failure semantics

Every routing finding is an Error under the kernel prefix, reported under
the `layout` phase. It is positioned at the origin of the declaration it
concerns, or at the carrier line of the directive that caused it. The
render leaves a refused declaration out and renders the rest of its file.

| Code | Name | Position | Meaning |
|---|---|---|---|
| EID-0048 | `UndeclaredFamily` | The unit's first declaration's origin | A unit's family is not among its plugin's declared families |
| EID-0049 | `UnknownTag` | The directive's carrier line | A tag override names no family the plugin declares |
| EID-0050 | `AmbiguousOverride` | The directive's carrier line | A filename override applies to more than one family of one plugin |
| EID-0051 | `NoDestination` | The declaration's origin, or the carrier line of a path override | No directory resolves for the declaration |
| EID-0052 | `EscapingPath` | The directive's carrier line | A path override is absolute or leaves the workspace root |
| EID-0053 | `PathCollision` | The second file's first declaration, the first file's as related | Declarations of two packages route to one path, two paths differ only in case, or one path is a directory of another |
| EID-0054 | `UnderivedPackage` | The referencing declaration's origin | A reference crosses into a file whose package the target derives no identity for |

A static contradiction in the configuration is a Build fault and never a
finding: an invalid policy, a directory outside the workspace root, an
import base that is not a package path, a filename of more than one path
element, a refinement naming a generator outside the plan or a tag the
generator does not declare, a `Dir` for a per-source or per-package family
under an alongside policy, a generator's `Dir` that no family reads, a
centralised policy without a directory, and a per-plan family without a
directory.

### Cost

Route reads the overrides on each declaration's origin once, and visits a
unit's declarations again only where one of them has an override. A unit
without one joins its file whole, without a copy, so no declaration
allocates. `BenchmarkRoute` measures the canonical scale of 1,000
packages, 10 files per package and 20 declarations per file on 4 cores of
an AMD Ryzen 9 9950X3D. Routing 200,000 declarations into 10,000 files
takes 14 to 18 ms and 30,179 allocations: per file one path, the target's
filename and the target's split, 30,000 in all, and 179 for the pass's
tables. The benchmark pins the ceiling at 31,000.

The crate facts cost the Rust parse two stamps per package. Its benchmark
parses the same scale as 1,000 crates of 11 packages each, and measures
2,509,005 allocations against 2,500,005 without the facts, under a ceiling
of 2,600,000. Each crate converts its two values to fact values once.

### Migration

| Caller | Change |
|---|---|
| `workspace/steps.go` | `compiledPlan` keeps the plan's `layout.Config` and its generators' families, validated in the plans step. `kernelPhases` gains `layout` |
| `workspace/run.go`, `workspace/write.go` | `layout` and `conventionalPath` are deleted. The run derives the residents and the modules once, calls `layout.Route` after the settle and renders its files |
| `backend/render/pass.go` | The grouping loop is deleted, and the pass renders `ctx.Files` |
| `backend/backend.go` | The built backend implements `FileSpeller` and `Packager`, and `Builder.Packages` declares a rule |
| `backend/backendtest` | `Files` routes a fixture's units into files before the render |
| `output` | The sinks stage by the shared clash rule, which folds case for directories as well as files |
| Four satellites' `spell/` | Each passes `Unit.FileKey` to its filename spelling and declares its package rule. The four backends' versions bump |
| Go backend | `Package` returns an error for the zero identity |
| Rust frontend | Every package of a crate takes the two module facts, and the frontend's version bumps |
| `eidos-sdk` | The facade regenerates for the new exported names |

## Alternatives considered

### Route after the render

The run keeps applying a path function to each rendered file, extended with
policies and overrides. It changes the least code. It lost because a file's
package must be known while its declarations render: the import home
decides which references a target qualifies, and Go's package clause comes
from it. A per-declaration override also splits a unit before any file
exists, and a path function over rendered files sees whole files.

### Route inside the backend

Each backend's render pass reads the policy and the directives and places
its own files. It keeps routing next to the filename rules. It lost because
four satellites would repeat the precedence rules, the override scoping and
the collision rules, and the manifest and the sweep would read routes out
of four implementations. The language part of routing is two functions,
and the backend keeps exactly those.

### Derive a package only under the centralised policy

Under this rule, a file beside its source takes its origin's package, and
the target names the package of a centralised file. It lost on three
targets. TypeScript makes every file a module, so a stub beside `store.ts`
is a module of its own and imports from `store.ts`. Rust makes every file a
module as well. A Go file in a `main` package has to declare `main`, and no
import path spells that name.

### Reserved keys that route every plugin's output

`out=` on any directive routes every plugin's output from the declaration,
so a companion plugin follows the primary one without its own directive. It
lost because two directives on one declaration then route each other's
output, and the order the author wrote them in fixes where both go. The
kernel `out` directive is the spelling for routing every plugin's output.

### A package refinement in the configuration

A refinement names the package a family's files declare, beside its
directory and filename. It lost because the package of a path is a fact of
the target language: a Go package is its directory, a Java package is its
clause and a TypeScript package is its file. A name that disagrees with the
path compiles in Go and nowhere else, and misleads every importer.

### Generated Go test files in the external test package

Every generated `_test.go` file declares the `<pkg>_test` package, so a
generated test exercises its package from outside. It lost because a
generated double in the package itself serves both kinds of test: the
package's own tests use it directly, and an external test package imports
the package's test build, which includes the double's exported names. A
consumer who wants the external package routes the file to a directory of
its own. A file whose declarations derive from the external test package
declares that package, because its declarations reference that package's
own names.

### A package stem in the per-package filename

The per-package filename keeps the package's last path element as its
stem, as in `store_stub.go` and `store_test_stub.go`, so two packages of
one directory spell two files. It lost because the specification names a
per-package file by its word and tag alone, and because a Go file of an
external test package has to be a test file whatever its stem: a
`store_test_stub.go` that declares `package store_test` does not build
beside a `store.go` that declares `package store`.

### Refuse a file without a package in Route

Route refuses every file whose target derives no package, with
`UnderivedPackage` at the file's first declaration. It lost because whether
a file needs a package is a fact of the target language. A TypeScript or
Rust file is a module named by its path, and a Java file without a clause
is in the unnamed package, which compiles. Only the Go target requires a
clause in every file, and the Go backend refuses there.

### Read a crate root off its path

The Rust rule reads a resident as a crate root where its module path
equals its file's path without the extension, which is how the frontend
names a crate of its own. It lost because a crate whose root directory has
the crate's name, such as a crate `acme` rooted at `acme/lib.rs`, then
reads every file module beside its root as a crate root. The module facts
state which modules are crates.

### Clamp a path override inside its source directory

A `..` element in `out=` is dropped, so an override never leaves its
package's directory. It lost because the workspace root is the boundary the
sink enforces, and silently rewriting what an author wrote hides the
mistake. An override that leaves the root is an Error.

## Drawbacks

- `workspace.Plan.Layout` changes type. Its one production caller,
  `workspace/steps.go`, changes, and a composition that sets the field
  changes with it.
- `plugin.RenderContext` gains `Files` and the pass stops grouping. 14 files
  construct a `RenderContext`, 12 of them tests, and each test routes its
  fixture through `backendtest.Files`.
- `plugin.RenderedFile.Name` becomes `Path`. 9 files construct a
  `RenderedFile`, 7 of them tests.
- `plugin` gains two interfaces, four values and a function type. `layout`
  is a new package with seven codes, 1,515 lines before its tests. The four
  satellites' package rules are 24 to 77 lines each before their tests.
- Every per-package filename changes: `svc_suite.go` becomes `suite.go`. A
  tree generated before the change keeps the old files until the sweep
  removes them.
- A Go package's test companion and a family of its external test package
  that spell one test file, such as `suite_test.go`, route to one path and
  collide. The author moves one of them with a tag or a path override.
- Go reads an external test package off its import path, which ends in
  `_test`. A directory named `x_test` that contains an ordinary package loads
  under the same kind of path, and Go spells that package's generated files
  as test files.
- A reference into a file of another package gains a qualifier and an
  import, which changes the bytes of every file where a centralised or
  redirected declaration was referenced bare.
- The kernel `out` directive has no plugin scope, so an author cannot route
  one plugin's output from a declaration that plugin's bare rule matched
  without also routing every other plugin's output from it.
- Rust's package rule needs a module in the target directory, so a Rust
  file routed into an empty directory has no package, and every reference
  into it is refused.
- A Go file whose package derives none is withheld at the render under
  `RefusedTemplate`, positioned at the file, and not under a layout code at
  the declarations routed into it.
- The crate facts add 9,000 allocations to the Rust parse of the canonical
  corpus, 0.36% of its 2,509,005.

## Open questions

None.

## Unresolved and future work

- A `plugin=` key on the kernel `out` directive, which scopes a routing
  override to one plugin whose rules have no directive of their own.
- A routing directive on a package, as the default for every declaration
  of the package, is not proposed here.
- TypeScript references between generated files spell their module path as
  the import specifier. A specifier relative to the importing file needs the
  importing file's path, which `File.Path` supplies, and is not proposed
  here.
- The TypeScript frontend stamps no `gen.module` or `gen.moduleRoot` facts,
  so `Modules` does not list its package manifests.
- A name in a structured body that refers to a declaration routed into
  another package is not qualified. The seventh step qualifies type
  references alone.

## References

| What | Where |
|---|---|
| The code this proposal changes, as it was when proposed | commit `223f5d1` |
| Routing and layout in the specification | [18-routing-and-layout.md](../architecture/18-routing-and-layout.md) |
| D27, routing by policy, refinements and overrides | [21-decisions.md](../architecture/21-decisions.md) |
| D46 and D50, families, cardinalities and tag spellings | [21-decisions.md](../architecture/21-decisions.md) |
| D69, package identity under centralised layout | [21-decisions.md](../architecture/21-decisions.md) |
| D73, the neutral module facts | [21-decisions.md](../architecture/21-decisions.md) |
| Go Modules Reference, "Modules, packages, and versions": a package path is the module path joined with the subdirectory containing the package, relative to the module root | https://go.dev/ref/mod |
| The go command, "Test packages": a test file whose package clause names the `_test` suffix is compiled as a separate package | https://pkg.go.dev/cmd/go#hdr-Test_packages |
| The Java Language Specification, section 7.4: a package declaration indicates the package a compilation unit belongs to | https://docs.oracle.com/javase/specs/jls/se21/html/jls-7.html#jls-7.4 |
| The Rust Reference, module source filenames: the path to a module's file mirrors its logical module path | https://doc.rust-lang.org/reference/items/modules.html |
