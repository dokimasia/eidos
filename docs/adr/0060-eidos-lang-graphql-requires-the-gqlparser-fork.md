---
adr: 0060
title: eidos-lang-graphql requires the fork of gqlparser
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0068
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0060: eidos-lang-graphql requires the fork of gqlparser

## Status

Accepted

## Context

gqlparser v2.5.63 is the latest release of gqlparser. It refuses four forms that the satellite
reads and writes:

- an interface extension that adds interfaces
- a braced Unicode escape
- the description of an operation, a fragment or a variable
- the directives of a directive definition and the extension of a directive

The specs of the satellite and of the conformance suite failed against v2.5.63 on the first three
forms, and the frontend could not read the fourth. The tracker of gqlparser has no issue or pull
request about any of the four forms.

The fork `dokimasia/gqlparser` merges a fix for each form into v2.5.63. Each fix is a branch of its
own for a pull request upstream. The specs of every module of the workspace pass against the fork.

## Decision

We will require the fork `github.com/dokimasia/gqlparser/v2` in `eidos-lang-graphql` at a commit of
its master, because the fork parses the four forms and no release of gqlparser does.

- The fork declares the module path `github.com/dokimasia/gqlparser/v2`, and the satellite imports
  its packages through that path.
- Each fix is also a pull request upstream.
- The first release of gqlparser that contains the four fixes replaces the fork.

## Alternatives Considered

### A replace directive

The satellite could require `github.com/vektah/gqlparser/v2` and replace it with the fork. A replace
directive applies only in the main module, so a consumer of the satellite would build it against
gqlparser without the fixes. `go install` at a version refuses a module with a replace directive,
so the root module, whose command `eidos` composes the satellite, cannot contain one.

### A release of gqlparser

A release of gqlparser with the four fixes would let the satellite keep the module path of upstream.
Upstream has not reviewed the fixes, so no date of such a release is known. A document that uses
one of the four forms would not load in the meantime, and the test cases of the four forms would
fail on every commit.

## Consequences

**Positive:**

- A document that uses one of the four forms loads.
- `go install` builds the command `eidos` with the fixes.

**Negative:**

- The fork must merge each release of gqlparser before the satellite can use it. Each merge renames
  the import paths of the files that the release adds or changes.
- A binary that also links gqlgen contains two copies of gqlparser under two module paths.

**Neutral:**

- The branches of the fixes do not contain the change of the module path, so their pull requests
  apply to upstream as they are.

## References

| What | Where |
|---|---|
| RFC-0068, the design of the satellite | ../rfc/0068-the-graphql-satellite.md |
| D171, the row of this decision | ../architecture/21-decisions.md |
| gqlparser | <https://github.com/vektah/gqlparser> |
| The fork of gqlparser | <https://github.com/dokimasia/gqlparser> |
| Go Modules Reference, the replace directive and `go install` | <https://go.dev/ref/mod> |
