---
rfc: 0079
title: The implements clause of an enum and a sum
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-20
updated: 2026-10-20
discussion: https://github.com/dokimasia/eidos/issues/44
supersedes: none
superseded-by: none
produces-adr: ADR-0080
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0079: The implements clause of an enum and a sum

## Summary

We propose that an enum and a sum state the contracts that they implement, as a struct states them.
`Enum` and `Sum` gain `Implements` on both sides of the model, under the fact `Implements`. The Java
frontend fills it from the interfaces of an enum, and the Rust frontend from the trait impls of an
enum and a data enum. The kernel's member walk and the Rust rules read it as they read the
`Implements` of a struct, and every backend states a verdict on it for both kinds.

## Motivation

- The Java frontend lowers `enum Op implements IntBinaryOperator { … }` without its interfaces, because
  the model has no place for them.
- The Rust frontend adds the trait of an impl block to a struct alone, so `impl std::error::Error for
  StoreError {}` on an enum is lost at the load.
- The Rust rules read `Implements` for three answers, and each is wrong for an enum and a data enum.
  `ErrorContract` reports the enum `StoreError` of the issue as no error type. The member walk under
  `ContributesImplements` gives an enum none of the methods of its traits. `Comparable` and
  `ZeroValue` miss a hand-written `impl PartialEq` and `impl Default`.

Java, Kotlin and C# enums implement interfaces, Swift enums conform to protocols, and Rust implements
traits for any type.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/symbol/schema` | `Implements` on `Enum` and `Sum`, under the fact `Implements`, and the regenerated model |
| Kernel | `eidos-core/rules` | the member walk reads the `Implements` of an enum and a sum |
| Kernel | `eidos-core/toolchain` | the surface of a package lists the contracts of an enum and a sum |
| SDK | `eidos-sdk` | the regenerated facade |
| Java | `eidos-lang-java` | the frontend reads the interfaces of an enum, and the backend writes the contracts of an enum and a sum |
| Rust | `eidos-lang-rust` | the frontend folds a trait impl onto an enum and a data enum, and the rules read it |
| TypeScript, Python, GraphQL | the backends | a verdict on the fact for an enum and a sum |

### The model

```go
package schema

type Enum struct {
    // The other fields are unchanged.
    Implements []*TypeRef `eidos:"both,walk,fact=Implements"` // the contracts that the enum satisfies
}

type Sum struct {
    // The other fields are unchanged.
    Implements []*TypeRef `eidos:"both,walk,fact=Implements"` // the contracts that every variant satisfies
}
```

The fact `Implements` exists for a struct, so every backend has a base verdict on it. A backend
whose verdict differs for an enum or a sum states an exception of the kind.

### The frontends

- The Java frontend reads the `interfaces` of an `enum_declaration`, as it reads them of a class. The
  reader of class files reads the interfaces of the class file of an enum.
- The Rust frontend folds the trait of a trait impl onto the declaration that the impl names: a
  struct, an enum or a data enum. The fold does not record the trait of a negative impl.
- The other frontends do not read such a clause. An enum of TypeScript or protobuf does not
  implement an interface, and neither does an enum or a union of GraphQL. Python writes the mixin of
  an enum as its `Underlying`.

### The rules

- The member walk follows the `Implements` of an enum and a sum under `ContributesImplements`.
- The Rust rules read the `Implements` of an enum and a sum where they read those of a struct:
  `ErrorContract`, `Comparable` and `ZeroValue`.

### The backends

| Target | Enum | Sum |
|---|---|---|
| Go | holds, because a Go type satisfies an interface without a clause | holds |
| Java | renders `enum Op implements IntBinaryOperator` | renders: the sealed interface that the sum lowers to extends the contracts |
| TypeScript | refuses, because a TypeScript enum implements nothing | renders: each variant interface extends the contracts |
| Python | refuses, because the metaclass of an enum conflicts with the metaclass of a Protocol | renders: each variant class takes the contracts as its bases |
| Rust | refuses, as for a struct, because an impl block owes the items of the trait | refuses |
| protobuf | refuses | refuses |
| GraphQL | refuses, because only an object type and an interface implement | renders: the object type of each variant implements the contracts |

TypeScript, Python and GraphQL give each variant its own copy of the contracts, because the settle
rewrites a reference in place.

### Failures

| Failure | Effect |
|---|---|
| A stated contract that the target refuses | the render reports it under `RefusedFact` and writes the declaration without it |
| A trait impl of a type that the crate does not declare | the fold records the trait nowhere |
| A GraphQL sum with contracts under the marker `graphql:input` | the lowering refuses the sum, because an input object implements no interface |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| `Implements` on an enum and a sum | in | Java, Kotlin, C#, Swift and Rust state the contracts of both kinds | none known |
| `Extends` on an enum | out | no language that eidos reads states a superclass of an enum, and Python's mixin is the `Underlying` of the enum | a language whose enum extends a class |

## Alternatives

### A stamp of the contracts

The frontends could stamp the contracts of an enum under a key of the language. The kernel's member
walk and the backends read the model, so each would read the stamp of each language.

### The contracts on each variant of a sum

A sum could state the contracts on its variants. Rust and Java implement a trait or an interface for
the whole type, so every variant would repeat one list.

## Drawbacks

- `Enum` and `Sum` gain one field, so each gains one more fact to state a verdict on.
- TypeScript, Python, Rust, protobuf and GraphQL refuse the fact on an enum. A generator that copies
  the contracts of a Java enum into TypeScript reports a refusal at the enum.
- A backend that renders the contracts of a sum writes them on every variant, as TypeScript, Python
  and GraphQL do. Each variant allocates its own copy of the contracts.
- Java refuses the methods of a sum, so a Java sum whose contract declares an abstract method lowers
  to variant classes that javac rejects.

## Open questions

None.

## Unresolved and future work

- The superclass of an enum.

## References

| What | Where |
|---|---|
| Issue 44, the implements clause of an enum and a sum | <https://github.com/dokimasia/eidos/issues/44> |
| The Java Language Specification, enum classes | <https://docs.oracle.com/javase/specs/jls/se21/html/jls-8.html#jls-8.9> |
