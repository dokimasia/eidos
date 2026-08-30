---
milestone: 0008
title: A consumer binary gets the full command surface
status: Planned
depends-on: 0007
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0008: A consumer binary gets the full command surface

## Goal

A binary whose `main` is registration plus `cli.Main(ws)` offers run,
plan, explain, prune, doctor, watch and version, with the flags, the
JSON output and the exit codes of
[20-cli.md](../architecture/20-cli.md), and its author writes no UX
code to get them.

## Done when

- [ ] `acceptancetest` drives a fixture binary as a process: exit
      codes 0, 1 and 64 per the contract, a seeded panic exits 2
      untouched, config discovery walks up to `.<brand>.yaml` and
      stops at the VCS root, and the generated output compiles.
- [ ] `run` works: `--dry-run` reports create, update, unchanged,
      stale and drifted; `--check` exits 1 on a non-empty diff;
      `--overwrite-drift`, `--cold` and `--plan` behave per contract;
      pattern narrowing applies the narrowed-sweep rule; and
      `mybrand run .` works from a `//go:generate` line.
- [ ] `explain` answers all four target forms across plans from
      persisted provenance.
- [ ] `prune` removes orphans and never touches a drifted or
      unmanifested file; its `--dry-run` lists what would go.
- [ ] `doctor` validates config against the published JSON Schema,
      lists dead suppressions, and flags deprecated usage.
- [ ] `+gen:diag off=<code>` suppresses that code at that declaration
      and nothing else: the run summary counts suppressions per code,
      and a kernel Error is not suppressible.
- [ ] `watch` re-runs when the fingerprint gate's poll reports a
      change; `version` prints the kernel version, the contract
      version, every plugin, and the plugin-set fingerprint.
- [ ] Every command's `--format=json` emits line-delimited events plus
      a summary under a versioned schema, and a second concurrent
      `run` fails naming the lock holder.
- [ ] A consumer command registers beside the kernels, and shadowing a
      kernel name like `run` is refused.

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
| 2026-08-30 | Pinned diagnostic suppression and its audit counts into Done when | A coverage audit against the architecture found them held by Scope reference only |
| 2026-08-30 | Added at position 8 | The commands wrap the engine, so they follow it. Everything a consumer script touches exists after this |
