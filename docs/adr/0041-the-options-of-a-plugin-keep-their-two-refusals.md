---
adr: 0041
title: The options of a plugin keep their two refusals
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0061
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0041: The options of a plugin keep their two refusals

## Status

Accepted

## Context

RFC-0006 states that the exported fields of the options struct of a plugin are its options, and
that an untagged field takes its lowered name as its key. `plugin.ValidateOptions` refuses an
exported field without the tag `opt`, and refuses an unexported field. RFC-0061 decides to keep the
code. The decision is recorded here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departure is this.

| Design | Code | Reason |
|---|---|---|
| An untagged option field takes its lowered name as its key | `ValidateOptions` refuses an exported field without the tag `opt` | A key that the author did not write changes when the field is renamed, and every config file that sets the old key then fails |
| An unexported field of the options struct is no option | `ValidateOptions` refuses an unexported field | The composition cannot populate an unexported field, so a value that the author meant to set would never reach the plugin |

## Alternatives Considered

### The lowered name as the key

The workspace could read an untagged field under its lowered name. A rename of the field in the
code of the plugin would then rename the key of the config file without a change that a reviewer
of the config file sees.

## Consequences

- Every option field states its key in the tag `opt`.
- A plugin declares a field that is no option outside the options struct.
