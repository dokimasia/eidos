---
rfc: 0082
title: The declaration files of the TypeScript library
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-20
updated: 2026-10-20
discussion: https://github.com/dokimasia/eidos/issues/48
supersedes: none
superseded-by: none
produces-adr: ADR-0084, ADR-0085, ADR-0086, ADR-0087
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0082: The declaration files of the TypeScript library

## Summary

We propose that the TypeScript frontend embeds the declaration files of the library of TypeScript
7.0.2. Each project of a workspace loads the library that its tsconfig selects as one dependency
unit. A global name of a source file, such as `Error`, then resolves to the declaration of the
library. The member walk of an interface that extends `Error` lists `name`, `message`, `stack` and
`cause`. The walk also lists a member with a computed name, such as `[Symbol.asyncDispose]`, so the
walk of `AsyncDisposable` is complete. A rule of the load that keeps two overloads apart by the
bounds of their type parameters completes the change.

## Motivation

- The frontend claims the files of the tree outside `node_modules`. No unit declares the library,
  so a global name such as `Error` has no target. The walk of an interface that extends `Error` or
  `AsyncDisposable` reports a gap, and a generator that needs the whole member set declines the
  interface. The reproduction of the issue logs `map[Disposed:false Failure:false Local:true]`.
- The Go frontend reads the standard library of Go from a store, and the Python frontend reads the
  standard library of Python from the embedded stubs of typeshed. A Go interface that embeds
  `io.Closer` has the method `Close` in its member set.
- The walk leaves out a member whose name is computed, under the gap reason `GapComputed`. The one
  member of `AsyncDisposable` is `[Symbol.asyncDispose](): PromiseLike<void>`, so its walk stays
  incomplete after the library loads.
- The DOM declares overloads that differ only in the bound of a type parameter, such as
  `createElement<K extends keyof HTMLElementTagNameMap>(tagName: K)` and
  `createElement<K extends keyof HTMLElementDeprecatedTagNameMap>(tagName: K)`. The discriminator
  of both spells `(K,ElementCreationOptions?)`, so the load reports the second as a duplicate and
  drops it. A probe that loaded the default library reported 17 such duplicates.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| TypeScript root | `eidos-lang-typescript` | `LibraryStore`, `Library` and `Global` name the store, a package of the library and a global reference |
| TypeScript library | `eidos-lang-typescript/frontend/library` | embeds the 108 declaration files of TypeScript 7.0.2 with its licence and notices |
| TypeScript frontend | `eidos-lang-typescript/frontend` | selects the library of each project, declares it as a shared input, places it in a dependency round, lowers its files, and resolves global names into it |
| TypeScript rules | `eidos-lang-typescript/rules` | classify a reference to a declaration of the library as they classify a global name today |
| TypeScript backend | `eidos-lang-typescript/backend` | writes a reference to a declaration of the library without an import |
| TypeScript grammar | `eidos-lang/treesitter/typescript` | pins the fork `dokimasia/tree-sitter-typescript`, whose grammars parse every file of the library |
| Kernel | `eidos-core/rules` | the member walk lists a computed member, and `GapComputed` is removed |
| Kernel | `eidos-core/frontend/load` | the discriminator of two overloads that it does not tell apart spells the bounds of their type parameters |
| Kernel | `eidos-core/store` | `Graph.AddDependencyPackage` adds a package of a dependency round to a graph that `New` returns, and `Graph.Dependency` reports it |
| Kernel | `eidos-core/plugin/plugintest` | `Loaded` adds each dependency package of the load beside the packages that `Source.Packages` lists, through `Fixture.LoadDependencies` |
| SDK | `eidos-sdk` | the regenerated facade |

### The embedded library

