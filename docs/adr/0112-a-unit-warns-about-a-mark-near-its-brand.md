---
adr: 0112
title: A unit warns about a mark near its brand
status: Accepted
date: 2026-10-28
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0112: A unit warns about a mark near its brand

## Status

Accepted

## Context

A unit reads its carriers under the three marks of the brand of the load: `brand:`, `+brand:` and
`-brand:`. Every other mark is comment text, so `//go:generate` and `//nolint:` stay with their
tools.

- Issue 102 reports that the rule also covers `//dokimy:suite`, a misspelled brand that no other
  tool reads. A run of dokimi over the reproduction wrote no file, reported no finding and exited
  0, also under `--strict`.
- A misspelled directive name under the right brand already fails. The registry of directives
  names the nearest registered name within two edits and within a third of the length of the
  longer spelling.
- A frontend passes each decorator and annotation to `SourceUnit.AttachSugar`, which treats a
  marker whose first name is not the brand as ordinary metadata. `@dokimy.suite` of TypeScript had
  the same gap.

## Decision

We will warn about a mark or a marker that is near the brand and is not the brand.

- `SourceUnit.Comment` reports a Warning under `MisspelledBrand`, EID-0083, at a line that opens
  with the shape of a mark whose name is near the brand. The shape is an optional sign, a name of
  ASCII letters, digits, hyphens and underscores, the colon and a letter. The line remains comment
  text.
- `SourceUnit.AttachSugar` reports the same Warning for a marker of two or more names whose first
  name is near the brand. The marker remains metadata.
- A name is near under the bound of a suggestion of a directive name. `internal/edit` contains the
  distance and the bound, and the registry of directives and the unit both read it.
- A unit of a dependency reports no such Warning.

## Alternatives Considered

### A flat bound of two edits

The issue requests two edits for every brand. Under the brand `acme`, two edits make the mark
`me:` of another tool near the brand. The bound of the registry allows one edit at four letters
and two edits from six letters on. It gives `dokimy` the same answer as the flat bound.

### An Error

An Error would fail every run over a source that uses a tool whose name is near the brand. A
Warning fails no run of eidos, and a run of dokimi under `--strict` reports it as an error.

## Consequences

**Positive:**

- The reproduction of the issue reports `the mark dokimy: is not the brand dokimi: of this run, so
  the line is comment text`, and `dokimi run --strict` exits 1.
- Every frontend gets the rule through `SourceUnit`, also for decorators and annotations.

**Negative:**

- A tool whose name is near the brand gets a Warning at each of its marks. A source that uses
  such a tool cannot turn the Warning off line by line.
- Each comment line in the shape of a mark costs a scan of its name. A name near the brand
  allocates the rows of its distance and the message of the Warning.

## References

| What | Where |
|---|---|
| D230, the row of this decision | ../architecture/21-decisions.md |
| The carriers of a directive | ../architecture/05-directives.md |
| Issue 102, a directive whose brand has a typo is comment text | <https://github.com/dokimasia/eidos/issues/102> |
