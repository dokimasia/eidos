---
adr: 0015
title: The delimited mark is stamped at the parse
status: Accepted
date: 2026-10-10
supersedes: none
superseded-by: none
rfc: RFC-0024
---

# ADR-0015: The delimited mark is stamped at the parse

## Status

Accepted

## Context

The marker `protobuf.delimited` marks a field whose message is encoded delimited. A proto2
group's field has it, and so does a field whose resolved `message_encoding` is `DELIMITED`. The
feature applies to a field of a message type alone, and protoc ignores it for a field of an enum
type.

The frontend stamps the marker while it parses a file, as it stamps the rest of protobuf's
residue. At the parse, a field's type is a spelling. A scalar type is a keyword, such as `int64`.
A message and an enum are both type names, and the link step resolves a type name only after
every unit is parsed. A file's `message_encoding` applies to every field that does not set its
own, so a file that sets `DELIMITED` applies it to the fields of an enum type as well.

The presence of a message field has the same problem. The protobuf rules report it through
`PresenceRules` after the link step. The projection then gives the field's type the optional
form. Every target translates that form.

## Decision

We will stamp the delimited marker at the parse on every field whose type is not a scalar and
whose resolved `message_encoding` is `DELIMITED`, because the frontend stamps the rest of
protobuf's residue at the parse and the marker does not change the field's type.

- A field of an enum type under an inherited `DELIMITED` has the marker.
- The docblocks of the field lowering and of `DelimitedKey` state this.

## Alternatives Considered

### A rule after the link step

A rule of the protobuf satellite reports whether a field is delimited. It reads the declaration
that the field's type resolves to, as `PresenceRules` does for presence.

It lost because dispatch selects the subjects of a generator's rules by fact keys. The result of a
projection rule is not a fact. The projection does not need the marker either, because the marker
does not change the field's type.

### Skipping a type that the file declares as an enum

The parse looks up the field's type among the enums of its own file. It does not mark a field
whose type is one of those enums.

It lost because the marker would then depend on where the enum is declared. A field of an enum of
the same file would have no marker. A field of an imported enum would have one.

## Consequences

**Positive:**

- The marker is a fact like the rest of the residue, and one pass of the parse sets it.

**Negative:**

- A field of an enum type under an inherited `DELIMITED` has a marker that protoc ignores. A
  consumer of the marker has to check that the field's type resolves to a message.

**Neutral:**

- A proto2 group's field has the marker because of the group's syntax. The resolved feature does
  not change that.
