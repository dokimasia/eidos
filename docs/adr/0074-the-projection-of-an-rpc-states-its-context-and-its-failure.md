---
adr: 0074
title: The projection of an rpc states its context and its failure
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

# ADR-0074: The projection of an rpc states its context and its failure

## Status

Accepted

## Context

A schema states the request and the response of an rpc and nothing else:

```protobuf
service Store {
  rpc Get(GetRequest) returns (Session);
}
```

Every binding of gRPC gives the handler of a call a context and a failure. grpc-go passes a
`context.Context` and fails a call through the error result. Connect-ES passes a `HandlerContext`
with an `AbortSignal`, and a handler fails when it throws. The protobuf rules projected an rpc with an
input and a value under no error model. The generator of the service fixture added the context
parameter and the failure itself, so every plugin that writes code against a service repeated both,
and no gate could read them from the rules.

## Decision

We will project every rpc with a parameter of the role `context` before its request and with the
error model `raised`, because every binding of gRPC gives a call a context and a failure.

- The kernel registers the context of a call as `WellKnownContext`, and the fold gives a reference to
  an identity of the registry its well-known shape.
- The optional capability `ContextRules` gives `Bound.CallableOf` the view of the context, which comes
  first among the parameters. The protobuf rules give it to every rpc.
- `ReturnRoles` of protobuf states `ErrorsRaised`, because the schema declares no type of the
  failure.
- Go spells the context `context.Context` and TypeScript `AbortSignal`. Java, Rust, Python, protobuf
  and GraphQL refuse it with the reason of the refusal.

## Alternatives Considered

### The binding in the projection

The rules of protobuf would read the policy of the plan and project an rpc as its binding serves it.
The projection of one declaration would then differ between two plans of one run, and the rules would
read a policy of each target.

### A context without a type

The view would put a context parameter without a reference before the request, and each generator
would spell the type of its target. Each generator would repeat the spelling of each target.

## Consequences

**Positive:**

- A generator over a service reads the context and the failure of an rpc from the view, and the
  service fixture writes its server without adding either.
- A gate of a plugin reads the cancellation and the failure of an rpc as it reads them of a Go
  function.

**Negative:**

- A plugin that counts the parameters of an rpc counts one more, and a client that takes the signal
  in the options of a call leaves the context out itself.
- Five spokes refuse the context, so a generator into Java, Rust or Python that keeps the context of
  an rpc reports a refusal at each rpc.

## References

| What | Where |
|---|---|
| RFC-0077, the design of this decision | ../rfc/0077-the-grpc-bindings-of-a-protobuf-service.md |
| D192, the row of this decision | ../architecture/21-decisions.md |
| Connect-ES v2.2.0, `implementation.ts` | <https://github.com/connectrpc/connect-es/blob/v2.2.0/packages/connect/src/implementation.ts> |
