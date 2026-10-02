---
milestone: 0006
title: Several plans run in one workspace
status: Planned
depends-on: 0005
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0019
---

# Milestone 0006: Several plans run in one workspace

## Goal

One run drives several plans in parallel over one frozen graph: scoped
sources, a merged manifest, cross-plan collision Errors, per-plan
sweep, typed exports in topological order, and checks that read the
records at Close.

## Done when

- [ ] `RunWorkspaceSuite` passes its cold legs: a cross-plan path
      collision is an Error at Close naming both plans; a plan seeded
      to fail leaves its siblings' staged output and manifest slices
      intact; a dependent plan reads its producer's export in
      topological order; removing a plan from the composition deletes
      exactly that plan's files, except drifted ones; audit mode
      reports an unmet contract at its declared severity.
- [ ] Two Go plans with different generator sets run against scoped
      sources, and a fixture asserts that a generator in a scoped plan
      cannot observe an out-of-scope package through its Reader.
- [ ] A binding-generator fixture reads names and import paths from
      the producer's `ExportDoc` instead of recomputing them, and a
      cycle between two plans' declared dependencies is a Build error
      naming both.
- [ ] A `WorkspaceCheck` fixture ("every interface got a stub") reads
      only the records and reports a positioned diagnostic. A check
      that needs a failed plan's records reports one Info and stands
      down.
- [ ] Two compositions over sibling roots of one repository each
      record their manifest in their own `.<brand>/`. Neither reads
      the other's sources, and removing a plan from one removes no
      file of the other.

## Why now

This starts after 0005. The one-backend-per-plan frame exists for
multi-target output, and milestone 0009 needs two plans in one run.
Milestone 0007 waits for this, because warm behaviour has to be proven
over the whole frame, including the merged Close, not over a
single-plan subset.

## Scope

The rest of
[08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md):
plans, source scopes, exports, Close, and multi-workspace
repositories.

## Not in this milestone

- The warm legs of workspacetest (an unchanged export re-running no
  dependent; checks seeing identical records warm): milestone 0007
  owns warm behaviour.
- A second target language: both plans here target Go. Milestone 0009.
- The reader of a `workspaces:` list, and its refusal of two roots
  that nest: milestone 0008, beside config discovery.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| Plan parallelism exposes a shared-state leak that read isolation was supposed to make impossible | 0007, 0009 | `-race` on every check. A leak found here is a design fault to fix now, while it is cheaper than under 0007's caching |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-02 | Linked RFC-0019. The last bullet asks for two compositions over sibling roots instead of a `workspaces:` list | The kernel defines no file format, so the list needs the config reader that milestone 0008 builds with config discovery. That reader is also the one place that can refuse two roots that nest for every list. Two compositions over sibling roots are independent without new kernel code, because each has its own tree, sink and state directory |
| 2026-08-30 | Added at position 6 | The frame must be whole before 0007 proves warm over it |
