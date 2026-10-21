---
adr: 0091
title: The TypeScript library loads one unit for each global name that a load uses
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: RFC-0084
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0091: The TypeScript library loads one unit for each global name that a load uses

## Status

Accepted

## Context

The accepted design of the library of TypeScript loads the library of each project as one
dependency unit, and a lib directive of a source file imports the package of the extended library.
An accepted design is not amended. This record replaces both parts, and the rest of that design
holds.

- A fixture of a one-file tree with the default library contains 69,312 declarations, and 69,303
  of them are declarations of the library. Its load took 394.4 ms and allocated 112.0 MB.
- A load of a tree that references one common type needs a small part of the library: 402
  declarations for `Map` and 3,513 for `Array`.
- The interfaces of one name merge across the files of the library, as `Array` across 8 files, so
  the declarations of a name are not in one file.
- A dependency round passes the uses of the loaded units, so a frontend can place a unit for one
  name of a package.

## Decision

We will load one dependency unit for each global name of the library that a load uses, with the
statements of the name in each file of the library, because a load of one common type needs at most
5.1% of the library.

- `library.Index` is the statements of the global names of each file of the release. The spec of
  the package derives it from the files with the grammar of TypeScript and writes it under
  `-update`.
- The store serves the statements of one name of one file at `<key>/<file>#<name>`.
- The unit of a name has the fragment of the name in each file of the library, in the order in
  which typescript-go loads the files. A module file is a member whole, and its global names share
  one unit with their statements in the other files. A namespace is one name.
- A unit uses each global name of its library that one of its references can resolve to, read from
  the tiers of the reference up to the first tier with a declaration of the unit. A name is a use
  only where a file of the release declares it, and a value of a carrier is not a use.
- A lib directive of a source file selects the library of that file, and the file does not import
  the library.
- The round reads the directives of the embedded files and lists the files of each library once for
  the process. It reads the first root of a library through its door.

## Alternatives Considered

### A unit for each file of the library

The round would place each file that declares a used name as one unit. `lib.dom.d.ts` declares
55,312 of the 69,303 declarations of the default library, so one name of the DOM would load 80% of
the library. A unit of one file also cannot merge the interfaces of one name across files.

### The closure in one round from a graph of names

An embedded graph of the names that each statement references would let the first round place the
whole closure. The graph would be a second derived artifact beside the index, and it would come from
the syntax of the statements and not from the references that the lowering resolves. The rounds
already repeat to a fixed point.

### An index that the frontend builds at run time

The frontend would parse each file of a selection once in a process and split it into statements.
The parse of the files of the default library costs about 57 ms of CPU, and each run of the command
line is a process of its own.

### Declarations that the link adds

The link would load a declaration of the library when a candidate of a reference names it. The load
builds its graph before the link, so a declaration that the link adds would need its own parse, its
own key and its own link inside the same pass.

### One load of the library for each process

The fixtures of a test binary would share one load of the library. A fixture would still index,
walk and enumerate 69,312 declarations, and a run of the command line loads once.

## Consequences

**Positive:**

- The load of the one-file tree of issue 56, which references `Map`, takes 2.5 ms and allocates
  0.9 MB, where it took 394.4 ms and allocated 112.0 MB. Its fixture has 411 declarations.
- A cold run of the command line over a corpus of TypeScript fixtures with the default library takes
  0.13 to 0.14 s and peaks at 72 to 77 MB, where it took 0.58 to 0.60 s and peaked at 232 to
  240 MB.
- The units of the names of the default library declare in each package what the whole library
  declares, positions aside. A spec of the frontend checks it.

**Negative:**

- A load runs up to 6 rounds for a common type and 15 for `HTMLElement`, which loads 43,231
  declarations.
- A namespace is one unit, so a tree that references `Date` or `Array` loads every declaration of
  `Intl`.
- A use is a superset of what resolves to the library. A type parameter with the name of a global
  of the library loads that declaration.
- A declaration of the library has its position in a fragment and not in the file of the release.
- A rule that looks up a declaration of the library finds it only where a reference of the load
  can resolve to it.
- The frontend keeps the bytes of each embedded file that it reads for the life of the process, up
  to the 3.8 MB of the release.
- The statements of the index take 142 KB of each binary that imports the frontend. A refresh of
  the library runs the spec of the index with `-update` and changes the version of the frontend.
- The module file binds its three global names into one unit.

## References

| What | Where |
|---|---|
| RFC-0084, the design of this decision | ../rfc/0084-the-library-declarations-that-a-load-uses.md |
| RFC-0082, the design whose unit of the library this record replaces | ../rfc/0082-the-declaration-files-of-the-typescript-library.md |
| D209, the row of this decision | ../architecture/21-decisions.md |
| Issue 56, the specs that load the TypeScript library | <https://github.com/dokimasia/eidos/issues/56> |
| typescript-go 7.0.2, the bundled library | <https://github.com/microsoft/typescript-go/tree/typescript/v7.0.2/internal/bundled/libs> |
