# Distribution and the CLI

*Builds on: [06](06-plugins.md) (the declarative host),
[08](08-workspace-and-plans.md) (the workspace a binary composes).
Feeds: [20](20-cli.md).*

## eidos is a library, full stop

The project ships **no binary, ever**. There is no
batteries-included executable, no rebuild tool and no `cmd/` in the
kernel. What it ships instead is everything a consumer needs to make
their own binary the product:

- **Command kernels**, in `eidos/cli`: run, plan, explain, prune,
  doctor, version and watch, plus config loading, flag conventions
  and diagnostic formatting. A consumer composes them, so every
  eidos-built binary offers the same commands, flags and output
  formats without its author writing any UX code.
- **The workspace builder and plan values.** A consumer's `main` is
  registration plus `cli.Main(...)`.
- **The declarative host** ([06-plugins.md](06-plugins.md)). A
  consumer that compiles it in lets its own users extend the tool
  without a toolchain.

Consumers answer the question about users who do not write Go, not
eidos. dokimi, the reference consumer, ships a batteries-included
binary that its users cannot extend. An organisation's platform team
ships theirs with the organisation's plugins and the declarative
host enabled. eidos' audience is the people who build those
binaries.

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
    ws := workspace.New().
        Frontends(gofrontend.New(), protofrontend.New()).
        Annotators(shapefull.Annotators()...).
        Plans(
            golang.ServerPlan(mygen.New()),
            typescript.ClientPlan(),
        )
    cli.Main(ws) // run | plan | explain | prune | doctor | watch
}
```

Config, written as YAML against the published JSON Schema, drives
the same typed structs. That is how a binary lets its users choose
among the plugins it compiled in: selection happens at run time,
existence at build time.

What a consumer's `go.mod` takes on, and under what discipline:

| Dependency | Discipline |
|---|---|
| `go.dokimi.dev/eidos/core` | semver, canary-tested, additive within a major ([15-compatibility.md](15-compatibility.md)) |
| `go.dokimi.dev/eidos/lang/<lang>` | its own cadence; declares a kernel range and proves it in CI |
| `go.dokimi.dev/eidos/plugin-shape` | its own cadence; the catalog grows by spec |
| third-party typed plugins | code you vet; the conformance suite is the acceptance test |
| declarative plugins | inert files; the manifest hashes into the fingerprint |

## Command kernels

run, plan, explain, prune, doctor, watch and version. Each is a
complete implementation with a versioned `--format=json` twin, so
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
