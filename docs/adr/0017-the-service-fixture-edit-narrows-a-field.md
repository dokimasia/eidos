---
adr: 0017
title: The service fixture's edit narrows a field
status: Accepted
date: 2026-10-10
supersedes: none
superseded-by: none
rfc: RFC-0024
---

# ADR-0017: The service fixture's edit narrows a field

## Status

Accepted

## Context

The workspace kit checks the warm path of a fixture with one edit of the fixture's source tree.
The kit applies the edit between a cold run and a warm run, and compares the warm run with a cold
run over the edited tree. The kit states a contract for the edit:

- The edit changes one declaration that the first plan generates from.
- The first plan's output changes.
- The first plan's export does not change.

The check of the export cutoff depends on the last part. A plan that depends on the first plan
reads its export, and after the edit that plan must not run any invocation again.

The export of a plan has an entry for every declaration of a rendered file, members included. An
entry records the declaration's names, kind, spelling, package and file. It does not record the
declaration's type.

The design specified a warm suite whose edit adds a field to the schema. An edit that added the
field `parts` to the message `Summary` failed the check of the export cutoff in all five versions.
The export of `go-server` gained an entry for the new field, and the dependent plan ran its
invocation again.

## Decision

We will make the service fixture's edit narrow the field `size` of `Summary` from `int64` to
`int32`, because the kit requires an edit that changes the first plan's output and not its export,
and an export entry does not record a type.

- The Go struct `Summary` changes the type of `Size` from `int64` to `int32`. The TypeScript
  interface changes the type of `size` from `bigint` to `number`.
- `EditService` replaces the text `int64 size = 1;` in the schema, and returns an error for a
  schema without that text.

## Alternatives Considered

### Add a field to `Summary`

The edit adds a field to the message `Summary`, as the design specified, so the warm run loads a
declaration that the cold run did not load.

It lost because the new field adds an entry to the export of `go-server`. The export changes, so
the check of the export cutoff fails in every version.

## Consequences

**Positive:**

- The warm suite and the workspace suite run over every version with the kit's contract
  unchanged.
- The check of the export cutoff covers the service fixture. A plan that depends on `go-server`
  does not run again after the edit.

**Negative:**

- No warm check of the service fixture runs an edit that adds or removes a declaration.
- The edit depends on the text `int64 size = 1;` in each version's schema. A change of that line
  makes the edit return an error, and every check that applies the edit fails.

**Neutral:**

- Each version's schema still declares `size` as `int64`, so the expected files do not change.

## References

| What | Where |
|---|---|
| D99, the export key and the entry that records no signature | ../architecture/21-decisions.md |
