---
adr: 0079
title: A handler marks the internal tests of an external family
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0078
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0079: A handler marks the internal tests of an external family

## Status

Accepted

## Context

A family that states `Output.External` wrote every Go test file into the external test package of
its directory. The go command compiles such a package apart, with access to the exported identifiers
of the package alone. A generator that checks an unexported declaration wrote a check that `go vet`
refused:

```text
vet: store/store_probe_test.go:21:17: name entryProbe not exported by package store
```

A template can write a reference as text, and the speller of a target qualifies a reference to a
source declaration at render, so no layout sees every reference that a test file makes.

## Decision

We will let a handler mark the declarations of an external family that test their package from
inside, and the Go target will write them into an internal test file of the family, because only the
generator knows which of its tests need the package under test.

- `Out.Internal` marks the declarations that a handle places. The flush lists their origins in
  `Unit.Inside`, in a unit of an external family alone.
- A routed unit keeps the `Inside` origins of its declarations, and a file with one is not external.
- `spell.Split` of the Go target writes the marked declarations into a part of their own, and
  `spell.Filename` names its test file with `internal` before the test suffix, as
  `store_probe_internal_test.go`.

## Alternatives Considered

### The visibility of the referents in the layout

The layout would keep a file in its package where the file refers to an unexported declaration. It
does not see a reference that a template writes, or a reference that the speller qualifies.

### A mark of the whole unit

The mark would make the whole file of a unit internal. One unexported subject would move every check
of a source file into the package under test, and an external check that imports a package which
imports the package under test would then form an import cycle.

### A second family with a tag that ends in `_test`

A plugin would declare a second family without External. The Go target spells a test file from the
tag `test`, and another target spells such a tag as a file that is no test.

## Consequences

**Positive:**

- A generator checks an unexported declaration, and the file compiles.
- The checks of exported declarations keep the external test package.

**Negative:**

- A source file with internal checks gives two Go test files where it gave one.
- The mark is per handle, so a check that needs the package under test and lacks the mark still
  fails to compile.

## References

| What | Where |
|---|---|
| RFC-0078, the design of this decision | ../rfc/0078-the-layout-of-a-test-fixture-and-internal-tests.md |
| D197, the row of this decision | ../architecture/21-decisions.md |
| Issue 45, a test of an unexported declaration | <https://github.com/dokimasia/eidos/issues/45> |
| `go help packages`, the external test package | <https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns> |
