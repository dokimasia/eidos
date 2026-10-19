---
adr: 0058
title: A GraphQL unit is a schema and its documents
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0068
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0058: A GraphQL unit is a schema and its documents

## Status

Accepted

## Context

A GraphQL schema spans files and often directories. gqlgen reads every file that the globs of its
`gqlgen.yml` match, such as `graph/*.graphqls`. A graphql-config project lists the globs of its schema
and of its documents in the same way. An extension merges into its type only where one parse sees
both, and a selection takes its type only from the schema that the parse has.

## Decision

We will make one unit of each schema that a graphql-config project or a gqlgen configuration
defines, together with its documents, and one unit of the GraphQL files of a directory outside every
such schema, because a schema spans directories in the layouts of gqlgen and graphql-tools.

- The partition reads the configurations of the directory of each selected file and of each
  directory above it. A directory has the first configuration of graphql-config in the order of its
  search of 26 names, and the first of `.gqlgen.yml`, `gqlgen.yml` and `gqlgen.yaml`.
- A schema claims the files that the entries of its keys `schema` and `documents` match, and a
  project of graphql-config also claims the files that its key `include` matches and its key
  `exclude` does not.
- A file joins the unit of the first schema that claims it. Every later schema that claims the file
  reads the package of that unit as a shared package.
- The package of a schema is the directory of its configuration, and `<directory>/<project>` for a
  project. The executable definitions of a unit load under its package followed by `/documents`.
- Every configuration that the frontend reads is a shared input of every unit.
- A configuration in TypeScript or JavaScript is code, which the frontend does not run. Such a
  configuration, a configuration that does not decode, and a configuration of gqlgen with an entry
  that gqlgen refuses define no schema. The parse of one unit reports each under `UnreadConfig`.

## Alternatives Considered

### A unit of one directory

Each directory could be one schema, as each directory of Go is one package. A type and its extension
in two directories would then load in two units, and the extension would lose its type.

### One package for the whole workspace

Every GraphQL file of a workspace could share one namespace. Two schemas of one workspace, such as two
subgraphs of a federated graph, would then collide on their root types.

## Consequences

**Positive:**

- The layouts of gqlgen and graphql-tools load as one schema, and the documents of a project take
  their types from its schema.

**Negative:**

- The partition reads up to 29 configuration names in each directory above a selected file, and every
  read folds into the key of every unit of the partition. An edit of one configuration changes the
  key of every unit.
- The satellite depends on a TOML parser for the two TOML names of graphql-config, and on doublestar
  for the globs of graphql-config.
- A project whose schema is a URL does not type its documents, because the load reads the workspace
  alone.

**Neutral:**

- An entry of `documents` that matches the source files of another language does not select a file
  of the GraphQL frontend.

## References

| What | Where |
|---|---|
| RFC-0068, the design of this decision | ../rfc/0068-the-graphql-satellite.md |
| D168, the row of this decision | ../architecture/21-decisions.md |
| graphql-config | <https://the-guild.dev/graphql/config/docs/user/usage> |
| gqlgen configuration | <https://gqlgen.com/config/> |