```go
package library

// Release is the release of TypeScript whose library [Files] embeds.
const Release = "7.0.2"

// Dir is the directory of [Files] that contains the declaration files.
const Dir = "libs"

// Files is the library of TypeScript at [Release]: below [Dir], each
// declaration file of the release, as libs/lib.es2024.d.ts. Any number of
// goroutines may read the tree.
//
//go:embed libs/*.d.ts
var Files embed.FS
```

- The files come from `internal/bundled/libs` of microsoft/typescript-go at the tag
  `typescript/v7.0.2`. That directory has 108 files and 3,785,246 bytes, and each file is the file
  that the npm package `typescript@7.0.2` installs.
- Each file has the Apache-2.0 header of Microsoft. `LICENSE` and `NOTICE.txt` of the release go
  beside the files. The notices cover the W3C and WHATWG texts of the DOM library.
- A refresh copies the files of a newer release and changes `Release`.

### The selection of a project

The tsconfig chain of a file states its library through three options of `compilerOptions`. A
configuration replaces each option that it extends.

| Option | Library |
|---|---|
| `noLib: true` | none |
| `lib` | the file of each entry. An entry is a name of the library or a file name, as `ES2024` or `lib.es2024.d.ts`, without regard to case. An empty list selects no file. An entry with a name that TypeScript 7.0.2 does not know reports under `UnknownLibrary` and does not select a file |
| neither | the default library of `target`. ES2015 selects `lib.es6.d.ts`, ES2016 to ES2025 select `lib.es<year>.full.d.ts`, ESNext selects `lib.esnext.full.d.ts`, and ES5 selects `lib.d.ts`. An unset target is ES2025. A target with a name that TypeScript 7.0.2 does not know reports under `UnknownLibrary` and is ES2025 |

`UnknownLibrary` is a new warning of the frontend, TYPESCRIPT-0009, at the configuration or at the
directive with the unknown name.

A file without a tsconfig has the default library of ES2025, which tsc 7.0.2 compiles such a file
with. A chain that does not read whole selects by the options of the configurations that read. The
table of names and the table of targets are the `LibMap` and the `targetToLibMap` of typescript-go
7.0.2.

- The files of a library are its root files and every file that a `/// <reference lib="…" />`
  directive at the head of a member names, followed to the end. The head of a file is its comments
  before its first token. The frontend reads a directive as typescript-go reads it: a `///` comment
  with the tag `reference` and its arguments in single or double quotes.
- The members keep the order in which typescript-go loads library files: `lib.d.ts` and
  `lib.es6.d.ts` first, then each file in the order of the names of the library, then every other
  file, each group sorted by file name. A declaration in more than one file merges in that order.
- The key of a library is its root files, sorted and joined with `+`, as
  `lib.es2024.d.ts+lib.esnext.disposable.d.ts`. The package of its declarations is the qualified
  path `typescript-lib://<key>`, which no workspace path can be. Two projects with one key share
  one unit.

A source file can state its own directives at its head:

- `/// <reference lib="dom" />` adds the files of the library `dom` to the library of the file. The
  file then has the key of the extended library, and the file imports its package, so the
  dependency round places it. TypeScript adds such a library to the whole program, and a unit of
  the frontend is one file, so the directive applies to its own file.
- Under `noLib`, a directive does not add a library, as in TypeScript. A file whose `lib` is an
  empty list has no library, and its directives do not add one. TypeScript adds them.
- A directive with a name that TypeScript 7.0.2 does not know reports under `UnknownLibrary`, and
  it does not add a library.
- tsc 7.0.2 ignores `/// <reference no-default-lib="true" />`, and so does the frontend.

### The store and the dependency round

```go
package typescript

// LibraryStore is the name of the store of the declaration files of the
// TypeScript library. A path inside the store is <key>/<file>, the file of
// one library of a project, as lib.es2024.d.ts/lib.es5.d.ts.
const LibraryStore = "typescript-lib"

// Library reports whether pkg is the package of a library of TypeScript:
// the qualified path of a key inside [LibraryStore]. It allocates nothing.
func Library(pkg string) bool

// Global reports whether a reference with the target target and the package
// pkg refers to a declaration of the global scope of TypeScript. That is a
// reference whose target and package are empty, or a reference to a
// declaration of a library. A declaration in the global package of the
// workspace has a target, and Global reports false for a reference to it.
// It allocates nothing.
func Global(target symbol.Identity, pkg string) bool
```

