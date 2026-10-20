---
adr: 0083
title: The spoke spells a type parameter at every depth of a type
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0081
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0083: The spoke spells a type parameter at every depth of a type

## Status

Accepted

## Context

A generator that translates a generic declaration into another language writes the type of each
member through `Emitter.Type`.

- The fold gave a reference to a type parameter the opaque shape, because the argument of the
  parameter decides its shape. No spoke spells an opaque shape.
- `Emitter.Type` spelled a reference to a type parameter by its name before the fold. A member of
  the type `T` translated, and a member of the type `list[T]` did not. The protobuf and GraphQL
  targets received the name of a parameter as a type, although neither language has type
  parameters.
- The TypeScript client of the shop fixture of the Python satellite left out the alias
  `Pair: TypeAlias = tuple[T_co, T_co]`. TypeScript had no spelling of the opaque shape of `T_co`.
- The declarative plugin translated the bounds of type parameters and the interface of an
  implementation through `Emitter.Type`. Rust spells a value of a trait as `Box<dyn Store>`, which
  is neither a bound nor the trait of an impl.

tsc 7.0.2 reports TS1274 for a variance annotation on a type parameter of a function or a method.
It reports TS2637 for one on an alias of a reference, a union, a list or a tuple. The TypeScript
backend wrote the declared variance of every type parameter.

## Decision

We will keep the identity of a type parameter in its opaque shape and pass the shape to the spoke
at every depth of a type because only the spoke of a target knows whether its language has type
parameters.

- The fold returns the form `Opaque` for a type parameter, with `Ref` set to the identity of the
  parameter. A reader that switches on the form, such as the derivation of samples, reads an
  unknown type as before.
- `Emitter.Type` and `Emitter.Supertype` spell no type parameter before the fold. A plan without a
  spoke refuses a type parameter as it refuses every other reference of another language.
- `spoke.Param` returns the name of the parameter without a target, and `spoke.Describe` returns
  "a type parameter" for the shape.
- `Emitter.TypeParams` translates the type parameters of a declaration. It translates their bounds
  as supertypes, and their defaults and the types of their values as types. The declarative
  plugin calls it, and so does the client generator of the shop. The declarative plugin
  translates the interface of an implementation through `Emitter.Supertype`.
- The spokes of Go, TypeScript, Java, Rust and Python spell a type parameter by its name. The
  spokes of protobuf and GraphQL refuse it.
- The TypeScript lowering leaves out the variance of the type parameters of a function, of a
  method, and of an alias whose target is neither a function type nor an object type. TypeScript
  infers the variance of a generic type from its structure.

## Alternatives Considered

### A form of a type parameter

The kernel would add a leaf form for a type parameter. Every switch on the forms of a shape in the
kernel, the satellites and the plugins would need a branch for it. Each reader that treats a
parameter as an unknown type would read the new form as opaque.

### The parameters in the emitter

The emitter would replace each parameter of a reference with its name before the fold. The fold
folds a whole reference, so the emitter would rebuild every structural form around a parameter,
such as the list of `list[T]`.

### A translation of type parameters in each generator

Each generator would translate the bounds and the defaults of its type parameters itself. The
declarative plugin did, and it translated a bound through `Emitter.Type`. A Rust plan of the plugin
would then bound a type parameter by `Box<dyn Store>`.

### A refusal of the variance in TypeScript

The TypeScript backend would refuse a declaration whose type parameter states a variance that
TypeScript does not accept there. The Python alias `Pair` would not translate, and neither would a
Python function over a covariant type variable.

## Consequences

**Positive:**

- A generator writes each member of a generic declaration whose type contains a parameter, such as
  `readonly [T_co, T_co]` in TypeScript and `[]T` in Go.
- protobuf and GraphQL refuse a type parameter with a finding at its owner. Neither writes the name
  of a parameter as a type.
- A bound takes the supertype spelling of the target in every generator that translates type
  parameters through the Emitter.

**Negative:**

- A type parameter at the top of a reference refuses in protobuf, in GraphQL and in a plan without
  a spoke, where the emitter spelled its name.
- A TypeScript alias of a tuple or a union, a TypeScript function and a TypeScript method lose the
  variance of their source. tsc checks the variance that it infers instead.
- The shape keeps the identity of a parameter and not its bound. A reader that derives a sample
  from the bound reads an unknown shape.
- The output of the backends of all seven languages changes. Each backend bumps its version, so the
  fingerprint of each composition changes.
- The Emitter has one more method. A generator that translates a type parameter by hand still has
  to translate each bound through `Emitter.Supertype`.

## References

| What | Where |
|---|---|
| RFC-0081, the design of this decision | ../rfc/0081-a-type-parameter-inside-a-structural-type.md |
| D201, the row of this decision | ../architecture/21-decisions.md |
| Issue 41, a type parameter inside a structural type | <https://github.com/dokimasia/eidos/issues/41> |
| The TypeScript handbook, variance annotations | <https://www.typescriptlang.org/docs/handbook/2/generics.html> |
