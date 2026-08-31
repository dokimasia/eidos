---
milestone: 0010
title: protobuf schemas drive generation
status: Planned
depends-on: 0009
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0010: protobuf schemas drive generation

## Goal

A proto file's messages and services project into the graph, and one
workspace turns them into Go and TypeScript output. This is the
schema-in work, and the satellite that carries it is read-only by
design.

## Done when

- [ ] eidos-lang-protobuf ships `frontend/` and `rules/` and nothing
      else. The read-only shape is visible in the module tree, per
      [11-languages.md](../architecture/11-languages.md).
- [ ] A service fixture generates Go server scaffolding and TypeScript
      client types in one run.
- [ ] `oneof` projects as Sum, messages as Struct, and proto enums
      arrive through `EnumRules`. The scalar policy table resolves,
      and Timestamp and Duration map to the well-known canonical
      identities and lower to `time.Time` and `Date`.
- [ ] frontendtest passes for the satellite, `testdata/features/`
      covers the proto rows, and the support matrix generates. There
      is no lowering or backend to test, and the anatomy says so
      structurally.

## Why now

This starts after 0009. The satellite itself needs only the frontend
kit from 0004, but the capability worth checking is a schema in and
two languages out, and that needs the TypeScript spoke. protobuf is
read-only permanently (D7), which makes this the cheapest satellite
and the proof that the read-only anatomy is a complete shape.

## Scope

The protobuf satellite per
[11-languages.md](../architecture/11-languages.md), parsing through
protocompile (D16), and the well-known-type mappings of
[03-projection.md](../architecture/03-projection.md).

## Not in this milestone

- Writing proto: refused permanently (D7), not deferred.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| protocompile's model diverges from the projection vocabulary in some corner of proto3 | 0014, through this milestone only | Carry the remainder as `proto.*` metadata at level 2 of the scale, and declare it in the feature matrix |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Added at position 10 | Cheapest satellite, placed where its demo (proto in, Go and TypeScript out) has both targets to arrive on |
