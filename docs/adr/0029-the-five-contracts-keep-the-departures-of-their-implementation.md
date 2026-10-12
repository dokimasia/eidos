---
adr: 0029
title: The five contracts keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0037
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0029: The five contracts keep the departures of their implementation

## Status

Accepted

## Context

The accepted design binds the name of a template's import in the name assignment of the file. It
clears the spelling of a composite that `rules.Substitute` restates, and lets a speller restate the
composite or refuse it. It also gives a view the resolver of the rules, lets the rules of a language
bless a resolved type as a well-known type, and refuses a drop of a derived metadata key.

The implementation met three places where the design is silent or where its signatures cannot keep
the facts that a spelling records. Each place is listed here, because an accepted design is not
amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| `ImportSet.Use` returns an error where its name binds another path or a declaration of the file | `Use` also returns an error where the file imports the same path or item under another name | The journal of the import set withdraws a binding by its path or item. A second name for one binding would let the rollback of a skipped declaration withdraw the binding of an earlier declaration |
| The Go backend restates every composite without a spelling through `golang.Restate` | The speller of the Go backend returns an error for a channel and for a function type whose last parameter is a list, where the substitution cleared the spelling. `golang.Restate` is unchanged | The model records the direction of a channel and the variadic mark of a parameter of a function type in the spelling alone. A restatement without them writes a bidirectional channel and a slice, which changes the type. The Go spoke calls `golang.Restate` for a translated channel, which has no spelling either |
| `typescript.Restate` refuses an optional, an array, a stream, a borrow, a wildcard and an inline type | It also refuses a function type whose last parameter is a list, and a function type with more than one result | The model records whether the last parameter of a function type is a rest parameter in the spelling of the function type alone. TypeScript returns one type from a function |

## Alternatives Considered

### Record the direction of a channel and the rest mark in the model

A field on the reference would let both restatements write every composite. It is a change of the
model, which a design of its own decides with the readers that it gains.

### Keep the spelling of a substituted channel

The written spelling contains the type parameter, so a backend would write the parameter where the
argument belongs. That is the defect that the design removes.

## Consequences

- A generic declaration whose substituted type contains a channel, or a function type whose last
  parameter is a list, renders without the declaration and reports the refusal of the speller.
- A template that imports one package under two names refuses its declaration with
  `RefusedTemplate`.

## References

- [RFC-0037](../rfc/0037-five-contracts-that-the-reviewed-defects-need.md)
