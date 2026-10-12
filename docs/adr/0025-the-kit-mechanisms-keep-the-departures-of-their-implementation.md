---
adr: 0025
title: The kit mechanisms keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0033
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0025: The kit mechanisms keep the departures of their implementation

## Status

Accepted

## Context

The accepted design gives each mechanism that the test kits repeat one implementation in the kit of
its concern. The mechanisms are the position of a finding, the rerun check, the workspace fixture,
the reader and the matcher of golden files, the bridge from a run to the toolchain kit, the claim of
a load stamp, the plugin fixture from source, and the application of the config file of a brand. The
design states two invariants that apply here:

1. No check that the design changes accepts what it refused before. Each unified check is at least
   as strict as every check that it replaces.
2. Every kit runs its loads, its runs and its reads of a record under the context of the test.

The implementation met five places where the code states another contract than the design. Each
place is listed here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| `backendtest.Rendered` returns the findings of the settle and the render in report order | `Rendered` returns them in the order of `diag.Sink.All`. The sink groups the findings by origin and keeps the report order within one origin | The sink records no order across origins, because two schedulings of one parallel run interleave the origins in different ways. A test that compares the findings of two origins compares them as a permutation |
| `rulestest.Fixture.Findings` are the findings of the load in report order | `Findings` are in the order of `diag.Sink.All` | The reason is the same as for `Rendered` |
| `plugintest.Loaded` stops the test where the load or the validation reports an Error | `Loaded` fails the test once for each Error, with the code, the position and the message of the finding, and continues | One run reports every Error of a broken source tree. The registration of the keys, the stamps and the validation do not depend on a clean load |
| `workspacetest.Generated` stops the test where root has no record of the brand | `Generated` also stops the test where the record lists no file | An empty output passes every toolchain check, because the check has no file to compile |
| `cli.Configure` returns an error for a file that does not read or decode, and for a file that lists workspaces | `Configure` also returns an error for a composition without a valid brand, and for a plugin directory of the file that does not load | A composition without a valid brand has no name of a config file. `Configure` returns the error of `Document.Apply`, which reports a plugin directory that does not load |

## Alternatives Considered

### Build the design as written

Each contract would read as the design states it. It lost for each row for the reason in the row.
Report order across origins needs a sequence number for each finding in the sink, and that number
differs between two schedulings of one parallel run. A load that stops at its first Error reports one
Error for each run of a broken tree.

### Write a second design

A new design would state the contracts as built. It lost because the departures do not change a
contract between modules beyond the rows of the table, and the code states each of them in its
docblocks.

## Consequences

**Positive:**

- A test reads the findings of a kit in the order of the sink of a run.
- One run of a broken source tree reports every Error of the tree.

**Negative:**

- A reader of the design finds a report order that `Rendered` and `Findings` do not have.
- `Generated` and `Configure` stop the test or return an error on three inputs that the design does
  not list.

**Neutral:**

- The code keeps the first invariant. Each departure refuses more or reports more, and none accepts
  what the design refuses.

## References

| What | Where |
|---|---|
| RFC-0033, the shared mechanisms of the test kits | [0033-the-shared-mechanisms-of-the-test-kits.md](../rfc/0033-the-shared-mechanisms-of-the-test-kits.md) |
