---
adr: 0077
title: A bound reference takes the package that the binder of its target returns
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

# ADR-0077: A bound reference takes the package that the binder of its target returns

## Status

Accepted

## Context

Under a gRPC binding, the Go spoke spells a reference to a protobuf message as protoc-gen-go names it,
as `*GetRequest`, and keeps the message as the target of the reference. The plan does not emit the
message, because protoc-gen-go writes it. The layout refused such a reference under
`UntranslatedReference`, because it looked for the referent among the declarations of the plan and
found none. No spoke knows the import path of the code of a binding. The spoke reads the shape of a
reference and the policy. The residents, the modules and the import base that decide the package of
a directory are in the placement of a file.

protoc-gen-go and protoc-gen-connect-go with `paths=source_relative` write the code of
`svc/store.proto` into `svc`, and protoc-gen-es writes `svc/store_pb.ts`. The default placement of
the plan writes the code of the same file at the same path.

## Decision

We will qualify a reference to code that a binding generates by the package that the binder of its
target returns, because the plan does not emit that code and only the target knows the package rule
of a directory.

- `emit.TypeRef.Bound` marks the reference. Only the emit model has the field, and the settle keeps
  the spelling of a bound reference.
- `plugin.Binder.Home(target, schema)` returns the package of the code of target. The layout passes
  the placement of the source file of target at the path where the default placement of the plan
  writes its code, below the output directory of a plan that centralises its files.
- The Go binder returns the package of the directory by the package rule of Go. A service takes the
  connect package below it, `<path>/<name>connect`, where protoc-gen-connect-go writes the stream
  types of a handler. The TypeScript binder returns the module `<dir>/<stem>_pb`.
- The layout refuses a bound reference under `UnderivedPackage` where `Home` returns an error or the
  target has no source file in the workspace, and under `UntranslatedReference` where the backend has
  no binder.

## Alternatives Considered

### The import path in the spoke

The spoke would set the package of a bound reference itself. The spoke reads the shape of a reference
and the policy, and neither contains the residents, the modules or the import base that decide the
package of a directory.

### The referent among the declarations of the plan

The plan would emit the messages itself, as the server generator of the service fixture does under
the choice `none`. A handler of a binding would then refer to two declarations of one name: the
declaration of the plan and the declaration of the binding.

## Consequences

**Positive:**

- A handler of grpc-go or connect-go that a plan writes compiles against the code of protoc-gen-go
  and protoc-gen-connect-go, and a handler of Connect-ES compiles against the code of protobuf-es.
- The layout does not depend on a binding. A target with a binder implements the package rule of its
  binding once.

**Negative:**

- The layout does not check where the code of a binding is. A buf configuration that writes the code
  into another tree than the plan's gives references that do not compile.
- The connect package imports the package of the messages, so a plan writes a handler of connect-go
  into another package than the messages.
- A code generator that writes its code at another path than its schema needs a change of the binder
  of its target, because the output directory of a binding is no setting of the plan.

## References

| What | Where |
|---|---|
| RFC-0077, the design of this decision | ../rfc/0077-the-grpc-bindings-of-a-protobuf-service.md |
| D195, the row of this decision | ../architecture/21-decisions.md |
| Go generated code reference of protobuf | <https://protobuf.dev/reference/go/go-generated/> |
| connect-go v2.0.1, the generated handler of a service | <https://github.com/connectrpc/connect-go/blob/v2.0.1/internal/gen/connect/ping/v1/pingv1connect/ping.connect.go> |
