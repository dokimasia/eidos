---
adr: 0076
title: The ring reports a release of an earlier compatibility line
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0076: The ring reports a release of an earlier compatibility line

## Status

Accepted

## Context

The ring job of CI tests the latest published version of each module that requires the kernel
against the kernel and the SDK facade of the checkout. The accepted design of the ring fails the
run when any module fails. The release job publishes a commit only after a CI run passed on the
same content. An accepted design is not amended. This record replaces that rule for a release of an
earlier compatibility line.

In v0, a minor version of the kernel can break its API, as Semantic Versioning allows for major
version zero. The changesets of the kernel mark each break with `**Breaking:**` and bump the minor.
Such a break can add a fact that every backend has to take a stance on, or change the output that
the golden files of a satellite contain. A released satellite does not know the new kernel, so its
tests fail against it. The next release of the satellite requires the new kernel, and the release
of the new kernel publishes the satellites with it.

The CI run of commit d2e50537 failed 8 of the 13 published releases this way. Each of them requires
the kernel at v0.2.0, and the pending changesets of the checkout release the kernel at v0.3.0. Their
tests failed in the coverage suite, in the render and in the golden files:

- The coverage suite of a backend requires a stance on every fact, and the kernel states the new
  facts `Wire`, `Receives` and `Passing`.
- The render warns for a fact without a spelling, such as `Methods` on a struct.
- The golden files of a release do not match the output of the new kernel. It writes a blank line
  before the provenance trailer and orders the declarations of a file by their position in the
  source.

No run can pass this rule while the kernel breaks the released satellites. The release that brings
the satellites up to the kernel waits on a passing run.

## Decision

We will report a published release whose go.mod requires the kernel or the SDK facade on an earlier
compatibility line than the version that the checkout releases, and not test it, because such a
release cannot know the API of the new line and its module releases on the new line with the
kernel.

- The compatibility line of a version in v0 is its major and its minor, such as v0.3. The line of a
  later version is its major, such as v2.
- The version that the checkout releases of the kernel or the facade is the version of its replace
  directive in `go.work`, raised by the highest bump of the module in the pending changesets. A
  release commit writes the new versions into `go.work` and consumes the changesets.
- The ring reads the requirements of a release from the go.mod at its tag.
- It writes `earlier <module>@<version>: <reason>` for such a release. The reason contains the
  requirement and the version that the checkout releases. The line does not fail the run.
- A module without a versioned replace directive in `go.work` does not make a release earlier.
- The ring tests every other release as before. A change of the kernel that breaks a release of the
  same line still fails the run.

## Alternatives Considered

### Fail every release whose tests fail

This is the rule of the accepted design. A kernel that breaks the satellites fails every released
satellite, and the release that brings them up to the kernel waits on a passing run. The CI run of
commit d2e50537 failed this way, and no later run could pass before the release.

### Test a release against the kernel that it requires

Each release could run its tests against the kernel of its own go.mod. The CI run of the release
tested that combination at the commit of its tag already. The ring exists to test a release
against a newer kernel.

### Excuse a failure that a breaking changeset announces

A pending changeset of the kernel that contains `**Breaking:**` could excuse each failure of the
ring. The release tooling computes the next version from the bump of a
changeset and not from its text. The line of the next version follows from the same bump, so the
ring and the release agree on which change breaks.

### Test a release of an earlier line and report the result

The ring job could test such a release and write the result without failing the run. A failure of
such a release is the expected result of a break, and it does not block a commit. A pass
does not change what the release of the new line publishes. Each such release costs a run of
`go test` in the ring job.

### Remove the ring until v1

Major version zero has no promise of compatibility, so the ring could wait for v1. A patch of the
kernel would then publish without a test against the released satellites. The changesets of the
kernel bump a break as a minor, so a patch keeps the API of its line, and the ring tests that.

## Consequences

**Positive:**

- A change of the kernel that breaks the satellites in v0 publishes after a passing run, together
  with the next releases of the satellites.
- The ring still tests every release on the line of the checkout. A patch of the kernel that breaks
  a release fails the run.
- Over the checkout of commit d2e50537, the ring reports the 13 published releases as earlier and
  exits with status 0.

**Negative:**

- In v0 every minor version of the kernel moves the line. From the first pending changeset that
  bumps the minor of the kernel until its release, the ring tests no release.
- Go selects the highest version that the modules of a build require. A consumer can build a
  release of an earlier line against the new kernel, and the ring does not report that such a build
  fails.
- The ring reads the changesets of the checkout. A changeset of the kernel with the wrong bump moves
  the line to the wrong version.

**Neutral:**

- The ring runs `git show` for the go.mod of each release before its test.

## References

| What | Where |
|---|---|
| RFC-0026, whose rule that a failing module fails the run this record replaces for a release of an earlier line | ../rfc/0026-the-reference-ensemble-and-the-canary-ring.md |
| RFC-0066, the test of a release from the tree of its tag | ../rfc/0066-the-ring-tests-a-release-from-its-tag.md |
| D194, the row of this decision | ../architecture/21-decisions.md |
| The CI run of commit d2e50537 | <https://github.com/dokimasia/eidos/actions/runs/39410772591> |
| Semantic Versioning 2.0.0, item 4 of the specification | <https://semver.org/#semantic-versioning-specification-semver> |
| Go Modules Reference, minimal version selection | <https://go.dev/ref/mod#minimal-version-selection> |
