<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Distribution and the CLI

*Builds on: [06](06-plugins.md) (the declarative host),
[08](08-workspace-and-plans.md) (the workspace a binary composes).
Feeds: [20](20-cli.md).*

## The library and the binary eidos

eidos is a library first. It provides everything a consumer needs to
make their own binary the product:

- **Command kernels**, in `eidos/cli`: run, plan, explain, prune,
  doctor, watch, version and schema, plus config loading, the global
  options and diagnostic formatting. A binary built with eidos is an
  app of go-cli, `go.stealthscale.io/cli`, with the kernel commands
  beside its own commands, so every eidos-built binary offers the same
  commands, options and output formats without its author writing any
  UX code.
- **The workspace builder and plan values.** A consumer's `main` is
  a composition function, `cli.New(...)` and `cli.Main(...)`, or an app
  of its own that mounts the kernel commands as subcommands of a group
  with `cli.Mount(...)`.
- **The declarative host**, in `eidos/declarative`
  ([06-plugins.md](06-plugins.md)). A binary admits declarative
  plugins into the plans that it marks `Extensible`, so its users
  extend the tool without a toolchain.

eidos also publishes one binary, `eidos`, the main package `cmd/eidos`
of the root module. It composes the frontends of Go, TypeScript, Java,
Rust, protobuf and GraphQL, the rules of Go, TypeScript, protobuf and
GraphQL, and one extensible plan for each of the targets golang,
typescript, java, rust, protobuf and graphql. It has no generator and
no annotator of its own. Its users add
declarative plugins to the plans in `.eidos.yaml`. A team that does
not write Go runs its plugins without a binary of its own
([21-decisions.md](21-decisions.md), D160).

`go install go.dokimi.dev/eidos/cmd/eidos@latest` installs the
binary. The root module has no replace directive, and the go command
builds it from published modules. The frontends of
TypeScript, Java and Rust parse through cgo. The build needs a C
compiler for them. Each release of the root module also publishes
archives for darwin, Linux and Windows and a Homebrew cask. ergon
builds the commands of `.ergon.yaml` with cgo off, so the job
`binaries` of `release.yml` builds them from `.goreleaser.yml` on
macOS: darwin with clang, Linux and Windows with zig.

A consumer with typed plugins still builds its own binary. dokimi,
the reference consumer, ships a batteries-included binary that its
users cannot extend. An organisation's platform team ships theirs with
the organisation's plugins and the declarative host enabled.

Considered and cut: a rebuild tool that turns config into a
generated `main` and runs `go build`. Even with `GOTOOLCHAIN`
managing itself, it drags a toolchain bootstrap into the path of
someone who does not write Go, and the people who would use it,
typed-plugin adopters, are Go developers who do not need it. Anyone
may build one later on top of the command kernels. eidos does not
owe it ([21-decisions.md](21-decisions.md), D13).

## The consumer binary

```go
func main() {
    compose := func() *workspace.Builder {
        return workspace.New().
            Brand("acme").
            Frontends(gofrontend.New(), protofrontend.New()).
            Annotators(shapefull.Annotators()...).
            Plans(
                golang.ServerPlan(mygen.New()),
                typescript.ClientPlan(),
            )
    }
    // run | plan | explain | prune | doctor | watch | version | schema
    cli.Main(cli.New("Generates the code of the acme platform.", compose), compose)
}
```

Config, written as YAML against the published JSON Schema, drives
the same typed structs. That is how a binary lets its users choose
among the plans it compiled in and set the options of its plugins:
selection happens at run time, existence at build time. A plan runs
whole or not at all, because the config file does not assemble a
plan from the plugins of the binary. The command `schema` prints the
JSON Schema of the config file of one binary, with the options of its
plugins, and a repository points an editor at the file that it writes:

```sh
acme schema > .acme.schema.json
```

The commands that a binary adds read their settings from the key
`extensions` of the same file. The command kernels never read that key,
and `Member.Extension` decodes one extension with the strict rules of
the file ([20-cli.md](20-cli.md)).

A plan that the binary marks `Extensible` also admits the declarative
plugins that the config file lists under the plan's key `plugins`.
Each entry is a plugin directory, absolute or relative to the
workspace root, with slashes:

```yaml
version: 1
plans:
    go-audit:
        plugins: [plugins/acme-audit]
    ts-audit:
        plugins: [plugins/acme-audit]
options:
    acme-audit: {redact: false}
```

The command kernels load each directory once for each workspace, so a
directory in two plans is one plugin in both plans. A directory that
does not load fails the command with a config error and the exit status
64. Build refuses plugins for a plan that the binary does not mark
`Extensible`, so a binary with a fixed product keeps its output fixed.
An extensible plan does not need a generator of its own. Without a
plugin it does not write a file. The next run of a plan whose last
plugin the config file dropped removes the files that the plugin wrote
([21-decisions.md](21-decisions.md), D161).

