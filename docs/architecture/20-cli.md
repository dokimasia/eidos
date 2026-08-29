# The command kernels

*Builds on: [14](14-distribution-and-cli.md),
[16](16-diagnostics.md), [17](17-output-and-determinism.md). This is
the consumer's surface.*

The `cli` package holds command kernels: complete command
implementations that a consumer's `main` composes. Compose them and
your users get the commands, flags, output formats and exit codes
specified here, and you write no UX code to get them. This document
is the contract for that surface.

## Composition

```go
func main() {
    ws := workspace.New().
        Frontends(gofrontend.New()).
        Annotators(shapefull.Annotators()...).
        Plans(golang.ServerPlan(mygen.New()))
    cli.Main(ws)
}
```

`cli.Main` wires up the command tree below, config discovery, the
global flags, diagnostic rendering
([16-diagnostics.md](16-diagnostics.md)) and the exit codes. The
consumer supplies its identity, meaning brand and version, through
workspace config, and that identity appears in help text, in
generated-file headers and in `version` output.

Consumers may add their own commands beside the kernels: an
installer, a migration, whatever the product needs. The kernel
command names are reserved, and a consumer command may not shadow
one, so `run` means the same thing in every eidos-built binary. The
composition surface, pinned:

```go
func Main(ws *workspace.Workspace, extra ...Command) int

type Command interface {          // the extension point
    Name() string                 // shadowing a kernel name is refused
    Run(ctx Context, args []string) int
}
```

## Config discovery

`--config <path>` wins outright. Otherwise discovery walks up from
the working directory to the first `.<brand>.yaml`, stopping at the
VCS root. That file names either one workspace or the explicit
`workspaces:` list
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

The filename comes from the brand and is never `eidos.yaml`, because
no eidos binary exists to claim that name
([14-distribution-and-cli.md](14-distribution-and-cli.md)), and two
eidos-built binaries sharing a repository must not read each other's
config.

The same scoping carries the state. Each brand owns `.<brand>/`
beside its config file, holding the manifest, the cache and the lock
([17-output-and-determinism.md](17-output-and-determinism.md)), one
per workspace root, found by the same walk, and one per entry under
`workspaces:`.

A binary composed purely in Go, with the workspace built in `main`
and no config file, skips discovery entirely. Config then only
selects among what the binary compiled in.

## Global flags

| Flag | Effect |
|---|---|
| `--format=text\|json` | human rendering, or line-delimited JSON under the versioned schemas |
| `--config <path>` | use this config; discovery is skipped |
| `--strict` | promote Warnings to Errors |
| `--verbose` | include Info diagnostics |
| `--no-color` / `NO_COLOR` | plain text. TTY rendering never affects file output |

## The commands

### run

Executes the workspace: every plan, or a subset through `--plan
<name>`, which repeats. Positional arguments narrow the source
patterns within the workspace scope.

`--dry-run` executes every phase and writes nothing, reporting the
manifest diff as create, update, unchanged, stale or drifted
([17-output-and-determinism.md](17-output-and-determinism.md)).

`--overwrite-drift` accepts the loss of hand edits explicitly.
Without it, drift fails the run.

`go generate` composes with `run` without any narrowing flag,
because it executes the command in the directive's package
directory. So

 //go:generate mybrand run .

is an ordinary pattern-narrowed run scoped to that package. D59's
refusal of symbol-level narrowing is untouched, and the
narrowed-run sweep rule
([08-workspace-and-plans.md](08-workspace-and-plans.md)) keeps
out-of-scope outputs safe.

`--cold` ignores the sealed state and the parse memo without
deleting either ([09-incrementality.md](09-incrementality.md)). It
is the handle to reach for when warm behaviour is in question, and
it is how the warm≡cold rung gets its fresh-state leg outside a
scratch directory.

`--check` is the CI gate. It runs with dry-run semantics and
promotes a non-empty diff, meaning any create, update, stale or
drifted entry, to an Error diagnostic naming the paths. Staleness
therefore exits 1 through the ordinary contract, and enforcing
"committed output is current" needs no JSON parsing.

