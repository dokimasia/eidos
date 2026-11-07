---
adr: 0124
title: A run does not report the ambiguous references of a dependency
status: Accepted
date: 2026-11-07
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0124: A run does not report the ambiguous references of a dependency

## Status

Accepted

## Context

The load resolves every type reference of every unit through the bindings of its file, the units of
a dependency round included. The first tier of candidates with a match decides, and its first
declaration is the target. When that tier contains two declarations, the load reports
AmbiguousReference, a Warning.

- dokimi issue 61 reports a tree of nine Go modules, one of which imports the S3 client of the AWS
  SDK. The references at lines 685 and 1076 of `api_client.go` of `service/s3`, a file of the
  module cache, reported AmbiguousReference. The strict setting of dokimi reports each Warning as an
  Error, and a Warning of a phase that every plan shares fails every plan. Every run in the tree
  failed and did not write a file, also a run whose patterns named another module.
- The Go frontend loads the dependency units of every module in the tree, because it reads `go.mod`
  and not `go.work`.
- The cause of those two findings was the binding of an import without an alias. A qualifier now
  admits a declaration by the name that its package declares, so each of the two references
  resolves to one package. A reference of a dependency can still match two declarations for another
  cause, and the author of the workspace cannot change the source of a dependency.
- A run already reports neither a finding at a carrier of a dependency unit nor a finding of the
  validation of the directives of a dependency package. The resolution of a dependency unit still
  reports its ambiguous references.

## Decision

We will not report an ambiguous reference of a unit of a dependency round, because the author of a
workspace cannot change the source of a dependency and one such finding fails every strict run of
the tree.

- The resolution step resolves a reference of a dependency unit as it resolves any other reference.
  The first tier with a match decides, and its first declaration is the target.
- The record of the reference contains no AmbiguousReference, and the load does not report one.
  When the load selects again from the recorded tiers of a kept or restored dependency unit, it
  records none either.
- A reference of a workspace unit reports AmbiguousReference as before.

## Alternatives Considered

### A strict failure of the plans that read a unit

A strict run would fail only the plans whose sources contain the unit of a finding. It lost because
the load is a phase that every plan shares, so its findings belong to no plan. The rule needs a map
from each finding of the load to the plans that read its unit.

### A load of the modules that the patterns name

A run would load only the dependency units of the modules that its patterns name. It lost because
the patterns restrict the commits of a run alone. A run with patterns loads, validates, annotates
and generates as a run without patterns does, and it reports each change outside the patterns as
withheld.

## Consequences

**Positive:**

- A strict run of a tree whose dependency has an ambiguous reference writes its files.
- The rule covers every frontend with a dependency round.

**Negative:**

- A defect of a frontend that makes a reference of a dependency match two declarations no longer
  shows in a run, as the binding of an import without an alias did. Such a reference takes the first
  declaration as its target without a report.
- A unit that an earlier version of eidos recorded keeps its findings until its key changes, so a
  warm run reports them again. The key of a unit folds the version of its frontend.

## References

| What | Where |
|---|---|
| D242, the row of this decision | ../architecture/21-decisions.md |
| Dependency packages and the scope of a plan | ../architecture/08-workspace-and-plans.md |
| No finding at a carrier of a dependency unit | [ADR-0111](0111-a-run-reports-no-finding-in-a-dependency.md) |
| The declared name of the package of a hit | [ADR-0122](0122-a-qualifier-admits-the-declared-name-of-the-package-of-a-hit.md) |
| dokimi issue 61, a strict run fails in every module of a tree | <https://github.com/dokimasia/dokimi/issues/61> |
| eidos issue 121, the binding of an import without an alias | <https://github.com/dokimasia/eidos/issues/121> |
