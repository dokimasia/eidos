---
adr: 0036
title: The position of a package keeps the departure of its implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0055
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0036: The position of a package keeps the departure of its implementation

## Status

Accepted

## Context

The accepted design places a package without a position at the position of its first file, and it
puts the change in the splice of the load. The splice runs after each unit has its region, which
contains the nodes that the unit built. A package of one unit is the node of that unit.

A change in the splice wrote the position into the node of a unit after the load had recorded its
region. A warm run that kept the region then read a node with a position, and a cold run read the
same node without one. The duplicate package name of the load reported at two positions, and the
warm and the cold graphs differed. The change is listed here, because an accepted design is not
amended.

## Decision

We keep the code as built where it departs from the accepted design. The departure is this.

| Design | Code | Reason |
|---|---|---|
| The splice places a package without a position at the position of its first file | The parse of each unit places each package of the unit without a position at the position of its first file in the unit. The splice keeps the position of the first unit's node, as it keeps its name | The position is part of the nodes of the unit, so a warm run that keeps the region of the unit keeps the same position. The first file of the first unit is the first file of the package in the order of the units |

## Alternatives Considered

### The position in the splice, outside the nodes of the units

The splice could set the position on a node of its own for every package. A package of one unit is
the node of that unit, so the splice would have to copy every such package, and the region of the
unit would still differ from the graph.

## Consequences

- A package that a frontend leaves without a position has the position of its first file in cold
  and warm runs alike.
- The finding of the load for a package that two units name differently reports at the first file
  of the later unit.