### plan

Shows the resolved execution picture without running anything:
plugins per phase in bucket and topological order, per-plan layout
policies, export edges between plans, and the source scope each plan
sees. This is a dry run of the composition, showing what would
execute, where `run --dry-run` applies the composition to sources
and shows what would change.

### explain

The provenance walk ([04-metadata.md](04-metadata.md)). The target
argument takes four forms, told apart by shape:

| Form | Example | Answers |
|---|---|---|
| path | `svc/store_stub.go` | which plan, plugins and source declarations produced this file |
| symbol identity | `golang:svc/store.Store#Get` | everything stamped on and generated from this symbol |
| key at position | `shape.name@svc/store.go:41` | who stamped it, at what authority, from which reads |
| code at position | `EIDGO-0412@svc/store.go:41` | the facts and reads behind the diagnostic |

Each answers across plans. This is the tool that makes fixing a
diagnostic cheaper than suppressing it.

### prune

The staleness sweep on its own: it removes manifested files whose
producing plan or declaration no longer exists. It never touches an
unmanifested file and never touches a drifted one
([17-output-and-determinism.md](17-output-and-determinism.md)), and
`--dry-run` lists what would go.

`prune` is also the whole-workspace complement to the narrowed-run
sweep rule
([08-workspace-and-plans.md](08-workspace-and-plans.md)): what a
scoped `run` deliberately leaves alone, `prune` reconciles.

### doctor

Diagnosis, no writes. It validates config against the published
schemas, checks the environment, reports deprecated directive and
API usage ([15-compatibility.md](15-compatibility.md)), lists dead
suppressions ([16-diagnostics.md](16-diagnostics.md)), and runs the
schema-evolution check, diffing the workspace's directive usage
against the current schema index so a consumer sees breaking
annotation changes before an upgrade bites.

### watch

`run` in a loop. It polls the fingerprint gate, since the stat and
hash pass is already the change detector, so there is no watcher
dependency and no daemon
([09-incrementality.md](09-incrementality.md)). `--interval` tunes
the poll, and the default is one second.

### version

The kernel version, the contract version, every registered plugin
with its version, and the composed plugin-set fingerprint, which is
the identity that keys caches and appears in the manifest.

## Machine output

Every command's `--format=json` emits line-delimited JSON under a
versioned schema, and the schemas are public API
([15-compatibility.md](15-compatibility.md)). The stream has the
same shape across commands: typed event objects, then one summary
object.

```json
{"event":"file","action":"update","path":"svc/store_stub.go",
 "plan":"go-stubs","hash":"sha256:9f2c…"}
{"event":"diag","code":"EIDGO-0412","severity":"error",
 "pos":"svc/store.go:41:2","msg":"chan int has no TypeScript spelling"}
{"summary":{"files":{"create":1,"update":1,"unchanged":40,"stale":0,
 "drifted":0},"errors":1,"warnings":0,"suppressed":{}}}
```

`plan` and `explain` emit their own event types under the same
frame. A consumer parses the events it knows and skips the rest,
which is how these schemas evolve additively like every other schema
surface.

Exit codes come from the diagnostic contract: 0 on success, 1 when
diagnostics failed the run, and 64 on a usage or config error. 2
means a crash and nothing else
([16-diagnostics.md](16-diagnostics.md)).

A consumer's CI composes these without parsing any message text:

```sh
myg run --format=json || exit 1     # generation gate
myg run --check                     # CI gate: committed output is current
myg run --dry-run --format=json     # review surface: what would change
myg doctor --format=json            # upgrade pre-flight
```

## What the kernels promise

They are the only I/O surface. They read config and sources, write
through sinks and the diagnostic layer, and touch nothing else. The
only hidden state is the brand's state directory `.<brand>/`,
holding the manifest, the cache and the lock
([17-output-and-determinism.md](17-output-and-determinism.md)).

Their flags, JSON schemas and exit codes are versioned with the
kernel and follow the compatibility policy, so a consumer's script
written against `run --format=json` survives every minor release.
