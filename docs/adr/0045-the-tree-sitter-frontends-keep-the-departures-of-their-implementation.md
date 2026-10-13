---
adr: 0045
title: The tree-sitter frontends keep the departures of their implementation
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0015
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0045: The tree-sitter frontends keep the departures of their implementation

## Status

Accepted

## Context

RFC-0015 designs the tree-sitter layer, the frontends of TypeScript, Rust and Java, the dependency
stores of Go and Java, and the re-exports of TypeScript and Rust. Later accepted designs decide 12
places where the code departs from the design. This record keeps 28 other departures. Fifteen
departures are defects on input that the compilers or the go command accept. The decisions are
recorded here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from RFC-0015, and fix the 14 defects that the following
section lists. One defect remains, and the last section of the decision describes it.

### The fixes

| Design | Defect | Fix |
|---|---|---|
| A declare module block is the package of its name | A relative specifier, as in `declare module './user'`, declared into a package named `./user` | The block reports `RelativeAugmentation`, TYPESCRIPT-0008, and the frontend does not load it. The parse of one file cannot tell which file the specifier names, and a package of the wrong file would change the resolution of the imports of that module |
| Signature depth leaves out a namespace member that is not exported | Every member of a namespace in a script was public | A member of a namespace is public in an ambient context alone, and exported by name otherwise |
| A broken tsconfig chain resolves without its baseUrl and paths | The files resolved through the configurations read before the fault | They resolve without a baseUrl and paths |
| `///` and `/** */` document the item after them | A plain comment above a Rust item documented it | A plain comment states its carriers alone |
| A binary takes the name of its target | `src/main.rs` took the crate name, so the binary of `my-app` joined the library `my_app` | It takes the name of the package with its hyphens |
| A target is a crate of its own | Every `build.rs` of a workspace was the one package `build_script_build` | The build script is `build_script_build` below the crate name |
| Every attribute is an entry of the annotations | The attributes of a `mod` item were recorded nowhere | They annotate the File node of the module, before its inner attributes |
| A trait impl adds its trait to the Implements of a struct | A negative impl, `impl !Send for A`, added `Send` | A negative impl does not add its trait |
| The Rust backend writes the attributes back | `#[name = value]` came back as `#[name(value)]` | RFC-0063 records the assigned form, and the Rust backend writes it |
| The Java, TypeScript and Rust backends write the annotations back | Java and Rust refused the annotations of a parameter, and Rust those of a payload entry | Java writes them before the type of a parameter, and Rust before the pattern of a parameter and before a payload entry |
| Type annotations are left out of a discriminator | A Java wildcard spelled its bound with its type annotations | The wildcard spells `?extends` or `?super` and the spelling of its bound |
| A release that ct.sym does not list fails the load | A release older than every listed release gave an empty index | Every release that no directory of ct.sym names fails the load |
| The reader leaves out the `Annotation` that an annotation interface extends | It left it out of every interface | It leaves it out of an annotation interface alone |
| modules.txt reads as the go command reads it | A module line of fewer than three fields closed the open module, and a lower version listed after a higher one was a provider | The open module stays open, and a version provides a package where it raises the version of its path |

### Departures that later designs decide

| Design | Code | Record |
|---|---|---|
| `frontend.Stores` builds the stores from the environment | The Go and Java frontends are store locators, and the home directory comes from `plugin.UserHomeVar` | RFC-0021, ADR-0024 |
| The shared inputs of a TypeScript file are its tsconfig chain | The nearest `package.json` follows the chain | RFC-0031, ADR-0024 |
| The TypeScript, Rust and Java frontends register nine, five and four keys | `typescript.readonly` and `typescript.optional` are fields of the model, and the satellites register the keys of generated files, abstract properties and conditional types | RFC-0036, RFC-0054, RFC-0031, ADR-0024 |
| A TypeScript callable is async from the source | A result of `Promise<T>` lowers to an async callable of `T` | RFC-0023 |
| An inline reference has no children | It has the types of its body as children | RFC-0056, ADR-0037 |
| The carriers of a declaration are the comments before it | The comment on its last line also attaches its carriers | RFC-0031 |
| TYPESCRIPT-0001 to 0004, RUST-0001 to 0007 and JAVA-0001 to 0006 | The satellites also report BadMarker, BadPackage and ReadonlyRest, and TYPESCRIPT-0008 of this record | RFC-0023, ADR-0024, RFC-0036 |
| `#[repr]` is an annotation | Its integer is the underlying type of the enum | RFC-0036, ADR-0028 |
| A File node has the annotations of a file | No skeleton writes them | RFC-0058 |
| The TypeScript backend writes the spelling of an intersection | It restates an intersection whose child spells otherwise | RFC-0037 |
| The Go frontend folds a method onto its receiver type | It folds onto a struct or an enum | RFC-0049 |
| The corpus entry of TypeScript loads and registers no rules | It grades projection levels and registers the TypeScript rules | RFC-0023 |

