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
produces-adr: ADR-0074, ADR-0075, ADR-0077
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
Connect-ES for TypeScript.

Under a binding, the spoke spells a reference to a protobuf declaration as the code generator of the
binding names it, and marks the reference bound. The code generator writes the code of a schema file
in the output tree of the plan at the path of the file. The layout qualifies a bound reference by the
package of that code, which a new optional role of a backend, `plugin.Binder`, returns. The same
role writes the handler of an rpc in the form of the binding, and `Emitter.Handler` asks it.

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
- A message of protoc-gen-go is code that the plan does not write. The layout refuses a translated
  reference whose referent the plan does not emit, and no spoke knows the import path of the code of
  a binding.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/symbol/schema`, `eidos-core/emit` | `TypeRef.Bound` on the emit side |
| Kernel | `eidos-core/rules` | `WellKnownContext`, `ContextRules`, and the view of the context in `Bound.CallableOf` |
| Kernel | `eidos-core/plugin`, `eidos-core/backend` | the role `Binder` with `Handler` and `Home`, `plugin.RPC`, `plugin.Translate`, `plugin.CanonicalHandler`, and `Builder.Binder` of the kit |
| Kernel | `eidos-core` | `Emitter.Handler` and the code `RefusedHandler` |
| Kernel | `eidos-core/layout` | the qualification of a bound reference |
| SDK | `eidos-sdk` | the regenerated facade |
| Rules | `eidos-lang-protobuf/rules` | the context of an rpc, and the error model `raised` |
| Spokes | `spell` of every satellite | the spelling of the well-known context |
| Go | `eidos-lang-go` | the policy `golang.grpc`, the bound spelling of a protobuf reference, and the binder of grpc-go and connect-go |
| TypeScript | `eidos-lang-typescript` | the policy `typescript.grpc`, the bound spelling under Connect-ES, and its binder |
| Conformance | `eidos-conformance/lang/protobuf` | the generators of the service fixture read the projection, and a handler generator writes through `Emitter.Handler` under each binding |

### Invariants

1. Every rpc projects with a parameter of the role `context` before its request and with the error
   model `raised`, whatever the target.
2. A binding changes the spelling of a target and not the projection.
3. Under the choice `none`, every target writes what it wrote before, with the context and the failure
   from the projection.
4. A bound reference refers to code that the code generator of the binding writes, and the layout
   qualifies it by the package of that code. The plan does not emit its referent.

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
reference to a protobuf declaration as protoc-gen-go names it, and marks the reference bound:

| protobuf | Go |
|---|---|
| a message | a pointer, as `*GetRequest`, and `*Session_Item` for a nested message |
| an enum | its name, as `Status`, and `Session_State` for a nested enum |
| a oneof | refused, because protoc-gen-go declares the interface of a oneof unexported |
| `google.protobuf.Timestamp`, `Duration`, `Empty` | `*timestamppb.Timestamp`, `*durationpb.Duration`, `*emptypb.Empty` |
| a wrapper, as `google.protobuf.StringValue` | `*wrapperspb.StringValue` |
| `Any`, `Struct`, `Value`, `ListValue`, `FieldMask` | `*anypb.Any`, `*structpb.Struct`, `*structpb.Value`, `*structpb.ListValue`, `*fieldmaskpb.FieldMask` |

Each package of a well-known type is the package of `google.golang.org/protobuf/types/known` of its
name. The spoke recognizes a well-known type of protobuf by the fully-qualified name of its reference,
and a well-known type is not bound, because its reference states its package.

Connect-ES uses the messages of protobuf-es v2. Under `connect-es` the TypeScript spoke spells a
message and an enum by their names with `_` between the names of nested declarations, as
`Session_State`, and marks the reference bound. A well-known type spells as the type of
`@bufbuild/protobuf/wkt` of its name, as `Timestamp`, and a wrapper as its message, as `StringValue`.
A oneof refuses, because protobuf-es declares a oneof as a field of its message and no type.

### The code of a binding

The code generator of a binding writes the code of a schema file at the path of the file, where the
default placement of the plan writes the code of the file: beside it, or below the output directory
of the plan where the plan centralises its files.

| Generator | The code of `svc/store.proto` beside it |
|---|---|
| protoc-gen-go with `paths=source_relative` | `svc/store.pb.go`, in the Go package of the directory `svc` |
| protoc-gen-connect-go with `paths=source_relative` | `svc/svcconnect/store.connect.go`, in the package `svcconnect` |
| protoc-gen-es | `svc/store_pb.ts` |

```go
package emit

