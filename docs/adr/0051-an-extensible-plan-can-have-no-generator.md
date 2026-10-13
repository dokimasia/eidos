---
adr: 0051
title: An extensible plan can have no generator
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0065
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0051: An extensible plan can have no generator

## Status

Accepted

## Context

The plan step of Build refused every plan without a generator with `workspace: plan %q lists no
generator`. A config file adds generators only to a plan that the composition marks extensible. The
binary `eidos` declares an extensible plan for each of its four targets and has no generator of its
own. A workspace extends the plans of the targets that it generates. Build then refused the binary in
every workspace that extends fewer than four plans.

## Decision

We will accept an extensible plan without a generator in Build, because a binary that offers a plan
for each target does not know which plans a workspace extends.

- The plan step reports `lists no generator` only for a plan that is not extensible.
- The run of an extensible plan without a generator does not render a file. Its staging removes the
  files of its last generation.
- `plan` lists such a plan with its backend and without a generator line.

## Alternatives Considered

### A generator that emits nothing in each plan

Each plan of the binary could list a generator without rules, so Build would accept the plan. The
generator would appear in `plan` and in the fingerprint of every run. It would generate nothing.

### Plans that the command kernels disable

The command kernels could disable each extensible plan that the config file does not extend. A
disabled plan does not appear in `plan` as a plan that runs. The key `enabled` would then have two
sources. The plan step is the place where the composition and the config file meet.

## Consequences

**Positive:**

- A binary offers a plan for each target, and a workspace extends the plans that it needs.
- A config file that drops the last plugin of a plan removes the files of the plugin on the next run.

**Negative:**

- An extensible plan whose plugins a config file forgot runs without output and without a finding.
- The fault of a composition that forgets the generators of an extensible plan moves from Build to
  the output of `plan`.

**Neutral:**

- A plan that is not extensible still needs a generator.

## References

| What | Where |
|---|---|
| RFC-0065, the design of the binary eidos | ../rfc/0065-the-eidos-binary.md |
| D161, the row of this decision | ../architecture/21-decisions.md |
| The plan step of Build | ../architecture/08-workspace-and-plans.md |
