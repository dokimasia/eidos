---
adr: 0027
title: The renames keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0035
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0027: The renames keep the departures of their implementation

## Status

Accepted

## Context

The accepted design gives one name to each concept of the kernel, the Go satellite and the command
line. The position of a plugin in the schedule is a turn, the kinds of execution that `explain` lists
are execution kinds, the field of the JSON events that contains the member of a list is `member`,
and the bindings of a Go file are `golang.Bindings`.

The implementation met places where the code needs another name or one more rename than the design
states. Each place is listed here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| The unexported type `bucket` of the layout router becomes `destination` | The type is `batch`, and the field of the router that lists them is `batches` | The router already declares `destination` for the place where one declaration is written. A batch is the declarations of one destination |
| `golang.Bindings` and `NewBindings` replace `Scope` and `NewScope` | The function of the Go rules that derives the bindings of a file is `bindingsOf`, and the field of the frontend's lowering that keeps them is `bindings` | Both held a value of the renamed type under the old word, and the Go rules met `rules.Scope` and `scopeOf` in one expression |
| The sentences that use bucket for a turn change | The docblocks and the rendering document that called the shared helpers of a funcmap an overrideable bucket name the helpers and the override instead. A local variable of the load that grouped identities by name is `ids` | Bucket keeps one meaning in the kernel, the hash byte of a manifest document |
| The facade is generated again | The template of the model generator that writes a test of the matches also writes `Turn` | The generated test sets the turn of a phase context, and the mirror guard compares it with the template |

## Alternatives Considered

### Name the router's type destination as the design states

The router would need a new name for its existing `destination` type. Its docblocks, its key type
`destKey` and its methods use that name. A batch is the content of a destination, so the word needs
no second rename.

### Leave the old words in the unexported names

The Go rules would keep `scopeOf` beside `rules.Scope`, which is the confusion that the design
removes. An unexported name is read by the next author of the package as much as an exported one.

## Consequences

**Positive:**

- The word bucket has one meaning in the kernel, and Scope has one meaning in the Go rules.

**Negative:**

- A reader of the design finds a router type named destination that the code names batch.

**Neutral:**

- The renames do not change a stored byte or an exported name beyond the design.

## References

| What | Where |
|---|---|
| RFC-0035, one name for each concept | [0035-one-name-for-each-concept.md](../rfc/0035-one-name-for-each-concept.md) |
