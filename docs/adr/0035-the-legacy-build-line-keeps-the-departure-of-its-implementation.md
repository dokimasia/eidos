---
adr: 0035
title: The legacy build line keeps the departure of its implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0052
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0035: The legacy build line keeps the departure of its implementation

## Status

Accepted

## Context

The accepted design reads a `// +build` line where a Go file has no `//go:build` line, as go/build of
Go 1.27 reads it. Its first invariant requires that the frontend loads a file where the go command
builds it. Its table of failures states that a `// +build` line that does not parse constrains
nothing.

`go/build/constraint.Parse` returns no error for a line that `constraint.IsPlusBuild` accepts. It
reads a tag that is not valid, such as the `!` of `// +build !`, as the tag `ignore`, which no build
sets, and go/build then leaves the file out. The change is listed here, because an accepted design is
not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departure is this.

| Design | Code | Reason |
|---|---|---|
| A `// +build` line that does not parse constrains nothing | A `// +build` line with a tag that is not valid excludes the file, as the line reads as the tag `ignore` | The go command builds no such file, and the first invariant of the design requires the frontend to load a file only where the go command builds it |

## Alternatives Considered

### A line with a tag that is not valid constrains nothing

The frontend could read such a line as no constraint. It would then load the declarations of a file
that the go command leaves out of every build, which is the defect that the design removes.

## Consequences

- The frontend does not load the declarations of a file whose `// +build` line has a tag that is not
  valid, and stamps the line under `golang.constraint`.
