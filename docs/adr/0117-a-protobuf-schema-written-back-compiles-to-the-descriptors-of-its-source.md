---
adr: 0117
title: A protobuf schema written back compiles to the descriptors of its source
status: Accepted
date: 2026-10-28
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0117: A protobuf schema written back compiles to the descriptors of its source

## Status

Accepted

## Context

We mirrored the public modules googleapis, grpc, opentelemetry, protovalidate and envoy through
the conformance mirror plan, compiled each mirror with buf, and compared its descriptors with the
descriptors of its source. The source frontend and the descriptor frontend of RFC-0088 failed in
the same files.

- The settle respelled every declared name through the hook of the protobuf backend. The message
  `GoogleRE2` became `GoogleRe2` and the enum value `TLSv1_0` became `TL_SV1_0`, while a reference
  kept the name as declared. envoy did not compile.
- The backend wrote the fields of a message and then its nested declarations, so a oneof between
  two fields moved after them and the fields of the descriptor changed order.
- The backend wrote an rpc with `{}` as an rpc with `;`, which drops its empty options.
- The backend sorted the imports of a file, which reorders the dependencies of the descriptor.
- The filename of a mirror snake-cased its stem, so `test-extension.proto` and
  `test_extension.proto` wrote one file.

## Decision

We will write a protobuf schema so that it compiles to the descriptors of its source.

- A backend declares the role `plugin.NameKeeper` through `Builder.KeepsOwnNames` where every name
  that a source of its language declares is a name of its target. The settle then keeps the name of
  an emitted declaration that carries the name of its origin of that language. A name of another
  language still respells. The protobuf backend declares the role.
- `protobuf.Stated` states the marker `protobuf.follows` on a nested message, enum, oneof or
  extension, with the member of its message that it follows in the source, and the backend places
  it there.
- Both frontends stamp `protobuf.body` on an rpc with a body, `protobuf.Stated` states it as the
  marker `protobuf.body`, and the backend writes `{}` for it.
- The backend writes the imports of the header in the order of the header, and every other import
  after them in the order of its path.
- A source stem of lower-case letters, digits, underscores and hyphens keeps its spelling in the
  filename of a protobuf file.

## Alternatives Considered

### Names of the target's language kept for every target

The settle could keep such a name for every target. The Go backend exports a declaration of Go by
the case of its name, and the specs of the Go, Java, Rust and TypeScript backends respell a name of
their own language, so the rule is a role of the backend.

### A Pascal-case name kept by the protobuf backend

The protobuf backend could keep a name that is in Pascal case already. A name of another language,
such as `HTTPServer` of Go, would then also keep its spelling, and an enum value would still change.

### Members ordered by field number

The backend could place a oneof among the fields by the numbers of its members. A source that
declares its fields out of number order would still change order.

## Consequences

**Positive:**

- The five modules compile to the same descriptors as their sources through both frontends, 850
  compiled files in all.
- A generator that writes a schema of protobuf, such as a mirror, reproduces the schema of the
  conformance fixture byte for byte.

**Negative:**

- The layout of a multi-line option, a number in hex and a blank line between members stay in the
  form of the backend. The descriptors do not change.

## References

| What | Where |
|---|---|
| D235, the row of this decision | ../architecture/21-decisions.md |
| The settle of a name of the target's language | ../architecture/10-cross-language.md |
| The channels of a protobuf generator | ../architecture/11-languages.md |
| RFC-0088, a protoc plugin host | ../rfc/0088-a-protoc-plugin-host.md |
