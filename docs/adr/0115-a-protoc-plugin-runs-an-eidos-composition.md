---
adr: 0115
title: A protoc plugin runs an eidos composition
status: Accepted
date: 2026-10-28
supersedes: none
superseded-by: none
rfc: RFC-0088
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0115: A protoc plugin runs an eidos composition

## Status

Accepted

## Context

`eidos run` loads protobuf from `.proto` files through the source frontend. `protoc`, `buf generate`
and a remote plugin on the BSR pass a plugin no source. They pass a `CodeGeneratorRequest` with the
`FileDescriptorProto` of every file to generate and of everything those files import. buf resolves
the dependencies of a buf module, such as a module on the BSR, which eidos has no store for. New
generators of eidos have to run under `buf generate` beside `protoc-gen-go`.

## Decision

We will run an eidos composition as a protoc plugin through the module `eidos-protoc`
(`go.dokimi.dev/eidos/protoc`), as RFC-0088 designs it.

- `protoc.Main(compose Compose)` is the entry point of a binary `cmd/protoc-gen-<name>` in the
  repository of the generators. `compose` is the composition function that the binary passes to
  `cli.New`. `protoc.Run` is the same over a reader, a writer and a log.
- The package `lang/protobuf/descriptor` is a frontend that lowers the descriptors into the graph
  that the source frontend gives for the same schema. It reports an Error under `MissingSourceInfo`,
  PROTO-0007, for a file to generate without source code info.
- The host turns the request into the input of a cold run: the files to generate form an in-memory
  tree, and every other file is a file of the store `protoc`, which the first dependency round of
  the frontend loads. Under ADR-0114 each declaration of an imported file is a dependency, so no
  generator runs on it.
- The parameter sets the config: `<plugin>.<option>=<value>` and `policy.<key>=<choice>`, with a JSON
  value or a string.
- The response states `FEATURE_PROTO3_OPTIONAL`, `FEATURE_SUPPORTS_EDITIONS` and the editions from
  proto2 to 2026. It carries every file of the in-memory sink, or every Error finding in its `error`
  field and no file. The log gets each Warning and Info.
- The protobuf satellite exports `frontend.Resolve` and `frontend.ImportPath`, so both frontends
  resolve a reference and an import alike.

## Alternatives Considered

### A subcommand of `eidos-cli`

A subcommand would make the plugin a mode of the command line of a binary. buf and the BSR run a
plugin binary by its name and pass the request on standard input, so a binary that publishes on the
BSR needs an entry point of its own.

### A graph that the host builds

The host could lower the descriptors into a `store.Graph` and pass it as the graph of the input. It
would then repeat the splice and the resolution of the load, and mark dependencies itself.

### The `.proto` files of the working directory

The plugin could parse the source files beside the request. buf runs a plugin over inputs that are
not on disk, such as a BSR module or an archive.

## Consequences

**Positive:**

- A repository runs the generators of eidos through `buf generate`, over every input of buf.
- A generator runs under the host as it runs under `eidos run`.

**Negative:**

- Two frontends lower one language. The parity spec of the descriptor package compares their graphs
  for proto2, proto3, Edition 2023 and custom options.
- A composition that writes one file for each plan needs `strategy: all` in `buf.gen.yaml`.
- Each call is a cold run.

## References

| What | Where |
|---|---|
| D233, the row of this decision | ../architecture/21-decisions.md |
| The protoc plugin binary | ../architecture/14-distribution-and-cli.md |
| RFC-0088, a protoc plugin host | ../rfc/0088-a-protoc-plugin-host.md |
| ADR-0114, a dependency per declaration | 0114-a-dependency-is-a-declaration-that-a-dependency-round-loaded.md |
| The protocol of a protoc plugin | <https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/compiler/plugin.proto> |
