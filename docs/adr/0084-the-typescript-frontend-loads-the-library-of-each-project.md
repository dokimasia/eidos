---
adr: 0084
title: The TypeScript frontend loads the library of each project from the embedded release
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0082
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0084: The TypeScript frontend loads the library of each project from the embedded release

## Status

Accepted

## Context

The TypeScript frontend claims the files of the tree outside `node_modules`. No unit declared the
library of TypeScript, so a global name such as `Error` had no target.

- The member walk of an interface that extends `Error` or `AsyncDisposable` reported a gap. A
  generator that needs the whole member set declined the interface.
- The Go frontend reads the standard library of Go from a store, and the Python frontend reads the
  standard library of Python from the embedded stubs of typeshed. The TypeScript frontend read no
  library.
- Each project selects its library through the options `noLib`, `lib` and `target` of its
  tsconfig. A Node project and a browser project of one workspace have different globals.
- The release of TypeScript that a project installs is below `node_modules`, and the load does not
  read `node_modules`.

## Decision

We will embed the declaration files of TypeScript 7.0.2 and load the library that the tsconfig of
each project selects as one dependency unit, because a global name of a source file then resolves
to the declaration of its library.

- `noLib` selects no file. `lib` selects the file of each entry, by its name or its file name
  without regard to case. Without either option, `target` selects its default library, and an
  unset target is ES2025. A file without a tsconfig has the default library of ES2025.
- An entry, a target or a directive with a name that TypeScript 7.0.2 does not know reports under
  `UnknownLibrary`, TYPESCRIPT-0009.
- The files of a library are its root files and each file that a `/// <reference lib>` directive
  of its files names. They keep the order in which typescript-go loads library files.
- The key of a library is its root files, sorted and joined with `+`. Its declarations load into
  the package `typescript-lib://<key>`, so two projects with one key share one unit.
- The partition probes the store for the first root file of the library of each file. It adds the
  path to the shared inputs of the file where the store has it. The dependency round places one
  unit for each key, and a load without the store has no library.
- A lib directive at the head of a source file adds its library to the library of that file alone.
  The file imports the package of the extended library.
- A library unit parses its members concurrently and lowers them in order. Its script declarations
  and its `declare global` blocks load into the package of the library. The interfaces of one name
  merge across the files of the unit. A merged method that equals a method of the interface does
  not add a member, and neither does a merged property of a name that the interface has. A syntax
  error of a member does not report.
- The global scope of a source file is the global package of the workspace and then its library.
  A member of a library has its library alone. `globalThis.X` probes the global scope alone.
- The rules treat a reference to a declaration of a library as they treat a global name without a
  target. `ShapeOf` returns the shape of `Builtin` for it. `ParamRole`, `ErrorContract` and
  `Constructors` read the `AbortSignal` and the error classes of the library as the globals of
  TypeScript. `ErrorContract` does not take a field that the library declares as the message or
  the cause of an error.
- The backend writes a reference to a declaration of a library, a call of a function of a library
  and an array of the `Array` of a library without an import. A declaration of the file that hides
  a declaration of the library refuses the reference.

## Alternatives Considered

### The library in the global package of the workspace

The library would declare into the package with the empty path, which a script of the workspace
declares into. The load keeps the first of two declarations of one identity, so a script that
declares `Event` would hide the `Event` of the library or be hidden by it. Two projects with two
libraries could not share one package.

### One unit for each file of the library

TypeScript merges an interface across the files of the library, as `Array` across 8 files, and the
frontend merges declarations inside one unit. For the default library, a probe reported 219
duplicate declarations of the 88 interfaces that the library declares in more than one file.

### One library for the whole workspace

The union of the libraries of every project would load once. A workspace with a Node project and a
browser project would resolve the names of the DOM in the Node project. The member set of `Array`
would have the members of the newest library in an ES5 project.

### The library as an import of every file

Each file would import the package of its library. Every file of every TypeScript load would gain
an import that its source does not state, and the graph of every TypeScript test would change.

### The declaration files below node_modules

The load would read the library of the release that each project installs. The load reads no
`node_modules`, and a package of `node_modules` needs the resolution of Node with its exports, its
typesVersions and the `@types` packages.

### A table of the members of the library in the rules

The rules would complete the walk of a global name from a table of members. The default library
declares 1,663 interfaces and more than 10,000 fields and methods, and a table would duplicate
each.

## Consequences

**Positive:**

- The walk of an interface that extends `Error` lists `name`, `message`, `stack` and `cause`.
- The load of the default library takes 362 to 368 ms on four cores. It allocates 84 MiB and keeps
  44 MiB. It does not report a finding.
- A warm load restores a library unit whose key is unchanged, as it restores every unit.

**Negative:**

- The binaries that import the TypeScript frontend embed 3.8 MB of declaration files.
- A cold load of a project with the DOM parses 2.9 MB more. A workspace with two libraries parses
  the files that they share twice.
- The view contains the declarations of the library, 1,663 interfaces for the default library.
- Every unit of a TypeScript load with the store has one more shared input, so every unit key
  changes once.
- A global declaration of the workspace with the name of a library interface hides the library
  interface in the files of the workspace. Its walk lists the members of the workspace declaration
  alone.
- A reference directive of a source file applies to its own file, where TypeScript applies it to
  the program.
- The selection follows TypeScript 7.0.2, whatever release a project installs. A refresh of the
  library also refreshes the table of names, the table of targets and the spec of the syntax
  errors of the library.

**Neutral:**

- The Go and Python frontends read their standard libraries from stores already, and the TypeScript
  library is one more store.

## References

| What | Where |
|---|---|
| RFC-0082, the design of this decision | ../rfc/0082-the-declaration-files-of-the-typescript-library.md |
| D202, the row of this decision | ../architecture/21-decisions.md |
| Issue 48, the members of an interface that extends the library | <https://github.com/dokimasia/eidos/issues/48> |
| typescript-go 7.0.2, the bundled library | <https://github.com/microsoft/typescript-go/tree/typescript/v7.0.2/internal/bundled/libs> |
| typescript-go 7.0.2, the load of library files | <https://github.com/microsoft/typescript-go/blob/typescript/v7.0.2/internal/compiler/fileloader.go> |
