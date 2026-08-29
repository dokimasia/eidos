# The command kernels

*Builds on: [14](14-distribution-and-cli.md),
[16](16-diagnostics.md), [17](17-output-and-determinism.md). This
is the consumer's surface.*

What a consumer's binary offers its users is mostly what eidos
offers the consumer: the `cli` package is command *kernels* —
complete command implementations a `main` composes — so every
eidos-built binary presents the same commands, flags, output
formats, and exit codes without the consumer writing UX code. This
document is the contract for that surface.

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

`cli.Main` wires: the command tree below, config discovery, the
global flags, diagnostic rendering
([16-diagnostics.md](16-diagnostics.md)), and exit codes. The
consumer supplies identity (brand, version) through workspace
config; it appears in help text, generated-file headers, and
`version` output.

Consumers may add their own commands beside the kernels — an
installer, a migration, whatever their product needs. The kernel
command names are reserved: a consumer command may not shadow one,
so `run` means the same thing in every eidos-built binary. The
composition surface, pinned:

```go
func Main(ws *workspace.Workspace, extra ...Command) int

type Command interface {          // the extension point
    Name() string                 // shadowing a kernel name is refused
    Run(ctx Context, args []string) int
}
```

## Config discovery

`--config <path>` wins. Otherwise discovery walks up from the
working directory to the first `.<brand>.yaml` (stopping at the
VCS root), which names either one workspace or the explicit
`workspaces:` list ([08-workspace-and-plans.md](08-workspace-and-plans.md)).
The filename is brand-derived, never `eidos.yaml` — no eidos
binary exists to claim that name
([14-distribution-and-cli.md](14-distribution-and-cli.md)), and
two eidos-built binaries sharing one repository must not read
each other's config. The same scoping carries the state: each
brand owns `.<brand>/` beside its config file — manifest, cache,
lock ([17-output-and-determinism.md](17-output-and-determinism.md))
— one per workspace root, found by the same walk, one per entry
under `workspaces:`. A binary composed purely in Go (workspace
built in `main`, no config file) skips discovery entirely; config
then only *selects* among what the binary compiled in.

## Global flags

| Flag | Effect |
|---|---|
| `--format=text\|json` | human rendering, or line-delimited JSON under the versioned schemas |
| `--config <path>` | explicit config; disables discovery |
| `--strict` | promote Warnings to Errors |
| `--verbose` | include Info diagnostics |
| `--no-color` / `NO_COLOR` | plain text (TTY rendering never affects file output) |

## The commands

### run

Executes the workspace: every plan, or `--plan <name>` (repeatable)
for a subset. Positional arguments narrow the source patterns within
the workspace scope. `--dry-run` executes every phase and writes
nothing, reporting the manifest diff — create / update / unchanged /
stale / drifted
([17-output-and-determinism.md](17-output-and-determinism.md)).
`--overwrite-drift` accepts the loss of hand edits explicitly; drift
otherwise fails the run.

`go generate` composes with `run` without any narrowing flag: it
executes the command in the directive's package directory, so

 //go:generate mybrand run .

is an ordinary pattern-narrowed run scoped to that package. D59's
refusal of symbol-level narrowing is untouched, and the
narrowed-run sweep rule
([08-workspace-and-plans.md](08-workspace-and-plans.md)) keeps
out-of-scope outputs safe.

`--cold` ignores the sealed state and the parse memo without
deleting either ([09-incrementality.md](09-incrementality.md)) —
the debugging handle when warm behavior is in question, and how
the warm≡cold rung gets its fresh-state leg outside a scratch
directory.

`--check` is the CI gate: dry-run semantics, with a non-empty
diff — any create, update, stale, or drifted entry — promoted to
an Error diagnostic naming the paths. Staleness therefore exits 1
through the ordinary contract, and "committed output is current"
needs no JSON parsing to enforce.

### plan

The resolved execution picture without running: plugins per phase in
bucket and topo order, per-plan layout policies, export edges
between plans, the source scope each plan sees. This is `DryRun` of
the *composition* (what would execute), where `run --dry-run` is the
composition applied to *sources* (what would change).

### explain

The provenance walk ([04-metadata.md](04-metadata.md)). The target
argument has four forms, disambiguated by shape:

| Form | Example | Answers |
|---|---|---|
| path | `svc/store_stub.go` | which plan, plugins, and source declarations produced this file |
| symbol identity | `golang:svc/store.Store#Get` | everything stamped on and generated from this symbol |
| key at position | `shape.name@svc/store.go:41` | who stamped it, at what authority, from which reads |
| code at position | `EIDGO-0412@svc/store.go:41` | the facts and reads behind the diagnostic |

Across plans, in every case. The tool that makes fixing cheaper
than suppressing.

### prune

The staleness sweep, standalone: removes manifested files whose
producing plan or declaration no longer exists. Never touches
unmanifested files, never touches drifted files
([17-output-and-determinism.md](17-output-and-determinism.md));
`--dry-run` lists what would go. `prune` is also the
whole-workspace complement of the narrowed-run sweep rule
([08-workspace-and-plans.md](08-workspace-and-plans.md)): what a
scoped `run` deliberately leaves untouched, `prune` reconciles.

### doctor

Diagnosis, no writes: config validation against the published
schemas; environment checks; deprecated directive and API usage
([15-compatibility.md](15-compatibility.md)); dead suppressions
([16-diagnostics.md](16-diagnostics.md)); and schema-evolution
checks — the workspace's directive usage diffed against the current
schema index, so a consumer sees breaking annotation changes before
an upgrade bites.

### watch

`run` in a loop: polls the fingerprint gate (the stat/hash pass is
the change detector — no watcher dependency, no daemon,
[09-incrementality.md](09-incrementality.md)) and re-runs on
change. `--interval` tunes the poll; the default is one second.

### version

Kernel version, contract version, every registered plugin with its
version, and the composed plugin-set fingerprint — the identity
that keys caches and appears in the manifest.

## Machine output

Every command's `--format=json` emits line-delimited JSON under a
versioned schema; the schemas are public API
([15-compatibility.md](15-compatibility.md)). The stream shape is
uniform across commands — typed event objects, then one summary
object:

```json
{"event":"file","action":"update","path":"svc/store_stub.go",
 "plan":"go-stubs","hash":"sha256:9f2c…"}
{"event":"diag","code":"EIDGO-0412","severity":"error",
 "pos":"svc/store.go:41:2","msg":"chan int has no TypeScript spelling"}
{"summary":{"files":{"create":1,"update":1,"unchanged":40,"stale":0,
 "drifted":0},"errors":1,"warnings":0,"suppressed":{}}}
```

`plan` and `explain` emit their own event types under the same
frame; a consumer parses the events it knows and skips the rest —
additive evolution, like every schema surface. Exit codes are the
diagnostic contract's: 0 success, 1 diagnostics failed the run, 64
usage or config error; 2 is a crash and nothing else
([16-diagnostics.md](16-diagnostics.md)). A
consumer's CI composes these without parsing prose:

```sh
myg run --format=json || exit 1     # generation gate
myg run --check                     # CI gate: committed output is current
myg run --dry-run --format=json     # review surface: what would change
myg doctor --format=json            # upgrade pre-flight
```

## What the kernels promise

They are the only I/O surface: kernels read config and sources,
write through sinks and the diag layer, and touch nothing else — no
hidden state beyond the brand's state directory (`.<brand>/`:
manifest, cache, lock —
[17-output-and-determinism.md](17-output-and-determinism.md)). Their flags, JSON schemas, and exit codes version with
the kernel and follow the compatibility policy; a consumer's script
written against `run --format=json` survives every minor release.