```go
package frontend

// Stores returns the stores that the load passes to the TypeScript
// frontend: the embedded library under [typescript.LibraryStore]. The store
// serves the file of every library at <key>/<file>, and reads no
// environment.
//
// Error modes: Stores returns no error.
func Stores(getenv func(key string) string) (map[string]fs.FS, error)
```

- The partition reads the tsconfig chain of each file already. It probes the store for the first
  root file of the library of the file, as
  `typescript-lib://lib.es2025.full.d.ts/lib.es2025.full.d.ts`, and adds that path to the shared
  inputs of the file where the store has it. The read of the probe folds into the key of every
  unit of the partition. The parse of a source file reads the key of its library from that
  shared input.
- The frontend implements the dependent role. Its first round places one unit for each library key
  among the shared inputs of the round and among its needs. Those needs are the packages of the
  libraries that a source directive extends. The round reads the directives of each member
  through its recorded door, so the reads fold into the key of the unit.
- The round does not place or report a need that is a module specifier. A relative or mapped
  specifier refers to a file of the workspace. A package specifier refers to a package of
  `node_modules`, which the load does not read.
- A load without the store has no library. The partition does not add a shared input, and the
  parse does not add a global tier or an import. The round does not place a unit. The tests of the
  frontend pass no store unless they test the library. The command line locates the store through
  `Stores`.

### The lowering of a library

- A member of a library unit declares its script declarations and its `declare global` blocks into
  the package of its library. `lib.es2025.iterator.d.ts` is the one module of the release. Its
  declarations that it does not export load into the package of its own path, and the signature
  depth leaves them out.
- The frontend merges the interfaces of one name in one package across the files of one unit, in
  the order of the members. Before, it merged them inside one file. A source unit has one file, so
  its merge is unchanged.
- A merged method whose name, type parameters, parameters and results equal those of a method
  already in the interface does not add a member. `lib.dom.d.ts` declares `SubtleCrypto` twice
  with five equal overloads.
- A merged property of a name that the interface already has does not add a member either,
  because TypeScript requires both declarations of a property to have one type.
  `lib.es2021.intl.d.ts` declares four properties of `Intl.DateTimeFormatOptions` again.
- The frontend does not report a syntax error of a library member, because the author of the
  workspace cannot change the library. The grammar of the fork `dokimasia/tree-sitter-typescript`
  parses every file of the release, `[K in keyof readonly any[]]?: boolean` in
  `lib.es2015.symbol.wellknown.d.ts` included. A spec of the satellite requires every file of the
  embedded library to parse without a syntax error, so a refresh with a construct that the grammar
  does not parse fails the spec.

### Resolution

The global tiers of a source file are the global package of the workspace and then the package of
its library. The global tier of a library member is the package of its library alone, so a script
of the workspace cannot hide a declaration that the library references.

- A bare name probes the enclosing namespaces, the package of the file, its imports, and then the
  global tiers.
- A qualified name `Intl.DateTimeFormat` probes the namespace `Intl` in each global tier.
- `globalThis.X` probes `X` in the global tiers alone.

A global declaration of the workspace and a declaration of the library of one name do not merge.
The workspace declaration takes the name in the files of the workspace, as the first of two
declarations of one name in two files of the workspace takes it.

### The rules

The rules classify a reference to a declaration of the library as they classified the global name
before:

- `Builtin` classifies a reference for which `typescript.Global` reports true. `Date` becomes the
  well-known timestamp, `Array<T>` a mutable list and `AsyncIterable<T>` a stream, as before.
