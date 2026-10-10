---
milestone: 0008
title: A consumer binary gets the full command surface
status: Done
depends-on: 0007
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0021
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Milestone 0008: A consumer binary gets the full command surface

## Goal

A binary that mounts the command kernels, through `cli.Main` or in a
command line of its own, offers run, plan, explain, prune, doctor,
watch and version, with the flags, the JSON output and the exit codes
of [20-cli.md](../architecture/20-cli.md), and its author writes no UX
code to get them.

## Done when

- [x] `acceptancetest` drives a fixture binary as a process: exit
      codes 0, 1 and 64 per the contract, a seeded panic exits 2
      untouched, config discovery walks up to `.<brand>.yaml` and
      stops at the VCS root, and the generated output compiles.
- [x] A `.<brand>.yaml` that declares a `workspaces:` list runs each
      member over its own root, with its own `.<brand>/`. A list
      whose roots nest exits 64 before any member runs.
- [x] `run` works: `--dry-run` reports create, update, unchanged,
      stale, drifted, foreign and withheld; `--check` exits 1 on a
      non-empty diff; `--overwrite-drift`, `--adopt`, `--cold` and
      `--plan` behave per contract;
      pattern narrowing applies the narrowed-sweep rule; and
      `mybrand run .` works from a `//go:generate` line.
- [x] `explain` returns all four target forms across plans from
      persisted provenance.
- [x] `prune` removes orphans and never touches a drifted or
      unmanifested file; its `--dry-run` lists what would go.
- [x] `doctor` validates config with the Go types from which the
      published JSON Schema is generated, lists dead suppressions, and
      flags deprecated usage.
- [x] `+<brand>:diag off=<code>` suppresses that code at that declaration
      and nothing else: the run summary counts suppressions per code,
      and a kernel Error is not suppressible.
- [x] `watch` re-runs when the fingerprint gate's poll reports a
      change, and `version` prints the main module's version, the
      versions of the kernel and of the command kernels, every plugin,
      the composition fingerprint and the executable's digest.
- [x] Every command's `--format=json` emits line-delimited events plus
      a summary under a versioned schema, and a second concurrent
      `run` fails naming the lock holder.
- [x] A consumer command registers beside the kernels in `cli.Main`,
      which refuses one that shadows a kernel name like `run`, and
      `acceptancetest` passes over a binary whose own dispatcher mounts
      the kernels under a group.

## Why now

This starts after 0007: `--cold`, `--check`, `watch` and honest run
stats need the engine, and `explain` walks facts the sealed state
persists. Milestone 0013 waits on this, because acceptancetest and the
reference binary drive this surface. Consumers cannot exist without
it.

## Scope

[20-cli.md](../architecture/20-cli.md), the command-kernel surface of
[14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md),
and the machine-output schemas of
[16-diagnostics.md](../architecture/16-diagnostics.md).

## Not in this milestone

- `doctor`'s schema-evolution diff: it needs the per-release schema
  index, which milestone 0014 introduces.
- A binary shipped by eidos: refused permanently (D2).

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The JSON schemas become API at 0014, so late shape changes are only cheap until then | 0014 | Treat the schemas as frozen from this milestone's end, and let 0014's index formalise it |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-08 | Ticked every bullet and set the milestone to Done | `acceptancetest` passes over two Go binaries of the conformance module: one calls `cli.Main`, and the dispatcher of the other mounts the kernels under the group `gen`. Its checks cover the statuses 0, 1, 64 and 2, discovery up to a version control marker, the JSON stream of every command except `watch`, a cold second run that rewrites no file, a list of two workspaces, a list whose roots nest, the lock and the compiled output. The tests of the commands cover each action of a dry run, `--check`, `--overwrite-drift`, `--adopt`, `--cold`, `--plan`, narrowing and `run .` in a package directory, the four targets of `explain` across plans, a prune that leaves a drifted file and an unmanifested file, the findings of `doctor`, the suppression counts of the summary, a `watch` pass after an edit and `version`. The module `eidos-cli` has 100% statement coverage |
| 2026-10-08 | `doctor` validates config with the Go types from which the published JSON Schema is generated | The decoder of the config file and the generator of the schema read one set of Go types, and a golden test keeps the published file equal to the generated schema. A validator of the schema file would check the same rules a second time |
| 2026-10-08 | Linked RFC-0021. The goal and the last bullet mount the kernels through `cli.Main` or the binary's own command line, and `acceptancetest` passes over a binary that mounts them under a group | A consumer's binary has commands of its own and can have a command-line framework, so the kernels are values that any command line mounts |
| 2026-10-08 | `version` prints no contract version here, and milestone 0014 has it print one | The kernel declares no contract version before 0014 adds the handshake that checks it |
| 2026-10-08 | `--dry-run` also reports foreign and withheld paths, and `run` gains `--adopt` | A narrowed run withholds the changes outside its patterns, and a file without the brand's frame at a routed path is an outcome of its own |
| 2026-10-02 | The `version` bullet names the composition fingerprint and the executable's digest in place of the plugin-set fingerprint | The sealed state's header checks those two digests, and a unit's key no longer folds a fingerprint of the composition |
| 2026-10-02 | Added the bullet for a `workspaces:` list | Milestone 0006 proves that two compositions over sibling roots are independent, and leaves the list to the config reader, because the kernel defines no file format. The reader is the one place that can refuse two roots that nest for every list |
| 2026-09-30 | The suppression criterion's carrier changed from `+gen:diag` to `+<brand>:diag` | The carrier mark follows the composition's brand |
| 2026-08-30 | Pinned diagnostic suppression and its audit counts into Done when | A coverage audit against the architecture found them held by Scope reference only |
| 2026-08-30 | Added at position 8 | The commands wrap the engine, so they follow it. Everything a consumer script touches exists after this |
