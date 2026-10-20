---
adr: 0062
title: A wire field keeps the name that its generator wrote
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0071
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0062: A wire field keeps the name that its generator wrote

## Status

Accepted

## Context

The settle respells every field name in the convention of the target. A generator that writes the
types of a wire format writes keys that a consumer matches by string. The TypeScript respell turned
the key `__typename` of a GraphQL response into `typename`, and `first_name` into `firstName`. The
GraphQL client of the conformance fixture predicted the respell and wrote each such key as a quoted
computed property, so the generator copied the convention of the target.

One declaration of a source can render as a type of a wire format and as a domain type in one plan.
An override on the origin applies to both, so only the generator that emits a declaration knows
whether its fields are keys.

## Decision

We will mark a key of a wire format with `emit.Field.Wire`, which the settle keeps as the generator
wrote it, because a consumer matches the key by string and a respelled key breaks the consumer.

- The settle skips the respell hook and the override of the origin for a wire field. The field joins
  the check of the members of its host.
- TypeScript writes a wire name bare where it is an identifier and quoted otherwise. GraphQL and
  protobuf write it where their grammars admit it. Their lowerings refuse any other name.
- Go, Java and Rust reject a wire field, because each writes a key in a tag, an annotation or an
  attribute.

## Alternatives Considered

### The workaround in each generator

The generator copies the convention of the target. It also quotes a key that TypeScript admits bare.

### An override that the generator stamps

An override is a fact of the origin. One origin renders more than one declaration. The settle also
reads no name at plugin authority, because the lowering stamps one on every declaration that it
renders.

### A respell that keeps every field name of TypeScript

A plan from Go into a TypeScript domain model expects camel case. A key is the exception, and the
generator knows it.

### A wire name of the source

The source states a key through its own syntax, such as a protobuf `json_name`. A generator that
needs that key reads it and writes it as the name of a wire field.

## Consequences

**Positive:**

- The client of a GraphQL response writes `__typename: "Product"`, as graphql-codegen does.
- A generator of the types of a JSON format does not predict the convention of the target.

**Negative:**

- The emit model gains one field and the fact `Wire`, so every coverage table states a verdict for
  it and the sealed state runs cold once.
- A wire name skips the check of reserved words in the respell hook.
- A generator that marks a field of a Go, Java or Rust declaration has its declaration withheld.

## References

| What | Where |
|---|---|
| RFC-0071, the design of this decision | ../rfc/0071-wire-names.md |
| D174, the row of this decision | ../architecture/21-decisions.md |
| graphql-codegen, the specification of its plugin for operations | <https://github.com/dotansimha/graphql-code-generator/blob/master/packages/plugins/typescript/operations/tests/ts-documents.spec.ts> |