- The rules implement `ShapeRules`. `ShapeOf` returns the shape of `Builtin` for a declaration of
  the library, so a library type that the table does not classify folds to the opaque shape, as it
  did without a target.
- `ParamRole` reads the library's `AbortSignal` as the context. `ErrorContract` and `Constructors`
  read the library's `Error` and the native error classes as the global error classes.
- The resolution of the rules, which `LiteralFor` reads, is unchanged. A global name in an authored
  value keeps its text.

### The backend

The backend writes a reference whose target is a declaration of the library by its spelling,
without an import, as it writes a global name without a target. A value that refers to a
declaration of the library does not import it either, and neither does a call of a library
function. A declaration of the file that hides a global type still refuses the reference.

### The member walk

The walk lists a field or a method with a computed name as a member, under its name as written,
such as `[Symbol.asyncDispose]`. The walk of `AsyncDisposable` then has one member and no gap. Two
arrivals of one computed name merge or shadow each other as two arrivals of any name do.

```go
package rules

const (
    GapUnresolved GapReason = iota + 1
    GapNotMembered
    GapCyclic
    GapGeneric
    GapConflict
)
```

A generator that writes a language without computed names reads `Computed` on the member's field
or method. The backends of Go, Java, Rust, protobuf and GraphQL already reject a declaration with
the fact, with an Error at the declaration. The Python backend refuses the fact with a warning.

### The discriminator of overloads

The load spells the discriminator of a callable from the types of its parameters. Two overloads of
one name in one scope can share that spelling and still be two overloads, because their type
parameters have other bounds. The load then spells the type parameters of each such overload in
brackets before its parameters, each with `:` before each bound and `=` before its default. The
first overload of `createElement` becomes
`createElement([K:keyof HTMLElementTagNameMap]K,ElementCreationOptions?)`. Every other callable
keeps its discriminator. Overloads whose type parameters are equal too remain a duplicate.

### The fixture of a plugin

A plan admits each dependency package of its language, so the walk of a plugin visits the library
in a run. The fixture of a spec of the plugin contains the same packages:

- `plugintest.Loaded` adds each package of a dependency round. It adds them also where
  `Source.Packages` lists the packages of the tree. A fixture whose source lists its packages and
  contains the store of the library folds `Map<string, string>` to the map form. Its walk of an
  interface that extends `Error` is complete.
- `Fixture.LoadDependencies` adds such a package through `store.Graph.AddDependencyPackage`.
  `Graph.Dependency` then reports the package in a graph that `store.New` returns. A rule of the
  generate phase does not run on a declaration of the package, and an annotator does. A run treats
  the package the same way.

```go
package store

// AddDependencyPackage adds a parsed package of a dependency round in the
// same way as [Graph.AddPackage]. After [Graph.Freeze], [Graph.Dependency]
// reports true for the package and each of its declarations.
//
// Error modes: the errors of [Graph.AddPackage].
func (g *Graph) AddDependencyPackage(p *node.Package) error
```

```go
package plugintest

// LoadDependencies adds parsed packages of a dependency round to the graph.
// A rule of the generate phase does not run on their declarations, and an
// annotator and a rule of the emit phase do.
func (f *Fixture) LoadDependencies(tb assert.TB, pkgs ...*node.Package) *Fixture
```

### Cost

A probe loaded the files of a library as units of the workspace, one unit for each file, on four
cores.

| Library | Files | Bytes | Load | Allocated | Kept |
|---|---|---|---|---|---|
| `lib.es5.d.ts` | 3 | 233,387 | 26 ms | 6 MiB | 3 MiB |
| `lib.es2024.d.ts` and `lib.esnext.disposable.d.ts` | 70 | 534,760 | 45 ms | 16 MiB | 8 MiB |
| `lib.es2025.full.d.ts`, the default | 82 | 2,937,099 | 309 ms | 80 MiB | 42 MiB |
| `lib.esnext.full.d.ts` | 93 | 2,984,092 | 313 ms | 82 MiB | 44 MiB |

