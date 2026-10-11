<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Benchmarking

*Builds on: [09](09-incrementality.md) (the claims under test).*

The performance target in [09-incrementality.md](09-incrementality.md)
is a claim: ten thousand packages or more, warm regeneration under a
second, and warm cost that tracks the edit rather than the corpus. The
check of every commit executes two parts of the claim. A warm run over
an unchanged tree does not repeat any work. Each hot path allocates
what its contract states. A release does not wait on a benchmark run,
and it does not publish benchmark results.

## Zero work over an unchanged tree

A probe asserts that a warm run over an unchanged tree does not hash a
file, parse a unit, decode a region, run an invocation or call a check.
The run leaves every file of the workspace as it was. The probe counts
work, not milliseconds, so it holds on every runner. The warm and cold
suite of the workspace runs it over each fixture
([13-testing-and-conformance.md](13-testing-and-conformance.md)).

## The allocation contract of the micro benchmarks

Micro benchmarks pin the per-operation costs of the hot paths: Callable
projection, TypeOf, Resolve, per-kind template render, the fingerprint
pass per file, and a metadata read through the tracking Reader. Every
micro benchmark pins allocations per operation alongside time.
Allocation counts do not move with machine load, so a failed ceiling
means a real code change.

An Allocs test of the package checks the same ceiling, so the check of
every commit runs it. A ceiling that only a benchmark can measure, such
as one that excludes the setup of each call, states so in the docblock
of its benchmark.

A small count per call has an exact ceiling, because one allocation
more is the defect there. A run over a corpus that allocates hundreds
of thousands of times or more has a round budget about 5% above one
measurement. Its docblock states the measured count and the size of the
corpus.

The discipline is uniform. Benchmarks run one package at a time,
because a whole-suite run inflates timings even though allocation
counts survive it. The CPU count is fixed, warm-up iterations are
excluded, and the harness has profiling flags, so any benchmark can
emit CPU and heap profiles for the regression that it caught.

## Who runs what

- **The kernel** benchmarks its own layers: the engine, the store,
  fingerprinting, the sealed-graph format and the template machinery.
- **Language satellites** benchmark their frontends, their projection
  rules and their backends ([11-languages.md](11-languages.md)).
- **eidos-reference** is the canary of the ring
  ([15-compatibility.md](15-compatibility.md)), and does not run a
  benchmark.
