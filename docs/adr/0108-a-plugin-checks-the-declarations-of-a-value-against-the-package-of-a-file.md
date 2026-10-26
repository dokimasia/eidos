---
adr: 0108
title: A plugin checks the declarations of a value against the package of a file
status: Accepted
date: 2026-10-26
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0108: A plugin checks the declarations of a value against the package of a file

## Status

Accepted

## Context

`Bound.Importable` reports whether the code of a package can refer to the referent of a type
reference, so a generator declines a declaration before it records it. A generator also records
values that a template writes, such as the starting value of a field and the errors that a generated
function returns.

- An `emit.Value` refers to declarations through its callee and its type, and through the fields, the
  keys, the arguments and the inner value that it contains.
- `rules.ImportRules` takes a `node.TypeRef`, and a value has neither form. The SDK had no check of a
  value.
- The Go backend checks the import of each declaration of a value when a template writes the value. A
  refusal fails the file template, so the render withholds every record of the file. Issue 85 reports
  that a default of an internal package withheld the suites of two interfaces of one file.

## Decision

We will let a plugin check a value before it records it, through an optional rule of its language.

- `rules.ValueImportRules` declares `ImportableValue(pkg, v)`, which returns the reason that the code
  of the package `pkg` cannot refer to a declaration of `v`, and nil where it can.
- `Bound.ImportableValue(pkg, v)` calls the rule, and returns nil for a language without it, as
  `Bound.Importable` does.
- The Go rules implement it through `golang.CheckReference`. They check a callee by its package and
  its name, and each type reference by the package that the Go backend imports for it. They check the
  fields, the keys, the arguments and the inner value the same way.
- The conformance matrix lists the rule.

## Alternatives Considered

### A second method of `rules.ImportRules`

A language that implements `ImportRules` would have to implement the check of a value too. A separate
optional rule leaves every implementation of `ImportRules` unchanged.

### A check of the name in the render of a callee

The Go backend would refuse an unexported callee of another package when a template writes it, as it
refuses the import. The bodies of the shared backend fixture call an unexported function of another
package, and the render would refuse them. A plugin that checks its values before it records them
does not reach the refusal of the render.

## Consequences

**Positive:**

- A generator leaves out a value that the file cannot refer to, and the file keeps its other records.
- The Go rules apply one rule to a type reference and to a value.

**Negative:**

- The render of a Go callee checks its import alone. A plugin that records an unexported callee of
  another package without the check writes a file that does not compile.

## References

| What | Where |
|---|---|
| D226, the row of this decision | ../architecture/21-decisions.md |
| ADR-0105, the import rule of the target in the layout | 0105-the-layout-refuses-a-declaration-whose-file-cannot-import-a-reference.md |
| The projection vocabulary | ../architecture/03-projection.md |
| Issue 85, a plugin cannot check the references of a value against the package of a file | <https://github.com/dokimasia/eidos/issues/85> |
