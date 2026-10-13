---
adr: 0038
title: The facade keeps the identifiers of its packages
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0059
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0038: The facade keeps the identifiers of its packages

## Status

Accepted

## Context

RFC-0006 estimated the root package of the authoring API at ten generated Match types and roughly
fifteen hand-written exported symbols, and the package `plugin` at another dozen. RFC-0012 makes
the SDK a generated facade that re-exports the plugin-facing packages of the kernel. RFC-0059 keeps
the facade as it is and decides against a curated list of its identifiers. The estimate of
RFC-0006 and the code differ, and the decision is recorded here, because an accepted design is not
amended.

## Decision

We keep the code as built where it departs from the accepted design. The departure is this.

| Design | Code | Reason |
|---|---|---|
| The root package has ten generated Match types and roughly fifteen hand-written symbols | The root package has 63 exported identifiers. 24 are generated: a Match type and its rule constructor for each of the twelve subject kinds of the schema, parameters and returns included. 39 are hand-written: the builder, the effects, the predicates, the wrappers, the template options, the readers and the writers of facts, the mirror helpers, the tag of tests and the codes of two refusals | Each subject kind of the schema has its Match type, and each identifier of the hand-written part is in an accepted design, such as the predicates `Not`, `NameEquals` and `HasSlot` of RFC-0041 |
| The package `plugin` has about a dozen identifiers for an author | `plugin` has 150 exported identifiers, which the facade re-exports whole. An annotator or a generator imports eleven packages of the facade, with 745 identifiers, and the facade has 979 in 23 packages | The facade re-exports a package whole, as RFC-0012 states, and the import lint keeps plugin code on the import paths of the facade. A list of identifiers would hide nothing, because the import paths of the kernel stay public |

## Alternatives Considered

### A curated list of identifiers

The generator of the facade could re-export the identifiers of a list. The kernel's import paths
would still export every identifier, so the list would narrow the documentation of the facade and
not what a plugin can import. Each change of the kernel would need a decision about the list.

## Consequences

- An author reads the documentation of eleven packages with 745 identifiers.
- The generator re-exports a new exported identifier of a re-exported package at its next run.
