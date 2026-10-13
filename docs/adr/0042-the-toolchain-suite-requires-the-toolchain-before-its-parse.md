---
adr: 0042
title: The toolchain suite requires the toolchain before its parse
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0061
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0042: The toolchain suite requires the toolchain before its parse

## Status

Accepted

## Context

The rationale of D92 states that "parsing needs no toolchain, so grammar stays checkable
everywhere". The TypeScript adapter parses through tsc, the Java adapter through java and the Rust
adapter through rustfmt. A parse check that ran before `Require` passed without a parse where tsc
was missing. `RunToolchainSuite` now asks `Require` once, before all three checks. RFC-0061 records
the change of the rationale, because the decision log keeps its rows.

## Decision

`RunToolchainSuite` asks `Require` before its parse, type and test checks. A missing toolchain
skips the three checks locally and fails them in CI, as D92 states for the type and test checks.
The Go adapter parses with the standard library, and the suite still asks `Require` before its
parse, because one call serves every adapter. The quoted sentence of D92 does not hold for the
suite.

## Alternatives Considered

### A call of Require before each check

The suite could ask `Require` before the type and test checks alone, and let an adapter that parses
without its toolchain run the parse check everywhere. Each adapter would then state whether it
parses without its toolchain. The TypeScript adapter, whose parse ran before `Require`, shows the
cost of a wrong statement.

## Consequences

- On a machine without the toolchain of a language, the suite checks no part of the output of that
  language.
- `AssertParses` remains for a caller that parses without the suite, and such a caller asks
  `Require` first where its adapter needs a toolchain.