type TypeRef struct {
    // The other fields are unchanged.

    // Bound marks a reference to code that the binding of the plan's
    // target generates for Target, such as a message that protoc-gen-go
    // writes under a gRPC binding. Only the emit model has it.
    Bound bool
}
```

The layout qualifies a bound reference by the package that `Binder.Home` returns for its target. It
reads the source file of the target through the index of the plan, and passes the placement of that
file at the path where the default placement of the plan writes its code: the package of the target,
the residents and the modules of the directory, and the import base of the plan. The layout sets the
package of the reference where that package differs from the package of the referencing file, and
records it among the referents of the file. It does not look for a referent among the declarations
of the plan.

- The Go binder returns the package of the directory, by the package rule of Go: the package that the
  Go files of the directory declare, then the module that contains the directory, then the import
  base. For a service, it returns the connect package below it, `<path>/<name>connect`, whose stream
  types a handler of connect-go names.
- The TypeScript binder returns the module `<dir>/<stem>_pb`.

### The handler of an rpc

```go
package plugin

// Binder is the optional role of a backend whose target writes code
// against the code generator of a gRPC binding. [eidos.Emitter.Handler]
// calls it for the handler of an rpc, and the layout calls it for the
// package of a bound reference.
type Binder interface {
    // Handler returns the method that serves the rpc r under the policy p.
    // translate spells a reference of r through the spoke of the plan.
    // Under a policy that selects no binding, Handler returns the method
    // of CanonicalHandler in the form of its target. It returns an error
    // for an rpc that the binding cannot serve.
    Handler(r RPC, p Policy, translate Translate) (*emit.Method, error)
    // Home returns the package of the code that the code generator of the
    // binding writes for target. schema places the source file of target.
    // Home returns an error for a target whose code no binding of the
    // target generates.
    Home(target symbol.Identity, schema Placement) (symbol.Identity, error)
}

// Translate spells a reference of the graph in the target of a plan, as
// Emitter.Type translates a type, and returns the reason of a refusal.
type Translate func(ref *node.TypeRef) (*emit.TypeRef, error)

// RPC is an rpc that a Binder serves: the identity, the documentation and
// the visibility of the rpc, the service that declares it, and its
// projection, which has its name.
type RPC struct {
    ID, Service symbol.Identity
    Doc         []string
    Visibility  symbol.Visibility
    Callable    rules.Callable
}

// CanonicalHandler returns the canonical method that serves the rpc r: the
// parameters and the returns of the projection, the asynchrony of the rpc,
// and under ErrorsRaised the failure in Throws, as types spells the
// well-known error.
func CanonicalHandler(r RPC, p Policy, types TypeSpeller, translate Translate) (*emit.Method, error)
```

```go
package eidos