- The DOM is 2,349,483 bytes of the default library. `lib.es2025.d.ts` without the DOM has 76 files
  and 574,301 bytes, a fifth of the default.
- A library unit parses its files on one worker of the load. The frontend parses the trees of the
  members of one unit concurrently and lowers them in order, so the parse of a library uses the
  cores of the load.
- The default library loads as one unit in 362 to 368 ms on four cores. The unit allocates 84 MiB
  and keeps 44 MiB, and its load does not report a finding.
- A cold run of the binary over a tree of one TypeScript file takes 0.46 to 0.51 s and peaks at 152
  to 163 MB under the default library. Under `noLib` it takes 0.05 to 0.06 s and peaks at 25 MB. A
  warm run with the default library takes 0.04 s and peaks at 23 MB.
- A warm load keeps or restores a library unit whose key is unchanged, as it does every unit.
- The binaries that import the frontend grow by the 3.8 MB of the files.

### Failures

| Failure | Effect |
|---|---|
| A tsconfig whose `lib` or `target` has a name that TypeScript 7.0.2 does not know | an `UnknownLibrary` warning at the configuration, and the entry selects no file or the target is ES2025 |
| A source directive with a name that TypeScript 7.0.2 does not know | an `UnknownLibrary` warning at the directive, and the directive does not add a library |
| A load without the library store | no library and no finding |
| A syntax error of a library member | no finding, and the declarations that the parser recovered load |
| Two overloads with equal parameters and equal type parameters | `DuplicateDeclaration` at the second, as before |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The library of each project, from the embedded release | in | each project of a workspace sees the globals that its tsconfig selects, and every machine loads one library | none known |
| The library and the defaults of TypeScript 7.0.2 for every project, whatever release the project installs | in | the release that a project installs is in `node_modules`, which the load does not read, and 7.0.2 is the latest release | the declaration files below `node_modules` |
| The default library for a file without a tsconfig | in | tsc 7.0.2 compiles such a file with `lib.es2025.full.d.ts` | none known |
| A reference directive of a source file | in, for its own file | a unit is one file, and its key cannot fold the directives of the other files of a program | a unit of a whole project |
| The declaration files below `node_modules` | out | the load reads no `node_modules`, and a package needs the resolution of Node with its exports, its typesVersions and the `@types` packages | an issue or a corpus that extends a type of an installed package |
| `libReplacement` | out | it replaces a library with an `@typescript/lib-*` package of `node_modules` | the declaration files below `node_modules` |
| A merge of a global declaration of the workspace with the library | out | the model has no relation for a declaration that two units declare, which merges across the files of the workspace lack too | a design of declaration merges across units |
| A computed member in the walk | in | a generator of TypeScript implements `[Symbol.asyncDispose]` and `[Symbol.iterator]`, and the backends of the other languages reject or refuse the fact already | none known |
| The bounds in the discriminator of overloads that collide | in | the overloads are distinct in TypeScript and in Java, and every other identity keeps its spelling | none known |
| A syntax error of a library member | no finding | the author of the workspace cannot change the library, and the spec of the satellite requires the library to parse without an error | none known |
| The grammar of the fork `dokimasia/tree-sitter-typescript` | in | the upstream grammar does not parse `keyof readonly any[]` of the library, and it reads `readonly` before a union as the `readonly` of the union | a release of the upstream grammar that parses both |
| The dependency packages in the fixture of a plugin that lists its packages | in | a plan admits them, so a spec sees the fold and the walk of a run | none known |

## Alternatives

### The library in the global package of the workspace

The library would declare into the package with the empty path, which a script of the workspace
declares into. The load keeps the first of two declarations of one identity, so a script that
declares `Event` would hide the library's `Event` or be hidden by it, with a warning. Two projects
with two libraries could not share one package.

### One unit for each file of the library

