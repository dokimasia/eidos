---
milestone: 0007
title: A warm run redoes only what changed
status: Planned
depends-on: 0006
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0020
---

# Milestone 0007: A warm run redoes only what changed

## Goal

A second run over an unchanged tree executes no rule and produces a
byte-identical manifest. After one edit, only the artifacts whose
recorded inputs changed derive again. The fingerprint gate, the sealed
state, the parse memo and early cutoff exist and are licensed by the
warm≡cold check.

## Done when

- [ ] `RunWarmColdSuite` passes: a warm run over an unchanged tree
      re-executes nothing (the run stats report no rule ran) with a
      byte-identical manifest, and after one declaration edit the warm
      output equals a fresh cold run byte for byte.
- [ ] The fingerprint gate is stat-first: over an unchanged tree a
      probe counts zero content hashes, and a touched-but-identical
      file hashes once and loads nothing.
- [ ] The sealed state matches
      [09-incrementality.md](../architecture/09-incrementality.md):
      generations named by their content with an atomic `CURRENT`
      swap, a region-lazy graph (a probe counts decoded regions against
      touched ones), persisted facts that early cutoff diffs against,
      and the artifact table returning dirtiness by lookup.
- [ ] The state follows the executable: a rebuild that changes no code
      keeps it, and one that changes code runs the next run cold with
      one `ColdState` Info.
- [ ] Commit is incremental: a probe compares bytes written after one
      edit against the state's size, a no-change run writes no
      generation, and clean regions and bags carry by segment
      reference.
- [ ] A truncated or version-skewed generation falls back to cold with
      one Info, and the run completes correctly.
- [ ] Early cutoff works: an annotator that re-runs and stamps
      identical values leaves every downstream artifact green, which a
      fixture checks by counting executions.
- [ ] The (symbol, key) grain holds: an edit relevant to one plan's
      read keys re-runs that plan's dirty artifacts and no other plan,
      on the two-plan fixture from milestone 0006.
- [ ] The edge format has four kinds of edge: a per-identity edge, a
      fact edge at a subject and a key, a kind or directive membership
      edge under the reader's scope, and a package-member edge for a
      package that `PackageOf` returns. A fixture counts executions for
      the two new edges: a declaration of an enumerated kind that enters
      outside the reader's scope re-runs nothing, and an edit to one
      member of a package read through `PackageOf` re-runs the reader.
- [ ] The parse memo restores a branch-switch fixture without
      reparsing, its size cap evicts least-recently-used entries at the
      commit, the cold mode ignores both layers without deleting them,
      and two workspaces that open one memo ledger restore each other's
      units.
- [ ] The warm legs of `RunWorkspaceSuite` left open in milestone 0006
      now pass: an unchanged export re-runs no dependent, and a check
      sees identical records cold and warm.
- [ ] Warm and cold runs report identical findings, and a plan that
      failed in one run executes its pending work in the next.
- [ ] The manifest is 256 documents, and a warm run after one edit
      rewrites only the documents whose entries changed.

## Why now

This starts after 0006 because warm has to hold over the full frame,
including the merged Close and export edges. The machinery here is why
the architecture looks the way it does (D9, D10, D17), and it is the
plan's highest technical risk, so it comes as early as the frame
allows.

## Scope

All of [09-incrementality.md](../architecture/09-incrementality.md).

## Not in this milestone

- Performance numbers and gates: milestone 0013 measures. This
  milestone proves correctness only.
- A daemon or a remote cache: refused by D9 and D106, not deferred.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| A warm≡cold failure exposes a read-tracking gap in 0002, 0004 or 0005 code | 0008, 0013 | The check diffs manifests, so a failure names its artifact. Tracked Readers have been the only read path since 0002, which bounds where a gap can hide |
| The most novel machinery in the plan, so the estimate is the least reliable | 0008, 0013, 0014 | Milestones 0009 to 0012 do not depend on this one and proceed in parallel if it slips |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-02 | Aligned Done when with the accepted RFC-0020: generations named by their content, the executable check, a shared memo, identical findings with pending work for failed plans, and the manifest's 256 documents | The decisions RFC-0020 recorded change what a warm run has to prove |
| 2026-10-02 | Linked RFC-0020 | The milestone's design: the fingerprint gate, the sealed state, re-execution by artifact, the parse memo and the warm checks |
| 2026-09-30 | Added the edge format to Done when, with scoped membership edges and package-member edges | `ByKind` records its kind-membership edge without the reader's scope, `PackageOf` returns a whole package on one identity edge, and `Claim.Derived` lists point reads alone, so no document stated the edges invalidation follows |
| 2026-08-31 | Pinned incremental commit and the no-change skip into Done when | The incrementality design gained the write-side contract: a full-generation rewrite scales with the corpus and would fail the warm-one-edit gate on the commit alone |
| 2026-08-30 | Added at position 7 | The riskiest milestone, placed as early as its dependency on the whole frame allows |
