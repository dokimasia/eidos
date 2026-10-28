---
rfc: 0088
title: A protoc plugin host
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-28
updated: 2026-10-28
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0114, ADR-0115
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0088: A protoc plugin host

## Summary

We propose a host that runs an eidos composition as a protoc plugin, so `buf generate` and `protoc`
run new eidos generators over protobuf schemas.

- The module `eidos-protoc` (`go.dokimi.dev/eidos/protoc`) is the entry point of a plugin binary,
  as `eidos-cli` is the entry point of a command line. A plugin author writes
  `cmd/protoc-gen-<name>` in the repository of the generators, and calls `protoc.Main` with the
  function that composes the generators. protoc and `buf generate` run the binary, and the binary
  can be published as a remote plugin on the BSR, because it reads nothing but its standard input.
- A descriptor frontend loads the `FileDescriptorProto`s of the request into the node model, with
  the projection, the facts and the directive carriers of the source frontend. The files to
  generate load into the workspace. Every other file of the request loads as a dependency.
- The run is cold. It writes into an in-memory sink, keeps no ledger, and returns every file in the
  response. buf writes the files.

## Motivation

eidos loads protobuf from source through its source frontend, which parses with the experimental
parser of protocompile. A protoc plugin receives no source. protoc and buf compile the schema and
pass descriptors: `proto_file` holds the `FileDescriptorProto` of every file to generate and of
everything those files import, in topological order, with fully qualified type names. Comments
arrive through `source_code_info`.

A generator that runs under `buf generate` reads what `eidos run` cannot read:

- The dependencies of a buf module, such as a module on the BSR. buf resolves them and passes their
  descriptors. eidos has no store for the module cache of buf.
- Every input of buf, such as a BSR module, a Git repository, an archive or an image.

A repository then runs every code generator through `buf generate`, beside `protoc-gen-go` and
`protoc-gen-connect-go`.

## Detailed design

### Components

| Component | Responsibility |
|---|---|
| `lang/protobuf/descriptor` | The frontend that lowers `FileDescriptorProto`s into the node model |
| The module `eidos-protoc` | The host: the request as the input of a run, the composition, the response |
| `protoc.Main`, `protoc.Run` | The entry point of a plugin binary, and the same over a reader and a writer |
| The parameter of the request | The option values of the plugins and the choices of the policies |
| The composition of the plugin author | The brand, the rules, the plans, the generators and the backends |
| `cmd/protoc-gen-<name>` | The binary, in the repository of the generators |

### The request as a load

The host turns the request into the input of an ordinary run, so the partition, the splice, the
resolution and the dependency rounds of the load apply unchanged.

- **The tree.** Each file of `file_to_generate` is a file of an in-memory tree at its proto path,
  such as `acme/weather/v1/weather.proto`. Its bytes are the serialized `FileDescriptorProto`. The
  host takes the descriptor from `source_file_descriptors`, which keeps the source-retention
  options, and from `proto_file` where the request has none.
- **The store.** Every other file of `proto_file` is a file of the store `protoc` at its proto path.
- **The dependency round.** The descriptor frontend implements `plugin.Dependent`. Its first round
  returns every file of the store as a unit, as a Java classpath returns its units whatever the needs
  are. The load parses these units at `DepthSignatures` as dependency units, so a dependency reports
  no finding at a carrier and no rule of the generate phase runs on its declarations.
- **A shared package.** A file to generate and a file that it imports often declare one proto
  package, such as `acme.v1`. The kernel marks a dependency per declaration: a declaration is a
  dependency when a dependency round loaded its file, and a package is a dependency only when a
  dependency round loaded every file of it. A generator then runs on the declarations of the file to
  generate and on no declaration of the imported file.
- **The output paths.** The layout routes beside the source by default, so a generated file has the
  directory of its proto file. The response names each file relative to the output directory of the
  plugin, as `paths=source_relative` of `protoc-gen-go` does.

### The descriptor frontend

The descriptor frontend has the language, the comment syntax, the projection and the facts of the
source frontend.

