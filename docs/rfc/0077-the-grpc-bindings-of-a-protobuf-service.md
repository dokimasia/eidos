---
rfc: 0077
title: The gRPC bindings of a protobuf service
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-20
updated: 2026-10-20
discussion: https://github.com/dokimasia/eidos/issues/6
supersedes: none
superseded-by: none
produces-adr: ADR-0074, ADR-0075
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0077: The gRPC bindings of a protobuf service

## Summary

We propose that the projection of an rpc states the two channels that every gRPC binding gives a
call: a failure and a context. The kernel registers the context of a call as a well-known type, and
a new optional capability, `ContextRules`, lets the rules of protobuf put a parameter with the role
`context` before the request of each rpc. Every rpc has the error model `raised`. A lowering policy of
the Go target and of the TypeScript target selects a gRPC binding: grpc-go or connect-go for Go, and
Connect-ES for TypeScript. Under a binding, the spoke spells a reference to a protobuf declaration as
the code generator of the binding names it, and a new optional role of a backend, `plugin.Binder`,
writes the handler of an rpc in the form of the binding.

## Motivation

The service fixture of the conformance module writes this Go interface for `svc/store.proto`:

```go
type StoreServer interface {
    Get(ctx context.Context, request GetRequest) (Session, error)
    Watch(ctx context.Context, request GetRequest) (iter.Seq2[Event, error], error)
}
```

protoc-gen-go v1.36.11 and protoc-gen-go-grpc v1.6.2 write this interface for the same schema:

```go
type StoreServer interface {
    Get(context.Context, *GetRequest) (*Session, error)
    Watch(*GetRequest, grpc.ServerStreamingServer[Event]) error
    mustEmbedUnimplementedStoreServer()
}
```

- A generated file that uses the spelling of eidos does not compile against a server that implements
  the grpc-go interface. protoc-gen-go names the nested enum `Session_State`, a message is a pointer,
  and a streaming side is a stream of the binding.
- The protobuf rules project an rpc with an input and a value under no error model. The fixture's
  generator adds the context parameter and the failure itself, so every plugin that writes code
  against a service repeats both, and its gates read neither from the rules.
- grpc-go reports a failure through the error result and passes the context to a unary handler.
  Connect-ES v2.2.0 passes a `HandlerContext` with an `AbortSignal` to every handler, and a handler
  fails when it throws.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/rules` | `WellKnownContext`, `ContextRules`, and the view of the context in `Bound.CallableOf` |
| Kernel | `eidos-core/plugin`, `eidos-core` | the role `Binder`, `Builder.Binder` of the kit, and `Emitter.Handler` |
| SDK | `eidos-sdk` | the regenerated facade |
| Rules | `eidos-lang-protobuf/rules` | the context of an rpc, and the error model `raised` |
| Spokes | `spell` of every satellite | the spelling of the well-known context |
| Go | `eidos-lang-go` | the policy `golang.grpc`, the spelling of a protobuf reference under a binding, and the handlers of grpc-go and connect-go |
| TypeScript | `eidos-lang-typescript` | the policy `typescript.grpc`, the spelling of a protobuf reference under Connect-ES, and its handlers |
| Conformance | `eidos-conformance/lang/protobuf` | the generators of the service fixture read the context and the failure from the projection |

### Invariants

1. Every rpc projects with a parameter of the role `context` before its request and with the error
   model `raised`, whatever the target.
2. A binding changes the spelling of a target and not the projection.
3. Under the choice `none`, every target writes what it wrote before, with the context and the failure
   from the projection.

### The context of a call

```go
package rules

// WellKnownContext is the context of a call: its cancellation and its
// deadline, as Go's context.Context and the signal of a Connect-ES handler.
var WellKnownContext = wellKnown("context")

