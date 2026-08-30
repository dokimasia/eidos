---
adr: 0005
title: Group generated files by concern
status: Accepted
date: 2026-08-30
supersedes: none
superseded-by: none
---

# ADR-0005: Group generated files by concern

## Status

Accepted

## Context

The model generator writes kind structs, walk code, owner rewiring,
JSON codecs and slot accessors for 21 kinds on two sides. That output
has to land in some file layout, and the layout decides what a
reviewer sees in every schema-touching diff for the life of the
project.

The repository already treats the `.gen.go` suffix specially:
`.ergon.yaml` excludes `*.gen.go` from license-header enforcement.
RFC-0002 specifies the generator and names the files.

## Decision

We will write generated code as one file per concern per side,
named `*.gen.go` (`kinds.gen.go`, `walk.gen.go`, `rewire.gen.go`,
`json.gen.go`, and `slots.gen.go` on the emit side), because a
reviewer reads a schema change as one concern at a time and the
suffix rides the tooling exclusions the repository already has.

## Alternatives Considered

### One file per kind

`node/struct.gen.go`, `node/method.gen.go` and so on: 42 files, each
holding its kind's struct, walk case, rewire case and codec together.
It lost because the concerns interleave inside every file, the
templates fragment the same concern's logic across kinds, and the
tree churns by two files for every kind added. Reviewing "what did
this schema edit do to traversal" means opening 21 files instead of
one.

### One file per side

`node/generated.gen.go` and `emit/generated.gen.go`. It lost because
each file lands in the low thousands of lines, every generated diff
collides in one path, and a reviewer looking for the JSON change
scrolls past the structs and the walker to find it.

## Consequences

**Positive:**

- Ten stable file names; a schema edit shows up as a per-concern
  diff.
- The `.gen.go` suffix keeps generated files out of license-header
  enforcement without new configuration, and gives linters and review
  tools the conventional signal.

**Negative:**

- A single schema field edit touches up to five files per side in the
  diff, because every concern regenerates.
- Concern files grow with the kind count. At 21 kinds they stay in
  the hundreds of lines; a much larger model would reopen this.

**Neutral:**

- The layout is invisible to importers: package contents are
  identical whichever layout the generator writes.

## References

- [RFC-0002](../rfc/0002-model-generator.md), the model generator and
  its output-file table
- [.ergon.yaml](../../.ergon.yaml), the `*.gen.go` license-header
  exclusion
