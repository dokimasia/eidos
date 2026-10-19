---
milestone: 0014
title: Consumers can depend on tagged modules
status: Planned
depends-on: 0008, 0010, 0011, 0012, 0013
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Milestone 0014: Consumers can depend on tagged modules

## Goal

A consumer outside this repository runs `go get` against tagged
modules and builds a binary on them. Every compatibility claim in
[15-compatibility.md](../architecture/15-compatibility.md) is executed
by CI, and the generated documentation artifacts publish with the tag.

## Done when

- [ ] The API baseline files are committed and the surface diff runs
      per release: an addition updates the baseline in review, and a
      removal fails. The contract-version handshake fails a mismatched
      build naming the component and both versions, and `version`
      prints the contract version.
- [ ] The schema index publishes per release, the evolution classifier
      passes additive fixtures and blocks a seeded breaking change,
      and `doctor` runs the consumer-side diff. This closes the piece
      milestone 0008 left open.
- [ ] Every artifact in the table of
      [14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md)
      generates in the tag pipeline, and a seeded stale page blocks
      the tag the way a failing check does.
- [ ] The support matrix, the parity matrix and the diagnostic-code
      index publish beside the tag.
- [ ] Tags exist: `eidos-core/v0.1.0`, and first tags for eidos-cli,
      eidos-lang, eidos-lang-go, eidos-lang-typescript,
      eidos-lang-protobuf, eidos-plugin-shape and eidos-reference, each
      declaring a kernel range its CI proves at the declared minimum and
      maximum.
- [ ] From an empty directory outside this repository, the consumer
      example from
      [14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md)
      builds against the tags alone and runs `run`, `plan` and
      `explain` over a fixture project.

## Why now

Last, after 0008 and 0010 through 0013. The release freezes the
surfaces every earlier milestone was free to change, so it must not
come earlier, and the gates from 0013 must exist before the first tag
so no claim ships unexecuted. dokimi's adoption and every external
consumer wait on this.

## Scope

[15-compatibility.md](../architecture/15-compatibility.md) executed
end to end, the documentation table of
[14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md),
and the first version tags.

## Not in this milestone

- A v1.0 stability horizon: the additive discipline applies from this
  first tag, and the major-version promise is a separate, unscheduled
  decision.
- Java, Kotlin, PHP and Rust satellites: unscheduled, see the
  [index](README.md).

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The first multi-module tag train exposes ordering problems in the release tooling | the tag date only | Dry-run the whole train on a fork before running it here |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-19 | The milestone ships in no fixed release, and no bullet is ticked | The releases of 2026-10-13 tagged every module of the fifth bullet before this milestone. The tags are v0.1.0 and then v0.2.0 or v0.1.1. No CI job proves a kernel range at its declared minimum, because every module builds against the kernel at the commit and the ring tests each release against the same kernel. The manifest loader refuses a manifest of another contract version with a fault that contains the plugin and both versions. `version` prints the contract version. No API baseline is committed. The schema index, the evolution classifier, the diff of `doctor` and the generation of the documentation in the tag pipeline do not exist. The releases have no assets, and CI uploads the support matrix as an artifact of each run. The consumer example of 14-distribution-and-cli.md calls `golang.ServerPlan`, `typescript.ClientPlan` and `shapefull.Annotators`. No module declares them, so the example does not build against the tags |
| 2026-10-11 | The fourth bullet publishes no benchmark comparison against the previous release | We do not gate or report benchmark regressions between releases |
| 2026-10-08 | The handshake bullet has `version` print the contract version, and eidos-cli joins the first tags | Milestone 0008's `version` has no contract version to print before the handshake exists, and the command kernels are a module of their own |
| 2026-08-30 | Added at position 14 | The release comes last because it freezes what everything before it was still free to change |
