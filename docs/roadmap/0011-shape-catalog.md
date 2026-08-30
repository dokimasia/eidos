---
milestone: 0011
title: The shape catalog classifies callables
status: Planned
depends-on: 0005
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0011: The shape catalog classifies callables

## Goal

A catalog author adds a spec under `spec/` and the registries
regenerate through a real eidos workspace. Detectors read only
`Callable` and stamp `shape.*` facts on any Tier-1 language, and a
wrong inference costs the source author one directive line, not a
fork.

## Done when

- [ ] The spec JSON Schema is published, and a spec missing its
      falsifiability section fails CI before review sees it.
- [ ] The tools module runs an eidos workspace over `spec/` through a
      YAML frontend and generates the name constants typed per form,
      the param constants, the directive schemas and the detector
      precedence order. A mirror guard reruns the workspace and diffs
      the tree.
- [ ] A shape spec without a `Detect` implementation is a compile
      error through the wiring map, and an implementation without a
      spec fails the orphan test.
- [ ] The worked trio from
      [12-shape-catalog.md](../architecture/12-shape-catalog.md)
      (writer, tx, atomic) validates, generates and stamps over Go
      fixtures. `Delete(v) error` goes to the deleter shape per
      declared precedence, and `explain` shows the losing claim.
- [ ] `+gen:shape <name>` overrides a detector at directive authority,
      and `meta drop=shape.writer` removes the whole fact family.
- [ ] The form grid is enforced: a mixin spec carrying `precedence:`
      fails the schema, and a contract role-arity violation is a
      positioned Error.
- [ ] The Tier-3 import ban lint is green in this module's CI, and the
      catalog module's `go.mod` requires only the kernel. The tools
      module, with its own `go.mod`, is the only place eidos-lang-go
      appears.

## Why now

This starts after 0005: regenerating the registries is itself an eidos
run that produces Go, and detectors read the `Callable` projection
that 0004 implemented. Nothing in the frame waits for the catalog, so
its position past 0005 is capacity, not dependency. dokimi, the
reference consumer, gates on this milestone more than on any other,
because its checks consume these stamps.

## Scope

All of [12-shape-catalog.md](../architecture/12-shape-catalog.md).

## Not in this milestone

- The full catalog vocabulary: the catalog grows spec by spec after
  this. This milestone builds the mechanism and the worked trio.
- Generating checks from stamps: that is the consumer's business
  (dokimi), outside this repository.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The spec template under-determines real checks, and the first consumer corpus finds it | dokimi's adoption, and 0014's verbatim spec rendering | Extend the template before 0014 freezes it. It is data under a published schema, so extension is additive |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Added at position 11 | First self-hosted generation: legal here because the tools module may depend on eidos-lang-go where the kernel may not |
