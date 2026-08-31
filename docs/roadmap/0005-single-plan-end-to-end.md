---
milestone: 0005
title: A single plan runs end to end
status: Planned
depends-on: 0002, 0003, 0004
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0005: A single plan runs end to end

## Goal

One `Workspace.Run` over real Go source executes Load, Link, Freeze,
Annotate, Generate, Layout, Render, Sink and Close for one plan, and
leaves committed files, a manifest and provenance behind. The stubgen
half of
[one declaration, end to end](../architecture/00-one-declaration-end-to-end.md)
runs as a fixture.

## Done when

- [ ] `RunPipelineSuite` passes: fixture plugins plus the Go backend
      over Go source produce the expected bytes, routed per layout,
      with a manifest slice and clean diagnostic discipline.
- [ ] The end-to-end fixture works: an interface carrying
      `//+gen:stub tag=test` produces `svc/store_stub_test.go` beside
      its source, as the end-to-end document describes, minus the
      TypeScript plan.
- [ ] Layout resolves per
      [18-routing-and-layout.md](../architecture/18-routing-and-layout.md):
      the three cardinalities, tags with the target spelling the join
      (`store_stub.go`, `suite_test.go`), `out=` and `tag=` overrides,
      refinement precedence. A within-plan collision and a collision
      with a hand-written file are Errors naming both sides.
- [ ] Centralised layout derives package identity from the
      `gen.module` facts, `importBase` covers an output directory
      outside every module, and a missing identity is refused at the
      referencing declaration, only when a cross-reference needs the
      qualification (D69).
- [ ] Drift and adoption work: editing a generated file makes the next
      run refuse with an Error naming the file, and a byte-equal
      unmanifested file is adopted silently.
- [ ] Close runs for one plan: the versioned manifest arrives in
      `.<brand>/`, the in-scope sweep removes an orphaned output, and
      audit mode reports a fixture completeness contract at its
      declared severity.
- [ ] The commit is two-phase: staging, then the plan commit, then
      `CommitRun` strictly last. A test crashes between the last two
      and the next run corrects by deriving again and writing nothing
      new.
- [ ] Cancelling the run's context stops it between units of work:
      writes stay atomic, nothing arrives mid-file or mid-manifest, and
      the report says what committed.
- [ ] Running twice produces byte-identical trees, and the second run
      touches no mtime.
- [ ] A `PerPackage` accumulator file assembles from many matches
      ordered by subject identity, and stays byte-identical under
      `-race` with in-bucket parallelism enabled.

## Why now

This joins 0002 (dispatch and facts), 0003 (rendering and sinks) and
0004 (a real graph). It is the first run of the product as specified,
and every later milestone extends this run rather than building beside
it.

## Scope

The Run frame and phase order of
[08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md)
for a single plan,
[18-routing-and-layout.md](../architecture/18-routing-and-layout.md),
the manifest, drift and adoption parts of
[17-output-and-determinism.md](../architecture/17-output-and-determinism.md),
and the accumulator Emitter of
[06b-authoring.md](../architecture/06b-authoring.md).

## Not in this milestone

- Several plans, exports and cross-plan checks: milestone 0006.
- Warm behaviour: every run here is cold. The ledger writes artifact
  rows that nothing reads yet. Milestone 0007.
- The CLI: tests call `Workspace.Run` directly. The commands are
  milestone 0008.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| First contact between 0002's authoring API and real plugin code forces API revisions | 0006 and everything after | Planned for: nothing tags before 0014, and the checks pin behaviour so revisions cannot drift silently |
| This is the spine: a slip here slips every open milestone | all | Cut sideways, not lengthwise: fixture plugins stay minimal, and layout corner cases move to 0006 before the phase order does |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Pinned centralised-layout package identity and cancellation semantics into Done when | A coverage audit against the architecture found them held by Scope reference only |
| 2026-08-30 | Added at position 5 | The first end-to-end run. Placed as early as its three inputs allow |
