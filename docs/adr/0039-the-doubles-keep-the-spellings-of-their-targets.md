---
adr: 0039
title: The doubles keep the spellings of their targets
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0059
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0039: The doubles keep the spellings of their targets

## Status

Accepted

## Context

RFC-0059 decides that the facade has no target constants, and states that the doubles of the
reference ensemble take their two targets from the root packages of the Go and the TypeScript
satellites. The package `internal/double` declares `GoTarget` and `TypeScriptTarget` with the
spellings `"golang"` and `"typescript"`. Its comment states that a plugin of both languages imports
neither satellite, and it spells the key of the constraint fact of the Go satellite for the same
reason. Its spec checks both constants against `golang.Target` and `typescript.Target`. The
implementation of RFC-0059 found this design, and the decision is recorded here, because an
accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departure is this.

| Design | Code | Reason |
|---|---|---|
| The doubles take their two targets from the root packages of the two satellites | `internal/double` keeps the two spellings, and its spec checks each against the target of its satellite | A plugin of two languages imports no satellite, as the package states. The spec fails where a satellite renames its target, so the spellings cannot drift without a failing test. A change of the package would be an optional refactor that fixes no defect |

## Alternatives Considered

### The targets of the satellites

The doubles could import `golang.Target` and `typescript.Target`. A plugin of two languages would
then depend on both satellites for two spellings, and the package would still spell the key of the
constraint fact of the Go satellite.

## Consequences

- The facade has no target constants, and a neutral plugin restates the spelling of each target
  that it serves.
- The spec of `internal/double` is the check that the spellings match the satellites.