- **Projection.** A message is a struct, a oneof is a sum, a service is an interface, and so on,
  through the table of the source frontend. A type name of a descriptor is fully qualified, so a
  reference has its spelling with the leading dot, and the resolution of the load resolves it.
- **Versions and features.** The frontend reads `syntax` and `edition` of each file, and resolves
  the four features through the table of the satellite, from the default of the edition and the
  options of the file, the enclosing messages, the oneof and the declaration.
- **Facts.** The frontend stamps the keys of the satellite: the wire number of a field, the reserved
  ranges, the options of a declaration with `go_package`, the syntax, the proto package, the oneof,
  the streams, the map entries and the features. The gRPC bindings read the Go package of a file
  from these facts.
- **Positions.** A location of `source_code_info` has a zero-based line and column. The frontend
  adds one to each, so a position has the line and the column of the source.
- **Comments.** protoc attributes the leading, trailing and detached comments of each declaration,
  as the source frontend does. The frontend passes the leading and trailing comments of a
  declaration through `SourceUnit.Comment` and attaches their carriers. It refuses a carrier in a
  detached comment under `UnaddressedCarrier`, as the source frontend does.
- **Missing source info.** A file to generate without `source_code_info` has no comments, so its
  directives would disappear without a finding. The frontend reports an Error under
  `MissingSourceInfo` for such a file. buf includes source info unless `--exclude-source-info` is
  set.

### The host

```go
// Compose returns the builder of the composition of a plugin binary: the
// brand, the rules, the plans, the generators and the backends.
type Compose func() *workspace.Builder

// Main runs the composition that compose returns as a protoc plugin. It
// reads a CodeGeneratorRequest from standard input, writes a
// CodeGeneratorResponse to standard output, and exits with status 0. It
// exits with status 1 and a message on standard error for a request
// that does not decode and for a composition that does not build.
func Main(compose Compose)

// Run runs the composition that compose returns over the request that r
// contains, writes the response to w, and writes each Warning and Info
// of the run to log.
func Run(ctx context.Context, r io.Reader, w, log io.Writer, compose Compose) error
```

A binary of the generators of `acme` is one call:

```go
func main() { protoc.Main(acme.Compose) }
```

- **The composition.** `compose` is the function that the author passes to `cli.New` for a command
  line, so the binary and the command line register the generators once. It registers no frontend,
  no output and no ledger. The host adds the descriptor frontend and an in-memory sink. A
  composition that registers the frontend of protobuf as well fails to build, because two frontends
  have one name.
- **The parameter.** The host splits the parameter of the request, such as the `opt` entries of
  `buf.gen.yaml`, at commas into `key=value` entries, and sets them as the config of the
  composition:
  - `<plugin>.<option>=<value>` sets one option of one plugin. The option is the key after the last
    dot, because the key of an option has no dot.
  - `policy.<key>=<choice>` selects the choice of one policy, such as `policy.golang.grpc=connect-go`.
  - A value is decoded as JSON where it is JSON, such as `true`, `3` or `["a","b"]`, and is a string
    otherwise.
  - An entry without `=` or with an empty key is an error of the request.
- **The response.** After the run, the host returns each file of the sink in path order as a
  `CodeGeneratorResponse.File` with its name and its content. Each file has the frame of the brand,
  as a file of `eidos run` has.
- **Features.** The response states `FEATURE_PROTO3_OPTIONAL` and `FEATURE_SUPPORTS_EDITIONS`, the
  minimum edition `EDITION_PROTO2` and the maximum edition `EDITION_2026`, the newest edition that
  the satellite loads.
- **Findings.** The host writes the Error findings of the run, one line each in the form of
  `eidos run`, into the `error` field of the response, and returns no file. protoc and buf then fail
  the generation and print the message. The host writes every Warning and Info to the log, which
  `Main` sets to standard error.

### The invocation strategy

buf calls a local plugin once for each directory of the input by default. A plan that writes a file
per plan, a generator that reads the declarations of another directory, and a check across packages
need every file in one call. The documentation of the host requires `strategy: all` in
`buf.gen.yaml` for such a composition.

