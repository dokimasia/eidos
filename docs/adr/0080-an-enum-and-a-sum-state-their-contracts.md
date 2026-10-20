---
adr: 0080
title: An enum and a sum state the contracts that they implement
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0079
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0080: An enum and a sum state the contracts that they implement

## Status

Accepted

## Context

The model gave a struct its contracts in `Implements` and gave an enum and a sum none. The Java
frontend lowered `enum Op implements IntBinaryOperator` without its interfaces. The Rust frontend
added the trait of an impl block to a struct alone, so `impl std::error::Error for StoreError {}`
on an enum was lost at the load. The Rust rules then reported such an enum as no error type, and
they missed a hand-written `impl PartialEq` and `impl Default` of an enum.

Java, Kotlin and C# enums implement interfaces, Swift enums conform to protocols, and Rust
implements a trait for any type.

## Decision

We will state the contracts of an enum and a sum in `Implements` on both sides of the model, under
the fact `Implements`, because the kernel's member walk, the rules and the backends read the model
and no stamp of a language.

- The Java frontend reads the interfaces of an enum in source and in a class file. The Rust
  frontend folds a trait impl onto a struct, an enum or a data enum.
- The member walk and the Rust rules read the `Implements` of an enum and a sum where they read
  those of a struct.
- Java writes the contracts of an enum after `implements`, and the sealed interface of a sum
  extends the contracts of the sum.
- TypeScript, Python and GraphQL give each variant of a sum its own copy of the contracts. They
  refuse the contracts of an enum, and so do Rust and protobuf.
- The coverage of Go takes the verdict `Holds` for both kinds, because a Go type satisfies an
  interface without a clause.

## Alternatives Considered

### A stamp of the contracts

The frontends would stamp the contracts of an enum under a key of the language. The member walk and
every backend would then read one stamp for each language.

### The contracts on each variant of a sum

A sum would state the contracts on its variants. Rust and Java implement a trait or an interface for
the whole type, so every variant would repeat one list.

## Consequences

**Positive:**

- A Java enum keeps its interfaces from the load to the render.
- The Rust rules answer the error contract, the equality and the zero value of an enum and a data
  enum from its trait impls.

**Negative:**

- TypeScript, Python, Rust, protobuf and GraphQL refuse the contracts of an enum. A generator that
  copies the contracts of a Java enum into TypeScript reports a refusal at the enum.
- Each variant of a sum allocates its own copy of the contracts in TypeScript, Python and GraphQL.
- Java refuses the methods of a sum, so a Java sum whose contract declares an abstract method lowers
  to variant classes that javac rejects.

## References

| What | Where |
|---|---|
| RFC-0079, the design of this decision | ../rfc/0079-the-implements-clause-of-an-enum-and-a-sum.md |
| D198, the row of this decision | ../architecture/21-decisions.md |
| Issue 44, the implements clause of an enum and a sum | <https://github.com/dokimasia/eidos/issues/44> |
