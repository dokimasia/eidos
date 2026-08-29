---
adr: 0001
title: Use ADRs for architecture decisions
status: Accepted
date: 2026-08-30
supersedes: none
superseded-by: none
---

# ADR-0001: Use ADRs for architecture decisions

## Status

Accepted

## Context

The architecture specification records 80 settled decisions, D1 through D80.
Each is one row in the decision log at
[21-decisions.md](../architecture/21-decisions.md): the decision, its
reasoning in a single line, a revisit trigger where one was recorded, and a
link to the context document carrying the full argument. The arguments
themselves live inline in those documents — the case against a daemon is a
section of [09-incrementality.md](../architecture/09-incrementality.md), not a
paragraph in the log.

Eight decisions carry an explicit revisit trigger: D9, D12, D13, D16, D23,
D29, D45 and D77. Each names the event that would reopen it — an IDE surface
actually being built, a native compiler port publishing a public importable
API, an organisation demonstrably saturating CI-restore bandwidth. When one
fires, the decision is re-argued against evidence the original did not have,
and a one-line table row cannot hold that argument.

The decision log already declares this directory as the destination: "Each of
these will be recorded as an ADR in `docs/adr/`; this log is their source."
Until now the directory did not exist, so the specification promised a record
it did not keep.

The practice is already the repository's. Context documents carry "Considered
and refused" sections — per-plan model transforms in 08, an embedded database
in 09, machine-supplied toolchain frontends in 11 — each naming the
alternative and the reason it lost.

## Decision

We will record architecture decisions as ADRs under `docs/adr/`, numbered and
immutable once accepted.

The decision log remains the source of record for the decision set: every
settled decision appears there, and it is what a reader consults to find out
what has been decided. An individual decision graduates to a full ADR when
its recorded revisit trigger fires, or when its argument outgrows one table
row. A graduated decision's log row then links to its ADR.

## Alternatives Considered

### Migrate all 80 decisions to ADRs immediately

Generate one ADR per log row and retire the log.

Rejected because the arguments already have a home. Each decision's full
reasoning lives inline in the context document that owns the mechanism, where
it is read alongside the machinery it justifies. Copying it into an ADR
creates a second copy that drifts, and an ADR generated from a one-line row
would be thinner than the specification text it duplicates. The specification
is closed on exactly this rule: every component, contract, and policy
"appears in exactly one" of its documents.

### Keep the decision log only, and drop the ADR claim

Delete the sentence promising `docs/adr/` and let the log be the whole record.

Rejected because it leaves the eight revisit triggers with nowhere to land. A
trigger firing produces a new argument against new evidence; writing that into
the log means either a table row too small to hold it, or an edit to the row
recording the original decision — destroying the history the log exists to
keep.

## Consequences

**Positive:**

- A reopened decision has somewhere to be re-argued in full, with its
  supersession chain visible, without editing the record of what came before.
- The promise the specification already makes is kept.
- Decisions that never reopen cost nothing: they stay one row, and no thin
  ADR is written to satisfy a convention.

**Negative:**

- Decisions live in two places. A reader must know that the log is the index
  and an ADR, where one exists, is the current record.
- A supersession must be written in both the new ADR and the superseded
  decision's log row. Miss the second and the log presents a superseded
  decision as current.
- "Outgrows one table row" is a judgment call. Until a concrete case sets the
  bar, two contributors will draw the line differently.

**Neutral:**

- ADR numbering is independent of the D-numbers. D-numbers remain the
  specification's identifiers; a graduated decision carries both.