Each file would be a unit, as the probe loaded it. TypeScript merges an interface across the files
of the library, as `Array` across 8 files, and the frontend merges declarations inside one unit.
For the default library, the probe reported 219 duplicate declarations of the 88 interfaces that
the library declares in more than one file.

### One library for the whole workspace

The union of the libraries of every project would load once. A workspace with a Node project and a
browser project would resolve the names of the DOM in the Node project, and the member set of
`Array` would have the members of the newest library in an ES5 project.

### The library as an import of every file

Each file would import the package of its library, and the round would place every need of the
library prefix. Every file of every TypeScript load would gain an import that its source does not
state, and the graph of every TypeScript test would change. The design takes the import for a file
with its own directive alone.

### The round reads the tsconfig files of its shared inputs

The round would compute a library for each tsconfig among its shared inputs. The shared inputs
list every configuration of every chain without the files that each governs. A base that only
other configurations extend would get a library unit of its own, and a file without a tsconfig
would get none.

### A table of the members of the library in the rules

The rules would complete the walk of a global name from a table of members. The default library
declares 1,663 interfaces and more than 10,000 fields and methods, and a table would duplicate
each.

### A policy that lists computed members for TypeScript alone

The member policy would gain a field that lists computed members. Only the TypeScript frontend sets
`Computed`, so the field would have one value in every language, and the gap would remain for a
reader that no language serves.

### The bounds in every discriminator

The load would spell the type parameters of every generic callable. The identity of each generic
callable of TypeScript, Python and Java would change, also where no other overload collides with it.

## Drawbacks

- The binaries that import the TypeScript frontend embed 3.8 MB of declaration files.
- A cold load of a project with the DOM parses 2.9 MB more and keeps about 42 MiB more of graph. A
  workspace with two libraries parses the files that they share twice.
- The view contains the declarations of the library. A query over every interface of the view
  meets the 1,663 interfaces of the default library.
- Every unit of a TypeScript load with the store has one more shared input, so every unit key
  changes once.
- Every module specifier of a TypeScript load becomes a need of the first dependency round, which
  the round ignores.
- A global declaration of the workspace with the name of a library interface hides the library
  interface in the files of the workspace. Its walk lists the members of the workspace declaration
  alone.
- A reference directive of a source file applies to its own file, where TypeScript applies it to
  the program.
- `GapComputed` is removed, which breaks the kernel and the SDK.
- An annotator in a fixture with the store of the library visits the declarations of the library.
  A source that lists its packages does not change that, and a run visits them too.
- A refresh of the library also refreshes the two tables of typescript-go. A construct of the new
  release that the grammar does not parse needs a change of the fork.
- The fork of the grammar takes each change of upstream by hand.

## Open questions

None.

## Unresolved and future work

- The declaration files below `node_modules`, through the same store kind and the dependent role.
- A merge of declarations across units, which a global augmentation of the library and a merge
  across the files of the workspace both need.

## References

| What | Where |
|---|---|
| Issue 48, the members of an interface that extends the library | <https://github.com/dokimasia/eidos/issues/48> |
| Issue 52, the dependency packages of the fixture of a plugin | <https://github.com/dokimasia/eidos/issues/52> |
| typescript-go 7.0.2, the bundled library | <https://github.com/microsoft/typescript-go/tree/typescript/v7.0.2/internal/bundled/libs> |
| typescript-go 7.0.2, the names of the library | <https://github.com/microsoft/typescript-go/blob/typescript/v7.0.2/internal/tsoptions/enummaps.go> |
| typescript-go 7.0.2, the load of library files | <https://github.com/microsoft/typescript-go/blob/typescript/v7.0.2/internal/compiler/fileloader.go> |
| The TypeScript handbook, triple-slash directives | <https://www.typescriptlang.org/docs/handbook/triple-slash-directives.html> |
| The fork of the grammar | <https://github.com/dokimasia/tree-sitter-typescript> |
