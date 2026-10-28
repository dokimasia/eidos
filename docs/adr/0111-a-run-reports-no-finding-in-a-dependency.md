---
adr: 0111
title: A run reports no finding in a dependency
status: Accepted
date: 2026-10-28
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0111: A run reports no finding in a dependency

## Status

Accepted

## Context

A dependency round loads the packages that a workspace uses from a store, such as the module
cache of Go. The dispatch skips each declaration of such a package for a rule of the generate
phase. An annotator runs on every declaration, so the load attaches the directives of a dependency
unit, and the freeze validates them.

- Issue 96 reports a module that imports the test kits of core v0.33.0. The kits contain the
  directives of dokimi. A run of dokimi over the module reported 88 findings in the files of core,
  failed both plans and did not write a file.
- 52 findings were EID-0040. A bare name of a reference param resolves in the external test package
  of its directory, and a dependency round does not load that package.
- 36 findings were GOLANG-0003. The Go frontend refuses each carrier that no declaration takes. The
  header lines of a file that eidos generates, such as `// dokimi:plugin stub`, are such carriers in
  a generated file of a dependency.
- A dependency unit and a workspace unit under a `Signatures` root both load at
  `plugin.DepthSignatures`, so the depth does not mark a dependency unit.

## Decision

We will report no finding at a carrier of a dependency unit and no finding of the validation of a
directive of a dependency package.

- `plugin.NewDependencyUnit` builds a unit at `DepthSignatures` that reports no finding at a
  carrier, and `SourceUnit.Dependency` reports such a unit. The load builds every unit of a
  dependency round through it.
- A dependency unit attaches each carrier that the grammar of directives accepts, so an annotator
  reads its directives. `Comment`, `AttachCarriers`, `AttachSugar`, `RefuseCarriers` and
  `RefuseSugar` report nothing for it.
- The freeze validates the directives of a subject of a dependency package into a sink that the run
  discards, on a cold run and on a warm run. The directives that pass enter the table. A directive
  that fails does not give a fact. The state of a warm run records no finding for such a subject.
- The header lines of a generated file get no rule of their own.

## Alternatives Considered

### A rule for the header of a generated file

The Go frontend could skip the header that the output contract writes. The rule would cover one
frontend and one cause. The EID-0040 findings and the carriers of the other frontends in a
dependency would remain.

### No directives of a dependency

The load could attach no directive of a dependency unit. An annotator then could not read the
directives of a dependency, such as a `subject` directive of a test kit that a plan of the workspace
reads through its type.

### A warning in place of an Error

A warning does not fail a run of eidos. The strict setting of dokimi reports each warning as an error, so
the run of the issue would still fail. The author of the workspace cannot change the source of a
dependency either way.

## Consequences

**Positive:**

- A run of dokimi over the module of the issue reports no finding and writes both stub files.
- The rule covers every frontend with a dependency round: Go, Java, TypeScript and Python.

**Negative:**

- A directive of a dependency that fails validation is dropped without a report.
- A frontend still reports a finding of a dependency unit that is not at a carrier, such as a file
  that does not parse.

## References

| What | Where |
|---|---|
| D229, the row of this decision | ../architecture/21-decisions.md |
| Dependency packages and the scope of a plan | ../architecture/08-workspace-and-plans.md |
| Issue 96, a run fails at the carriers of a dependency unit | <https://github.com/dokimasia/eidos/issues/96> |
| dokimi issue 39, the report from dokimi | <https://github.com/dokimasia/dokimi/issues/39> |
