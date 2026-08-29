# Diagnostics

*Builds on: [05](05-directives.md) (the diag directive). Feeds:
[20](20-cli.md) (exit codes and machine output), and the failure
semantics of every document.*

Every layer of the system reports through one diagnostic model. A
diagnostic is data, not prose: consumers script against it, tests
assert on it, docs anchor to it.

## The diagnostic

| Field | Content |
|---|---|
| Code | stable identifier (`EID-####` kernel, `EIDGO-####` etc. satellites, consumer prefixes registered at Build) |
| Severity | `Error \| Warning \| Info` |
| Position | file:line:column of the declaration that caused it — every diagnostic is positioned; a positionless report is a bug in its emitter |
| Message | one sentence, present tense, naming the thing and the refusal or finding |
| Origin | the plugin (or kernel phase) that emitted it |
| Related | zero or more secondary positions (the colliding twin, the export it depends on) |

Codes are API ([15-compatibility.md](15-compatibility.md)): a
changed meaning is a new code. Code uniqueness within a prefix and
prefix ownership are enforced at workspace Build, the same way
metadata namespaces are. Every registered code generates a named
constant in its repo (`codes.NoTargetSpelling`, carrying
`EIDGO-0412`) — tests and consumer tooling assert on the constant;
the digit form is boundary spelling for humans, carriers, and JSON.

The shape, pinned:

```go
type Diag struct {
    Code     Code            // generated constant; EID-#### is spelling
    Severity Severity        // Error | Warning | Info
    Pos      position.Pos    // never zero — a positionless Diag is a
                             // bug in its emitter, caught by plugintest
    Msg      string          // one sentence, present tense
    Origin   PluginID        // or the kernel phase
    Related  []position.Pos  // the colliding twin, the export, …
}

func RegisterCode(p Prefix, s CodeSpec) Code

type CodeSpec struct {
    Number  int    // unique within the prefix, enforced at Build
    Meaning string // anchored in the code index; a changed meaning
}                  // is a new code, never an edit
```

## Severity and failure semantics

- **Error** — the run is wrong: a refusal, a violated contract, a
  collision. Any Error fails the run: the workspace returns a joined
  error carrying every code, after completing what plan isolation
  allows ([08-workspace-and-plans.md](08-workspace-and-plans.md)).
- **Warning** — the output stands but something deserves a human:
  a deprecated directive, a violated completeness contract declared
  advisory ([04-metadata.md](04-metadata.md)). Warnings never fail a
  run; `--strict` promotes them to Errors for consumers who want
  that gate.
- **Info** — provenance and progress facts, off by default, on with
  `--verbose`.

Consumer binaries built on the command kernels exit 0 on success,
1 when diagnostics failed the run, and 64 — the sysexits
`EX_USAGE` convention — on usage or config errors. 2 is
deliberately unclaimed: a Go panic exits 2, and with no panic
recovery (below) that exit unambiguously means "crash, file a
bug". No script ever parses stderr to tell a crash from a typo.

## Panics

eidos does not recover plugin panics. A panic is a defect, not a
diagnostic: the run crashes with the plain Go stack, and the exit
is a crash, outside the exit-code contract. Two facts make the
policy safe to hold. The two-phase commit
([08-workspace-and-plans.md](08-workspace-and-plans.md)) means a
crash mid-run leaves disk exactly as the previous generation
wrote it — staging is discarded, nothing partial lands. And the
conformance suite's does-not-panic assertions
([13-testing-and-conformance.md](13-testing-and-conformance.md))
are where panics are caught before a plugin ships. Recovery
machinery would convert defects into Warning-shaped noise someone
learns to ignore, attached to a run whose remaining output just
lost its witness. The crash's exit code is Go's own 2 — unclaimed
by the contract above precisely so it stays unambiguous.

## Suppression

False positives happen; a public framework owes a targeted opt-out
that cannot rot into a global mute:

- `+gen:diag off=EID-1234` at a declaration suppresses that code at
  that declaration — `diag` is a kernel-owned directive, so the
  carrier and grammar are ordinary directive machinery
  ([05-directives.md](05-directives.md)), read by the diagnostic
  sink at filter time rather than stamped as a fact.
- Suppression is scoped to a code at a position. There is no
  file-level or workspace-level suppression of Errors; a check that
  is wrong workspace-wide is disabled by not running the plugin, and
  a *kernel* Error is never suppressible — it reports a broken run,
  not an opinion.
- Suppressions are audited: the run summary counts them per code,
  and `doctor` lists every suppression whose code no longer fires —
  dead suppressions are the lint debt of this system.

## Machine output

`--format=json` renders diagnostics as line-delimited JSON under a
versioned schema shared with every command kernel
([14-distribution-and-cli.md](14-distribution-and-cli.md)): one
object per diagnostic, a final summary object with counts by
severity and code, suppression counts included. The schema is public
API. The shape, concretely:

```json
{"code":"EIDGO-0412","severity":"error","pos":"svc/store.go:41:2",
 "msg":"chan int has no TypeScript spelling",
 "origin":"lowering/typescript","related":["svc/api.go:12:1"]}
{"summary":{"errors":1,"warnings":0,"suppressed":{"EID-0007":2}}}
```

## Explain integration

`explain` accepts a diagnostic code plus position and walks the
provenance that produced it ([04-metadata.md](04-metadata.md)):
which facts, stamped by whom, derived from which reads. A diagnostic
a consumer cannot trace to its inputs is a diagnostic they will
suppress instead of fix; the walk is what makes fixing cheaper than
suppressing.
