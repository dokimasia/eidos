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
two declarations that route to one path under different packages are an
Error that names both.

## Motivation

### A generated file is written under its import path

The run routes a rendered file through `Plan.Layout`, a
`func(pkg symbol.Identity, name string) string` that it applies after the
render (`workspace/write.go:98`). A nil function joins the package path to
the filename. For Go the package path is the import path, so the stub of
`svc/store.go` in module `example.com/acme` is written to
`example.com/acme/svc/store_stub.go`, a directory tree named after the
module path. No layout policy places it beside its source.

The routing a source author writes is validated and then ignored.
Validation admits `out=` and `tag=` on every directive, and the kernel
`out` directive takes `path` and `tag`
(`directive/validate.go:312`, `directive/kernel.go:77`). The run reads
neither when it writes a file. Outside the directive package, the witness
annotator reads the two keys, and it reads them to skip them.

The render pass groups units by their origin package and their spelled
filename, and gives every file its origin package as the import home
(`backend/render/pass.go:220`, `:496`). A file written into another package
would spell its origin's declarations without a qualifier, and a Go file in
a `main` package declares the name its import path assumes, not `main`.

The filename spelling also disagrees with the specification. A
per-package family's file is the family word plus the extension, `suite.go`.
`naming.FilenameParts` puts the routing key's last segment in front of the
word for every key, and a per-package unit's key is its package path, so Go
spells `svc_suite.go` (`eidos-lang/naming/filename.go:17`).

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
| `plugin.File` | `core/plugin` | One routed output file: its path, its package and its units |
| `plugin.FileSpeller` | `core/plugin` | The target's filename half: how a unit splits into files and what each is named |
| `plugin.Packager`, `plugin.Placement`, `plugin.Module` | `core/plugin` | The target's package half: the package a file at a routed path declares |
| `RenderContext.Files` | `core/plugin`, `core/backend/render` | The render pass renders the files Layout composed and groups nothing |
| `Builder.Packages` | `core/backend` | The kit's hook for a target's package rule |
| Seven routing codes | `core/layout` | Positioned Errors with stable codes |
| Package rules | each backend satellite | Go, TypeScript, Java and Rust each state where a file's package comes from |

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
each plan's Layout reads only its own store and the frozen graph.

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
    PolicyInherit Policy = 0 // inherit
    // PolicyAlongside places a file in the directory of the source it
    // derives from.
    PolicyAlongside Policy = 1 // alongside-source
    // PolicyCentralised places a file under the configured output
    // directory, at its source package's workspace-relative directory.
    PolicyCentralised Policy = 2 // centralised
)

// Valid reports whether p is one of the three policies.
func (p Policy) Valid() bool

// Config is one plan's routing configuration. Build validates it: the
// policies are valid, every directory is workspace-relative and inside
// the workspace root, a centralised policy has a directory, and every
// refinement names a generator of the plan and a family that generator
// declares.
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

// Family names one declared output family: a generator and a tag,
// the empty tag naming the primary family.
type Family struct {
    Plugin plugin.ID
    Tag    string
}

// Refinement overrides a plan's routing for one generator or one
// family. A zero field inherits from the enclosing scope. Build
// refuses a Dir for a per-source or per-package family under an
// alongside policy, because the run writes such a file beside its
// source and reads no directory for it.
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
validates the configuration in the plans step beside the other plan faults.
A misspelled plugin or tag fails Build and never waits for a run. A
per-plan file has no source directory to be written beside, so Build
refuses a plan that declares a per-plan family and gives that family no
directory.

### The resolution pass

