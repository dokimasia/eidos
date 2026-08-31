---
milestone: 0003
title: Emit renders to deterministic Go and TypeScript
status: Planned
depends-on: 0001
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0003: Emit renders to deterministic Go and TypeScript

## Goal

A hand-built emit graph renders through the Go and TypeScript
backends to formatter-clean files, byte-identical across two runs,
with the generated-file header, the provenance trailer, and sinks
that write atomically and only on change. Two backends from the
start is deliberate: the kit's API is held by having two consumers
of the render procedure, not one.

## Done when

- [ ] `RunBackendSuite` passes for eidos-lang-go and
      eidos-lang-typescript over hand-built emit fixtures: every emit
      kind renders, two runs produce identical bytes, and a format
      failure reports a positioned Error while the pass continues with
      the remaining files.
- [ ] The backend kit exists with the surface in
      [11-languages.md](../architecture/11-languages.md):
      `FileTemplate`, `KindTemplates`, `Funcs`, `Imports`, `Finalise`,
      and one `CommentSyntax` value shared with the future frontend.
- [ ] Slot contents render through the kind machinery, ordered by
      capability topology then plugin name. Appending a method into a
      field slot fails at the append, naming the slot.
- [ ] A body renders from each of the four content forms in
      [07-rendering.md](../architecture/07-rendering.md): empty,
      scaffolding statements, a `TemplateRef` resolved in the emitting
      plugin's tree, and `Verbatim`.
- [ ] A body-claiming template that drops the `{{slots}}` marker fails
      template lint, and a pending contribution into such a body is an
      Error naming the emitting plugin and counting what went
      unplaced. This completes the template-lint half of plugintest
      that milestone 0002 left open.
- [ ] Spelling a type feeds the file's one `ImportSet`, and the
      rendered import block is grouped and sorted the way the target's
      own formatter leaves it: gofmt for Go, the satellite's canonical
      printer for TypeScript.
- [ ] The sink stages, then commits: identical bytes leave the file
      and its mtime untouched, renames are atomic, a path escaping the
      root is refused, and `Discard` leaves no trace. Disk, memory and
      fan-out sinks ship.
- [ ] Every rendered file opens with the generated-code header and
      ends with the `<brand>:provenance sha256:<hash>` trailer, and a
      test recomputes the hash from the body bytes.

## Why now

This starts after 0001, which declares the emit kinds and their slots.
It does not need 0002: both milestones depend only on 0001, and their
order in the plan is arbitrary. Milestone 0005 joins this rendering
path to 0002's dispatch and 0004's graph.

## Scope

[07-rendering.md](../architecture/07-rendering.md), the sink, header,
trailer and determinism rules of
[17-output-and-determinism.md](../architecture/17-output-and-determinism.md),
the backend-kit half of
[11-languages.md](../architecture/11-languages.md), and the Go spoke of
the `Lowering` seam from
[10-cross-language.md](../architecture/10-cross-language.md), because
rendering spells types through it.

## Not in this milestone

- The manifest, drift and adoption: they need runs. Milestone 0005.
- The policy machinery for contested mappings: the first contested
  mapping arrives with the Go-to-TypeScript lowering. Milestone 0009.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| text/template reports errors at execute time and checks nothing statically | 0005 | The template-lint check is the designed answer, and it arrives in this milestone rather than later |
| One kit API serving two languages grows a per-language escape hatch | 0009 | Two backends arrive together, so a rule that fits only one language is found here, where changing the kit is free, and not at the cross-language milestone |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | The marker-rule bullet names the emitter and a count, not both plugins | A slot statement carries no attribution, so the contributor is unknowable by construction; the emitter whose template dropped the marker is the party that can fix it |
| 2026-08-30 | Added the TypeScript backend beside Go | Two consumers of the render procedure are what hold the kit's API; the Go-to-TypeScript lowering and its contested mappings stay at their own milestone, because a backend renders the neutral emit graph and needs no policy machinery |
| 2026-08-30 | Added at position 3 | Rendering is testable over hand-built emit graphs, so it runs in parallel with 0002 rather than after it |
