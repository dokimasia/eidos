---
milestone: 0001
title: One schema generates both models
status: In progress
depends-on: none
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0001, RFC-0002
---

# Milestone 0001: One schema generates both models

## Goal

A contributor changes the declaration model by editing
`eidos-core/symbol/schema` and regenerating. The `node` and `emit`
models come out of the generator, and a hand edit to either fails CI.

## Done when

- [ ] `eidos-core/symbol` holds the `Kind` enum, the walk interfaces
      and `schema/` with every kind in the inventory of
      [02-symbol-model.md](../architecture/02-symbol-model.md).
- [ ] `internal/gen` reads the schema and writes both models: kind
      structs, `Walk`, `RewireOwners`, JSON encoding, slot
      declarations with typed accessors, and the mirror guards. The
      output is committed.
- [ ] Editing a generated file by hand makes `make check` fail: the
      mirror guard reruns the generator and diffs the tree.
- [ ] A test asserts that `internal/gen` imports only the standard
      library.
- [ ] `symbol.Identity` carries package path, kind, name and the
      signature discriminator, and two overloads get distinct
      identities.
- [ ] An unknown schema annotation makes the generator fail with an
      error naming the annotation.

## Why now

Nothing had to exist first. Everything else keys on these models: the
kind-indexed triggers and their Match types generate from this schema,
the store holds node values, emitters build emit values, and the
conformance rungs walk both. Milestones 0002 and 0003 cannot start
until this is done.

## Scope

[02-symbol-model.md](../architecture/02-symbol-model.md): the `symbol`
vocabulary, the schema annotation set, both generated models, and the
`internal/gen` tool. Decisions D4 and D40, and the slot-declaration
half of D39.

The detail lives in
[RFC-0001](../rfc/0001-symbol-model-contract.md) (the symbol contract
and schema) and [RFC-0002](../rfc/0002-model-generator.md) (the
generator and mirror guard). ADRs
[0002](../adr/0002-identity-carries-source-language.md) to
[0005](../adr/0005-concern-grouped-generated-files.md) record the
settled decisions.

## Not in this milestone

- The hand-written node runtime (resolver, queries, freeze): goes to
  milestone 0002.
- The hand-written emit runtime (slot append semantics, the builder,
  rendering): goes to milestone 0003.
- Link, and identity surviving a reparse: goes to milestone 0004,
  which has real parses to prove it on.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The schema annotation vocabulary proves too small once rendering (0003) or real languages (0004, 0009) arrive | 0003, 0004 | Extend the schema and regenerate. The vocabulary grows by kernel decision, and a regeneration is the designed cost |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Status Planned to In progress | RFC-0001 and RFC-0002 accepted; implementation starts |
| 2026-08-30 | Linked RFC-0001, RFC-0002 and ADR-0002 to 0005 | The design is written; the milestone points at the documents that hold it, and the documents never point back |
| 2026-08-30 | Added at position 1 | First milestone of the initial plan: every other milestone consumes the generated models |
