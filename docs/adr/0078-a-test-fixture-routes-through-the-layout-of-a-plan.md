---
adr: 0078
title: A test fixture routes through the layout of a plan
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

# ADR-0078: A test fixture routes through the layout of a plan

## Status

Accepted

## Context

`backendtest.Rendered` routed the store of a fixture through `backendtest.Files`, which places each
file beside its source and gives the package rule no family. A test file of an external family kept
the package of its directory, an `out` directive moved no file, and a bare reference into another
package stayed bare. A spec that pinned the rendered tree of a plugin pinned other files than a
workspace writes for the same units.

`plugintest.Fixture` has what the layout reads: the graph, the facts, the validated directives and
the scope.

## Decision

We will route the store of a test fixture through the layout of a plan where the fixture states
the input of the layout, because only the layout applies the families, the routing directives and
the qualification of references that a workspace applies.

- `backendtest.Fixture.Layout` is a `layout.Input`. The context of a render sets its store, the
  halves of the backend, its target and its sink.
- `plugintest.Fixture.Layout` returns the input over a fixture: the index, the residents and the
  modules of the graph, the families of each plugin and the registry of their schemas.
- `backendtest.Fixture.Context` returns the error of the layout for a defect of its input.
- A fixture without a layout keeps the routing of `backendtest.Files`.

## Alternatives Considered

### Families in `backendtest.Files`

`Files` would read the families of the plugins and apply `External`. It would repeat the routing of
`out` directives and the qualification of references, which the layout implements.

## Consequences

**Positive:**

- A spec renders the files that a workspace writes, so it pins the package of an external family
  and a qualified reference.
- A plugin author tests the routing of a plugin without a workspace run.

**Negative:**

- `backendtest.Fixture.Context` returns an error, which breaks each caller outside the kernel.
- `plugintest` and `backendtest` depend on the layout, so a change of the layout input changes
  their API.

## References

| What | Where |
|---|---|
| RFC-0078, the design of this decision | ../rfc/0078-the-layout-of-a-test-fixture-and-internal-tests.md |
| D196, the row of this decision | ../architecture/21-decisions.md |
| Issue 46, the layout of a fixture | <https://github.com/dokimasia/eidos/issues/46> |