What a consumer's `go.mod` takes on, and under what discipline:

| Dependency | Discipline |
|---|---|
| `go.dokimi.dev/eidos/core` | semver, canary-tested, additive within a major ([15-compatibility.md](15-compatibility.md)) |
| `go.dokimi.dev/eidos/cli` | semver; the command kernels, whose options, JSON schemas and exit codes follow the same policy ([20-cli.md](20-cli.md)) |
| `go.stealthscale.io/cli` | semver; the command line of every eidos-built binary, which `eidos/cli` requires |
| `go.dokimi.dev/eidos/protoc` | semver; the host of a protoc plugin binary, whose parameter mapping and response follow the same policy |
| `go.dokimi.dev/eidos/declarative` | semver; the manifest format and its JSON Schema follow the same policy |
| `go.dokimi.dev/eidos/lang/<lang>` | its own cadence; declares a kernel range and proves it in CI |
| `go.dokimi.dev/eidos/plugin-shape` | its own cadence; the catalog grows by spec |
| third-party typed plugins | code you vet; the conformance suite is the acceptance test |
| declarative plugins | inert files; the manifest hashes into the fingerprint |

## The protoc plugin binary

A composition of generators also runs as a protoc plugin, so `protoc`,
`buf generate` and a remote plugin on the BSR run it over the
descriptors of a schema. The module `eidos-protoc`
(`go.dokimi.dev/eidos/protoc`) is the host. The binary is
`cmd/protoc-gen-<name>` in the repository of the generators, and calls
`protoc.Main` with the same composition function that it passes to
`cli.New`:

```go
func main() { protoc.Main(acme.Compose) }
```

- **The load.** The host adds the descriptor frontend of
  `lang/protobuf/descriptor`. The files to generate form an in-memory
  tree, and every other file of the request is a file of the store
  `protoc`, which a dependency round loads. A generator runs on no
  declaration of an imported file, also where the imported file shares
  the proto package of a file to generate
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)).
- **The parameter.** `<plugin>.<option>=<value>` sets an option of a
  plugin, and `policy.<key>=<choice>` selects the choice of a policy.
  `plan.<plan>.<field>=<value>` sets `importBase`, `dir`, `policy` or
  `disabled` of a plan, with the names of a config file. A Go plan needs
  `importBase`, because a request carries no Go module. A value is JSON
  where it decodes as JSON, and a string otherwise. A comma inside a JSON
  string, list or object stays in its value.
- **The run.** Each call is a cold run into an in-memory sink, without
  a ledger. The response carries every file, and buf writes them. The
  Error findings go into the `error` field of the response, and the
  Warning and Info findings go to standard error.
- **The strategy.** A plugin of eidos needs `strategy: all` in
  `buf.gen.yaml`. buf calls a local plugin once per directory by
  default, and a declaration that refers to a message of another
  directory is then refused, because the call does not generate that
  message. The response then opens with `protoc.ErrImportedReference`,
  which names the strategy.

```yaml
version: v2
clean: true
plugins:
  - local: protoc-gen-acme
    out: gen
    strategy: all
    opt:
      - plan.go.importBase=example.com/app/gen
```

([21-decisions.md](21-decisions.md), D233 and D234.)

## Command kernels

run, plan, explain, prune, doctor, watch, version and schema. Each is
a complete implementation with a versioned `--format=json` twin, so
consumer binaries are scriptable by default.
[20-cli.md](20-cli.md) specifies the per-command contracts, the
flags, config discovery and the exit codes.

## Documentation ships with the release

A public framework's second deliverable is its documentation, and
wherever the truth already lives in a registry, the page is
generated from it. A page that can disagree with a registry is a
support ticket waiting to happen.

| Artifact | Generated from | When |
|---|---|---|
| funcmap reference | backend and plugin registrations | per release |
| directive reference | registered schemas and their constants | per release |
| shape catalog pages | eidos-plugin-shape `spec/`, verbatim | per eidos-plugin-shape release |
| support matrices | the completeness check's run | per satellite release |
| config reference | the published JSON Schema | per kernel release |
| diagnostic-code index | the diag registry, with stable anchors | per release |
| parity matrix | the key registrations audit ([04-metadata.md](04-metadata.md)) | per release |

Each generated artifact publishes beside the release's compatibility
artifacts ([15-compatibility.md](15-compatibility.md)), and
regenerating it is part of the tag pipeline: a stale page blocks the
tag the way a failing check does. Three things stay hand-written: the
tutorials, the concept documents, and these architecture documents.
