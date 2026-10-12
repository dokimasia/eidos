---
adr: 0028
title: The facts of the model keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0036
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0028: The facts of the model keep the departures of their implementation

## Status

Accepted

## Context

The accepted design gives the model a field for each fact that a frontend parses and the model
dropped: the optional mark, the mutability of an index signature and the computed mark of a method,
the computed mark of a field, the mutability and the absent members of a reference, and the
underlying type of an enum. The rules read the fields, and each backend and spoke renders a fact,
refuses it or rejects its declaration.

The implementation met places where the design states a verdict or a spelling that the code cannot
keep, and places where the design is silent and its invariants decide. Each place is listed here,
because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| The Go coverage rejects an abstract declaration | The Go coverage refuses an abstract declaration, as it did before | The Go frontend marks every method of a Go interface abstract, and the TypeScript frontend every method of a TypeScript interface. The Go backend renders an abstract interface method. A rejection would leave out every interface with a method |
| The Go coverage rejects the mutability of a field or a variable | The mutability of a field or a variable renders, and the mutability of a method is rejected through an exception of the method kind | The Go and TypeScript frontends state a mutable binding on every variable and field. Go writes a mutable binding without a keyword, and the Go guard returns an error for an immutable one |
| The Rust spoke spells a borrow as `&mut T` or `&T` by its mutability | The Rust spoke refuses a borrow of either mutability, as it did before | A Rust borrow in a field needs a lifetime, and a shape has none. The mutability of the borrow does not supply one |
| The spokes refuse the read-only forms that the design lists | The Go spoke also refuses an immutable array, the Rust spoke an immutable array and tuple, and the Java spoke an immutable array | A holder can write through each of these collections, and a spoke never drops a constraint of a shape |
| The TypeScript spoke writes a read-only list and tuple | It also writes an immutable map as `Readonly<Record<K, V>>`, `Readonly<Partial<Record<K, V>>>` or `ReadonlyMap<K, V>`. `Readonly` joins the global types of the satellite | TypeScript has a spelling for each form, and a spoke refuses only a form without a spelling |
| The design names no spelling of a borrow in TypeScript and Java | The TypeScript and Java spokes write a borrow of either mutability as its type, as they did before | A borrow is the mode in which a callee receives a value, and neither language states a mode. A refusal would refuse every Rust function with a shared reference |
| The Rust backend writes the repr attribute of an underlying type | The frontend lifts the integer type out of the repr attribute, so `#[repr(C, u8)]` leaves the annotation `repr(C)` and the underlying type `u8`. The backend writes `#[repr(u8)]` after the other attributes | The fact has one record, and a mirror does not write the integer twice |
| The Go rules bound the value outside a set by the underlying type | The rules compare values as int64, and an integer of the platform width has the range of 32 bits | Go defines int and uint as 32 or 64 bits wide, and a generated constant must compile on every platform |
| The design adds no fact to the restatement of a reference | `rules.EmitRef` copies the asynchrony and the mutability of a reference | The emit model has both fields, and the restatement dropped the asynchrony of a stream before |
| The fixture adapter of TypeScript writes a read-only list and tuple | It also writes an immutable map and an optional with its absent members, such as `T\|null` | The parity test compares the adapter with the frontend, and the frontend writes these spellings |

## Alternatives Considered

### Keep every verdict of the design

The Go backend would leave out every interface and every struct that a frontend loads, because each
has an abstract method or a mutable field. The verdict would reject the declarations that the
backend renders today.

### Value-dependent verdicts in the coverage

The coverage could contain a verdict for each value of a fact, such as one for a mutable binding and
one for an immutable one. The coverage table of every backend would grow by a dimension, and the guard
of the vocabulary already returns an error for the value that a target cannot write.

## Consequences

**Positive:**

- Every declaration that rendered before the facts rendered after them, and the new findings report
  only facts that a target drops or cannot write.

**Negative:**

- A reader of the design finds two Go verdicts and one Rust spelling that the code does not keep.
- A mutable binding renders in Go without a finding, and an immutable one fails in the vocabulary
  instead of in the guard.

**Neutral:**

- The departures do not change a field of the model or a stored byte beyond the design.

## References

| What | Where |
|---|---|
| RFC-0036, the facts that the model drops | [0036-the-facts-that-the-model-drops.md](../rfc/0036-the-facts-that-the-model-drops.md) |
