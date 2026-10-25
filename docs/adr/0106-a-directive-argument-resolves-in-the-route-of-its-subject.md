---
adr: 0106
title: A directive argument resolves in the route of the output of its subject
status: Accepted
date: 2026-10-25
supersedes: ADR-0102
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0106: A directive argument resolves in the route of the output of its subject

## Status

Accepted

## Context

A route moves the output of a plugin for a declaration into another directory: the reserved key
`out=` of a directive of the plugin, the kernel directive `out` of the declaration, or the kernel
directive `out` of its package. The layout applies the routes after validation, from the validated
directives.

- The Go rules resolve a bare name of a directive argument in the package of the subject, in the
  dot imports of its file, and then in the external test package of the directory of the subject.
  ADR-0102 decided that order.
- The suite generator of dokimi writes the conformance test of a struct under `out` into the
  external test package of the `out` directory, and that test calls the factory of the directive
  `subject`. Issue 81 reports that a factory of that external test package did not resolve by its
  bare name, with EID-0040. The full import path of the factory resolved.
- The resolver of validation received the subject, the spelling and the resolution kind. It did not
  know the plugin of the directive, so it could not read the route of the output of that plugin.

## Decision

We will give the scope of a resolution the directory of the route of the subject, and resolve a
bare name of Go in the external test package of that directory as well.

- `directive.Resolver` takes the plugin that registered the schema of the directive, and the empty
  plugin for a schema of the kernel. Validation passes it for each reference param.
- `rules.Scope` has `Route`, the workspace-relative directory into which the output of that plugin
  moves for the subject, and the empty string where no route moves it.
- `layout.RouteDir` reads the route from the raw directives of the subject and of its package, with
  the precedence of the layout. The reserved key `out=` of a directive of the plugin takes precedence
  over the kernel directive `out` of the subject, which takes precedence over the kernel directive
  `out` of the package. A path that ends in a file name routes into the directory of that file. The
  resolver of the workspace and the resolver of the plugin test kit both set the route.
- The Go rules resolve a bare name in this order:
  - the package of the subject and the dot imports of its file
  - the external test package of the directory of the subject
  - the external test package of the directory of the route
- That last package has the path of the package of the subject's directory joined with the route
  below the directory of the subject's file. Its path ends in the suffix `_test`. The directories of
  one module mirror its import paths, so the joined path is the import path of the route.
- This decision supersedes the order of a bare name of ADR-0102. The scope of the package of
  ADR-0102 is unchanged.

## Alternatives Considered

### The route from the validated directives

Validation computes the validated directives that the layout reads. The resolver runs inside
validation and reads the raw directives of the subject and of its package instead.

### The external test package of the route first

A factory in the external test package of the directory of the subject would no longer resolve under
a route, also where the plugin writes code into that directory. The order of ADR-0102 keeps every
name that resolved before.

### A route for every plugin of the subject

The resolver would try the route of each plugin with a directive on the subject. A name could then
resolve in the route of a plugin whose directive does not read it.

## Consequences

**Positive:**

- The tree of issue 81 validates without EID-0040, and the factory `newBox` resolves in the external
  test package of `shapetest`.
- A warm run validates a subject again after the directives of its package change, because the scope
  of the subject reads its package.

**Negative:**

- A route into another module joins a path that no package of the view has. A name of that module
  does not resolve, as before.
- The scope reads the routes of directives alone. Validation runs once for every plan, so the
  routing of the configuration of one plan, such as its centralised policy, is not part of it.
- A factory in the external test package of the directory of the subject still resolves under a
  route. The generated code in the directory of the route cannot import that package. A plugin checks
  the reference with `Bound.Importable` against the package of its file.
- The suite generator of dokimi compares the package of a factory with the external test package of
  the directory of the struct. It refuses the factory of the tree of issue 81 with its own finding
  until it compares with the package of the file of its test.
- `directive.Resolver` takes a fourth parameter, so a resolver outside the kernel changes its
  signature.

## References

| What | Where |
|---|---|
| D224, the row of this decision | ../architecture/21-decisions.md |
| ADR-0102, the scope of a directive argument of Go | 0102-the-go-rules-resolve-a-directive-argument-in-the-scope-of-its-package.md |
| ADR-0105, the import rule of the target | 0105-the-layout-refuses-a-declaration-whose-file-cannot-import-a-reference.md |
| The routing and the layout | ../architecture/18-routing-and-layout.md |
| The Go satellite | ../architecture/11-languages.md |
| Issue 81, a directive under out cannot refer to a function of the external test package of the out directory | <https://github.com/dokimasia/eidos/issues/81> |
