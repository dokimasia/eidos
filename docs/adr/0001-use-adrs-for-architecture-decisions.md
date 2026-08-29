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

The architecture specification holds 80 settled decisions, D1 through D80.
The table in [21-decisions.md](../architecture/21-decisions.md) lists them.
Each row gives the decision, one line of reasoning, a link to the document
that argues it, and for some decisions the event that would make us
reconsider. The table calls that event a revisit trigger.

The full argument for a decision sits in the document that owns the
mechanism. The case against running a daemon is a section of
[09-incrementality.md](../architecture/09-incrementality.md). The table row
only points at it.

Eight decisions name an event that would make us reconsider: D9, D12, D13,
D16, D23, D29, D45 and D77. The events are things like someone building an
IDE surface, a native compiler port publishing a public importable API, and
an organisation running out of CI-restore bandwidth. When one of them
happens, we argue the decision again against evidence we did not have the
first time. That argument does not fit in a table row.

Before this record existed, the table said these decisions would be written
up as ADRs in `docs/adr/`, and that directory did not exist.

We already write down what we reject. Three specification documents carry
"Considered and refused" sections: 08 on per-plan model transforms, 09 on an
embedded database, and 11 on machine-supplied toolchain frontends. Each one
names the alternative and says why we did not take it.

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
within a few months. The specification states the rule directly: every
component, contract, and policy "appears in exactly one" of its documents.

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
- "No longer fits in one row" is a judgement call. Until a real case sets the
  bar, two contributors will draw the line differently.

**Neutral:**

- ADR numbers and D-numbers are separate. D-numbers stay the specification's
  identifiers, and a decision with an ADR has both.
