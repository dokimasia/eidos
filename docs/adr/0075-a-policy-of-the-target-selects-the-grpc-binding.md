---
adr: 0075
title: A policy of the target selects the gRPC binding
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0077
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0075: A policy of the target selects the gRPC binding

## Status

Accepted

## Context

A Go file that refers to the messages of a service compiles against the code of one binding.
protoc-gen-go names the nested enum of a message `Session_State`, refers to a message through a
pointer, and writes the well-known types of protobuf into the packages of
`google.golang.org/protobuf/types/known`. eidos spelled the same references `SessionState`, `Session`
and `time.Time`, so a generated file did not compile beside the code of the binding. The handler of an
rpc also differs between bindings: grpc-go passes a streaming handler a stream of `grpc`, and
connect-go passes a stream interface of its own package.

## Decision

We will select the gRPC binding of a target through a lowering policy of the target, because the
binding decides the spelling of a reference and of a handler and not the projection.

- `golang.grpc` chooses among `none`, `grpc-go` and `connect-go`, and `typescript.grpc` between
  `none` and `connect-es`. The default `none` keeps the spelling of every reference.
- Under a binding, the spoke spells a reference to a protobuf declaration as the code generator of
  the binding names it.
- The optional role `plugin.Binder` of a backend writes the handler of an rpc in the form of the
  binding, and `Emitter.Handler` asks it.

## Alternatives Considered

### The messages of eidos

A binding would keep the messages of eidos and write adapters to the messages of the binding. The
adapters would copy every field on every call, and a server of the binding would still need its own
interface.

## Consequences

**Positive:**

- A plan selects the binding of its target once, and every plugin of the plan writes code that
  compiles against that binding.
- A directive of the target overrides the binding for one service.

**Negative:**

- The spoke of Go reads the fully-qualified names of the well-known types of protobuf, so a new
  well-known type needs a row.
- A oneof has no reference under a binding, because protoc-gen-go declares its interface unexported
  and protobuf-es declares no type for it.

## References

| What | Where |
|---|---|
| RFC-0077, the design of this decision | ../rfc/0077-the-grpc-bindings-of-a-protobuf-service.md |
| D193, the row of this decision | ../architecture/21-decisions.md |
| Go generated code reference of protobuf | <https://protobuf.dev/reference/go/go-generated/> |
