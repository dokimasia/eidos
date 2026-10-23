---
adr: 0103
title: A record keeps the data of the call that places it
status: Accepted
date: 2026-10-23
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0103: A record keeps the data of the call that places it

## Status

Accepted

## Context

A family that claims its files places records through `Out.Record(data, decls...)`, and its file
template writes each file from the records and their data. The rendering guide and ADR-0019 state
that a record has the data of its match.

- `plugin.Unit.Data` was a map from an origin to its data. The flush attached the data of a call to
  the origin of each declaration. Where two calls attached data to one origin of a unit, the data
  of the first call in canonical match order applied.
- The render read the data of each record by the origin of the record. Every record of one origin
  rendered with the data of the first call.
- Issue 76 reports a generator of conformance suites that writes one test for each instance of the
  repeatable directive `subject` on a struct. Each test is a record with the origin of the struct,
  because the layout places the test beside the struct. The unit had the test of the first
  interface twice and no test of the second.
- The settle passes each declaration through the lowering hook of the target. The hook can replace
  a record with declarations of the same origin. The Go backend replaces an enum with a defined
  type and its constants.
- The router routes the declarations of a unit one at a time and drops each declaration that a
  finding refused. It then merges the units of each file. The split of a target copies the unit
  into each part, as the split of an internal test file of Go does.
- The kernel already keys the unit of each declaration, the references of a unit and its emitted
  names by the declaration.

## Decision

We will key the data of a unit by the record that it belongs to.

- `plugin.Unit.Data` is a map from a record to the data of the `Out.Record` call that placed it.
- Records of one origin each keep the data of their own call. A declaration that two calls place
  keeps the data of the first call in canonical match order.
- The declarations that replace a record in the lowering take the data of the record. The settle
  moves the data in a copy of the map of the unit, because the caller can hold the map beside the
  store.
- The router merges the data of the units of a file without a precedence rule, because a record
  belongs to one unit.
- The render reads the data of each record by the record. The view of a file template does not
  change.

## Alternatives Considered

### A list of data beside the declarations of a unit

A list with the data of `Decls[i]` at `Data[i]` would need every step that moves declarations to
move the list in step. The lowering, the router, the drop of refused declarations and the split of
each target move declarations. A target whose split leaves the list out would give each later
record the data of another record.

### A key of the origin and the name of a record

A lowering keeps the origin of a record and can change its name. The name of a record can also
repeat in a unit, as an init function repeats in each file of a package.

### A field for the data on each emit declaration

The symbol schema generates the emit model of every backend, so a field of type any would enter
each of them and the settle.

## Consequences

**Positive:**

- The generator of issue 76 writes one test for each instance of `subject`, each with the
  references of its own suite.
- A record has the data of its match, as the rendering guide and ADR-0019 state.

**Negative:**

- The type of `plugin.Unit.Data` changes. A consumer that reads the data of a unit by an origin,
  such as the spec of a plugin, reads it by the record.
- Each step that replaces a record must pass the data of the record on. The lowering of the settle
  is the one such step.
- A unit in which the lowering replaces a record with data allocates a copy of its data.

## References

| What | Where |
|---|---|
| D221, the row of this decision | ../architecture/21-decisions.md |
| ADR-0019, records and the file template of a family | 0019-a-family-claims-its-files-through-a-template.md |
| ADR-0092, the instances of a directive that a handler reads | 0092-a-handler-reads-the-instances-of-its-directives.md |
| The records of a claimed file | ../architecture/07-rendering.md |
| Issue 76, two records of one origin in a unit read the data of the first | <https://github.com/dokimasia/eidos/issues/76> |