// ContextRules is the optional capability of a language whose callables
// take a context that their signatures do not state, as every rpc of
// protobuf takes the cancellation and the deadline of its call.
// [Bound.CallableOf] asks it for each callable and puts the view of the
// context before the parameters.
type ContextRules interface {
    // Context returns the view of the context of the callable sym, with the
    // role ParamContext, and reports false for a callable without one.
    Context(sym symbol.Symbol, v View) (ParamView, bool)
}
```

- The protobuf rules return the view `ctx` of a reference to `WellKnownContext` for every rpc.
- `IsWellKnown` reports true for the context, and the fold gives a reference to it the shape of the
  well-known reference.
- Every spoke spells the context:

| Target | Spelling |
|---|---|
| Go | `context.Context` |
| TypeScript | `AbortSignal`, and `HandlerContext` of `@connectrpc/connect` under the binding `connect-es` |
| Java, Rust, Python | refused, because the language has no standard type of the context of a call |
| protobuf, GraphQL | refused, because a schema states no context |

### The failure of an rpc

`ReturnRoles` of protobuf returns the error model `ErrorsRaised` for every rpc. The schema declares no
type of the failure, and every binding fails a call with a status of gRPC. A generator states the
failure of the target from the model, as the Go backend lowers a failure into the error result.

### The bindings

| Policy | Choices | Default |
|---|---|---|
| `golang.grpc` | `none`, `grpc-go`, `connect-go` | `none` |
| `typescript.grpc` | `none`, `connect-es` | `none` |

A binding is a policy of the declaration, so a plan selects it for a service and a directive of the
target can override it for one service.

### A reference under a binding

grpc-go and connect-go use the messages of protoc-gen-go. Under both choices the Go spoke spells a
reference to a protobuf declaration as protoc-gen-go names it:

| protobuf | Go |
|---|---|
| a message | a pointer, as `*GetRequest`, and `*Session_Item` for a nested message |
| an enum | its name, as `Status`, and `Session_State` for a nested enum |
| a oneof | refused, because protoc-gen-go declares the interface of a oneof unexported |
| `google.protobuf.Timestamp`, `Duration`, `Empty` | `*timestamppb.Timestamp`, `*durationpb.Duration`, `*emptypb.Empty` |
| a wrapper, as `google.protobuf.StringValue` | `*wrapperspb.StringValue` |
| `Any`, `Struct`, `Value`, `ListValue`, `FieldMask` | `*anypb.Any`, `*structpb.Struct`, `*structpb.Value`, `*structpb.ListValue`, `*fieldmaskpb.FieldMask` |

Each package of a well-known type is the package of `google.golang.org/protobuf/types/known` of its
name. The spoke recognizes a well-known type of protobuf by the fully-qualified name of its reference.

Connect-ES uses the messages of protobuf-es v2. Under `connect-es` the TypeScript spoke spells a
message and an enum by their names with `_` between the names of nested declarations, and a
well-known type as the type of `@bufbuild/protobuf/wkt` of its name. A oneof refuses, because
protobuf-es declares a oneof as a field of its message and no type.

### The handler of an rpc

```go
package plugin

