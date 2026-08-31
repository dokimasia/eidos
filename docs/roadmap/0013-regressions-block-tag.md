---
milestone: 0013
title: Regressions block the tag
status: Planned
depends-on: 0007, 0008, 0009
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0013: Regressions block the tag

## Goal

The release pipeline refuses a tag when the canary ring breaks or a
benchmark budget regresses. The corpus generator, the scenario matrix
and the first dedicated-runner baselines exist, and eidos-reference is
both the canary and the benchmark rig.

## Done when

- [ ] `conformance/bench` generates seeded corpora at the S, M and L
      sizes of [19-benchmarking.md](../architecture/19-benchmarking.md),
      and the same seed produces the same bytes.
- [ ] The seven macro scenarios and the micro suite run with per-phase
      attribution, peak RSS and allocation counts, and every micro
      benchmark pins allocations per operation.
- [ ] `budgets.yaml` is committed. Scaling-rule and allocation gates
      run in ordinary CI; absolute-time gates run on the dedicated
      runner profile and compare distributions against the recorded
      baseline.
- [ ] The first baseline is recorded from the dedicated runner and
      committed with the stated headroom factor, and a budget that
      predates its baseline is marked advisory and reported as such.
- [ ] Warm-no-change at L meets the sub-second bound on the dedicated
      profile. The D17 target is measured here for the first time,
      not asserted.
- [ ] eidos-reference composes an API-typical multi-plan,
      cross-language workspace. acceptancetest drives its binary, and
      warm≡cold and workspacetest run over its Go-plus-TypeScript
      composition.
- [ ] The ring job runs eidos-reference, eidos-plugin-shape and the
      language satellites against kernel HEAD, and a seeded breaking
      change on a kernel branch blocks the tag in a demonstration run.

## Why now

This starts after 0007 (the layers under measurement), 0008
(acceptancetest drives a binary) and 0009 (the cross-language and
multi-plan scenarios need two languages). It must precede 0014: a
first release without executed gates would ship the performance and
compatibility claims unproven, against the ground rule that claims are
executed.

## Scope

[19-benchmarking.md](../architecture/19-benchmarking.md), the
reference ensemble named in
[01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md), and
the canary ring of
[15-compatibility.md](../architecture/15-compatibility.md).

## Not in this milestone

- The API-surface baseline and the schema index: milestone 0014 owns
  the compatibility artifacts. This milestone builds the machinery
  that blocks a tag; 0014 adds what the tag publishes.
- The catalog and declarative plugins joining the ring: they join as
  they arrive (0011, 0012); the ring runs whatever exists.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The L-size target misses on the dedicated profile | 0014, with rework arriving back in 0007's layers | That is what measuring before tagging is for. The scenario matrix attributes the miss to a phase, and the profile flags ship with the harness |
| No dedicated runner exists yet, and someone has to provision one | 0013, 0014 | Provision it while 0008 runs. Scaling and allocation gates run on ordinary CI meanwhile, so only the absolute-time gates wait |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Added at position 13 | The gates must exist before the first tag, and they need the engine, a binary and two languages to measure |
