---
adr: 0053
title: The ring tests a release from the tree of its tag
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0066
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0053: The ring tests a release from the tree of its tag

## Status

Accepted

## Context

The ring job of CI tests the latest published version of each module that requires the kernel
against the kernel and the SDK facade of the checkout. The accepted design of the ring wrote a
scratch module that required the version and replaced the kernel and the facade with the checkout.
It then ran `go test` over the packages of the version from the module cache. An accepted design is
not amended. This record replaces that step.

The CI run of commit cf24fc1c was the first run of the ring over releases. It failed four of twelve
modules:

- A module zip leaves out each directory that contains a go.mod. The fixtures of conformance and
  reference under testdata are such directories.
- The go.mod of a published module replaces its siblings with relative directories. A test that
  runs the go command in the module cache applies them. The module cache does not have the
  directories.
- A zip does not have a file outside its module. The tests of plugin/shape/tools read two files of
  its parent module.
- The requirements of the kernel of the checkout did not enter the build list, because the scratch
  module did not require the kernel.

## Decision

We will test each published module of the ring from the tree of its release tag, with its replace
directives swapped for the kernel and the SDK facade of the checkout, because that tree has the
fixtures and the files that the tests of a release read.

- The ring reads the tree with `git archive` of the tag `<dir>/<version>`. The tag of the module at
  the root is the version. git uses the line endings with which the go command archives a module
  version.
- It writes the directories and the regular files of the tree into the scratch directory of the
  module. Any other entry fails the module.
- In the go.mod of the module, it drops each replace directive and replaces the kernel and the
  facade with their directories in the checkout.
- `go test -mod=mod -count=1 ./...` runs in the directory of the module without a workspace.
- The ring job checks out the whole history with the tags.

## Alternatives Considered

### The scratch module with the kernel among its requirements

The scratch module could require the kernel and the facade. The requirements of the kernel would
then enter the build list. The zip would still not have a nested module or a file outside the
module. Its go.mod would still replace the siblings with directories that the module cache does not
have.

### Releases that a zip can test

The fixtures could keep their go.mod under another name. The go.mod of each module could leave its
replace directives to `go.work`. The release v0.1.0 would stay untestable, because both changes
apply to new releases alone. The tests of plugin/shape/tools would still read files of their
parent module.

### The workspace of the tag

The ring could keep the go.work of the tree with the kernel and the facade of the checkout. The
tests would build against the siblings at the tag. A consumer builds against the versions that the
module requires.

## Consequences

**Positive:**

- The tests of a release read the fixtures and the files that they read in the CI run of the
  release.
- The twelve releases of v0.1.0 pass the ring against the checkout of cf24fc1c.
- The requirements of the kernel of the checkout raise the dependencies of a release. A consumer
  that requires the newer kernel builds with the same versions.

**Negative:**

- The ring job fetches the whole history and every tag.
- Each release writes the whole repository at its tag into the scratch directory of the run.
- A checkout without the tag of a release fails the ring for that release.
- The ring does not test the zip that the proxy serves.

**Neutral:**

- The ring runs git in the root of the checkout.

## References

| What | Where |
|---|---|
| RFC-0066, the design of this decision | ../rfc/0066-the-ring-tests-a-release-from-its-tag.md |
| RFC-0026, whose scratch module this record replaces | ../rfc/0026-the-reference-ensemble-and-the-canary-ring.md |
| D163, the row of this decision | ../architecture/21-decisions.md |
| The CI run of commit cf24fc1c | <https://github.com/dokimasia/eidos/actions/runs/38446301335> |
