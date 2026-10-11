---
milestone: 0013
title: Regressions block the tag
status: Planned
depends-on: 0007, 0008, 0009
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0026
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Milestone 0013: Regressions block the tag

## Goal

The release pipeline refuses a tag when the canary ring breaks. The
canary is eidos-reference. It contains nineteen reference plugins,
and its binary composes them in an API-typical workspace with two or
more plans in Go and TypeScript.

## Done when

- [x] eidos-reference contains nineteen reference plugins on the
      authoring surface. Its generators write a repository, a mock, a
      recording stub, an HTTP handler, its middleware stack, a
      validator, a registry, a builder, the text form of an enum and
      the checks of a package's errors. Its contributors add the
      authentication, metrics and tracing entries to the middleware
      chain, and the recover and audit entries to a handler. Its
      weavers add a debug trace and an audit record to the methods
      that other plugins generate. Its annotators stamp each struct
      that implements io.Writer, and the declared defaults of fields.
- [x] eidos-reference composes an API-typical multi-plan,
      cross-language workspace. acceptancetest drives its binary, and
      warm≡cold and workspacetest run over its Go-plus-TypeScript
      composition.
- [ ] The ring job runs eidos-reference, eidos-plugin-shape and the
      language satellites against kernel HEAD, and a seeded breaking
      change on a kernel branch blocks the tag in a demonstration run.

## Why now

This starts after 0007 (warm≡cold over the reference workspace), 0008
(acceptancetest drives a binary) and 0009 (the reference workspace
needs two languages). It must precede 0014, because a first release
without the ring would publish compatibility claims that no run has
executed.

## Scope

eidos-reference and the canary ring of
[15-compatibility.md](../architecture/15-compatibility.md).

## Not in this milestone

- No benchmark gate. A release does not wait on a comparison of
  benchmark runs, so this milestone builds no seeded corpus, scenario
  matrix, `budgets.yaml`, baseline or dedicated runner. Each package
  keeps the allocation ceilings of its benchmarks and Allocs tests.
- The sample and witness annotators and the classification of
  callables: the kernel's `authored` package and eidos-plugin-shape
  contain them.
- The API-surface baseline and the schema index: milestone 0014 owns
  the compatibility artifacts. This milestone builds the machinery
  that blocks a tag; 0014 adds what the tag publishes.
- The catalog and declarative plugins joining the ring: they join as
  they arrive (0011, 0012); the ring runs whatever exists.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The ring job adds a build and a test run of every satellite to the tag pipeline, and GitHub cancelled the Windows job of the Go checks at its former limit of 30 minutes | the tag date only | Run the ring as a job of its own, so its time does not add to the Go checks |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-11 | The third bullet waits for a demonstration run | The ring job runs the program `internal/ring` of eidos-reference, which tests the latest release of each module that requires the kernel against the checkout. No module has a release yet, so a breaking change of the kernel blocks the tag through the Go checks of CI alone. The tests of the program run it against a module proxy that serves a release whose test fails against the checkout, and the run fails |
| 2026-10-11 | Ticked the first two bullets | Each of the nineteen plugins passes `plugintest.RunPluginSuite`. A cold run of the composition writes the 18 files of `testdata/want`, and `RunWorkspaceSuite` and `RunWarmColdSuite` pass over the tree with the stores that the command line passes. `go vet` passes over the Go output, the five generated Go tests pass with their 24 subtests, and `tsc` type-checks the TypeScript output. The binary passes eight of the nine checks of `acceptancetest`. It has no command that panics, so its test leaves out `AssertPanic` |
| 2026-10-11 | Linked RFC-0026 | The RFC designs the nineteen plugins, the composition, the workspace tree, the reference expression of the scaffolding and the ring job |
| 2026-10-11 | eidos-reference contains nineteen reference plugins, which the first bullet lists | An API-typical canary exercises each pattern of a real ensemble. Its hosts declare slots that other plugins fill, its weavers append to the methods of other plugins, and its annotators stamp keys that its generators read. The stub, builder and enum generators each write a file of checks beside their output |
| 2026-10-11 | Cut the seeded corpora, the scenario matrix, the budgets, the baselines, the dedicated runner and the measurement of warm-no-change at L. The goal and the risks concern the canary ring alone | We do not gate a release on benchmark regressions. Each package keeps the allocation ceilings of its benchmarks and Allocs tests |
| 2026-08-30 | Added at position 13 | The gates must exist before the first tag, and they need the engine, a binary and two languages to measure |
