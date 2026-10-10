---
milestone: 0009
title: TypeScript comes out of a Go workspace
status: Done
depends-on: 0006
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0023
---

# Milestone 0009: TypeScript comes out of a Go workspace

## Goal

A workspace with a Go plan and a TypeScript plan generates both from
one graph in one run. Canonical shapes flow through the hub, policy
choices resolve at Build, per-target names stamp at Annotate, and a
shape the target cannot spell is refused with a positioned code.

## Done when

- [x] eidos-lang-typescript contains the full satellite anatomy.
- [x] The end-to-end path completes: the fixture from
      [00-one-declaration-end-to-end.md](../architecture/00-one-declaration-end-to-end.md)
      produces `store_stub_test.go` and `store.ts` in one run.
- [x] Policy works per
      [10-cross-language.md](../architecture/10-cross-language.md):
      `typescript.int64` registers with `bigint`, `string` and `number`,
      config selects one, a directive overrides one declaration, and
      the `Policy` a lowering receives is total.
- [x] Naming annotators register automatically for every targeted
      language, and `explain` traces a `typescript.name` stamp to its
      origin.
- [x] Refusal is fully supported: `chan int` into the TypeScript plan and
      a TypeScript union into a Go plan each report a stable
      positioned code, and neither guesses.
- [x] TypeScript decorators lower as native sugar to canonical
      directives.
- [x] pipelinetest passes for the satellite, and the support matrix
      generates as a CI artifact.

## Why now

This starts after 0006, because Go plus TypeScript is two plans in one
run. It does not depend on 0007 or 0008 and can proceed in parallel
with them; it sits after them in the order because the same people
build the kernel first, not because anything blocks it.

## Scope

[10-cross-language.md](../architecture/10-cross-language.md) end to
end, and the TypeScript satellite per
[11-languages.md](../architecture/11-languages.md).

## Not in this milestone

- The tree-sitter platform in eidos-lang, the TypeScript frontend and
  its frontendtest and corpus checks: milestone 0004.
- Other tree-sitter satellites: Rust and Java load in milestone 0004,
  and the rest are unscheduled, see the [index](README.md).

## Risks to the sequence

No risk that moves another milestone is recorded.

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-09 | Ticked every bullet and set the milestone to Done | The conformance fixture of a Go plan and a TypeScript plan writes `svc/store_stub_test.go` and `svc/store.ts` in one run, passes the workspace suite and the warm suite, and tsc type-checks the client. `svc/store.ts` declares `expires: bigint` without a selection, `expires: string` under `typescript.int64: string` in config, and `expires: number` under `//+acme:typescript int64=number` on the field. A choice outside the key's choices fails Build, and in a directive it reports `BadSpelling` at the carrier. The explanation of `typescript.name` on `Put` lists the directive's claim first and the claim of `typescript-lowering` with the value `put`. A Go field `Events chan int` reports `RefusedType` at the field in the TypeScript plan, and a TypeScript union reports `RefusedType` at the property in a Go plan. The TypeScript, Rust and Java corpora lower a marker of the brand to its directive. The TypeScript pipeline fixture passes `pipelinetest.RunPipelineSuite`, and tsc type-checks its stub. The matrix test writes the support matrix, and the CI job uploads it with `-artifacts`. The gate passes, and the race detector passes over every touched module |
| 2026-10-09 | Linked RFC-0023 and set the milestone to In progress | The RFC describes the hub, the policies, the target names, native sugar and the TypeScript satellite that the bullets need |
| 2026-09-30 | Moved the tree-sitter bindings bullet, the frontend and corpus checks, the wazero note and the cgo risk to milestone 0004 | Milestone 0004's widening of 2026-09-01 pulled the tree-sitter platform and the TypeScript frontend forward, and this file still scheduled them |
| 2026-09-30 | The TypeScript keys are spelled `typescript.*` | A satellite claims the namespace its language identity spells, and the identity is `typescript` |
| 2026-08-31 | Moved the TypeScript-formatter risk to milestone 0003 | It belongs with the backend, which moved there on 2026-08-30; the row had stayed behind |
| 2026-08-30 | The TypeScript backend moved to milestone 0003 | Two consumers hold the render kit's API, so the backend arrives beside Go's; this milestone keeps the frontend anatomy, the hub, the policies and the lowering |
| 2026-08-30 | Added at position 9 | The second language proves the hub. TypeScript before protobuf because the schema-in work (0010) wants a second target to arrive on |
