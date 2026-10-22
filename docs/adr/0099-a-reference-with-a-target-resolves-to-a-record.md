---
adr: 0099
title: A reference with a target resolves to a record
status: Accepted
date: 2026-10-22
supersedes: ADR-0019
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0099: A reference with a target resolves to a record

## Status

Accepted

## Context

A family that claims its files writes each of its files from records through a file template. The
check of colliding names, the name tables and the exports left records out, and no reference
resolved to a record. Records of one name can repeat in a package, such as an init function in each
file. The check of colliding names would refuse such records.

A cold run and a warm run did not apply that rule in the same way.

- The layout of a cold run finds the referent of a translated or an emitted reference among the
  declarations of every unit that it routes, records included. A cold run qualified a reference to
  a record by the package of the file of the record.
- A warm run that keeps the file of a record finds the referent in the name table of the kept files.
  `plugin.NamesOf` did not list records, so the warm run refused the referencing declaration under
  `UnemittedReference`, EID-0081.
- In the reproduction of issue 69, a test file calls `RunThingSuite` of a suite file through an
  emitted reference. An edit of the doc comment of the tested interface failed the next run, and a
  cold run of the same tree passed. A generator of conformance suites for 132 implementations needed
  a cold run after each such edit.
- A warm run that stopped emitting a record kept the files that referred to it, because no recorded
  name changed. A cold run refused those references.
- The layout of a cold run qualified a bare reference to the name of a record in another package. A
  warm run that kept the file of the record left the reference bare.

## Decision

We will let a reference with a target resolve to a record in a cold and in a warm run, because such
a reference has the origin and the name of its referent. Records stay out of the check of colliding
names, out of bare references and out of the exports, because the name of a record can repeat.

- `plugin.NamesOf` lists the name of each record with `NameEntry.Record` set.
- `plugin.Names.InScope` lists the records of a scope. `InPackage`, `OfOrigin` and `InFileScope` do
  not list records.
- The state keeps one row for each record, under its source scope. The origin and the path of the
  file key the row apart from the rows of other records of the same name. The state format is 8.
- The layout finds the referent of a translated or an emitted reference among the records of its
  units and of the kept files. It leaves a bare reference to the name of a record as written.
- The check of colliding names and the receiver of a method leave records out. A group reads no
  scope edge for the name of a record.
- The settle rewrites no reference to the name of a record.

## Alternatives Considered

### Cold runs refuse a reference to a record

The layout would leave records out of its referents in a cold run as well. A generator of suites
would then write each suite function in a family without a template, and the template of a suite
would lose the data of its records. The use case of issue 69 writes the suite of each interface
through a template and calls it from the test of each implementation.

### Records as ordinary names of the name table

`plugin.NamesOf` would list records without a mark. A warm run would then check records for
collisions and resolve bare references to them, and a cold run does neither. A package with an init
function in each of its files would then fail a warm run and pass a cold run.

### A method of `plugin.Names` for the records of a scope

A fifth method would return the records of a scope apart from its other names. The layout reads the
referent of a reference with a target through `InScope` already. A mark on each entry keeps one
sorted list for each scope and leaves the interface at four methods.

## Consequences

**Positive:**

- In the reproduction of issue 69, the warm run after the edit qualifies `RunThingSuite` by the
  package of the kept suite file. It writes the same test as a cold run, and the test calls
  `modeltest.RunThingSuite()`.
- A warm run refuses a reference into a record that the run no longer emits, as a cold run does.
- A bare reference to the name of a record has one result in a cold and in a warm run.

**Negative:**

- A generation of format 7 does not open, so the first run after the update is cold.
- The name table of a plan has one row for each record.
- A cold run leaves a bare reference to the name of a record in another package as written. A
  generator refers to a record through an emitted reference.
- The settle does not follow a reference to a record. A reference resolves to a record only where
  the settled name of the record is the spelling of the reference.

## References

| What | Where |
|---|---|
| D217, the row of this decision, which supersedes D115 | ../architecture/21-decisions.md |
| ADR-0019, the rule for records that this decision replaces | 0019-a-family-claims-its-files-through-a-template.md |
| ADR-0088, the emitted reference | 0088-the-layout-qualifies-an-emitted-reference.md |
| The references that the layout qualifies | ../architecture/18-routing-and-layout.md |
| Issue 69, a warm run refuses an emitted reference to a record | <https://github.com/dokimasia/eidos/issues/69> |
