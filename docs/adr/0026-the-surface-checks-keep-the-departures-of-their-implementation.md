---
adr: 0026
title: The surface checks keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0034
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0026: The surface checks keep the departures of their implementation

## Status

Accepted

## Context

The accepted design adds two checks of generated output to the toolchain kit. `Surface` and
`AssertSurface` render and match the public surface of the output, and `AssertDocumented` requires a
doc comment on each public declaration at file level. Both load the output through the frontend of
its language. The design also takes the adapter and the fixture of `RunToolchainSuite` as values.

The implementation met three places where the code states another fact or contract than the design.
Each place is listed here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| `core/toolchain` gains six imports, the store among them | It gains five imports: the load, the plugin package, the node package, the output package and the diagnostics package | The checks read the declarations of each package from the graph that the load returns, and name no type of the store |
| A method whose host the graph contains and whose host is not public is not in the surface | The check finds the type of a method at file level by the owner of the identity of the method, among the declarations at file level of its package. A method of a type that is not public is not in the surface and needs no doc comment | The load gives a method at file level its file as its host and does not resolve its receiver. The owner of the identity of the method is the name of its receiver, which the load derives from the spelling of the receiver |
| The failures of `AssertSurface` and `AssertDocumented` are a load that fails, a load with an Error, a fixture without a public declaration and a golden that differs | Both also stop the test where the frontend is nil | A nil frontend cannot load the output, and the language of the messages comes from the frontend |

## Alternatives Considered

### Build the design as written

The checks would look up the host of a method in the graph. The graph contains the file as the host
of a method at file level, so every such method would be public by its own visibility, and a method
of a type that is not public would be in the surface. An import of the store would serve no
statement of the code.

### Change the load to resolve the receiver

The load would give a method at file level the type of its receiver as its host. Every reader of the
host of a method would change, and the frontends of Go and Rust, whose methods are at file level,
record the receiver as a spelling. The checks need the name of the type alone, which the identity of
the method already contains.

## Consequences

**Positive:**

- A method of a type that is not public stays out of the surface and out of the documentation check,
  as a consumer cannot call it.

**Negative:**

- A reader of the design finds a host lookup in the graph that the code does not make, and an import
  count that is one too high.

**Neutral:**

- A method is public by its own visibility where its package does not declare a type with the name
  of its receiver.

## References

| What | Where |
|---|---|
| RFC-0034, the surface and documentation checks of generated output | [0034-the-surface-and-documentation-checks-of-generated-output.md](../rfc/0034-the-surface-and-documentation-checks-of-generated-output.md) |
