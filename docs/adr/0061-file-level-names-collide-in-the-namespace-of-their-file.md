---
adr: 0061
title: File-level names collide in the namespace of their routed file
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0070
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0061: File-level names collide in the namespace of their routed file

## Status

Accepted

## Context

The settle checked whether two file-level names collide in the package that a unit derives from.
The layout routes the units after the settle, and a target reads every name in the package of its
routed file. A unit and its routed file can have different packages:

- The Go packages `svc/a` and `svc/b` each declare `type User struct`, and a GraphQL plan wrote
  `type User` twice into its one namespace without a finding. graphql-js refuses such a schema.
- One Go package that declares `user_record` in one file and `userRecord` in another refused a
  TypeScript plan with a module for each file, although no module declares both names.

The package of a file is not always the namespace of its names. A protobuf file is a package of its
own for its imports, and the files of one protobuf package declare their names in one namespace.

## Decision

We will check the file-level names of a plan in the layout, in the namespace of each routed file,
because the layout is the first step that knows the routed package of every declaration.

- The layout derives the namespace of each file beside its package, through the optional role
  `plugin.NameScoper` of the target, and records it in `plugin.File.Scope`. A target without the
  role uses the package. The protobuf backend returns the protobuf package of the file.
- The layout reports `CollidingNames` for two file-level names of one namespace and receiver with one
  settled name, and keeps every declaration. The Error fails the plan.
- A warm run compares the names of its routed files with the kept names of each namespace, which the
  sealed state records under the namespace of their file.
- The settle checks the members of each host, because a member collides in its host wherever the
  host is routed.

## Alternatives Considered

### The routed package before the settle

The path of a routed file can depend on a settled name, and the package rule of a target reads the
path. So the layout cannot know the routed package of a unit before the settle names its
declarations.

### Both checks

The settle would still refuse the names of one source package that collide in no file. A collision
whose two scopes agree would report twice.

### A check in each backend

The rule is the same in every target. Six backends would each contain a copy of it.

### An edge of each routed scope

A render reads the names of its own file alone, and the layout reads the kept names without a
render. In GraphQL every file of a plan is in one namespace. So the edge would render the whole plan
again for each new name.

## Consequences

**Positive:**

- A plan that writes one name twice into one namespace fails with a positioned finding before a
  toolchain refuses the file.
- A plan that writes valid files is not refused for a collision in a source package.
- The files of one protobuf package check their names in one namespace, as protoc resolves them.

**Negative:**

- The settle no longer reverts a respelled collision. The render writes the settled names, and the
  plan commits nothing.
- The sealed state lists each file-level name a second time, under the namespace of its file, and
  each artifact records its namespace. The format of the sealed state changes.
- A type outside the kernel that implements `plugin.Names` adds `InFileScope`.

**Neutral:**

- A file without a package that the target derives has no namespace, so its names join no check.

## References

| What | Where |
|---|---|
| RFC-0070, the design of this decision | ../rfc/0070-names-collide-in-the-routed-package.md |
| D173, the row of this decision | ../architecture/21-decisions.md |
| graphql-js 17.0.2, the rule that refuses two types of one name | <https://github.com/graphql/graphql-js/blob/v17.0.2/src/validation/rules/UniqueTypeNamesRule.ts> |
