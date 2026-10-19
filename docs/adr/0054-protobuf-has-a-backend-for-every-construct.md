---
adr: 0054
title: protobuf has a backend for every construct of its five versions
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0067
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0054: protobuf has a backend for every construct of its five versions

## Status

Accepted

## Context

protobuf was read-only, because a schema language is read and never written. That decision left
two kinds of generator without a target:

- A generator that derives a schema from the declarations of Go, TypeScript, Java or Rust.
- A generator over protobuf sources that writes a schema, such as one that adds the options of a
  validation library to the fields of a schema.

The residue of protobuf is a set of stamps on the node declarations, such as the wire number of a
field and the reserved ranges of a message. A backend reads the emit model alone, and the render
context has no fact store. A generator therefore states each construct in the emit model. A target
for a subset of protobuf cannot write the schemas that use the rest.

## Decision

We will write protobuf through a backend that writes every construct of proto2, proto3 and the
editions 2023, 2024 and 2026, because generators from other languages and generators over protobuf
sources need a protobuf target.

- A generator states each construct through one of five channels: a field of the model, the tag
  entry `number` of a field, an annotation for an option, a marker for a keyword or a statement that
  is not an option, and the header of a file.
- The header marker `protobuf:syntax` selects the version of a file, and a file without it is
  proto3. The templates write each construct in the syntax of the version of its file. A construct
  that the version does not spell fails its kind template under `RefusedTemplate`.
- The backend writes a shape without a protobuf form as the wrapper message that the protobuf
  ecosystem uses, and the signature of a method as a request and a response message. It numbers the
  fields of a message that it synthesizes by their position.
- `protobuf.Stated` and `protobuf.Header` restate the stamps of a protobuf source in the five
  channels.
- The toolchain adapter compiles each written file through the experimental compiler of
  protocompile at the pinned commit.

## Alternatives Considered

### Keep protobuf read-only

A reader of protobuf does not need a backend, but the generators of the context have no other
target.

### Write editions alone

Edition 2023 expresses every construct of proto2 and proto3 through features. A consumer whose
toolchain does not read editions could not use the output, and a generator that adds options to a
proto3 schema would rewrite every file of the schema into another syntax.

### A field of the model for the number

`Field.Number` with a fact of its own would state the number in the model. Every other backend would
state one more verdict, and the protobuf frontend would move the number out of its stamp. The tag of
a field is a slot that a target joins into its tag syntax already.

### A printer of descriptors

protoreflect's `protoprint` prints descriptors to `.proto` files. A backend over it would bypass the
kind templates, so the template overrides and the slots of plugins would not apply.

## Consequences

**Positive:**

- A plan from any source language can write protobuf, and the hub has a protobuf spoke.
- A generator over protobuf sources writes a schema as its source states it, through `Stated` and
  `Header`.

**Negative:**

- A refusal for the version of a file happens at render and not at the settle, because the layout
  routes a declaration into its file after the settle.
- The number and the options are text. A wrong value fails the compile of the toolchain adapter and
  not the run.
- A synthesized message numbers its fields by position, so a parameter added in front of another one
  renumbers the fields after it.

**Neutral:**

- The emitter copies a reference of a protobuf source as written into a plan of the protobuf target,
  so a mirror keeps the encodings `sint32` and `fixed64` of its source.

## References

| What | Where |
|---|---|
| RFC-0067, the design of this decision | ../rfc/0067-the-protobuf-backend.md |
| D164, the row of this decision, which supersedes D7 | ../architecture/21-decisions.md |