// Handler returns the method of the plan's target that serves rpc in the
// form of the binding that the policy of the plan selects for it. It
// reports false for a refusal, after it reports the refusal under
// RefusedHandler at the rpc.
func (e *Emitter) Handler(rpc *node.Method) (*emit.Method, bool)
```

`Emitter.Handler` projects the rpc and calls the binder of the backend. Where the backend has no
binder, the Emitter writes the method of `CanonicalHandler`. The translations read through the view
of the invocation, as `Emitter.Type` reads. `Builder.Binder(handler, home)` of the backend kit declares
a binder. The built backend implements the role, and without a declaration its `Handler` returns the
method of `CanonicalHandler` and its `Home` returns an error.

The handlers of the bindings, for the service `Store`, are:

| rpc | grpc-go | connect-go | connect-es |
|---|---|---|---|
| unary | `Get(context.Context, *GetRequest) (*Session, error)` | `Get(context.Context, *GetRequest) (*Session, error)` | `get(request: GetRequest, context: HandlerContext): Promise<Session>` |
| server stream | `Watch(*GetRequest, grpc.ServerStreamingServer[Event]) error` | `Watch(context.Context, *GetRequest, svcconnect.StoreWatchServerStream) error` | `watch(request: GetRequest, context: HandlerContext): AsyncIterable<Event>` |
| client stream | `Upload(grpc.ClientStreamingServer[Chunk, Summary]) error` | `Upload(context.Context, svcconnect.StoreUploadServerStream) (*Summary, error)` | `upload(requests: AsyncIterable<Chunk>, context: HandlerContext): Promise<Summary>` |
| bidi stream | `Chat(grpc.BidiStreamingServer[Event, Event]) error` | `Chat(context.Context, svcconnect.StoreChatServerStream) error` | `chat(requests: AsyncIterable<Event>, context: HandlerContext): AsyncIterable<Event>` |

- grpc-go is the form of protoc-gen-go-grpc v1.6.2, and a streaming handler reads the context of the
  call from its stream.
- connect-go is the form of protoc-gen-connect-go v2.0.1. A stream type is the struct
  `<Service><Method>ServerStream` of the connect package of the service. The connect package
  imports the messages, so a plan writes a handler of connect-go into another package than the
  messages.
- connect-es is the form of the `ServiceImpl` of Connect-ES v2.2.0. The context comes last, and the
  requests of a client stream are `requests`.
- Under `none` the Go binder writes the canonical method without its asynchrony, because Go has no
  asynchronous method, as `Get(ctx context.Context, request GetRequest) (Session, error)`. The
  TypeScript binder writes the canonical method without `Throws`, because a TypeScript method fails
  when it throws, as `get(ctx: AbortSignal, request: GetRequest): Promise<Session>`.

### Failures

| Failure | Effect |
|---|---|
| A target without a spelling of the context | the spoke refuses the shape, and the Emitter reports the refusal at the rpc |
| A reference to a oneof under a binding | the spoke refuses it with the reason of the binding |
| A binding that cannot serve an rpc | `Handler` returns an error, and the Emitter reports it under `RefusedHandler` at the rpc |
| A bound reference to a target whose code no binding generates | `Home` returns an error, and the layout refuses the declaration under `UnderivedPackage` |
| A bound reference in a plan whose backend has no binder | the layout refuses the declaration under `UntranslatedReference` |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The code of a binding at the path of its schema | in | protoc-gen-go and protoc-gen-connect-go with `paths=source_relative` and protoc-gen-es write there, and buf's managed mode derives `go_package` from the directory | a binding whose code the plan reads from another tree than its own |
| The `go_package` option of a schema | out | The Go binder derives the package from the directory, whose Go files declare the package of `go_package` where the load reads them | a schema whose `go_package` names another package than its directory, without Go files beside it |
| The handlers of Java, Rust and Python | out | grpc-java, tonic and grpcio have no binding in this proposal | a binding of one of them |

### Migration

The view of every rpc gains the context and the error model `raised`. A generator that added a
context parameter or a failure for an rpc reads them from the view, and the service fixture of the
conformance module does so through `Emitter.Handler`. A plugin that counted the parameters of an rpc
counts one more. The default choice `none` keeps the spelling of every reference.

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

### The import path in the spoke

The spoke could set the package of a bound reference itself. The spoke reads the shape of a reference
and the policy, and neither contains the residents, the modules or the import base that decide the
package of a directory.

### The referent among the declarations of the plan

The plan could emit the messages itself, as the service fixture's server generator does under `none`.
A handler of a binding would then refer to two declarations of one name: the declaration of the plan
and the declaration of the binding.

## Drawbacks

- The kernel gains one well-known type, one optional capability, one role of a backend with two
  methods, one field of the emit model and one diagnostic code. Every spoke states the spelling of
  the context, and five of them refuse it.
- A plugin that counts the parameters of an rpc counts the context, which changes its output.
- The spoke of Go reads the fully-qualified names of the well-known types of protobuf, so a new
  well-known type needs a row.
- A oneof has no reference under a binding.
- The layout does not check where the code of a binding is. A buf configuration that writes the code
  into another tree than the plan's gives references that do not compile.

## Open questions

None.

## Unresolved and future work

- The bindings of Java, Rust and Python: grpc-java, tonic and grpcio.
- The client stubs of the bindings.
- The output directory of a binding as a setting of the plan.

## References

| What | Where |
|---|---|
| Issue 6, the gRPC binding of each target | <https://github.com/dokimasia/eidos/issues/6> |
| protoc-gen-go-grpc v1.6.2 | <https://github.com/grpc/grpc-go/tree/cmd/protoc-gen-go-grpc/v1.6.2/cmd/protoc-gen-go-grpc> |
| Go generated code reference of protobuf | <https://protobuf.dev/reference/go/go-generated/> |
| connect-go v2.0.1, the generated handler of a service | <https://github.com/connectrpc/connect-go/blob/v2.0.1/internal/gen/connect/ping/v1/pingv1connect/ping.connect.go> |
| Connect-ES v2.2.0, `implementation.ts` | <https://github.com/connectrpc/connect-es/blob/v2.2.0/packages/connect/src/implementation.ts> |
| protobuf-es v2, the generated code | <https://github.com/bufbuild/protobuf-es/blob/main/docs/src/content/docs/reference/generated-code/index.md> |
| protobuf-es v2, the nested types | <https://github.com/bufbuild/protobuf-es/blob/main/docs/src/content/docs/reference/generated-code/features.md> |
