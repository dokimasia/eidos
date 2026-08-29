# Distribution and the CLI

*Builds on: [06](06-plugins.md) (the declarative host),
[08](08-workspace-and-plans.md) (the workspace a binary composes).
Feeds: [20](20-cli.md).*

## eidos is a library, full stop

The project ships **no binary, ever** — no batteries-included
executable, no rebuild tool, no `cmd/` in the kernel. What it ships
instead is everything a consumer needs to make *their* binary the
product:

- **Command kernels** (`eidos/cli`): run, plan, explain, prune,
  doctor, version, watch — plus config loading, flag conventions,
  and diagnostic formatting. A consumer composes them, so every
  eidos-built binary presents the same command surface, flags, and
  output formats without the consumer writing UX code.
- **The workspace builder and plan values**: a consumer's `main` is
  registration plus `cli.Main(...)`.
- **The declarative host** ([06-plugins.md](06-plugins.md)): a
  consumer that compiles it in gives *its* users
  extension-without-toolchain.

The "users who don't write Go" question is answered by
**consumers**, not by eidos: dokimi — the reference consumer — ships
a batteries-included non-extensible binary to its users; an org's
platform team ships theirs with the org's plugins and the
declarative host enabled. eidos' audience is the people who build
those binaries.

Considered and cut: a rebuild tool (config → generated main →
`go build`). Even with `GOTOOLCHAIN`
self-management it drags a toolchain bootstrap into a non-Go user's
path, and its users — typed-plugin adopters — are Go developers who
don't need it. Anyone may build one later on the command kernels; eidos
doesn't owe it. ([21-decisions.md](21-decisions.md), D13.)

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

Config (YAML with the published JSON Schema) drives the same typed
structs for binaries whose users select among compiled-in plugins —
selection at run time, existence at build time.

What a consumer's `go.mod` actually takes, and at what discipline:

| Dependency | Discipline |
|---|---|
| `go.dokimi.dev/eidos/core` | semver promise, canary-tested, additive within a major ([15-compatibility.md](15-compatibility.md)) |
| `go.dokimi.dev/eidos/lang-<lang>` | own cadence; declares and CI-proves its kernel range |
| `go.dokimi.dev/eidos/plugin-shape` | own cadence; catalog grows by spec |
| third-party typed plugins | code you vet; conformance suite is the acceptance test |
| declarative plugins | inert files; manifest hashes into the fingerprint |

## Command kernels

run, plan, explain, prune, doctor, watch, version — each a complete
implementation with a versioned `--format=json` twin, so consumer
binaries are scriptable by default. The per-command contracts,
flags, config discovery, and exit codes are specified in
[20-cli.md](20-cli.md).

## Docs are a product

A public framework's second deliverable, and generated-from-truth
wherever truth exists — a docs page that can drift from a
registry is a support ticket on a delay:

| Artifact | Generated from | When |
|---|---|---|
| funcmap reference | backend + plugin registrations | per release |
| directive reference | registered schemas + their constants | per release |
| shape catalog pages | eidos-plugin-shape `spec/`, verbatim | per eidos-plugin-shape release |
| support matrices | the completeness rung's run | per satellite release |
| config reference | the published JSON Schema | per kernel release |
| diagnostic-code index | the diag registry, stable anchors | per release |
| parity matrix | the key registrations audit ([04-metadata.md](04-metadata.md)) | per release |

Each generated artifact publishes beside the release's
compatibility artifacts ([15-compatibility.md](15-compatibility.md))
and regenerating it is part of the tag pipeline — a stale page
blocks the tag the way a failing rung does. Hand-written and
staying that way: the tutorials, the concept docs, and these
architecture documents.
