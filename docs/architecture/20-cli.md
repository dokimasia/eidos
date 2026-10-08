# The command kernels

*Builds on: [14](14-distribution-and-cli.md),
[16](16-diagnostics.md), [17](17-output-and-determinism.md). This is
the consumer's surface.*

The `cli` package of `go.dokimi.dev/eidos/cli` contains the command
kernels: seven complete commands that a consumer's binary mounts.
Mount them and your users get the commands, flags, output formats and
exit codes specified here, and you write no UX code to get them. This
document is the contract for that surface.

## Composition

`cli.Kernels` takes a function that composes the workspace and
returns the seven commands as values. A binary without a command line
of its own passes the same function to `cli.Main`:

```go
func main() {
    cli.Main(func() *workspace.Builder {
        return workspace.New().
            Brand("acme").
            Frontends(gofrontend.New()).
            Annotators(shapefull.Annotators()...).
            Plans(golang.ServerPlan(mygen.New()))
    })
}
```

A command calls the function once for each workspace it covers,
because it applies the config file to the builder before Build runs,
and a plugin instance belongs to one workspace. The brand that the
builder declares names the config file, the state directory and the
frame of every generated file. `version` prints the main module's path
and version from the binary's build information.

Consumers add their own commands beside the kernels: an installer, a
migration, whatever the product needs. A kernel command parses its own
flags and arguments, renders its own output and returns its own exit
status, so it mounts in any command line:

- `cli.Main` dispatches among the kernels and the consumer's
  `cli.Command` values, and panics on a consumer command that shadows
  a kernel name.
- A binary with a command line of its own, such as a cobra tree or a
  dispatcher over the standard library, mounts the values that
  `cli.Kernels` returns, at its root or under a group. Its host passes
  each command the arguments that follow the command's name, writes
  nothing to the standard streams around it, and exits with the status
  that it returns.

`run` means the same thing in every eidos-built binary: under
`cli.Main` because a shadowing command is refused, and under any other
host because `acceptancetest` runs each kernel name on the built
binary. The composition surface, pinned:

```go
type Compose func() *workspace.Builder // a fresh composition per call

type Command interface {
    Name() string
    Synopsis() string
    Usage() string
    Run(ctx context.Context, stdio IO, args []string) int // 0, 1 or 64
}

func Kernels(compose Compose) []Command
func Main(compose Compose, extra ...Command) // exits; panics on a shadowed name
func Exit(status int) error                 // nil, or an ExitError for a host whose commands return errors
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

Without a config file, the composition runs as the binary compiled it,
and the root is the directory that contains the version control
marker, or the working directory outside a repository. A config file
only refines what the binary compiled in: it disables and rescopes
compiled-in plans, sets their layouts and the plugins' options, and
assembles nothing new
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## Shared flags

Every kernel command accepts these flags after its name, between its
arguments included, and `cli.Main` also accepts them before the
command's name. `-h` and `--help` print the command's usage.

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
<name>`, which repeats and brings in the plans that each selected plan
depends on. Positional patterns narrow the commit: the run computes
every plan over the whole graph, and commits only the changes to files
that derive from a package the patterns admit
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

`--dry-run` executes every phase and writes nothing, reporting the
manifest diff as create, update, unchanged, stale, drifted, foreign or
withheld ([17-output-and-determinism.md](17-output-and-determinism.md)).

`--overwrite-drift` accepts the loss of hand edits explicitly.
Without it, drift fails the run. `--adopt` replaces a file without the
brand's frame at a path that a plan routes to.

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
it is how the warm≡cold check gets its fresh-state leg outside a
scratch directory.

`--check` is the CI gate. It runs with dry-run semantics and
promotes a non-empty diff, meaning any path that the run would create,
update or remove, or would refuse to write, to an `OutOfDate` Error at
that path. Staleness
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

Each returns across plans. `explain` reads the sealed state's live
generation and runs nothing, so it describes the last run that wrote a
generation. This is the tool that makes fixing a diagnostic cheaper
than suppressing it.

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

Diagnosis, no writes. It validates config with the same Go types
from which the published schema is generated, checks the environment,
reports deprecated directive and API usage
([15-compatibility.md](15-compatibility.md)), lists dead
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

The main module's path and version, the versions of the kernel and of
the command kernels, the contract version, every registered plugin
with its version, the composition fingerprint and the executable's
digest. The sealed state's header records the last two, and a
difference in either runs the next run cold
([09-incrementality.md](09-incrementality.md)).

## Machine output

Every command's `--format=json` emits line-delimited JSON under a
versioned schema, and the schemas are public API
([15-compatibility.md](15-compatibility.md)). The stream has the
same shape across commands: a `start` event that states the schema's
version, typed events, then one `summary` event. Standard output
contains the stream alone, and standard error is empty unless the
process panics.

```json
{"event":"start","schema":"1.0","command":"run","brand":"acme",
 "binary":{"path":"example.com/acme/cmd/acme","version":"v2.3.0"}}
{"event":"file","action":"update","path":"svc/store_stub.go",
 "plan":"go-stubs","found":"intact","hash":"sha256:9f2c…"}
{"event":"diag","code":"EIDGO-0412","severity":"error",
 "pos":"svc/store.go:41:2","msg":"chan int has no TypeScript spelling"}
{"event":"summary","status":1,"files":{"create":0,"update":1,
 "unchanged":40,"stale":0,"drifted":0,"foreign":0,"withheld":0},
 "plans":{"committed":0,"failed":1},"errors":1,"warnings":0,"infos":0,
 "suppressed":{}}
```

`plan` and `explain` emit their own event types under the same
frame. A consumer parses the events it knows and skips the rest,
which is how these schemas evolve additively like every other schema
surface.

Exit codes come from the diagnostic contract: 0 on success, 1 when
diagnostics failed the run, and 64 on a usage or config error. 2
means a crash and nothing else
([16-diagnostics.md](16-diagnostics.md)). A command returns the status,
and its host exits with it.

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