```go
// Input is one plan's routing input.
type Input struct {
    // Emit is the plan's settled store.
    Emit *plugin.Emit
    // Config is the plan's validated configuration.
    Config Config
    // Outputs are the families each generator of the plan declares.
    Outputs map[plugin.ID][]plugin.Output
    // Speller is the backend's filename half.
    Speller plugin.FileSpeller
    // Packager is the backend's package half, nil where the backend
    // declares none.
    Packager plugin.Packager
    // Index resolves origins: their source files, their packages and
    // their validated directives.
    Index *plugin.Index
    // Directives names the plugin that registered each directive's
    // schema, which scopes the reserved routing keys.
    Directives *directive.Registry
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
// Route reports routing problems to the sink and returns an error only
// for a defect in its inputs: a nil store, an unsettled store or a nil
// speller.
func Route(in Input) ([]plugin.File, error)
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
   package's. An author's path override resolves against the source
   directory. Otherwise a centralised policy writes under
   `path.Join(Dir, sourceDirectory)`, an alongside policy writes in the
   source directory, and a per-plan family writes in its configured
   `Dir`. A declaration with no source directory, such as one whose
   origin is in a dependency store, is `NoDestination`.
4. **Filename.** An author's override that names a file takes
   precedence, then the family's configured `File`, then the target's
   spelling, `FileSpeller.FileName` of the unit after
   `FileSpeller.SplitUnit`. A per-source file's name joins the key's stem,
   the family word and the tag. A per-package or per-plan file's name
   joins the word and the tag alone, so a per-package `suite` family in
   the test companion is `suite_test.go` under Go and `suite.test.ts`
   under TypeScript.
5. **Package.** Route calls the target's `Packager.PackageAt` once per
   file for the package a file at that path declares. Without a
   `Packager`, Route gives every file the package of its first unit,
   which is the package every file has today. A target that derives no
   package returns an error, and the file's package is the zero identity,
   which step 7 refuses only where a reference needs it.
6. **Grouping.** Declarations with one path form one file. Two files at
   one path with different packages, two paths that differ only in case,
   and a path that another path needs as a directory are each
   `PathCollision`, naming both declarations. The declarations that route
   to the colliding paths are refused, and the remaining files render.

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
reference within one package does not need a qualifier, so a centralised
file that nothing references renders with no finding.

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
directive take precedence, field by field.

A path override is relative to the source directory. A trailing slash
names a directory, and the target's spelling names the file. Without a
trailing slash, the last element is the filename. The joined path must be
inside the workspace root: an absolute path, or one whose `..` elements
leave the root, is `EscapingPath`. A filename override on a declaration
from which the plugin emits into more than one family is `AmbiguousOverride`,
because every family would be written to one file. A directory override, or
a filename override together with `tag`, scopes to one family and is never
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
    // Pkg is the package the file declares, with the name the target
    // writes in its package clause. The zero identity means the target
    // derives no package for the path.
    Pkg symbol.Identity
    // Units are the file's units in the store's order. A unit that
    // routing split keeps its plugin, family and key, and contains only
    // the declarations written in this file.
    Units []Unit
}

// FileSpeller is a target's filename half. Layout calls it once per
// unit, on one goroutine.
type FileSpeller interface {
    // SplitUnit reshapes one unit into the units the target writes as
    // separate files, such as one unit per public type. It returns the
    // unit whole where the target writes it as one file.
    SplitUnit(u Unit) []Unit
    // FileName spells one unit's filename from its cardinality, key,
    // family word and tag, in the target's case and join, with the
    // target's extension.
    FileName(u Unit) string
}

// Packager is a target's package half: the package a file at a routed
// path declares, which is the package the target language's frontend
// names when it reads that file.
type Packager interface {
    // PackageAt returns the package a file declares, with Name set to
    // the name its package clause writes. It returns an error naming
    // what is missing where the target derives no package for the path.
    PackageAt(p Placement) (symbol.Identity, error)
}

// Placement is what a target reads to name a routed file's package.
type Placement struct {
    // Path is the routed file, workspace-relative.
    Path string
    // Origin is the package of the file's first unit: the package its
    // declarations derive from.
    Origin symbol.Identity
    // Resident lists the packages the load placed in the file's
    // directory, each with its declared name, sorted by identity.
    Resident []symbol.Identity
    // Modules are the toolchain modules the load resolved, innermost
    // root first.
    Modules []Module
    // ImportBase and BaseDir are the plan's import base and the
    // directory it names, for a directory no module contains.
    ImportBase, BaseDir string
}

// Module is one toolchain module the load resolved, from the
// gen.module and gen.moduleRoot facts on its packages.
type Module struct {
    Lang symbol.Lang
    // Path is the module's identity: a Go module path, a Maven
    // artifact.
    Path string
    // Root is the workspace-relative directory the module is declared
    // in.
    Root string
}
```

The run builds `Modules` and the directory index behind `Resident` once,
from the frozen graph and the fact store, before the plans run. The kit
implements `FileSpeller` on every backend from its declared `Naming` and
`Split`, and implements `Packager` where the backend declares one through
`Builder.Packages`.

Each satellite states its rule in its own `spell/` package, from the rule
its frontend applies when it reads the file:

| Target | The package of a file at a routed path |
|---|---|
| Go | The package the directory's non-test files declare. In a directory with no package, the innermost module root that contains it, joined with the directory's path relative to that root. Otherwise `ImportBase` joined with the path relative to `BaseDir`. Otherwise none. A `_test.go` file declares the directory's package, so it compiles into the test binary and sees the package's unexported names |
| TypeScript | The file's own module path: its path without the extension |
| Java | The origin's package, because a Java package is declared by the file's clause and one package may be written under several source roots |
| Rust | The module the directory's files belong to, joined with the file's stem. In a directory with no module, none |

The Go rule matches the import path the Go frontend gives a directory
(`eidos-lang-go/frontend/partition.go:113`), and the backend shares that
derivation through the satellite's root package. A Go file written into a
`main` package declares `main`, because the clause name comes from the
resident package.

### The render renders files

`plugin.RenderContext` gains `Files []File`, and the render pass renders
those files. It no longer groups units and no longer calls `Naming` or
`Split`. For each file the pass sets the import home to the file's package
path, gives the skeleton the file's base name and package, and positions
its findings at the file's path. `plugin.RenderedFile.Name` becomes
`Path`, the routed path, so the stamp and the staging read the destination
from the rendered value.

The backend suite renders a hand-built emit store without a graph to route
against. It groups the fixture's units by package and spelled name, which is
the grouping the pass performs today. The pass then renders those files.

### Failure semantics

