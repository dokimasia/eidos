---
adr: 0022
title: A rules implementation reads a named key through rules.Fact
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

# ADR-0022: A rules implementation reads a named key through rules.Fact

## Status

Accepted

## Context

The accepted design hides the fact store and the read set of `rules.View` behind `rules.NewView`.
It gives a rules implementation two tracked reads: `rules.Fact` with a handle, and `rules.FactNamed`
with the name of a key.

The rules of Go, TypeScript and protobuf read keys that other registrants register. Go reads the
constant values and the type sets, TypeScript reads the namespaces, and protobuf reads the labels
and the closed markers. Each implementation looked the key up in the registry of the raw fact
store, and then read through `rules.Fact` with the handle that the lookup returned.

`meta.Get` and `meta.Fact` already resolve a handle from `meta.Named` by its name, in the registry of
the store that they read. A name that the registry does not contain under the value type reads
absent. `meta.Fact` records the read by the name of the key in both cases.

## Decision

The view has no `rules.FactNamed`. A rules implementation reads a key of another registrant through
`rules.Fact` with a handle from `meta.Named`, as the predicate of a plugin names such a key.

- The docblock of `rules.Fact` states that it resolves a named handle by its name.
- The Go rules keep one handle for the constant values and one for the type sets. The TypeScript and
  protobuf rules build the handle at each read. Building a handle does not allocate.

## Alternatives Considered

### rules.FactNamed

`rules.FactNamed[T](v, id, name)` reads the key with a name, as the accepted design specified. Its
body is `rules.Fact(v, id, meta.Named[T](name))`.

It lost because it adds a second spelling of one tracked read. The accepted design counts two
spellings of a tracked read as a defect of the authoring API.

## Consequences

**Positive:**

- A rules implementation has one tracked read, and the view does not export a second spelling of
  it.

**Negative:**

- A view without facts and a view whose registry does not contain the key read absent alike. The Go
  rules then derive int for an empty interface bound in a view without facts, because such a view
  does not read a type set. The registry lookup refused that bound before. Every view that a run
  builds has facts.
- A read of a named key resolves the name in the registry on every call. The enumeration projection
  of Go resolves it once for each variant, where the lookup ran once for each enumeration before.

**Neutral:**

- A read of a key that the registry does not contain records an edge by its name. The registry of a
  composition does not change during a run, so the run never stamps a fact of that name, and the
  edge never marks a result dirty.

## References

| What | Where |
|---|---|
| RFC-0030, one tracked fact read | [0030-the-contracts-of-the-spi-and-the-authoring-api.md](../rfc/0030-the-contracts-of-the-spi-and-the-authoring-api.md) |
