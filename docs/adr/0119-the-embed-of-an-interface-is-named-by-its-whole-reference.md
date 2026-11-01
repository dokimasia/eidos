---
adr: 0119
title: The embed of an interface is named by its whole reference
status: Accepted
date: 2026-11-01
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0119: The embed of an interface is named by its whole reference

## Status

Accepted

## Context

The load named every embed by the bare name of the type that it embeds. A Go interface that embeds
`a.Source` and `b.Source` had two embeds of one identity. The load reported the second
under `DuplicateDeclaration` and dropped it, so a strict run of dokimi failed, and the model of the
interface lacked the second embed. Go accepts such an interface, because the embed of an interface
is not a field. The method set of the interface contains the methods of every embedded interface.

The bare name serves the embed of a struct. Go promotes the embed of a struct as a field under that
name, and the member walk records the field under the same name. Go refuses a struct with two embeds
of one bare name.

## Decision

We will name the embed of an interface by its whole reference, and keep the bare name for the embed
of a struct.

- The name of the embed of an interface is the spelling of its reference with its qualifier and its
  type arguments in angle brackets, as the discriminator of a callable spells a type. The embeds of
  `a.Source` and `b.Source` have the names `a.Source` and `b.Source`, and an embed of `Set[int]` has
  the name `Set<int>`.
- The embed of a struct keeps the bare name of its type. A struct with two embeds of one bare name
  reports `DuplicateDeclaration`, as before.
- An embed keeps the form of a member identity under the owner chain of its host, such as
  `golang:example.com/repro/c.Credentials#a.Source`.

## Alternatives Considered

### A discriminator for the embed of an interface

The embed could keep its bare name and take its qualifier as its discriminator. An identity spells
its discriminator for a callable alone, so two embeds would spell one identity in a finding and in
`explain`.

### The import path of the embedded type

The name could contain the import path of the embedded type. The load assigns the identities before
the link resolves a reference, so the path is not known when the load names the embed.

### A numbered suffix for the second embed

The second embed of one bare name could take a numbered suffix. The suffix follows the order of the
embeds, so a reorder of the source would change the identity of an embed that did not change.

## Consequences

**Positive:**

- An interface that embeds two interfaces of one bare name loads without a finding, and its model
  keeps both embeds.
- A directive on each of the two embeds has a subject of its own.

**Negative:**

- The identity of the embed of an interface no longer contains the bare name alone, so a query of
  `explain` by the earlier identity does not find a declaration.
- An interface that embeds one type under the names of two imports of its package has two embeds
  with two identities.

## References

| What | Where |
|---|---|
| D237, the row of this decision | ../architecture/21-decisions.md |
| The identity of a symbol | ../architecture/02-symbol-model.md |
| The Go specification, embedded interfaces | <https://go.dev/ref/spec#Embedded_interfaces> |
