---
adr: 0063
title: A supertype translates through the spelling of its position
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0072
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0063: A supertype translates through the spelling of its position

## Status

Accepted

## Context

A generator translates every type of another language through `Emitter.Type`, and the spoke of the
target spells the type without its position. Rust spells the two positions of a trait apart. The
type of a value that is a trait is the boxed trait object `Box<dyn Node>`, because Rust 2024 refuses
a bare trait in a type position with E0782 and `dyn Node` alone is unsized. A trait that another
trait extends is the bare name, `trait Named: Node`, and rustc refuses `Box` there. Go, TypeScript,
Java and GraphQL spell both positions alike.

## Decision

We will translate a type that a declaration extends, implements or embeds, or that bounds a type
parameter, through `Emitter.Supertype`, because only the generator knows the position and only the
spoke of the target knows its spelling.

- `Emitter.Supertype` translates as `Emitter.Type` does, and spells the shape through the optional
  role `plugin.SupertypeSpeller` of the backend. A backend without the role spells the supertype as
  a value.
- The backend kit declares the spelling through `Builder.Supertypes`.
- The Rust spoke spells a reference to an interface as `Box<dyn Name>`, and its supertype spelling is
  the flat name with value arguments.

## Alternatives Considered

### A position in the shape

A shape is the type, and the same type takes both positions. Five of the six spokes would ignore the
field.

### A rewrite in the Rust lowering

The lowering would rebuild the written spellings of arrays, tuples and function types around a
rewritten child. The spoke builds those spellings already.

### A supertype that bypasses the spoke

The spoke of a target spells a well-known supertype, such as `std::error::Error` for the well-known
error in Rust. An emitter that builds the flat name itself cannot write that spelling.

## Consequences

**Positive:**

- A field, a parameter and a result of a trait type compile in Rust, and so does a supertrait.
- The spokes of the other targets do not change.

**Negative:**

- A generator chooses the method by position. A supertype that a generator translates through
  `Emitter.Type` becomes `Box<dyn …>` in a Rust supertrait. rustc refuses that supertrait.
- The spoke does not see the methods of a trait, so it boxes a trait that is not dyn-compatible.
  rustc refuses such a field in the toolchain check.

## References

| What | Where |
|---|---|
| RFC-0072, the design of this decision | ../rfc/0072-translated-supertypes.md |
| D175, the row of this decision | ../architecture/21-decisions.md |
| Rust error E0782, a trait object without `dyn` | <https://doc.rust-lang.org/error_codes/E0782.html> |
