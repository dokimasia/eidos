---
adr: 0032
title: The inputs of a route keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0044
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0032: The inputs of a route keep the departures of their implementation

## Status

Accepted

## Context

The accepted design adds the route of a unit, the path of the kernel `out` directive on a package,
the refinements of a generator and of a family in the config file, and the refinements in `plan`. A
group reads the declaration edge of the package of each origin of its units, so a warm run routes
the group again after an edit of the directive on the package.

`explain` identifies each read of a record from the edges of the subjects that it meets. It did not
identify the new read of a group, so each group of an explanation counted one more unidentified read.
`plan` met a refinement without a field. A composition can declare one in Go. Each change is listed
here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| The design changes no read that `explain` identifies | `explain` identifies the declaration edge of the package of each subject that it meets as a read of the declaration of the package | The group of each file reads that edge. Without the change, an explanation reports the read of the package as unidentified in each group |
| `plan` prints one line for each refinement of a generator and for each refinement of a family | `plan` leaves out a refinement that sets no field, in the text and in the JSON event | A refinement without a field changes no route. A line for it states nothing, and its JSON object would be empty |

## Alternatives Considered

### Count the read of the package as unidentified

The count of unidentified reads tells a user how much of a record `explain` cannot name. The read of
the package is a read that `explain` can name, so counting it would raise the count without reason.

### Print an empty refinement

`plan` could print `refine generator mockgen:` with no field. A config file cannot write such a
refinement, because a generator whose entry sets families alone refines no field of the generator. A
composition that declares one in Go gains a line that states nothing.

## Consequences

- An explanation of a declaration lists the read of the declaration of its package in the group of
  each file that the plans generated from it.
- `plan` prints a line for each refinement that sets at least one field.

## References

- [RFC-0044](../rfc/0044-the-inputs-of-a-route.md)
