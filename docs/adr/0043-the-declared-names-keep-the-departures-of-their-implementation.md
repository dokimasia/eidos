---
adr: 0043
title: The declared names keep the departures of their implementation
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0061
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0043: The declared names keep the departures of their implementation

## Status

Accepted

## Context

RFC-0061 gives the render the name that the package clause of each loaded package declares, and the
Go backend binds an import under that name. Its implementation departs from the design in three
places. The decisions are recorded here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from RFC-0061. The import set takes the names through
`ImportSet.SetNames`, and `ImportSet.Packages` returns the paths that a file recorded. The
departures are these.

| Design | Code | Reason |
|---|---|---|
| The workspace looks a package up in the index of the plan | The workspace looks a package up in the graph of the run, whatever the sources of the plan admit | The index admits the packages of the sources of the plan alone. A file can import a package outside them, and the first invariant of RFC-0061 covers every package that the run loaded |
| `ImportSet.Declared` records the path | `ImportSet.Declared` records the path of a package that the run loaded | A group reads the declaration edge of each recorded package. An edit that adds a package to the load or removes it changes the targets of the references into the package, so the run renders their files again through the declaration edges of their origins. The path of a package outside the load, such as a package of the standard library, would add a read that no edit makes dirty |
| The import block states the name where it differs from the last element of the path | The Go import block also states a name that equals the last element of the path and differs from the name that the package declares | An import without a name binds the declared name. A template that states the last element as the name, and a suffixed name that equals the last element of another path, would otherwise refer to the package by a name that the import does not bind |

## Alternatives Considered

### The design as written

eidos could look the names up in the index of the plan, record every path, and leave out each name
that equals the last element of the path. A file would import a package outside the sources of its
plan under the name that its path assumes. Each group would read one edge for each package of the
standard library that its files import. A template that states the last element of the path for a
package that declares another name would write a file that does not compile.

## Consequences

- A plan imports a package outside its sources under the declared name of the package.
- The record of a group lists the declaration edges of loaded packages alone.
- A Go import block can state a name that equals the last element of the path.
- RFC-0061 has its departures in this record.
