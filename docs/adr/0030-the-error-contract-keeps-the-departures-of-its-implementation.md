---
adr: 0030
title: The error contract keeps the departures of its implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0042
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0030: The error contract keeps the departures of its implementation

## Status

Accepted

## Context

The accepted design adds the base of a sentinel name, a well-known identity for the error value and
the error contract of a type. It moves the method set and the predicate of a method that returns a
string from the Go annotator into the Go rules, and has the annotator stamp `golang.satisfiesError`
through the error contract.

The implementation met two places where the design costs a member walk that it does not need, or
where its rule for the cause selects the wrong field. Each place is listed here, because an accepted
design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| The Go annotator stamps `golang.satisfiesError` through `ErrorContract` | The annotator computes one method set through `MethodSet` and stamps both `golang.satisfiesError` and `golang.satisfiesStringer` through `NullaryString` over it. `ErrorContract` finds the method `Error` through the same `NullaryString` | `ErrorContract` computes a method set of its own, so a stamp through it walks the members of every type of the workspace twice. The stamp and the contract still decide through one predicate |
| The cause of a Go error type is the first field of the type `error`, where the type declares `Unwrap() error` | The cause is the first field of the type `error` that a struct declares, where the struct declares `Unwrap() error` itself and an embed does not promote it | An `Unwrap` that an embed promotes returns the cause of the embed, so a field of the outer struct is not the cause that it returns |

## Alternatives Considered

### Stamp through ErrorContract

The annotator would call `ErrorContract` for the error and `MethodSet` for the stringer. That is a
second member walk of each type in the cold run, for a fact that the first walk already decides.

### Take the cause from the promoted Unwrap

The contract could follow the embed and return the field of the embedded struct. The sentinel plugin
writes a test that sets the cause in a composite literal of the outer struct, which cannot set a
promoted field.

## Consequences

- The Go rules export `MethodSet` and `NullaryString`, and the annotator calls both.
- A struct that embeds an error type and declares a field of the type `error` has no cause in its
  contract unless it declares `Unwrap() error` itself.

## References

- [RFC-0042](../rfc/0042-the-error-contract-of-a-type.md)
