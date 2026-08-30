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

The architecture specification settles 80 decisions, D1 through D80, and
lists them in one table. Each row gives the decision, one line of reasoning,
a link to the document that argues it, and for some decisions the event that
would make us reconsider. The table calls that event a revisit trigger.

A row does not carry the argument. The document that owns the mechanism
does: the case against running a daemon is a section of the incrementality
document, and the row only points at it.

Eight rows name an event that would make us reconsider: D9, D12, D13, D16,
D23, D29, D45 and D77. Someone builds an IDE surface. A native compiler port
publishes an importable API. An organisation runs out of CI-restore
bandwidth. When one of those happens we argue the decision again against
evidence we did not have the first time, and that argument is longer than a
row.

Before this record existed, the table said these decisions would be written
up as ADRs in `docs/adr/`, and that directory did not exist.

We already write down what we reject. Three specification documents carry a
"Considered and refused" section: the workspace document on per-plan model
transforms, the incrementality document on an embedded database, and the
languages document on toolchain frontends that fork `go list`, invoke Node
or require a JVM. Each names the alternative and says why we did not take
it.

## Decision

Record architecture decisions as ADRs in `docs/adr/`. Give each one a number.
Once you accept an ADR, do not change what it argues. If the decision itself
changes, write a new ADR that supersedes it.

Keep using the table in `21-decisions.md` to look up what has been decided.
Every settled decision stays listed there. Write a full ADR for a decision
when the event you said would make you reconsider actually happens, or when
the reasoning no longer fits in one row. Then link that row to the ADR.

## Alternatives Considered

### Migrate all 80 decisions to ADRs immediately

Write one ADR per row and delete the table.

We rejected this because the reasoning is already written down. Each decision
is argued in the document that owns the mechanism, where you read it next to
the machinery it explains. An ADR built from a one-line row would say less
than the specification text it copies, and the two copies would disagree
within a few months. The specification states the rule outright: every
component, contract and policy appears in exactly one document.

### Keep the table only, and drop the ADR claim

Delete the sentence pointing at `docs/adr/` and keep the table as the whole
record.

We rejected this because the eight decisions above would have nowhere to
record a new argument. When one of those events happens, someone has to write
that argument down. Writing it in the table means either a row too small to
hold it, or editing the row that records the original decision. Editing the
row loses the history the table exists to keep.

## Consequences

**Positive:**

- When we reopen a decision, there is somewhere to argue it in full, and the
  older record stays as it was.
- The table now points at a directory that exists.
- A decision nobody reopens costs nothing. It stays one row, and nobody
  writes a thin ADR to satisfy a convention.

**Negative:**

- Decisions live in two places. A reader has to know that the table lists
  everything, and that the ADR, where there is one, holds the current
  argument.
- When you supersede an ADR you have to edit the new ADR and the table row.
  Forget the row and the table shows a superseded decision as current.
- Whether reasoning still fits in one row is a judgement call. Until a real
  case decides it, two contributors will draw the line in different places.

**Neutral:**

- ADR numbers and D-numbers are separate. D-numbers stay the specification's
  identifiers, and a decision with an ADR has both.

## References

- [21-decisions.md](../architecture/21-decisions.md), the table of 80
  settled decisions and their revisit triggers
- [README.md](../architecture/README.md), the rule that every component,
  contract and policy appears in exactly one document
- [08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md),
  [09-incrementality.md](../architecture/09-incrementality.md) and
  [11-languages.md](../architecture/11-languages.md), the three
  "Considered and refused" sections
