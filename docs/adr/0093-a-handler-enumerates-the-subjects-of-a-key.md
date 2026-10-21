---
adr: 0093
title: A handler enumerates the subjects of a key under a membership edge
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: RFC-0085
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0093: A handler enumerates the subjects of a key under a membership edge

## Status

Accepted

## Context

`Fact` and `FactOf` read the fact of one subject. The fact store indexes the subjects of each key
at stamp time, and the dispatcher reads that index for the gate `HasKey`. A handler cannot read
the index.

- A generator of conformance suites binds the checks that a consumer declares on functions of the
  package of a suite. The annotator half stamps a fact on each check with the identity of the
  method that it checks. The generator half runs on the interface in another package, and it lists
  the checks of its methods.
- A walk of every package of the reader with a call of `FactOf` for each function records a
  declaration edge on every declaration of the scope. An edit of any declaration then runs the
  handler of every suite again.
- The read set records a membership edge for an enumeration by kind and by directive. A warm run
  makes such an edge dirty in the package where the membership changed, and each plan runs its
  readers of the edge where its scope admits the package.

## Decision

We will let a handler enumerate the subjects of a key through `Subjects` under a membership edge
on the key, because the fact store indexes the subjects of each key already. An edit of another
declaration then leaves the handler clean.

- `Subjects(m, k)` yields the subjects under the scope of the match on which `k` reads present, in
  identity order, and records the edge when the range starts. A graph match enumerates too.
- A named handle resolves by its name in the registry of the run, because the presence of a key
  does not depend on its value type. A name that the registry does not contain records the edge
  and does not yield a subject. The zero key does not record an edge.
- The read set has a sixth grain, `RecordKey` and `Keys`, and `state.KeyEdge` hashes the edge with
  a tag of its own. A record lists its edges by hash, so the format of the state does not change.
- The settle step of a warm run makes the edge of a key dirty in the package of each subject whose
  presence of the key changed. A change of the value alone leaves the edge clean.
- `explain` lists the edge as a read of the grain `ReadKey`.

## Alternatives Considered

### A walk of the scope with `FactOf`

The generator half would walk every package of its reader and read the fact of each function. The
walk records a declaration edge on every declaration of the scope, so an edit of any declaration
runs the handler of every suite again, and the walk costs the size of the scope on every run.

### A gate on the value of a fact

`KeyEquals` would admit the functions whose fact names a method of the interface, and a rule on
those functions would add each binding to the suite. The rule runs on each check and not on the
interface. The suite is one record of the invocation on the interface, and a record takes the data
of the first call in canonical match order, so the invocations of the checks cannot add to it.

### A resolution of a named handle by its value type

`Subjects` would resolve a named handle as `FactOf` does, by its name and its value type. The
resolution in the generic function keeps the compiler from inlining `Subjects`, and each range then
allocates its loop body and its state, three allocations for each invocation of the fixture.

## Consequences

**Positive:**

- A handler lists the subjects of a key in the scope without a walk, and runs again when a subject
  gains or loses the key.
- A range over `Subjects` allocates nothing in steady state, through the kernel and through the
  facade of the SDK.

**Negative:**

- A key on thousands of declarations runs its enumerators on most warm runs that stamp it.
- A handler that reads the value on each subject records one fact edge for each subject.
- A named handle of another value type lists the subjects of the key, and `FactOf` reads each value
  absent through it.
- An enumeration of a key that another match of the same phase call stamps depends on the order of
  the matches, as a read of such a fact through `FactOf` does.

## References

| What | Where |
|---|---|
| D211, the row of this decision | ../architecture/21-decisions.md |
| D55, the indexed dispatch | ../architecture/21-decisions.md |
| Issue 58, the subjects of a key | <https://github.com/dokimasia/eidos/issues/58> |
| go/analysis, `Pass.AllObjectFacts` | <https://pkg.go.dev/golang.org/x/tools/go/analysis#Pass> |
