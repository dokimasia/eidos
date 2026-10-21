---
adr: 0090
title: A dependency round passes the uses of the loaded units
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

# ADR-0090: A dependency round passes the uses of the loaded units

## Status

Accepted

## Context

A dependency round passes each import of the loaded units whose package the load has not loaded as
a need. Each need is the path of a package, so a frontend can place a whole package and nothing
smaller.

- The TypeScript library is one package for each project. Its default form has 69,303 declarations
  under 2,233 global names.
- A load of a one-file tree with that library took 394.4 ms and allocated 112.0 MB. Without the
  library the same load took 0.1 ms.
- A load of a tree that references `Map` needs 402 of those declarations.
- Only the frontend of a language can tell which names of a package a reference can mean. The
  rounds already repeat until no round has a need that an earlier round did not pass.

## Decision

We will let a unit record the bare identities that its references can resolve to outside the
workspace as its uses, and pass each use once to a dependency round of its frontend, because only
the frontend can tell which names of a package a reference can mean.

- `SourceUnit.Use` records a use with its language, its package and its name.
  `SourceUnit.Uses` returns the uses of the unit, sorted.
- `DependencyRound.Uses` lists the uses of the loaded units of the frontend that no earlier round
  passed. A round after the first runs when it has a need or a use that no earlier round passed.
- A parsed unit has the uses of its source unit, a restored unit the uses of its region, and a kept
  unit the uses of its record. `store.Region.Uses`, `load.UnitRecord.Uses` and
  `load.DoorRecord.Uses` keep them, and the format of the state goes from 5 to 6.
- The reuse of the recorded rounds compares the uses of the first round, as it compares its needs.
- A use that the frontend cannot place does not yield a unit or a finding.
- The scripted frontend of `frontendtest` records a use for each `use PATH NAME` statement, and its
  dependent places the file of the use where its store has the file.

## Alternatives Considered

### A use as an import of the package

Each file would import the package of its library with the names that it uses. Every file would
gain an import that its source does not state. The rules and the backend read the imports of a
file, so the graph of every TypeScript test would change.

### A use as a field of the file

`node.File` would have the uses of the file, and the codec of the model would keep them. The graph
of every TypeScript load would change, and so would the fingerprint of each file with a use. A fact
of the load would enter the model of the source.

### A parse memo that the fixtures of a test binary share

`plugintest.Loaded` would restore a dependency unit that an earlier fixture parsed. The memo
restores the parse and the lowering, which take 69% of the CPU time of a load. The link, the loops
over the declarations and the enumeration of the annotate phase would remain in each fixture. A
run of the command line loads once, so it would not get faster.

## Consequences

**Positive:**

- A frontend can place a unit for one name of a package. The TypeScript frontend places one unit
  for each global name of its library that a load uses.
- A frontend whose units do not record uses receives none, and its rounds do not change. A load
  without uses does not allocate for them.

**Negative:**

- A record of an earlier build does not read, so the first load after the change is cold.
- A use that the frontend cannot place does not yield a finding. A declaration that the frontend
  misses shows only as a reference without a target.
- A frontend with uses runs more rounds. A load of a tree that references `HTMLElement` runs 15
  rounds of the TypeScript frontend, where it ran one.
- `SourceUnit` has two more methods, and `DependencyRound`, `Region`, `UnitRecord` and
  `DoorRecord` have one more field each.

## References

| What | Where |
|---|---|
| RFC-0084, the design of this decision | ../rfc/0084-the-library-declarations-that-a-load-uses.md |
| D208, the row of this decision | ../architecture/21-decisions.md |
| Issue 56, the specs that load the TypeScript library | <https://github.com/dokimasia/eidos/issues/56> |
