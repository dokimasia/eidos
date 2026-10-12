---
adr: 0023
title: A template passes a signature slot to its helper by address
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0030
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0023: A template passes a signature slot to its helper by address

## Status

Accepted

## Context

The accepted design turns the parameters, the results and the embeds of an emitted declaration, and
the tag of an emitted field, into slots. It states that a backend template reads `.Params.Items` and
`.Returns.Items`, as it reads `.Fields.Items`.

text/template calls a method through reflection. For each call of `Items`, it looks the method up by
name, builds the function type, binds the receiver and stores the result. A profile of the render of
the Go backend shows about six allocations for each such call. text/template passes the address of an
addressable field to a function that takes a pointer, and that argument costs no allocation.

The render of the scaled corpus of 200,000 declarations measured these counts on 2026-10-12:

| Backend | Before the slots | With `.Items` in the templates |
|---|---|---|
| Go | 19,385,404 | 21,630,373 |
| Rust | 8,131,154 | 8,756,751 |

Both counts with `.Items` exceed the budgets of the renders.

## Decision

A template passes a slot of a signature to its helper by address, and reads `.Items` only where it
ranges over the entries outside the render of a backend.

- The `params` and `results` helpers of the Go, TypeScript, Java and Rust backends take the slot by
  address. A template calls them as `{{params .Params}}` and `{{results .Returns}}`, and a direct
  caller passes `&m.Params`.
- The Go backend ranges over the embeds of a struct or an interface through the helper `embeds`, the
  method expression of `Slot.Items`, as `{{range embeds .Embeds}}`. It writes the tag of a field
  through the helper `tag`, which takes the field.
- A body template of a plugin, such as a mock or a stub of the reference ensemble, ranges over
  `.Params.Items` and tests a slot with `.Returns.Len`.

## Alternatives Considered

### `.Params.Items` in every template

Every template reads a slot through its `Items` method, as the accepted design states. It lost
because each read costs about six allocations: 2,244,969 more in the Go render, 11.6%, and 625,597
more in the Rust render, 7.7%.

### A helper that takes `any`

A helper takes the slot or a slice as `any` and switches on its type. It lost because text/template
allocates an interface value for each argument of an interface type, and the switch adds a second
form of the same argument.

### A slot as a slice type

A slot is a slice type, so a template ranges over it without a call. It lost because a slot keeps
the seal flag of its store beside its entries, and a slice type has no room for the flag.

## Consequences

**Positive:**

- The renders of TypeScript, Java and Rust count within 551 allocations of their counts before the
  slots. The Go render counts 19,741,361, 1.8% above its count before the slots, from the calls of
  `embeds` and `tag`.

**Negative:**

- `Speller.Params` and `Speller.Results` take a `*emit.Slot`, so a direct caller with a slice builds
  a slot first.
- A template that passes `.Params.Items` to `params` parses and fails at render with a type error.

**Neutral:**

- The kind templates read the signature slots as they read the slices before the slots, so the
  template text of `params` and `results` calls does not change.

## References

| What | Where |
|---|---|
| RFC-0030, slots for tags, embeds, parameters and returns | [0030-the-contracts-of-the-spi-and-the-authoring-api.md](../rfc/0030-the-contracts-of-the-spi-and-the-authoring-api.md) |
