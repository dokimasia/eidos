---
adr: 0125
title: A data family frames its file in the comment syntax that it states
status: Accepted
date: 2026-11-07
supersedes: ADR-0113
superseded-by: none
rfc: RFC-0091
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0125: A data family frames its file in the comment syntax that it states

## Status

Accepted

## Context

A data file had no frame, so a sink recognised it by the digest that the manifest of the previous
generation records for its path. The manifest is in the state directory, which a repository does not
commit. In a clone without the state directory, the sink classified the data file of the brand as
foreign. dokimi could not add or remove a suite there, because the run refused its lock with
`ForeignFile`.

## Decision

We will stamp the file of a data family with the frame of the brand in the comment syntax that the
family states, because the frame is the record of origin that a fresh clone has.

- `plugin.Output.Comment` declares the comment syntax of the file of a data family. The layout
  refuses it on a family without a name. The composition refuses a syntax with neither a line form
  nor a block form.
- The run stamps the file through a contract of the brand and that syntax. A sink classifies the
  file through its frame, as it classifies a source file.
- A data family without a comment syntax writes its file without a frame. A sink recognises that
  file by the recorded digest. The rule that the run writes every data file without a frame applies
  to such a family alone.
- Under `run --check`, the line set compares the bodies of the committed file and the fresh file. A
  removed line has its line in the committed file.
- `output.Body` returns the body of a stamped file and the number of lines above it.

## Alternatives Considered

### The manifest in the repository

Every change of a generated file changes a document of the manifest. The proof of a data file would
be in a second file. A merge can take the two files from two branches.

### A header that the plugin checks

A header does not contain a digest of the body. The run would overwrite a file that a person writes
with the header. The check of drift compares a body with its digest, so the run would also overwrite
an edit of its own file.

### The comment syntax of the target

The lock of a Go plan would have `//` lines, and the format of the lock has no such comment. The
comment syntax belongs to the format of the file.

## Consequences

**Positive:**

- A run recognises its framed data file in any clone, and refuses a file of a person at its path.
- An edit of a framed data file is drift, which the run refuses without `--overwrite-drift`.

**Negative:**

- The frame adds four lines and one line for each plugin to the file, and its trailer changes with
  every change of the body.
- A reader of the file outside eidos skips the lines of the frame.
- A committed data file without a frame remains foreign in a clone without the state directory until
  a commit contains the framed file.

## References

| What | Where |
|---|---|
| RFC-0091, the frame of a data file | ../rfc/0091-the-frame-of-a-data-file.md |
| D243, the row of this decision | ../architecture/21-decisions.md |
| ADR-0113, the data family, whose data files had no frame | 0113-a-data-family-writes-one-file-under-a-name-of-its-own.md |
| Issue 67 of dokimi, a clone without `.dokimi/` cannot add or remove a suite | <https://github.com/dokimasia/dokimi/issues/67> |
