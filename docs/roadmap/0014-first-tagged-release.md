---
milestone: 0014
title: Consumers can depend on tagged modules
status: Planned
depends-on: 0008, 0010, 0011, 0012, 0013
ships-in: v0.1.0
deadline: none
deadline-source: none
prd: none
rfc: none
---

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
      build naming the component and both versions.
- [ ] The schema index publishes per release, the evolution classifier
      passes additive fixtures and blocks a seeded breaking change,
      and `doctor` runs the consumer-side diff. This closes the piece
      milestone 0008 left open.
- [ ] Every artifact in the table of
      [14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md)
      generates in the tag pipeline, and a seeded stale page blocks
      the tag the way a failing rung does.
- [ ] The support matrix, the parity matrix, the diagnostic-code index
      and the benchmark comparison against the previous release
      publish beside the tag.
- [ ] Tags exist: `eidos-core/v0.1.0`, and first tags for eidos-lang,
      eidos-lang-go, eidos-lang-typescript, eidos-lang-protobuf,
      eidos-plugin-shape and eidos-reference, each declaring a kernel
      range its CI proves at the declared minimum and maximum.
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
| 2026-08-30 | Added at position 14 | The release comes last because it freezes what everything before it was still free to change |
