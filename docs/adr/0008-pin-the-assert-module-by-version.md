---
adr: 0008
title: Pin the assert module by version
status: Accepted
date: 2026-09-30
supersedes: ADR-0007
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0008: Pin the assert module by version

## Status

Accepted

## Context

Test code in every module asserts through `go.dokimi.dev/assert`.
The modules adopted the library while it was unpublished, through a
replace directive to a sibling checkout at `../../assert-go`. A
replace directive applies only in the main module's `go.mod`, so no
module that contains one can be published. Every checkout, CI's
included, also needs the sibling directory, and no file records
which state of it a run tested.

The library now resolves through its import path. The public module
proxy serves its commits from `github.com/dokimasia/assert-go`, and
it has no tagged release: the proxy's version list is empty, and
`@latest` resolves to a pseudo-version.

A pseudo-version pins one commit of a module that has no release,
and the go command resolves it through the proxy like any version.
A tagged release would freeze the library's API, which this
repository, the library's first consumer, still shapes.

The modules already follow this practice:

- All 13 require `v0.0.0-20260902112452-9d6eca9d7234`.
- No module contains a replace directive.
- Each `go.sum` records the library's checksum.

## Decision

We will require `go.dokimi.dev/assert` at one pseudo-version in every
module, without a replace directive, because a pinned commit
resolves to the same bytes on every machine and in CI while the
library has no tagged release.

Every module moves to a newer commit in the same change, so no two
modules test against two states of the library.

## Alternatives Considered

### Keep the replace directive to the sibling checkout

This repository's tests then run against a change to the library
without a commit in either repository. It lost because every
checkout and CI need the sibling directory, two machines can test
against two states of the library, and no module that contains a
replace directive can be published.

### Wait for a tagged release

A semantic version would identify the library's state in the
proxy's version list. It lost because a release freezes the API,
and this repository, the library's first consumer, still shapes it.

## Consequences

**Positive:**

- A checkout of this repository builds and tests without a sibling
  directory. CI checks out one repository, and the go command
  fetches the library through the module proxy.
- `go.sum` pins the library's content, so every machine tests against
  the same bytes.
- A module that asserts through the library can be published,
  because a consumer resolves the requirement through the proxy.

**Negative:**

- This repository takes a fix in the library only through a version
  change in all 13 modules and a commit.
- A `v0` pseudo-version has no compatibility guarantee. A move to a
  newer commit needs a full test run of every module.
- A consumer of a published eidos module resolves the library
  through the same pseudo-version.

**Neutral:**

- Test code still asserts through the library. Only the way each
  module resolves it changes.
- The decision records the practice every module already follows.

## References

| What | Where |
|---|---|
| Go modules reference, replace directives and pseudo-versions | https://go.dev/ref/mod |
| The pinned version at the module proxy | https://proxy.golang.org/go.dokimi.dev/assert/@v/v0.0.0-20260902112452-9d6eca9d7234.info |
| The library's repository | https://github.com/dokimasia/assert-go |
