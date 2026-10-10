---
adr: 0012
title: A package read whole depends on every member
status: Accepted
date: 2026-10-02
supersedes: none
superseded-by: none
rfc: RFC-0020
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0012: A package read whole depends on every member

## Status

Accepted

## Context

A warm run executes a plugin's read again only when an edge in the read's
record changed. The read grain was specified in two kinds:

- A targeted read, such as `Lookup` or a fact read, records an edge on one
  identity, and a change to that declaration runs the reader again.
- An enumeration, such as `ByKind` or a package's declaration list, records
  a membership edge. Adding or removing a member runs the reader again, and
  changing a member does not.

`PackageOf` returns the whole package node. A handler that takes it walks
the package's declarations through the node itself, without a further
tracked read. Under the membership rule, an edit to one of those
declarations leaves the reader clean. The warm run then keeps an output
that a cold run would render differently.

## Decision

We will make a package edge dirty when any declaration of the package
appears, disappears or changes, because a reader that takes a package whole
can walk every member without another tracked read.

- `PackageOf` records the package edge, and so does `Lookup` of the
  package's own identity.
- The read grain has four edges: a declaration, a fact at a subject and a
  key, a membership by kind or by directive under the reader's scope, and a
  package.
- `ByKind` and `ByDirective` keep their membership edges, and record a
  declaration edge for each declaration they yield.

## Alternatives Considered

### Keep the membership rule for package reads

A package read records which declarations the package contains, and a
member's edit runs only the readers that looked the member up. A reader of
a large package then runs again only when the package's membership changes.
It lost because a reader walks the members of the package it took without a
tracked read, so the rule misses exactly the edits that change what the
reader saw.

## Consequences

**Positive:**

- A reader that took a package whole runs again whenever the package's
  content changes, and never keeps a stale output.
- The edge needs no tracking inside the node tree.

**Negative:**

- A reader that took a package for one of its declarations runs again on an
  edit to any other declaration of the package.
- A plugin that wants member-level precision looks members up through
  `Lookup` instead of walking a package, and nothing enforces that choice.

**Neutral:**

- Enumerations by kind and by directive behave as before.

## References

| What | Where |
|---|---|
| D78, the read grain that this decision replaces for package reads | ../architecture/21-decisions.md |
| D10, red-green at (symbol, key) | ../architecture/21-decisions.md |
