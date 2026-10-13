---
adr: 0040
title: The reference ensemble keeps the departures of its implementation
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0060
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0040: The reference ensemble keeps the departures of its implementation

## Status

Accepted

## Context

RFC-0026 designs the reference ensemble and the canary ring, and lists its ADR as `tbd`. Its
implementation departs from it in five places, and the config example of RFC-0021 sets an option
that the ensemble does not have. RFC-0060 reviews the ensemble and keeps the code. The decisions
are recorded here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted designs. The departures are these.

| Design | Code | Reason |
|---|---|---|
| `Expr.Ref` is a `symbol.Identity` (RFC-0026) | `Expr.Ref` is a `*symbol.Identity` | The zero expression has no reference, as it has no value, and `Val` is a pointer for the same reason. A JSON encoder omits a nil pointer without a call of `Identity.IsZero` |
| mockgen writes `<I>Mock` into the family `mock`, and stubgen writes `<I>Stub` and a call record for each method into the family `stub` (RFC-0026) | Both families claim their files through a template of each target, and place one record for each double | The function field of each method has the function type of the method, and the scaffold has no neutral form of a function type, as RFC-0057 decides. The template of each target writes the fields and the bodies |
| enum reads the directive `text` on a constant (RFC-0026) | enum has no directive `text`. The text of a variant is its declared string, or its name without the name of the type | An enum variant is not a subject kind, so no rule of the authoring API runs on it |
| `cmd/eidos-reference` runs `acceptancetest.RunAcceptanceSuite` (RFC-0026) | It runs each check of the suite except `AssertPanic` | `AssertPanic` runs a command of the binary that panics, which a binary adds for the suite. The ensemble uses no test hook, as the invariants of RFC-0026 state |
| The tests of the ring program run it against a file proxy (RFC-0026) | They run it against an `httptest` server that serves the files of a fabricated proxy | One case needs the proxy to answer status 500 for one module, which a directory that the go command reads as a file proxy cannot answer |
| The config example sets `stubgen: {suffix: _stub}` (RFC-0021) | stubgen has no options | The plugins of the ensemble join the words that they declare as constants, as RFC-0060 decides |

## Alternatives Considered

### The designs as written

eidos could implement each design as RFC-0026 and RFC-0021 state it. An enum variant would become a
subject kind of the schema, with a Match type and a rule constructor. The binary would contain a
command that panics. The doubles would need a neutral function type in the scaffold. Each change
widens a contract beyond what the ensemble reads.

## Consequences

- The text of an enum variant follows the declaration of the variant alone.
- The acceptance check of a panic runs over the fixture binaries of eidos-cli, and not over the
  reference binary.
- RFC-0026 has its ADR in this record.