// Binder is the optional role of a backend that writes the handler of an
// rpc in the form of a gRPC binding. [eidos.Emitter.Handler] asks it under
// the binding that the policy of the plan selects for the service.
type Binder interface {
    // Handler returns the method that serves the rpc c under the policy p.
    // typ spells a reference of c through the spoke of the plan. Handler
    // returns an error for an rpc that the binding cannot serve.
    Handler(c rules.Callable, p Policy, typ func(*node.TypeRef) (*emit.TypeRef, error)) (*emit.Method, error)
}
```

`Emitter.Handler(rpc)` projects the rpc, asks the role and reports a refusal as `Emitter.Type` does. A
backend without the role writes the canonical method: the context, the request and the response,
with the failure in `Throws`. The handlers of the bindings, for the service `Store`, are:

| rpc | grpc-go | connect-go | connect-es |
|---|---|---|---|
| unary | `Get(context.Context, *GetRequest) (*Session, error)` | `Get(context.Context, *GetRequest) (*Session, error)` | `get(request: GetRequest, context: HandlerContext): Promise<Session>` |
| server stream | `Watch(*GetRequest, grpc.ServerStreamingServer[Event]) error` | `Watch(context.Context, *GetRequest, StoreWatchServerStream) error` | `watch(request: GetRequest, context: HandlerContext): AsyncIterable<Event>` |
| client stream | `Upload(grpc.ClientStreamingServer[Chunk, Summary]) error` | `Upload(context.Context, StoreUploadServerStream) (*Summary, error)` | `upload(requests: AsyncIterable<Chunk>, context: HandlerContext): Promise<Summary>` |
| bidi stream | `Chat(grpc.BidiStreamingServer[Event, Event]) error` | `Chat(context.Context, StoreChatServerStream) error` | `chat(requests: AsyncIterable<Event>, context: HandlerContext): AsyncIterable<Event>` |

- grpc-go is the form of protoc-gen-go-grpc v1.6.2, and a streaming handler reads the context of the
  call from its stream.
- connect-go is the form of protoc-gen-connect-go v2.0.1. A stream type is the interface
  `<Service><Method>ServerStream` of the connect package of the service, which protoc-gen-connect-go
  writes into the directory `<package>connect` beside the messages.
- connect-es is the form of the `ServiceImpl` of Connect-ES v2.2.0.

### Failures

| Failure | Effect |
|---|---|
| A target without a spelling of the context | the spoke refuses the shape, and the Emitter reports the refusal at the rpc |
| A reference to a oneof under a binding | the spoke refuses it with the reason of the binding |
| A binding that cannot serve an rpc | `Handler` returns an error, and the Emitter reports it at the rpc |

### Migration

The view of every rpc gains the context and the error model `raised`. A generator that added a
context parameter or a failure for an rpc reads them from the view, and the service fixture of the
conformance module does so. A plugin that counted the parameters of an rpc counts one more. The
default choice `none` keeps the spelling of every reference.

## Alternatives

### The binding in the projection

The rules of protobuf could read the policy of the plan and project an rpc as its binding serves it.
The projection of a declaration then differs between two plans of one run, and the rules would read a
policy of each target. Every binding has a failure and a context, so the projection states both and
the spoke spells them.

### A context without a type

The view could put a context parameter without a reference before the request, and each generator
would spell the type of its target. Each generator would then repeat the spelling of each target,
which the projection exists to state once.

### The messages of eidos

A binding could keep the messages of eidos and write adapters to the messages of the binding. The
adapters would copy every field on every call, and a server of the binding would still need its own
interface.

## Drawbacks

- The kernel gains one well-known type, one optional capability and one role of a backend. Every
  spoke states the spelling of the context, and five of them refuse it.
- A plugin that counts the parameters of an rpc counts the context, which changes its output.
- The spoke of Go reads the fully-qualified names of the well-known types of protobuf, so a new
  well-known type needs a row.
- A oneof has no reference under a binding.

## Open questions

None.

## Unresolved and future work

- The bindings of Java, Rust and Python: grpc-java, tonic and grpcio.
- The client stubs of the bindings.

## References

| What | Where |
|---|---|
| Issue 6, the gRPC binding of each target | <https://github.com/dokimasia/eidos/issues/6> |
| protoc-gen-go-grpc v1.6.2 | <https://github.com/grpc/grpc-go/tree/cmd/protoc-gen-go-grpc/v1.6.2/cmd/protoc-gen-go-grpc> |
| Go generated code reference of protobuf | <https://protobuf.dev/reference/go/go-generated/> |
| connect-go v2.0.1, the generated handler of a service | <https://github.com/connectrpc/connect-go/blob/v2.0.1/internal/gen/connect/ping/v1/pingv1connect/ping.connect.go> |
| Connect-ES v2.2.0, `implementation.ts` | <https://github.com/connectrpc/connect-es/blob/v2.2.0/packages/connect/src/implementation.ts> |
| protobuf-es v2, the generated code | <https://github.com/bufbuild/protobuf-es/blob/main/MANUAL.md> |
