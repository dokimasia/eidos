<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Diagnostics

*Builds on: [05](05-directives.md) (the diag directive). Feeds:
[20](20-cli.md) (exit codes and machine output), and the failure
semantics of every document.*

Every layer of the system reports through one diagnostic model. A
diagnostic is a record with named fields, and a code is one of them.
Consumers script against those fields, tests assert on the code, and
the documentation anchors to it.

## The diagnostic

| Field | Content |
|---|---|
| Code | a stable identifier: `EID-####` for the kernel, one prefix per satellite, such as `GOLANG-####` and `PROTO-####`, and consumer prefixes registered at Build |
| Severity | `Error \| Warning \| Info` |
| Position | file:line:column of the declaration that caused it. Every diagnostic is positioned, and one without a position is a bug in whatever emitted it |
| Message | one sentence, present tense, naming the thing and the refusal or the finding |
| Origin | the plugin, or the kernel phase, that emitted it |
| Related | zero or more secondary positions, such as the colliding twin or the export it depends on |

Codes are API ([15-compatibility.md](15-compatibility.md)), so a
changed meaning is a new code. Build enforces code uniqueness within
a prefix and prefix ownership, the same way it enforces metadata
namespaces. Every registered code is a named constant in its own
module, such as the Go frontend's `BadCarrier`, which spells
`GOLANG-0002`.
Tests and consumer tooling assert on the constant, and the digit
form is what humans, carriers and JSON see.

The shape, pinned:

```go
type Diag struct {
    Code     Code            // generated constant; EID-#### is the spelling
    Severity Severity        // Error | Warning | Info
    Pos      position.Pos    // never zero; a positionless Diag is a
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

**Error** means the run is wrong: a refusal, a violated contract, a
collision. Any Error fails the run. The workspace returns a joined
error carrying every code, after completing whatever plan isolation
allows ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

**Warning** means the output stands but a human should look: a
deprecated directive, or a completeness contract that was declared
advisory and went unmet ([04-metadata.md](04-metadata.md)). Warnings
never fail a run. `--strict` promotes them to Errors for consumers
who want that gate.

**Info** covers provenance and progress. It is off by default and on
with `--verbose`.

Consumer binaries built on the command kernels exit 0 on success, 1
when diagnostics failed the run, and 64 on a usage or config error,
following the sysexits `EX_USAGE` convention. 2 is deliberately
unclaimed. A Go panic exits 2, and since eidos does not recover
panics, that exit means "it crashed, file a bug" and nothing else.
No script has to read stderr to tell a crash from a typo.

## Panics

eidos does not recover a plugin panic. A panic is a defect rather
than a diagnostic, so the run crashes with the plain Go stack and
the exit falls outside the exit-code contract.

Two facts make that safe to hold. The two-phase commit
([08-workspace-and-plans.md](08-workspace-and-plans.md)) means a
crash mid-run leaves disk exactly as the previous generation wrote
it: staging is discarded and nothing partial arrives. And the
conformance suite asserts that fixtures do not panic
([13-testing-and-conformance.md](13-testing-and-conformance.md)),
which is where panics get caught before a plugin ships.

Recovery machinery would turn a defect into a warning that people
learn to ignore, attached to a run whose remaining output nobody can
trust. The crash keeps Go's own exit code 2, which the contract
above leaves unclaimed for exactly this reason.

## Suppression

False positives happen, and a public framework owes a targeted
opt-out that cannot decay into a blanket mute.

- `+<brand>:diag off=EID-1234` on a declaration suppresses that code at
  that declaration. `diag` is a kernel-owned directive, so the
  carrier and the grammar are ordinary directive machinery
  ([05-directives.md](05-directives.md)). The diagnostic sink reads
  it when filtering rather than stamping it as a fact.
- A suppression covers one code at one position. There is no
  file-level or workspace-level suppression of Errors. A check that
  is wrong across the whole workspace gets disabled by not running
  the plugin, and a kernel Error is never suppressible, because it
  reports a broken run rather than an opinion.
- Suppressions are audited. The run summary counts them per code.
  Every run reports a suppression that removed nothing as an Info
  under `UnusedSuppression`, and `doctor` lists those findings. Dead
  suppressions are this system's lint debt.
- The sinks that decide an outcome filter, and the sinks that collect
  one execution's findings for the sealed state do not. A warm run's
  recorded findings pass through the same filter, so an edit of a
  suppression changes what the next run reports.
- `--strict` promotes the Warnings that remain after the filter, so a
  suppressed Warning never fails a strict run.

## Machine output

`--format=json` renders diagnostics as line-delimited JSON under a
versioned schema shared with every command kernel
([14-distribution-and-cli.md](14-distribution-and-cli.md)): one
`diag` event per diagnostic, then a final `summary` event with counts
by severity, suppression counts per code included. The schema is
public API. Concretely:

```json
{"event":"diag","code":"EID-0029","severity":"error",
 "pos":"svc/store.go:41:2","msg":"chan int has no TypeScript spelling",
 "origin":"typescript","related":["svc/api.go:12:1"]}
{"event":"summary","status":1,"errors":1,"warnings":0,"infos":0,
 "suppressed":{"EID-0007":2}}
```

## Explain integration

`explain` takes a diagnostic code plus a position and walks the
provenance behind it ([04-metadata.md](04-metadata.md)): which
facts, stamped by whom, derived from which reads. A consumer who
cannot trace a diagnostic to its inputs will suppress it instead of
fixing it. The walk is what makes fixing the cheaper option.
