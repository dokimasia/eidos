---
adr: 0113
title: A data family writes one file under a name of its own
status: Accepted
date: 2026-10-28
supersedes: none
superseded-by: none
rfc: RFC-0087
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0113: A data family writes one file under a name of its own

## Status

Accepted

## Context

RFC-0087 designs a family of data files and the comparison of a set of lines under `run --check`.
dokimi needs a committed list of the checks that a run generated, so a lost check is one deleted
line in review and one finding of the plugin under `--check`.

## Decision

We will build the design of RFC-0087.

- `plugin.Output.Name` declares a data family. The layout refuses one that is not `PerPlan`, has no
  template or has a name that is not one path element. Its file has the zero package and
  `plugin.File.Data`.
- The render writes the output of the template of a data file without the skeleton and the
  formatter, and the run stages it without the frame.
- `output.Owner` takes the digest that the manifest records for a path. The run declares the digest
  of each previous entry of a plan and of each entry that the sweep removes. `Disk` classifies a file
  without a frame at the recorded digest as intact and at another digest as foreign, and a commit
  refuses the foreign one under `output.ErrRecordedDigest`.
- `plugin.Output.LineSet` declares a line set. Under `--check`, the run passes each line that the
  committed file in the tree has and the fresh file lacks to `plugin.LineSetReader.Removed`, and
  reports the finding at the line with the origin of the generator. `Builder.Removed` declares the
  function for a plugin that the SDK builds.
- `OutOfDate` remains for every line-set file that differs.

## Alternatives Considered

RFC-0087 compares the file name through the configuration of the plan, a frame in a data file and
line findings in the kernel. The RFC records the reasons against each.

## Consequences

**Positive:**

- A generator writes a data file under a name that it states, without a frame or a format step.
- A plugin reports a removed line of its file as its own finding, at the line.

**Negative:**

- The manifest is the only proof of the origin of a data file.
- A first run over a file of a person at the path of a data file withholds the write.
- The run declares the digest of every previous entry of a plan to the sink. The sink keeps one map
  entry for each.

## References

| What | Where |
|---|---|
| RFC-0087, data files and line sets | ../rfc/0087-data-files-and-line-sets.md |
| D231, the row of this decision | ../architecture/21-decisions.md |
| Routing and layout | ../architecture/18-routing-and-layout.md |
| The CLI, `run --check` | ../architecture/20-cli.md |
| Issue 98, a generator cannot write a data file under a name of its own | <https://github.com/dokimasia/eidos/issues/98> |
| Issue 99, `--check` reports a changed line-set file as a stale file | <https://github.com/dokimasia/eidos/issues/99> |
