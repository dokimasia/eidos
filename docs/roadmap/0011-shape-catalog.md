---
milestone: 0011
title: The shape catalog classifies callables
status: Done
depends-on: 0005
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0022
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Milestone 0011: The shape catalog classifies callables

## Goal

A catalog author adds a spec under `spec/` and the registries
regenerate through a real eidos workspace. Detectors read only
`Callable` and stamp `shape.*` facts on any Tier-1 language, and a
wrong inference costs the source author one directive line, not a
fork.

## Done when

- [x] The spec JSON Schema is published, and a spec missing its
      falsifiability section fails CI before review sees it.
- [x] The tools module runs an eidos workspace over `spec/` through a
      YAML frontend and generates the name constants typed per form,
      the param constants, the directive schemas and the detector
      precedence order. A mirror guard reruns the workspace and diffs
      the tree.
- [x] A shape spec without a `Detect` implementation is a compile
      error through the wiring map, and an implementation without a
      spec fails the orphan test.
- [x] The worked trio from
      [12-shape-catalog.md](../architecture/12-shape-catalog.md)
      (writer, tx, atomic) validates, generates and stamps over Go
      fixtures. `Delete(v) error` goes to the deleter shape per
      declared precedence, and `explain` shows the losing claim.
- [x] `+<brand>:shape <name>` overrides a detector at directive authority,
      and `meta drop=shape.writer` removes the whole fact family.
- [x] The form grid is enforced: a mixin spec carrying `precedence:`
      fails the schema, and a contract role-arity violation is a
      positioned Error.
- [x] The Tier-3 import ban lint is green in this module's CI, and the
      catalog module's `go.mod` requires only the SDK facade, which
      requires the kernel. The tools module, with its own `go.mod`, is
      the only place eidos-lang-go appears.

## Why now

This starts after 0005: regenerating the registries is itself an eidos
run that produces Go, and detectors read the `Callable` projection
that 0004 implemented. Nothing in the frame waits for the catalog, so
its position past 0005 is capacity, not dependency. dokimi, the
reference consumer, gates on this milestone more than on any other,
because its checks consume these stamps.

## Scope

All of [12-shape-catalog.md](../architecture/12-shape-catalog.md), and
the whole vocabulary of the catalog: 23 shapes, 58 mixins and 26
contracts, each with its spec.

## Not in this milestone

- Generating checks from stamps: that is the consumer's business
  (dokimi), outside this repository.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The spec template under-determines real checks, and the first consumer corpus finds it | dokimi's adoption, and 0014's verbatim spec rendering | Extend the template before 0014 freezes it. It is data under a published schema, so extension is additive |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-09 | Ticked every bullet and set the milestone to Done | The test of the tools module runs a workspace with the spec frontend over the 107 specs, and fails where `registry.gen.go`, `catalog/registry_wiring.gen.go` or `spec.schema.json` differs from the generated file. The decoder reports a spec without its falsifiability and a mixin with `precedence` as `SpecInvalid` at the line. python-jsonschema 4.26.0 accepts the 107 specs under the published schema and refuses both of those documents. `Detections` refers to the detector of each detected spec, and the test of `catalog` fails for a detector that it does not list. The conformance tests over Go stamp the shape that ranks first on a method of each detected shape, list `writer` after `deleter` on `Delete`, and show both in `explain`. They keep the detectors off a method with `+acme:shape writer`, remove the facts of a dropped writer, and stamp writer, atomic and tx on one method. They report `RoleArity` at the callable, on a cold run and on a warm run after an edit. The depguard rules `plugin-modules` and `shape-catalog` pass, the catalog module requires the SDK facade and the assert library, and the tools module is the only module of the catalog that requires `eidos-lang-go`. The layer `eidos-plugin-shape/...` has 100% statement coverage, 1,601 of 1,601 statements, and a run over the Go source of the kernel module classifies 7,811 callables without an Error |
| 2026-10-09 | The catalog module's path is `go.dokimi.dev/eidos/plugin/shape` | The Go backend takes the package name of a generated file from the last element of its import path when the run loads no Go file of the directory, and the path `go.dokimi.dev/eidos/plugin-shape` gives the name `plugin` |
| 2026-10-09 | The last bullet: the catalog module requires the SDK facade, which requires the kernel | A plugin module imports the kernel's plugin-facing packages through the SDK facade |
| 2026-10-09 | The milestone builds the whole vocabulary of 107 specs in place of the worked trio, and the full vocabulary leaves "Not in this milestone" | A consumer can move to the catalog without losing a classification that it reads, and every spec has its claim, falsifiability and counterexamples from the start |
| 2026-10-09 | Linked RFC-0022 | The RFC describes the specs, the generator, the plugins and the kernel mechanisms that the catalog needs |
| 2026-09-30 | The override criterion's carrier changed from `+gen:shape` to `+<brand>:shape` | The carrier mark follows the composition's brand |
| 2026-08-30 | Added at position 11 | First self-hosted generation: legal here because the tools module may depend on eidos-lang-go where the kernel may not |
