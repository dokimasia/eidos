---
adr: 0056
title: A field declared outside its type has Receives
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

# ADR-0056: A field declared outside its type has Receives

## Status

Accepted

## Context

The protobuf frontend refused an `extend` block at its position, because the model had no form for a
field that a message of another file receives. A schema lost each custom option that it declared. The
decision named its trigger: a consumer that needs an extension of a message that another file
declares. The protobuf backend writes every construct of the language, `extend` included.

`Method.Receives` names the type that a method attaches to when the method is declared outside that
type, as a Kotlin extension function and a Go method of a type of its file are. `Field` has no such
reference. A stamp of the extended message would not resolve. A generator into Go could then not
spell the extended message.

## Decision

We will give `Field` the reference `Receives` with the fact `Receives`, and load each field of an
`extend` block as a field whose `Receives` names the extended message, because an extension is a
field declared outside the type that it attaches to.

- A field of a block at file level is a declaration of the file. A field of a block inside a message
  is a nested declaration of the message. The message is the scope of the extension, not the type
  that it extends.
- The load gives such a field the identity of its declaring scope. The resolution step resolves
  `Receives`, and the importer role records the file of the extended message.
- The name table lists a field with `Receives` under the spelling of its `Receives`, as it lists a
  method with `Receives`.
- The protobuf rules leave a field with `Receives` out of the members of its host. The protobuf and
  GraphQL backends write it in an `extend` block. Go, TypeScript, Java and Rust refuse the fact.

## Alternatives Considered

### Extensions as stamps

The frontend could stamp the extended message of each extension field as text. A reference in a
stamp does not resolve, so a generator into Go could not spell the extended message.

### An extend block as a struct

Each block could load as a struct whose `Extends` names the extended message. A block has no name,
and a struct named after the extended message collides with every other block of the package that
extends it.

## Consequences

**Positive:**

- A schema keeps its custom options and its extensions at load, and a generator writes them back.
- Kotlin, Swift and C# extension properties have a form in the model.

**Negative:**

- The schema gains a field and a fact, so every sealed state runs cold once, and every backend states
  one more verdict.
- A protobuf extension that clashes with a message of its package fails in the compiler and not in
  the settle.

**Neutral:**

- `Method.Receives` keeps its form and has no fact.

## References

| What | Where |
|---|---|
| RFC-0067, the design of this decision | ../rfc/0067-the-protobuf-backend.md |
| D166, the row of this decision, which supersedes D112 | ../architecture/21-decisions.md |
