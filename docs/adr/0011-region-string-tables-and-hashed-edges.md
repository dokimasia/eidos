---
adr: 0011
title: Each region has its own string table, and edges are hashed
status: Accepted
date: 2026-10-02
supersedes: none
superseded-by: none
rfc: RFC-0020
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0011: Each region has its own string table, and edges are hashed

## Status

Accepted

## Context

The sealed state stores each frontend unit's share of the graph as a region,
and every record of what an execution read as edges between identities. The
state was specified with one intern table for every generation. It gave each
identity, key and path a dense identifier, grew without bound, and sent the
run cold to compact it once dead entries passed half the table.

The parse memo conflicts with that table. The memo keeps regions across
generations and across cold runs, in the encoding the generation uses. A
region encoded against one generation's identifiers reads only with that
generation's table. A cold run starts the table empty, so every older memo
entry becomes unreadable at the moment the memo exists to serve: after a
branch switch, a restored cache or a cold run.

Under the compaction rule, a run whose only fault was dead table entries
went cold and parsed the whole corpus.

## Decision

We will give each region a string table of its own and key every edge, and
every row keyed by an identity, by the first eight bytes of the SHA-256 of
its canonical spelling, because a region must decode without the generation
that wrote it, and two spellings that share a hash cost an extra execution
and never a missed one.

- A region in a generation and a region in the memo have one encoding, and
  each decodes alone.
- A row keyed by a hash stores the spelled key beside it, and a lookup
  compares the spelling.
- A run still interns identities, keys and paths into dense identifiers in
  memory. The state stores none of those identifiers.

## Alternatives Considered

### A global intern table

One table gives every identity, key and path a dense identifier for all
generations, and every region and record stores identifiers. Records are
smaller, and comparing two edges compares two integers. It lost because a
memo entry then depends on the intern table of the generation that wrote
it, a cold run makes every older entry unreadable, and a table that only
grows needs a compaction that rewrites every reference.

## Consequences

**Positive:**

- A memo entry decodes after any cold run, under any generation, and in
  another workspace that shares the memo.
- No run goes cold to compact a table.
- A region's bytes depend on the unit alone, so two runs that parse one
  unit write the same region.

**Negative:**

- A string that many regions use is stored once in each of them.
- An edge takes eight bytes on disk, where a dense identifier took four.
- Two edges that share a hash make a run execute more than an edit requires.
- A lookup by hash compares the stored spelling, so a hash-keyed row stores
  its key twice.

**Neutral:**

- The in-memory interning of a run is unchanged.

## References

| What | Where |
|---|---|
| D72, the sealed state's intern table that this decision replaces | ../architecture/21-decisions.md |
| D56, dense run-local identifiers | ../architecture/21-decisions.md |
| D77, the parse memo | ../architecture/21-decisions.md |
