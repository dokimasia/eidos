---
milestone: 0010
title: protobuf schemas drive generation
status: Done
depends-on: 0009
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0024
---

# Milestone 0010: protobuf schemas drive generation

## Goal

A proto file's messages and services project into the graph, and one
workspace turns them into Go and TypeScript output. This is the
schema-in work, and the satellite that carries it is read-only by
design.

## Done when

- [x] eidos-lang-protobuf ships `frontend/` and `rules/` and nothing
      else. The read-only shape is visible in the module tree, per
      [11-languages.md](../architecture/11-languages.md).
- [x] A service fixture generates Go server scaffolding and TypeScript
      client types in one run.
- [x] `oneof` projects as Sum, messages as Struct, and proto enums
      arrive through `EnumRules`. The scalar policy table resolves,
      and Timestamp and Duration map to the well-known canonical
      identities and lower to `time.Time` and `Date`. Everything but
      the lowering is met: the lowering is a target's own and arrives
      with the demo above.
- [x] frontendtest passes for the satellite, `testdata/features/`
      covers the proto rows, and the support matrix generates. There
      is no lowering or backend to test, and the anatomy says so
      structurally. Both kernel suites pass, and the conformance
      corpus has a graded entry per inventory row. The published
      support matrix remains.

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
| protocompile's model diverges from the projection vocabulary in some corner of proto3 | 0014, through this milestone only | Record the remainder as `protobuf.*` metadata at level 2 of the scale, and declare it in the feature matrix |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Added at position 10 | Cheapest satellite, placed where its demo (proto in, Go and TypeScript out) has both targets to arrive on |
| 2026-09-03 | The frontend and the rules landed ahead of the sequence, during milestone 0004's wave | The projection vocabulary needed a second read-only language to hold its shape honest, and protobuf needs nothing from 0009 to load. The demo bullet still waits on the TypeScript spoke |
| 2026-09-03 | Nested resolution, trailing comments, the whole well-known table, edition features and the proto2 residue followed, with a graded corpus entry | The first pass resolved no nested reference, dropped every trailing comment and mapped two well-known messages of twenty, which is a satellite that loads the easy shapes and loses the rest |
| 2026-09-24 | Resolution moved to shadowing tiers with every namespace split, comments to protoc's attribution, edition presence into the field's form, enum values to typed conversions, and literals to protoc's grammar | An audit found a sub-package unreachable from its parent namespace, legal shadowing reported as ambiguous, licence headers read as package documentation, and Go's literal grammar applied to protobuf text |
| 2026-09-30 | Status set to In progress | The read-only-shape bullet is checked, and the status read Planned |
| 2026-09-30 | The residue namespace is spelled `protobuf.*` | A satellite claims the namespace its language identity spells, and the identity is `protobuf` |
| 2026-10-10 | Linked RFC-0024, ticked every bullet and set the milestone to Done | The frontend parses through protocompile's experimental parser and loads proto2, proto3 and the editions 2023, 2024 and 2026, and it reports `UnknownEdition` for any other version. The conformance service fixture writes `svc/store_server.go` and `svc/store.ts` from one proto service in one run, and the schema of each of the five versions writes the same two files. The fixture passes the workspace suite and the warm suite in each version, tsc type-checks the client, and the Go toolchain type-checks the server. A oneof lowers to a Go interface with a marker method and to a TypeScript union of variant interfaces, a message to a Go struct and a TypeScript interface, and an enum through `EnumRules`. Timestamp and Duration lower to `*time.Time` and `*time.Duration` in Go, and to `Date` and the choice of `typescript.duration` in TypeScript. A NullValue field reports `RefusedType` in each plan. `frontendtest.RunFrontendSuite` passes over the protobuf frontend, the corpus grades every inventory row, and the matrix test writes the support matrix with the probes of Dynamic, the well-known `empty`, the asynchronous stream and `typescript.duration`. The gate passes, and the race detector passes over every touched module |
