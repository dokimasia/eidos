---
adr: 0086
title: The discriminator of colliding overloads spells their type parameters
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0082
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0086: The discriminator of colliding overloads spells their type parameters

## Status

Accepted

## Context

In a language that overloads, the load spells the discriminator of a callable from the types of its
parameters. Two overloads of one name in one scope then have two identities.

- The DOM declares overloads that differ only in the bound of a type parameter, such as
  `createElement<K extends keyof HTMLElementTagNameMap>(tagName: K)` and
  `createElement<K extends keyof HTMLElementDeprecatedTagNameMap>(tagName: K)`.
- The discriminator of both spelled `(K,ElementCreationOptions?)`. The load reported the second as
  a duplicate and dropped it.
- A probe that loaded the default library of TypeScript reported 17 such duplicates. The author of
  a workspace cannot change the library.
- TypeScript and Java both accept two overloads whose type parameters have other bounds.

## Decision

We will spell the type parameters of an overload whose discriminator another callable of its name
and scope shares, because their bounds tell such overloads apart while every other callable keeps
its identity.

- The type parameters go in brackets before the parameters, comma-joined. Each has its name, `:`
  before each bound and `=` before its default. The first overload of `createElement` becomes
  `createElement([K:keyof HTMLElementTagNameMap]K,ElementCreationOptions?)`.
- A colliding callable without type parameters spells the empty brackets.
- The rule applies to the functions at the top level of a package and to the methods of a type.
- Overloads whose type parameters spell alike too remain a duplicate, and the load reports
  `DuplicateDeclaration` at the second.

## Alternatives Considered

### The bounds in every discriminator

The load would spell the type parameters of every generic callable. The identity of each generic
callable of TypeScript, Python and Java would change, also where no other overload collides with
it.

### The duplicate of the load

Each colliding overload after the first would remain a duplicate under `DuplicateDeclaration`, and
the first would load alone. The DOM would lose 17 overloads, and the load of the default library
would report 17 findings.

## Consequences

**Positive:**

- The default library loads every overload of the DOM without a duplicate.
- A callable that no other callable of its scope collides with keeps its identity.

**Negative:**

- The identity of an overload depends on the other overloads of its scope. An edit that adds a
  colliding overload changes the identity of the overload that was there, and a stamp keyed on the
  old identity no longer applies.
- The discriminator of a colliding overload spells each bound in full, so its identity is longer.

## References

| What | Where |
|---|---|
| RFC-0082, the design of this decision | ../rfc/0082-the-declaration-files-of-the-typescript-library.md |
| D204, the row of this decision | ../architecture/21-decisions.md |
| Issue 48, the members of an interface that extends the library | <https://github.com/dokimasia/eidos/issues/48> |
