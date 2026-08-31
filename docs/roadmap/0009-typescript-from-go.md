---
milestone: 0009
title: TypeScript comes out of a Go workspace
status: Planned
depends-on: 0006
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0009: TypeScript comes out of a Go workspace

## Goal

A workspace with a Go plan and a TypeScript plan generates both from
one graph in one run. Canonical shapes flow through the hub, policy
choices resolve at Build, per-target names stamp at Annotate, and a
shape the target cannot spell is refused with a positioned code.

## Done when

- [ ] eidos-lang holds the tree-sitter bindings and the pinned
      TypeScript grammar and is their only importer.
      eidos-lang-typescript ships the full satellite anatomy.
- [ ] The end-to-end path completes: the fixture from
      [00-one-declaration-end-to-end.md](../architecture/00-one-declaration-end-to-end.md)
      produces `store_stub_test.go` and `store.ts` in one run.
- [ ] Policy works per
      [10-cross-language.md](../architecture/10-cross-language.md):
      `ts.int64` registers with `bigint`, `string` and `number`,
      config selects one, a directive overrides one declaration, and
      the `Policy` a lowering receives is total.
- [ ] Naming annotators register automatically for every targeted
      language, and `explain` traces a `ts.name` stamp to its origin.
- [ ] Refusal is fully supported: `chan int` into the TypeScript plan and
      a TypeScript union into a Go plan each report a stable
      positioned code, and neither guesses.
- [ ] TypeScript decorators lower as native sugar to canonical
      directives.
- [ ] frontendtest, backendtest and pipelinetest pass for the
      satellite, `testdata/features/` covers the TypeScript rows of
      the landscape table, and the support matrix generates as a CI
      artifact.

## Why now

This starts after 0006, because Go plus TypeScript is two plans in one
run. It does not depend on 0007 or 0008 and can proceed in parallel
with them; it sits after them in the order because the same people
build the kernel first, not because anything blocks it.

## Scope

[10-cross-language.md](../architecture/10-cross-language.md) end to
end, the TypeScript satellite per
[11-languages.md](../architecture/11-languages.md), and the eidos-lang
module per [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md).

## Not in this milestone

- Other tree-sitter satellites: unscheduled, see the
  [index](README.md).
- Wazero-based bindings: adopt when they mature. The binding choice is
  private to eidos-lang, so migrating later touches one module.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The tree-sitter Go bindings use cgo, which taxes every consumer build that embeds this satellite | 0013 (the reference binary builds it), consumers | Recorded in [11-languages.md](../architecture/11-languages.md). The binding choice is private to eidos-lang, so a wazero migration later changes one module |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-31 | Moved the TypeScript-formatter risk to milestone 0003 | It belongs with the backend, which moved there on 2026-08-30; the row had stayed behind |
| 2026-08-30 | The TypeScript backend moved to milestone 0003 | Two consumers hold the render kit's API, so the backend arrives beside Go's; this milestone keeps the frontend anatomy, the hub, the policies and the lowering |
| 2026-08-30 | Added at position 9 | The second language proves the hub. TypeScript before protobuf because the schema-in work (0010) wants a second target to arrive on |
