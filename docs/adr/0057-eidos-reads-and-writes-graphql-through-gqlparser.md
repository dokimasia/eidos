---
adr: 0057
title: eidos reads and writes GraphQL through gqlparser
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

# ADR-0057: eidos reads and writes GraphQL through gqlparser

## Status

Accepted

## Context

The satellites of eidos did not read or write GraphQL. A generator could not read a schema to write
the server types of an API, could not read the operations of an application to write a typed client,
and could not write a schema or an operation from the declarations of another language.

tree-sitter is the parser platform of the frontends, with two pure-Go exceptions: the standard
library's parser for Go and protocompile for protobuf. `github.com/vektah/gqlparser/v2` is the parser
of gqlgen. It is pure Go and keeps the comments of a document with their positions. It released
v2.5.61, v2.5.62 and v2.5.63 between 2026-10-07 and 2026-10-14. Its validator gives each selected
field the definition of the schema field that it selects.
It refuses three forms of the September 2025 edition: `implements` on an interface extension, a
braced Unicode escape, and the description of an operation, a fragment or a variable.

## Decision

We will add the satellite `eidos-lang-graphql`, which reads and writes every construct of GraphQL in
the September 2025 edition through gqlparser, because gqlparser is a pure-Go parser that keeps
comments with their positions and types selections. The satellite contributes the three missing
forms upstream.

- An object type loads as a struct, a field with arguments as a method, an interface as an
  interface, a union as an alias of a union, a custom scalar as a defined alias without a target,
  and a directive definition as an interface.
- An operation loads as a function whose result is the struct of its selection, named
  `<Operation><Kind>`. A fragment loads as a function whose result is `<Fragment>Fragment`. A
  selection on an interface or a union whose fragments select fields of particular types is a sum.
- A keyword that the model has no field for is a marker, an annotation whose name starts with
  `graphql:`. The schema statement is the header of a file. An operation and a fragment write their
  selection sets from the verbatim body of a function.
- A document that mixes both languages parses from two copies of the file, each with the
  definitions of the other language blanked out.
- Policies choose between a built-in type and a scalar of scalars.graphql.org for the shapes that
  GraphQL has no built-in type for.

## Alternatives Considered

### A tree-sitter grammar

`bkegley/tree-sitter-graphql` last changed in 2021, and `joowani/tree-sitter-graphql` has one star.
Both need the cgo of tree-sitter's Go bindings.

### graphql-go/graphql and graph-gophers/graphql-go

The first rejected an interface that implements an interface, a repeatable directive and three kinds
of extension in a probe. The second reads a `#` comment as a description by default and rejected the
extension of a scalar.

### wundergraph/graphql-go-tools

It accepted every form of the probes and requires 36 modules directly, among them gqlgen, gqlparser
itself and gRPC.

### A selection set as an inline type

The canonical shape of an inline type has no field names, and every spoke refuses it. A generator into
Go or TypeScript could not spell the result of an operation.

## Consequences

**Positive:**

- A generator reads a schema and its operations, and writes both, from any language of the hub.
- The support matrix has a GraphQL column.

**Negative:**

- A document that uses one of the three missing forms does not parse until a release of gqlparser
  contains the fixes.
- A selection set that a generator writes is text, which only the toolchain adapter checks against
  the schema.
- An anonymous operation takes the base name of its file, and two of them in files of one name
  collide.

**Neutral:**

- The executable definitions of a unit load into a package of their own, so the result types of the
  operations do not collide with the types of the schema.

## References

| What | Where |
|---|---|
| RFC-0068, the design of this decision | ../rfc/0068-the-graphql-satellite.md |
| D167, the row of this decision | ../architecture/21-decisions.md |
| GraphQL, September 2025 edition | <https://spec.graphql.org/September2025/> |
| gqlparser | <https://github.com/vektah/gqlparser> |