### Departures without an earlier record

| Design | Code | Reason |
|---|---|---|
| Parse through the binding, whose `Node` is the runtime's node | The layer calls the runtime's string parse and declares the runtime's types in its cgo preamble | The binding copies a kind's name into C memory that it never frees, and copies the source for each read through a call into Go |
| `Errors` yields every ERROR and MISSING node | It yields an ERROR node and not the nodes inside it | The ERROR node spans the positions of the nodes inside it |
| GOROOT is two levels above the go binary, with links resolved | The root above the binary as PATH names it comes first, and links resolve where it has no `src/runtime` | The go command finds its root in that order. The frontend reads the source of the standard library, so it requires `src/runtime` |
| The reasons of a need placed nowhere | The reason of a missing module also states that no vendor tree has it | The vendor tree is the first source that the frontend tries |
| A Go dependency member is every `.go` file except the tests | Files whose names start with `_` or `.` are left out | go/build leaves them out |
| The shared input of a vendored member is its go.mod | It is the go.mod beside the vendor tree | The frontend checks the vendor tree against that go.mod, and derives the import path from its directory |
| The imports of a member are the ones that its declarations name | The prune also keeps every dot import, and every import whose name its path does not state | A reference cannot be matched to such an import, so the prune loads more and not less |
| modules.txt reads as the go command reads it | The `## go` annotation is not read | The go command compares it with the running toolchain, and the frontend runs none |
| A spelling is verbatim | The TypeScript and Rust frontends spell a reference without its whitespace | A reformatted signature keeps its identity, and the overload rule compares those spellings |
| Two export-star modules that publish one name are an ambiguity | Two star modules compete where their candidates share a tier position | The tiers probe declarations, not modules. tsc refuses such a module (TS2308) and rustc such a glob import (E0659), so every valid module has its target |
| `extends` resolves a package as Node resolves it | It tries `<spec>`, `<spec>.json` and `<spec>/tsconfig.json` under each `node_modules` | A package that names its configuration in its `package.json` is not found |
| A file under tests, examples or benches that no target roots is a unit of its own | The files of a directory under examples or benches form one unit, as under tests | Cargo compiles a directory of an example or a bench as one crate |
| Only `[lib]` states a path | A declared path of a binary, a test, an example or a bench roots its target | Cargo honours the path of every target table |
| An associated type is an Alias with a nil Target | The default of an associated type is its Target | The default is the type that an impl takes where it states none |
| `#[doc]` adds documentation, and every attribute is an annotation | `#[doc = "…"]` is documentation alone | `///` is the short form of `#[doc]`, and the backend writes documentation as `///` |
| A Gradle build states its identity in configuration | The options are the release and the classpath, and a Gradle package loads without `gen.module` | The options of RFC-0015 define no Gradle identity, and the frontend runs no Gradle script |
| A member without a modifier is Package | An enum constructor without a modifier is Private | JLS 8.9.2 makes such a constructor private |
| A release that ct.sym does not list fails the load | Without the JDK store no release is checked | The release selects the entries of a multi-release JAR alone, where any release counts |
| Each class file has a File node | `module-info` has none | A module class declares no type |
| A longer Signature is ignored | Its parameters are ignored, and its type parameters, result and throws lower | JVMS 4.7.9.1 states the caveat for the parameters alone |
| A file that does not read reports JAVA-0006 | A member that does not read fails the load | The round read each JAR to check its digest, so a later failure comes from the environment. A member that does not decode reports JAVA-0006 |
| An element value is spelled as the source spells it | A class literal keeps the `$` of its binary name | A class file states the binary name, and RFC-0015 reads no nesting from a `$` |
| A load without the JDK store warns once for every JDK package | It warns once for every import path | Whether a path names a package or a class needs the package index of ct.sym |
| The Go backend refuses an intersection | It writes the spelling where each child spells as written | A Go intersection comes from a constraint that the Go frontend read, and its spelling is Go |
| A method of a defined type folds onto its type | A method of a defined type that lowers to an Alias stays at the file level with its receiver | An Alias has no list of methods |
| A changed byte of a dependency member re-keys its unit | The kit compares the fold of the reads that the parse makes through the unit | The key folds the digest of each member before the parse, so a key check cannot fail. The fold is the part that a frontend can get wrong |
| The parse budgets of TypeScript, Rust and Java | The ceilings are 1,930,000, 2,490,000, 1,910,000 and 1,774 | Each ceiling is about 5% above a measurement, and the measurements are below the estimates of the design |
| `BenchmarkGolang` in the conformance module | `BenchmarkCorpus` | The benchmark measures the corpus of any language |

