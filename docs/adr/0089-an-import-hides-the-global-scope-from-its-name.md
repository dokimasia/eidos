---
adr: 0089
title: An import hides the global scope from the name that it binds
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0089: An import hides the global scope from the name that it binds

## Status

Accepted

## Context

The TypeScript frontend returns the tiers of a bare name in the scope order of TypeScript. The
accepted design of the library has a bare name probe the enclosing namespaces, the package of the
file, its imports and then the global tiers. An accepted design is not amended. This record replaces
that order for a name that an import binds.

The resolution step takes the first tier with a candidate that the graph contains. The load does
not read a package of `node_modules`, so the tier of an import from `immutable` has no such
candidate.

- In a reproduction, `import { Set } from "immutable"` binds `Set`, and the field
  `tags: Set<string>` resolved to the `Set` of the library.
- `typescript.Global` reported true for the reference. The rules then classified the field as the
  set of TypeScript, and a generator wrote `Set<string>` without the import from `immutable`.
- TypeScript binds the name to the import in the scope of the module. The binding hides the global
  `Set` also where the module does not resolve. tsc 7.0.2 reports TS2307 alone for the file of the
  reproduction, because the field has the error type of the unresolved import.

## Decision

We will end the tiers of a bare name that an import binds at the module of the import, because
TypeScript binds the name to the import whether the module resolves or not.

- A name that a named or a default import binds probes the enclosing namespaces, the package of the
  file and the module of the import.
- A name that a namespace import binds probes the enclosing namespaces and the package of the file.
- A qualified name whose first name an import binds probes the module of the import alone, as
  before.
- A reference whose module the load does not contain keeps its spelling and its package without a
  target.

## Alternatives Considered

### The global tiers after the module of the import

This was the accepted order. A name that an import binds resolved to a declaration of the library
whenever the load did not contain the module of the import. The rules and the backend then read the
reference as a global of TypeScript.

## Consequences

**Positive:**

- In the reproduction the field keeps the package `immutable` without a target, and
  `typescript.Global` reports false for it.
- The rules and the backend read a reference through an import as TypeScript reads it.

**Negative:**

- A name that a file imports from a module that the load does not contain has no target, also where
  the library declares the name. A rule that needs the declaration of the reference finds none.
- The tiers of a bare name depend on whether an import binds it. A reader of the tiers cannot
  assume that the global tiers end every list.

## References

| What | Where |
|---|---|
| RFC-0082, whose order of the tiers of a bare name this record replaces for a name that an import binds | ../rfc/0082-the-declaration-files-of-the-typescript-library.md |
| D207, the row of this decision | ../architecture/21-decisions.md |
| Issue 55, the resolution of a name that an import binds | <https://github.com/dokimasia/eidos/issues/55> |
