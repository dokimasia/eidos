---
adr: 0098
title: The Go frontend loads the packages that a load uses
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: ADR-0102
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0098: The Go frontend loads the packages that a load uses

## Status

Accepted

## Context

The dependency round of the Go frontend placed one unit for each import that no loaded package
declared. A unit is the package directory, and the parse reads each file of it whole, function
bodies included.

- A one-file tree that reads `runtime.GOOS` in a value loaded 5 packages, 514 files and 2,341
  declarations in 44.6 to 46.1 ms with the stores of the Go frontend. Without the stores it loaded
  7 declarations in 0.07 to 0.17 ms.
- The load resolves no declaration for a value or a function body. The Go rules give a qualified
  name into a package outside the view a stand-in under its import path, so a value such as
  `runtime.GOOS` writes the same code without the package.
- A dependency round passes the uses of the loaded units since D208, and the TypeScript frontend
  places the declarations of its library by the uses of a load.
- A signature-only file of a dependency kept the imports that its exported signatures referred to,
  so the rounds after the first followed signatures and not bodies. The first round followed every
  import of the workspace.

## Decision

We will load a Go dependency package only where a declaration or a directive of the load uses it.

- A unit records a use for each declaration outside the package of a file that a named type of a
  declaration of the file can resolve to. The uses are the candidates that the bindings of the
  file return: the declaration that a qualifier binds, the name in every unaliased import for a
  qualifier that no import binds, and the name in every dot import for an exported bare name.
- A unit also records the candidates of each scalar and each element of a list that an argument of
  a directive on the file states, because a witness or a callable in scope can refer to a
  declaration of another package.
- The dependency round places the package of each use once. The round does not place or report an
  import without a use. A used package that the build places nowhere reports under EID-0047 where
  the package is a need of the round.
- A signature-only file keeps the imports whose packages it uses.
- The frontend version is 0.9.0, because a unit of an earlier version does not record uses.

## Alternatives Considered

### A purge of function bodies before the parse

gopls deletes the contents of each block outside a struct or an interface type before it parses
the files of a package for the references between its declarations. A purge with go/scanner cost
15.5 ms for the files of `runtime`, against 16.3 ms for the parse that it saves. The purge also
loads every imported package.

### A unit for each used name

The TypeScript library has a generated index of the statements of each file, so a unit can contain
the statements of one name. A Go package of the module cache has no such index, and the methods of
a type can be in any file of its package. Each use would still read every file of the package.

### A report of every import that places nowhere

The load does not need a package that no declaration and no directive refers to. The go command
already reports such an import as a defect of the build.

## Consequences

**Positive:**

- The tree that reads `runtime.GOOS` in a value loads 1 package and 7 declarations in 0.06 ms with
  the stores.
- A package that only function bodies or values refer to does not load, in the first round or in a
  later one.

**Negative:**

- A load that refers to a type of a package still loads the whole package and the packages of its
  exported signatures. A field of `runtime.Frame` loads 5 packages in 44 ms, and a field of
  `http.Handler` loads 37 packages in 64 ms.
- The round does not report an import without a use when no required module provides it.
- Recording the uses adds 5,000 allocations to a parse of 200,000 declarations, 0.12% of 4,157,013.

## References

| What | Where |
|---|---|
| D216, the row of this decision | ../architecture/21-decisions.md |
| D208 and ADR-0090, the uses of a dependency round | 0090-a-dependency-round-passes-the-uses-of-units.md |
| The Go satellite | ../architecture/11-languages.md |
| Issue 60, a tree that imports runtime loads in 45 ms with the stores | <https://github.com/dokimasia/eidos/issues/60> |
