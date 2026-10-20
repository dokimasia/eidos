---
adr: 0081
title: A function and a method mark an overload signature
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0080
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0081: A function and a method mark an overload signature

## Status

Accepted

## Context

TypeScript and Python split an overloaded callable into overload signatures without a body and one
implementation. The model had no field for the split.

- The TypeScript frontend loaded each overload signature as a function and left out the
  implementation. The TypeScript backend wrote a body for every function and refused a class with
  two methods of one name.
- The Python frontend marked an overload with the decorator `overload` and the implementation with
  a stamp of its own. No backend of another language read either of them.
- The discriminator of a callable spells its parameters. TypeScript code often states an overload
  whose parameters spell as the parameters of the implementation. The implementation then had the
  identity of that overload.

tsc 7.0.2 reports TS2391 for overload signatures without an implementation. pyright 1.1.413
reports `reportNoOverloadImplementation` for a module without one.

## Decision

We will mark an overload signature with `Overload` on `Function` and `Method` because a backend of
TypeScript or Python writes a signature without a body and its implementation with one.

- The mark is under the fact `Overload`. The callable of the name without the mark is the
  implementation.
- The discriminator of the implementation of an overloaded name starts with `=`. It then differs
  from the discriminator of an overload with the same parameters.
- `rules.IsImplementation` tells the implementation of a method from its signatures. The rules of
  TypeScript and Python leave it out of the constructors of a class.
- The TypeScript frontend marks the signatures of a name that its scope declares two or more
  callables of, and loads the implementation. The Python frontend marks a callable under the
  decorator `overload` of typing, and its stamp of the implementation is removed.
- TypeScript and Python render the mark. Go, Java, Rust, protobuf and GraphQL reject it.
- `lowering.GroupOverloads` puts the signatures of a method name before its implementation in the
  lowering of TypeScript and Python. The member walk keeps the implementation, and `Mirror` leaves
  out the mark.

## Alternatives Considered

### A marker of each satellite

Python would keep the decorator `overload`, and TypeScript would read a marker of its own. A
generator that translates between the two would map one marker to the other. The backends of the
other languages would ignore both markers.

### The body as the mark

A function without a body would be a signature. Every callable of the emit model has a body, and an
empty body is a stub that a generator fills, so no field of the model would tell a signature from
an empty implementation.

### The last function of a name as the implementation

The Python writer treated every function of a name before the last as an overload. The flush follows
the positions of the sources, so the last function of a name is the implementation only where the
source puts it last.

## Consequences

**Positive:**

- A generator that restates an overloaded TypeScript callable writes the implementation that tsc
  requires.
- A generator that writes the overloads of a Python callable into TypeScript writes signatures and
  an implementation.

**Negative:**

- `Function` and `Method` gain one field and the model gains one fact. Each of the seven backends
  states a verdict on it, and so does each coverage table of a test.
- The TypeScript frontend loads one more callable for each overloaded name. A reader that lists the
  callables of a TypeScript module lists the implementation beside the signatures.
- The keys of the Python satellite lose the stamp of the implementation, and the discriminator of
  each Python implementation changes.

## References

| What | Where |
|---|---|
| RFC-0080, the design of this decision | ../rfc/0080-overload-signatures-and-the-order-of-a-file.md |
| D199, the row of this decision | ../architecture/21-decisions.md |
| Issue 42, the overload signatures of TypeScript | <https://github.com/dokimasia/eidos/issues/42> |
| The TypeScript handbook, function overloads | <https://www.typescriptlang.org/docs/handbook/2/functions.html> |
| The specification of the typing of Python, overloads | <https://typing.python.org/en/latest/spec/overload.html> |
