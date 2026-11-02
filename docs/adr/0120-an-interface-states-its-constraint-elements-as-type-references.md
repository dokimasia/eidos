---
adr: 0120
title: An interface states its constraint elements as type references
status: Accepted
date: 2026-11-02
supersedes: ADR-0022
superseded-by: none
rfc: RFC-0089
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0120: An interface states its constraint elements as type references

## Status

Accepted

## Context

The Go frontend recorded the constraint elements of an interface, such as the union `struct{} | Ack`,
as their spellings under the stamp `golang.typeSet`. A spelling uses the import names of the file
of the interface and has no target. A generator in another package could not resolve a term. The
Go backend did not write constraint elements, so a copy of a constraint interface lost its type set.
RFC-0056 kept the stamp, because a declared interface had no reference for its elements.

## Decision

We will state the constraint elements of an interface as type references in the model.

- `Interface` has `Constraints` on both sides of the model, under the fact `Constraints`. Each
  element is one reference: a union with a child for each term, or the reference of a type term.
- The Go frontend fills the field in the order of the source and no longer stamps `golang.typeSet`.
  It keeps the stamp `golang.constraintInterface`.
- The Go rules read the terms of the field for the witness of a type parameter.
- This decision supersedes the rule of ADR-0022 that the Go rules keep a handle for the type sets.
  The Go rules read the constraint elements from the model, also in a view without facts.
- The Go backend writes each element on a line of its own after the embeds and the supertypes of
  the interface. TypeScript, Java, Rust, Python, protobuf and GraphQL refuse the fact.
- The surface of a package lists a line `<interface> constrains <spelling>` for each element.

## Alternatives Considered

### The terms in the stamp

A stamp of references is a fact of one language, so the resolution step, the walk of each backend
and the surface would not see the references.

### The elements as embeds

An embed promotes the members of its type, and a union has no members, so the member walk would
meet a contributor that it cannot follow.

## Consequences

**Positive:**

- A term of a constraint element has its target and the path of its package, so a generator in
  another package refers to it.
- A copy of a constraint interface into Go keeps its type set.

**Negative:**

- Each backend states a verdict on one more fact.
- A plugin that reads `golang.typeSet` reads the field instead.
- The Go backend writes the elements after the embeds and the supertypes, whatever their order in
  the source.

## References

| What | Where |
|---|---|
| RFC-0089, the constraint elements of an interface | ../rfc/0089-the-constraint-elements-of-an-interface.md |
| D238, the row of this decision | ../architecture/21-decisions.md |
| RFC-0056, the references that a generated file writes | ../rfc/0056-the-references-that-a-generated-file-writes.md |
| ADR-0022, the named handles of the rules, whose handle of the type sets this decision removes | 0022-a-rules-implementation-reads-a-named-key-through-fact.md |
