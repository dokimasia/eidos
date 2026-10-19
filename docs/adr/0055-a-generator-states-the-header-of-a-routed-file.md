---
adr: 0055
title: A generator states the header of a routed file
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0067
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0055: A generator states the header of a routed file

## Status

Accepted

## Context

A routed file has the identity and the annotations of the one source file that it derives from, and
a directive that a plugin places on a generated file was out. The decision named its own trigger: a
generator that writes a file-level directive of its own.

Two targets have such directives:

- A protobuf file states its syntax, its package, its imports and its options. A generator over Go
  sources has no protobuf source file whose annotations the layout could copy.
- A GraphQL file states the schema statement of its schema, with its directives and its root types.

The emit model has the kind `File`, whose annotations are the directives of a file. The layout and
the backends do not read it.

## Decision

We will take every `emit.File` of the units of a routed file as the header of that file, because the
statements of a protobuf file and the schema statement of GraphQL are file-level directives that a
generator states.

- `layout.Route` collects the documentation and the annotations of each `emit.File` of the units of
  a routed file into `plugin.File.Header`, in unit order. An annotation or a documentation that two
  of them state alike appears once.
- `plugin.Placement` gains the annotations of the header, so a package rule can read a package that
  the header states.
- The render passes the header to the file skeleton, and the builtin `header` returns it to every
  template of the file. The declaration run skips an `emit.File`.
- The coverage verdict of the annotations of a file applies to the header. Go, TypeScript, Java and
  Rust refuse them, because their skeletons do not write a header.

## Alternatives Considered

### File-level statements from the source file alone

The protobuf frontend could record its file-level statements as annotations of the file, and the
layout would pass them to a mirror. A generator over Go sources has no protobuf source, and a file
of a package or of a plan has no source file at all.

### A header for each plan

A policy could state one syntax and one set of options for every file of a plan. A mirror of a tree
that mixes proto2 and proto3 files would then rewrite every file into one version, and a plan could
not state a package for one file.

## Consequences

**Positive:**

- A generator states the syntax, the package, the imports and the options of a protobuf file, and
  the schema statement of a GraphQL file.
- A package rule reads a package that a header states.

**Negative:**

- The layout and the render change, and every target declares a verdict for the annotations of a
  file.
- A header annotation in the Go, TypeScript, Java or Rust target reports `RefusedFact`.

**Neutral:**

- A file whose units contain an `emit.File` and nothing else has no declaration to render.

## References

| What | Where |
|---|---|
| RFC-0067, the design of this decision | ../rfc/0067-the-protobuf-backend.md |
| RFC-0058, whose verdict on a directive of a plugin this record reverses | ../rfc/0058-the-build-constraint-of-a-generated-file.md |
| D165, the row of this decision, which supersedes D149 | ../architecture/21-decisions.md |
