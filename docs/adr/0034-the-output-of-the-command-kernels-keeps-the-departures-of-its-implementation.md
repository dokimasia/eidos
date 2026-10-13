---
adr: 0034
title: The output of the command kernels keeps the departures of its implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0046
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0034: The output of the command kernels keeps the departures of its implementation

## Status

Accepted

## Context

The accepted design adds the declarations of an explanation, the status of an empty explanation,
`--quiet`, the disabled plans of a description, the brand in `version` and `cli.ErrFormat`. Its
invariants require that `version` lists every plugin that the composition declares.

The plan step of Build checks the generators and the backend of each plan that runs, and the refine
step drops a disabled plan before it. `version` listed each plugin once by name, and the frontend and
the backend of one language share the name of the language. Each change is listed here, because an
accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| The design adds no fault to Build | Build reports a nil generator and a missing backend of a plan that the config disables | The description of a disabled plan reads the name of each generator and of the backend. The plan step checks only the plans that run, so a nil plugin of a disabled plan passed Build and `Describe` would panic on it |
| `explain` renders an error for a target that the generation records nothing about | `explain` renders the generation of each workspace before the error | Every explanation starts with the generation that it read, and the error contains the name of the same generation |
| `version` lists each plugin once | `version` lists each plugin once by its role and its name | A list by name alone left out the backend of a language whose frontend has the same name, together with the version of the backend. The design requires every plugin |

## Alternatives Considered

### Nil plugins of a disabled plan in its description

`Describe` could leave a nil generator out of a disabled plan, and give a plan without a backend an
empty backend. The description would then hide a defect of the composition that Build reports as
soon as the config enables the plan.

### The error of an empty explanation alone

`explain` could hold back the generation of an empty explanation. A script that reads the JSON events
would then miss the generation event of the workspace, and the error contains the name of the
generation alone.

## Consequences

- A composition with a nil plugin in a plan that its config disables no longer builds.
- `explain` writes the generation before the error of an empty explanation.
- `version` of a binary with the Go or the TypeScript satellite lists the backend of the language
  beside its frontend.

## References

- [RFC-0046](../rfc/0046-the-output-of-the-command-kernels.md)