Every routing finding is an Error under the kernel prefix. It is positioned
at the origin of the declaration it concerns, or at the carrier line of the
directive that caused it. The render leaves a refused declaration out and
renders the rest of its file.

| Code | Name | Position | Meaning |
|---|---|---|---|
| EID-0048 | `UndeclaredFamily` | The declaration's origin | A unit's family is not among its plugin's declared families |
| EID-0049 | `UnknownTag` | The directive's carrier line | A tag override names no family the plugin declares |
| EID-0050 | `AmbiguousOverride` | The directive's carrier line | A filename override applies to more than one family of one plugin |
| EID-0051 | `NoDestination` | The declaration's origin | No directory resolves for the declaration |
| EID-0052 | `EscapingPath` | The directive's carrier line | A path override is absolute or leaves the workspace root |
| EID-0053 | `PathCollision` | The second declaration's origin, the first as related | Two files at one path declare different packages, two paths differ only in case, or one path is a directory of another |
| EID-0054 | `UnderivedPackage` | The referencing declaration's origin | A reference crosses into a file whose package the target derives no identity for |

A static contradiction in the configuration is a Build fault and never a
finding: a refinement naming a generator outside the plan or a tag the
generator does not declare, a `Dir` for a per-source or per-package family
under an alongside policy, a centralised policy without a directory, a
per-plan family without a directory, and an invalid policy.

### Cost

Route is linear in the plan's declarations. Each declaration costs one
directive lookup for its origin, which the index returns from a map, and
one map insertion into its file's group. Each file costs one `PackageAt`
call and one `FileName` call per unit. The allocation bound is one
allocation per file and one per split unit, with no allocation per
declaration on the path where no override applies. The implementation
profiles the pass and then pins that bound with a benchmark at the
canonical scale of 1,000 packages, 10 files per package and 20 declarations
per file.

### Migration

| Caller | Change |
|---|---|
| `workspace/steps.go` | `compiledPlan.layout` becomes a `layout.Config`, validated in the plans step |
| `workspace/write.go` | `layout` and `conventionalPath` are deleted. The run calls `layout.Route` after the settle and renders its files |
| `backend/render/pass.go` | The grouping loop is deleted, and the pass renders `ctx.Files` |
| `backend/backend.go` | The built backend implements `FileSpeller`, and `Builder.Packages` declares a `Packager` |
| `backend/backendtest` | The fixture groups its units into files before the render |
| `eidos-lang/naming/filename.go` | `FilenameParts` takes the cardinality and drops the stem for a per-package or per-plan unit |
| Four satellites' `spell/` | Each declares its package rule, and its `Naming` passes the cardinality |
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

A file whose name ends in `_test.go` declares the `<pkg>_test` package, so a
generated test exercises its package from outside. It lost because a
generated double in the package itself serves both kinds of test: the
package's own tests use it directly, and an external test package imports
the package's test build, which includes the double's exported names. A
consumer who wants the external package routes the file to a directory of
its own.

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
  fixture through the suite's grouping helper.
- `plugin.RenderedFile.Name` becomes `Path`. 9 files construct a
  `RenderedFile`, 7 of them tests.
- `plugin` gains two interfaces and three values. `layout` is a new package
  with seven codes, about 600 lines before its tests by estimate. Each of
  the four satellites adds a package rule, about 40 lines before its tests
  by estimate.
- Every per-package and per-plan filename changes: `svc_suite.go` becomes
  `suite.go`. A tree generated before the change keeps the old files until
  the sweep removes them.
- A reference into a file of another package gains a qualifier and an
  import, which changes the bytes of every file where a centralised or
  redirected declaration was referenced bare.
- The kernel `out` directive has no plugin scope, so an author cannot route
  one plugin's output from a declaration that plugin's bare rule matched
  without also routing every other plugin's output from it.
- Rust's package rule needs a module in the target directory, so a Rust
  file routed into an empty directory has no package, and every reference
  into it is refused.

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
- The Rust and TypeScript frontends stamp no `gen.module` or
  `gen.moduleRoot` facts, so `Modules` lists no crate or package manifest
  for them.

## References

| What | Where |
|---|---|
| Routing and layout in the specification | [18-routing-and-layout.md](../architecture/18-routing-and-layout.md) |
| D27, routing by policy, refinements and overrides | [21-decisions.md](../architecture/21-decisions.md) |
| D46 and D50, families, cardinalities and tag spellings | [21-decisions.md](../architecture/21-decisions.md) |
| D69, package identity under centralised layout | [21-decisions.md](../architecture/21-decisions.md) |
| D73, the neutral module facts | [21-decisions.md](../architecture/21-decisions.md) |
| Go Modules Reference, "Modules, packages, and versions": a package path is the module path joined with the subdirectory containing the package, relative to the module root | https://go.dev/ref/mod |
| The Java Language Specification, section 7.4: a package declaration indicates the package a compilation unit belongs to | https://docs.oracle.com/javase/specs/jls/se21/html/jls-7.html#jls-7.4 |
| The Rust Reference, module source filenames: the path to a module's file mirrors its logical module path | https://doc.rust-lang.org/reference/items/modules.html |
