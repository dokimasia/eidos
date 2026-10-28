---
adr: 0114
title: A dependency is a declaration that a dependency round loaded
status: Accepted
date: 2026-10-28
supersedes: none
superseded-by: none
rfc: RFC-0088
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0114: A dependency is a declaration that a dependency round loaded

## Status

Accepted

## Context

The load marks each region of a sealed graph with `RegionInfo.Dependency` when a dependency round
loaded its unit. `store.Graph.Dependency` reported a declaration by the mark of its package, and the
sealed index marked a package only when the mark was on every region of it. Four parts of the kernel
read the result.

- The dispatch runs no rule of the generate phase on a subject of a dependency package.
- The freeze discards the findings of the validation of a subject of a dependency package, under
  ADR-0111, on a cold run and on a warm run.
- The scope of a plan admits a dependency package by its language alone.
- `plugintest` reports a fixture package as a dependency package.

RFC-0088 loads each file that a protoc request imports as a dependency unit. A file to generate and
a file that it imports often declare one proto package, such as `acme.v1`. That package was no
dependency package, so a generator of the eidos-protoc specs ran on the imported message `Shared`
and failed the run with EID-0051. A narrowed plan also refused the package, because its imported
file is in the store `protoc` and in no named directory.

## Decision

We will mark a dependency per declaration.

- A declaration is a dependency when a dependency round loaded the region that declares it. A
  package is a dependency when a dependency round loaded every region of it, as before.
- `store.Graph.Dependency` keeps its signature. For a package that only some marked regions
  contribute to, the sealed index decodes the regions of the package and records the mark of each
  declaration. For every other package it reads the summaries, decodes nothing and allocates
  nothing, as before.
- The dispatch and the validation silence of ADR-0111 ask per subject, so they apply the mark per
  declaration without a change.
- A pattern of `Sources.Packages` decides the workspace files of a package. A file that a store
  provides counts toward no directory, so a scope admits a shared package when its workspace files
  are in a named directory.
- A graph from `store.New` keeps the mark per package. It refuses two packages under one identity,
  so it cannot hold a shared package.

## Alternatives Considered

### A store path on the file of a declaration

The dispatch could treat a declaration whose file has a store path as a dependency. A vendored
package of a dependency round has a workspace path, and the round of a unit already states the
fact, so the path would be a second record of the mark.

### A package per file in the descriptor frontend

The descriptor frontend could give an imported file a package identity of its own. A reference
from the file to generate to the imported message then names another package than the source
frontend gives it, and the parity of the two frontends breaks.

## Consequences

**Positive:**

- A generator under eidos-protoc writes the messages of the file to generate and none of an imported
  file of the same proto package.
- A package that a frontend never shares with a dependency round reads as before.

**Negative:**

- `Dependency` decodes the regions of a shared package on its first call for a declaration of it.
  The dispatch has decoded them before it asks.
- A warm run records a change of round per package, so a file of a shared package that moves
  between the workspace and a store reruns the whole package.

## References

| What | Where |
|---|---|
| D232, the row of this decision | ../architecture/21-decisions.md |
| Dependency packages and the scope of a plan | ../architecture/08-workspace-and-plans.md |
| RFC-0088, a protoc plugin host | ../rfc/0088-a-protoc-plugin-host.md |
| ADR-0111, the silence of a dependency | 0111-a-run-reports-no-finding-in-a-dependency.md |
