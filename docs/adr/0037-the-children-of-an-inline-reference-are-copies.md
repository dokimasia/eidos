---
adr: 0037
title: The children of an inline reference are copies
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0056
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0037: The children of an inline reference are copies

## Status

Accepted

## Context

The accepted design lists the types of an inline body as the children of its reference, and keeps
the members of the body with their own types. It does not say whether a child and the type of its
member are one node or two.

The first implementation listed the type of each member as a child, so one node had two places in
the graph. The fixture builder of `nodetest` refuses a node in two places, because it positions and
links each node of a fixture once, and the Go and the TypeScript language kits failed on the inline
body of their fixture. The decision is recorded here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departure is this.

| Design | Code | Reason |
|---|---|---|
| A frontend lists the types of an inline body as its children, and the members keep their own types | Each child is a copy of the type of its member, which the kernel's new function `node.CloneRef` returns. `CloneRef` copies a reference and every node below it, the fields and the methods of an inline body included. The Go frontend gives each field of an entry such as `X, Y int` a copy of the child of the entry, and `nodetest.Inline` lists copies too | A node of the graph has one place, so a walk visits it once, a position belongs to one place, and a link of the load resolves one reference |

## Alternatives Considered

### One node in two places

The children could be the types of the members. The fixture builder would have to accept a
reference in two places, and every walk of the graph would visit such a reference twice. The links
of a warm run, which the load records by the position of each reference in the walk, would list it
twice.

### A second lowering of each type

A frontend could lower the type of a member a second time for its child. The TypeScript lowering of
an object type refuses the carriers in its members, so a second lowering would report each refusal
twice.

## Consequences

- An inline body costs one copy of each type of its members. `CloneRef` measures 29 allocations for
  a body with a field and a method of a receiver, a type parameter, a parameter, a result and a
  thrown type.
- `node.CloneRef` is public API of the kernel and of the SDK.