### The defect that remains

A method of an impl block in an inline Rust module, whose type the crate declares in another file,
resolves its references through the scope of the outer module. Link chooses the scope of a
declaration by the file of its position, and an inline module shares the file of its parent. A
reference of such a method that only the inline module imports keeps its spelling without a target.
The fix needs the load to record the scope of a declaration that the frontend folds onto another
type.

## Alternatives Considered

### Place a relative module augmentation in the package of the file module

The frontend could place `declare module './user'` in the package of `user.ts`. Where the module is
`user/index.ts`, the block would create a package that an import of `./user` finds first, and the
import would resolve to the augmentation.

### Group the candidates of each star module

The resolution step could take the first tier of each star module and compare the modules. Each
frontend would return a new type of candidates, and the record of a link would change. Valid source
gets the same targets from the tiers.

## Consequences

- The frontend reports `RelativeAugmentation` for a relative module augmentation and does not load
  it.
- A Rust item keeps a plain comment out of its documentation.
- The binary at `src/main.rs` has the name of the package, and each build script has a package of
  its own.
- RFC-0015 has its departures in this record.

## References

- [RFC-0015: Tree-sitter frontends, dependency stores and re-exports](../rfc/0015-tree-sitter-frontends-and-dependency-stores.md)
- [RFC-0021: The command kernels, the config file and the run lock](../rfc/0021-the-command-kernels.md)
- [RFC-0023: The cross-language hub and the TypeScript satellite](../rfc/0023-the-cross-language-hub.md)
- [RFC-0031: The shared helpers of the satellites](../rfc/0031-the-shared-helpers-of-the-satellites.md)
- [RFC-0036: The facts that the model drops](../rfc/0036-the-facts-that-the-model-drops.md)
- [RFC-0037: Five contracts that the reviewed defects need](../rfc/0037-five-contracts-that-the-reviewed-defects-need.md)
- [RFC-0049: Go enumerations](../rfc/0049-go-enumerations.md)
- [RFC-0054: TypeScript declarations that the model loses](../rfc/0054-typescript-declarations-that-the-model-loses.md)
- [RFC-0056: The references that a generated file writes](../rfc/0056-the-references-that-a-generated-file-writes.md)
- [RFC-0058: The build constraint of a generated file](../rfc/0058-the-build-constraint-of-a-generated-file.md)
- [RFC-0063: The assigned argument of an annotation](../rfc/0063-the-assigned-argument-of-an-annotation.md)
- [ADR-0024: The shared helpers keep the departures of their implementation](0024-the-shared-helpers-keep-the-departures-of-their-implementation.md)
- [ADR-0028: The facts of the model keep the departures of their implementation](0028-the-facts-of-the-model-keep-the-departures-of-their-implementation.md)
- [ADR-0037: The children of an inline reference are copies](0037-the-children-of-an-inline-reference-are-copies.md)
- The Java Language Specification, section 8.9.2, and the Java Virtual Machine Specification,
  section 4.7.9.1.
