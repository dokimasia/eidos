---
adr: 0085
title: The member walk lists a member with a computed name
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

# ADR-0085: The member walk lists a member with a computed name

## Status

Accepted

## Context

A field or a method of TypeScript can have a computed name, such as `[Symbol.iterator]`. The model
marks such a member with the fact `Computed` and keeps the brackets in its name.

- The member walk left such a member out and recorded a gap with the reason `GapComputed`. A
  reader of the walk learned that the member set was incomplete.
- The library of TypeScript declares computed members in central interfaces. The one member of
  `AsyncDisposable` is `[Symbol.asyncDispose](): PromiseLike<void>`, and `Iterable` declares
  `[Symbol.iterator]`. The walk of each such interface reported a gap after the library loaded. The
  walk of every interface that extends one reported the same gap.
- The backends of Go, Java, Rust, protobuf and GraphQL reject a declaration with the fact
  `Computed`, with an Error at the declaration. The Python backend refuses the fact with a
  warning. The TypeScript backend writes the member.

## Decision

We will list a field or a method with a computed name in the member walk under its name as
written, because a generator of TypeScript implements such members and every other backend refuses
the fact already.

- The walk records the member under its name with the brackets, such as `[Symbol.asyncDispose]`.
  Two arrivals of one computed name merge or shadow each other as two arrivals of any name do.
- `GapReason` has no `GapComputed`, and neither does the facade of the SDK.
- A generator for a language without computed names reads `Computed` on the field or the method of
  a member.

## Alternatives Considered

### The gap of the walk

The walk would keep the gap. A generator that needs the whole member set would decline every
interface that extends `AsyncDisposable`, `Iterable` or `AsyncIterable`.

### A policy that lists computed members for TypeScript alone

The member policy would gain a field that lists computed members. Only the TypeScript frontend sets
`Computed`, so the field would have one value in every language. The gap would remain for a reader
that no language serves.

## Consequences

**Positive:**

- The walk of `AsyncDisposable` has one member and no gap. The walk of an interface that extends it
  is complete.
- A generator of TypeScript writes `[Symbol.asyncDispose]` and `[Symbol.iterator]` from the member
  set.

**Negative:**

- The walk does not mark a computed member as a gap. A reader for a language without computed names
  checks `Computed` on each member itself.
- The removal of `GapComputed` breaks the kernel and the SDK.

## References

| What | Where |
|---|---|
| RFC-0082, the design of this decision | ../rfc/0082-the-declaration-files-of-the-typescript-library.md |
| D203, the row of this decision | ../architecture/21-decisions.md |
| Issue 48, the members of an interface that extends the library | <https://github.com/dokimasia/eidos/issues/48> |