### Failure handling

| Failure | Behaviour |
|---|---|
| A request that does not decode | A message on standard error and exit status 1 |
| A composition that does not build | A message on standard error and exit status 1 |
| A file to generate without source info | An Error under `MissingSourceInfo`, returned in the `error` field |
| An Error finding of the run | Every Error in the `error` field, and no file |
| A directive of an imported file that does not validate | No finding, as for every dependency |

### Parity of the two frontends

The source frontend and the descriptor frontend must give one graph for one schema. A spec of the
descriptor package compiles each schema of the specs of the source frontend with protocompile at
`SourceInfoStandard`, loads the descriptors, and compares the two graphs: the packages, the
declarations, their positions, the attached directives and the stamped facts.

### What does not change

- The source frontend and `eidos run` over `.proto` files.
- The generators and the backends. A generator runs under the host as it runs under `eidos run`.
- The kernel, except the mark of a dependency. The host composes the run through the workspace
  builder and the input of a run.

### Verdicts

- The request becomes a tree and a store, not a graph that the host builds. The load then applies
  its partition, its resolution and its dependency rounds, and the silence of a dependency.
- A second frontend, not a source tree rebuilt from descriptors. A descriptor holds no source text,
  and a printed schema would have positions that no reader of the schema sees.
- No ledger. Each call of a plugin is a cold run, and buf writes the files. `buf generate` followed by
  `git diff --exit-code` replaces `eidos run --check`.
- The host writes whole files. Insertion points are out, because a generator of eidos writes its own
  files and extends no file of another generator.
- The host is the module `eidos-protoc`, beside `eidos-cli`. A binary of a command line and a binary
  of a protoc plugin are two entry points of one composition. `eidos-cli` and the protobuf satellite
  do not depend on each other, and the kernel does not depend on protobuf.
- The descriptor frontend is a package of the protobuf satellite, because it shares the keys, the
  editions and the codes of the satellite, and the parity spec compares it with the source frontend
  in one module.
- The binary is in the repository of the generators, and eidos ships no plugin binary.
- The kernel marks a dependency per declaration. A mark per package would make a proto package that
  a file to generate shares with an imported file a workspace package, and a generator would write
  the declarations of the imported file. Go, TypeScript and Python never share a package between
  the workspace and a store, so their runs do not change.

## Alternatives considered

### A graph that the host builds

The host could lower the descriptors into a `store.Graph` and pass it as `workspace.Input.Graph`.
It would then repeat the splice and the resolution of the load, and mark dependency packages
itself.

### `eidos run` beside `buf generate`

`eidos run` loads local `.proto` files today and needs no new code. It does not read the
dependencies that buf resolves, such as a module on the BSR, and a repository then has two entry
points for code generation.

### The comments of the source

The plugin could read the `.proto` files of the working directory and parse them with the source
frontend. buf runs a plugin over inputs that are not on disk, such as a BSR module or an archive, so
the files are often absent.

## Drawbacks

- Two frontends lower one language. The parity spec guards the agreement, and a change of the
  projection changes both.
- A composition that writes per plan needs `strategy: all`, which the host cannot detect from a
  request.
- A run under the host is always cold, so a large schema pays the full run on each call.

## Open questions

None.

## References

| What | Where |
|---|---|
| The protocol of a protoc plugin | <https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/compiler/plugin.proto> |
| `buf.gen.yaml` v2, plugins and strategies | <https://buf.build/docs/configuration/v2/buf-gen-yaml/> |
| Compilation, descriptors and source code info | <https://buf.build/docs/reference/compilation-and-descriptors/> |
| protocompile, the compiler of buf | <https://github.com/bufbuild/protocompile> |
| The bindings of gRPC | ../architecture/10-cross-language.md |
| The silence of a dependency | ../adr/0111-a-run-reports-no-finding-in-a-dependency.md |
| Dependency packages and the scope of a plan | ../architecture/08-workspace-and-plans.md |
