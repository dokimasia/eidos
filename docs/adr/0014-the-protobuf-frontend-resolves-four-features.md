---
adr: 0014
title: The protobuf frontend resolves the four features it applies
status: Accepted
date: 2026-10-10
supersedes: none
superseded-by: none
rfc: RFC-0024
---

# ADR-0014: The protobuf frontend resolves the four features it applies

## Status

Accepted

## Context

protobuf has nine global features, and each language version gives each feature a default. The
frontend applies four of them to what it loads:

- It gives a singular field the optional form where `field_presence` is `EXPLICIT`.
- It marks an enum with `protobuf.closed` where `enum_type` is `CLOSED`.
- It marks a field with `protobuf.delimited` where `message_encoding` is `DELIMITED`.
- It marks a message or an enum with `protobuf.local` where `default_symbol_visibility` makes the
  declaration local to its file.

The other five features, `repeated_field_encoding`, `utf8_validation`, `json_format`,
`enforce_naming_style` and `enforce_proto_limits`, apply to the wire format, the JSON mapping and
the checks of protoc. The frontend stamps every features option under `protobuf.features` as the
schema writes it. No code of the frontend, the rules or a generator reads a resolved value of the
five.

The design specified a table of all nine defaults for each of the five versions, 45 values in
all. It also specified a test that compares the columns of proto2 to 2024 with the edition
defaults of protobuf-go's `descriptorpb`. The frontend's tests are black-box tests that observe
only the loaded output. The resolved values of the five other features do not change that
output, and a black-box test cannot observe them.

## Decision

We will resolve only the four features that the frontend applies, because a test can check a
resolved value only through the output that the value changes.

- The table of versions contains the defaults of the four features for each version, from
  `descriptor.proto` of protobuf v36.0.
- The tests read the expected defaults of proto2, proto3, 2023 and 2024 from the edition defaults
  that `descriptorpb` declares on the fields of `FeatureSet`, and compare the loaded output with
  them. The test states the defaults of 2026, because `descriptorpb` at v1.36.12 predates Edition
  2026.

## Alternatives Considered

### A table of all nine features

The table contains the 45 defaults of `descriptor.proto`, so a consumer of a wire feature finds
its value resolved. It lost because the frontend, the rules and the generators do not read the
five values. A black-box test could observe them only through an export for tests, and the
package's tests do not use such an export.

## Consequences

**Positive:**

- Each value of the table changes an output, and a test compares it with `descriptorpb` or with
  the test's own defaults of 2026.

**Negative:**

- A generator that needs a resolved wire feature, such as the packed encoding or the validation
  of UTF-8, needs the feature in the table and in the resolution first.
- The repository does not contain the defaults of the other five features, so adding one means
  reading `descriptor.proto` again.

**Neutral:**

- The stamp of the features options that a schema writes does not change.

## References

| What | Where |
|---|---|
| `descriptor.proto` of protobuf v36.0 | https://github.com/protocolbuffers/protobuf/blob/v36.0/src/google/protobuf/descriptor.proto |
| protobuf editions, the features | https://protobuf.dev/editions/features/ |
