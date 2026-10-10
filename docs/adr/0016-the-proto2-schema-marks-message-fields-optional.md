---
adr: 0016
title: The proto2 service schema marks its message fields optional
status: Accepted
date: 2026-10-10
supersedes: none
superseded-by: none
rfc: RFC-0024
---

# ADR-0016: The proto2 service schema marks its message fields optional

## Status

Accepted

## Context

The conformance module's service fixture runs one schema in each of the five protobuf versions:
proto2, proto3 and the editions 2023, 2024 and 2026. Every version must write the same Go file
and the same TypeScript file, so one pair of expected files checks all five runs.

The design described the proto2 file as the proto3 file with `required` on every field that the
proto3 file leaves unlabelled. The fields of a message type then translate differently in proto2
and in proto3:

- In proto3, a singular field of a message type has presence without a label. The protobuf rules
  report it, and the field translates as an optional.
- In proto2, the frontend stamps `protobuf.label` on a required field and does not give it the
  optional form. The protobuf rules do not report presence for a field with that stamp, so a
  required message field translates without an optional.

## Decision

We will write `optional` on the message fields of the proto2 service schema and `required` on its
scalar and enum fields, because the frontend gives a required field no optional form and every
version of the schema must write the same files.

- `Session.created`, `Session.lifetime`, `Session.attributes` and `Event.session` are `optional`.
  The default `field_presence` of proto2 is `EXPLICIT`, so the frontend gives each of them the
  optional form.
- Every other field that the proto3 file leaves unlabelled is `required`. These are the scalar
  fields and the enum field `Session.state`.
- `Session.note` is `optional`, as in the proto3 file. The map field and the members of the oneof
  have no label in any version.

## Alternatives Considered

### `required` on every unlabelled field

The proto2 file is the proto3 file with `required` wherever the proto3 file has no label, so one
rule over labels derives the proto2 file from the proto3 file.

It lost because the proto2 run then writes four fields without presence. A run of the fixture's
plans over that file wrote these fields:

- In Go, `Event.Session` is `Session`, `Session.Created` is `time.Time` and `Session.Lifetime` is
  `time.Duration`. The other four versions write `*Session`, `*time.Time` and `*time.Duration`.
- In TypeScript, the properties `session`, `created`, `lifetime` and `attributes` have no question
  mark. The other four versions write each of them with a question mark.

## Consequences

**Positive:**

- The runs over proto2, proto3 and the editions 2023, 2024 and 2026 write the same Go file and
  the same TypeScript file, and each run compares its output with one pair of expected files.
- The proto2 run still loads required scalar fields and a required enum field.

**Negative:**

- No run of the service fixture loads a required message field. Only the tests of the protobuf
  rules cover the presence of one.
- The proto2 label of a field depends on the field's type. A message field added to the schema
  needs `optional` in the proto2 file.

**Neutral:**

- The files of the editions do not change. They set `field_presence` to `IMPLICIT` in the file
  and to `EXPLICIT` on `note`, and the protobuf rules give their message fields presence.
