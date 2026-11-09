---
adr: 0126
title: The names of the length of an array are children of its reference
status: Accepted
date: 2026-11-09
supersedes: none
superseded-by: none
rfc: RFC-0092
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0126: The names of the length of an array are children of its reference

## Status

Accepted

## Context

A Go array type can state its length through a constant, as `[sizes.SaltSize]byte` does. The Go
frontend recorded a literal length in `Length` and kept any other length in the spelling alone. The
element was the one child of an array. The shared walk of a backend binds the imports of the children
of a reference alone. A suite of dokimi in another package then wrote `[sizes.SaltSize]byte{1}`
without the import of `sizes`. It wrote `[SaltSize]byte{1}` without its qualifier, and the suite did
not compile. The rules walk the references of a type, so they admitted a length that referred to an
unexported constant of another package. The Rust frontend kept a length expression such as
`SALT_SIZE` in `[u8; SALT_SIZE]` in the spelling alone too.

## Decision

We will list the names of the length of an array as the children of its reference after its element,
each with the package of its declaration, because the shared walk of each backend and the rules
already visit the children of a reference.

- A Go name is an identifier or a qualified identifier of the length. A predeclared identifier is no
  name, and neither is the field or the method that a selector of a value selects, nor the key of an
  element of a composite literal.
- A Rust name is an identifier or a path of the length expression.
- The package of a name is the import that its qualifier binds, or the package of the file. A Go file
  with a dot import and a Rust file with a glob use leave the package of an unqualified name empty.
- `Length` keeps the value of a literal length, and is 0 for an expression.
- The fold of an Array folds its element alone.
- `golang.Restate` writes the length again with the spellings that the walk writes for its names. The
  Go speller refuses an array without names whose length refers to another package.
- The Rust backend refuses an array whose name the walk writes otherwise, as it refuses each
  structural reference whose child the walk writes otherwise.

## Alternatives Considered

### The evaluated length

The frontend could record the value of the length in `Length`, and the backend would write
`[32]byte`. The Go frontend evaluates the constants of one package at a time with its imports
stubbed, so `sizes.SaltSize` has no value at the parse of `api`. The Rust frontend has no evaluator
of constant expressions. The generated file would also lose the constant that the source names.

### A field for the names

`TypeRef` could gain a field with the names of the length. Each walk of the model, the resolution
step and the rules would need to visit the field, and the schema would gain a field for one form.

### A refusal alone

The rules could refuse every array whose length refers to a declaration. dokimi would then refuse
each check whose input has such a field.

## Consequences

**Positive:**

- A generated file imports the package of each name of the length of an array, and qualifies a
  constant of the package of the source.
- The rules refuse a length that refers to a declaration that the file cannot refer to, such as an
  unexported constant of another package.

**Negative:**

- An Array with a length expression has more than one child, so a consumer reads its element at
  index 0.
- The fold drops the names, so the shape of an array does not record them.
- Each unit of Go and of Rust parses once more after the release, because the versions of both
  frontends rise.
- The Rust backend refuses an array whose name the walk writes otherwise, because it has no parser of
  a Rust type.

## References

| What | Where |
|---|---|
| RFC-0092, the names of the length of an array | ../rfc/0092-the-names-of-the-length-of-an-array.md |
| D244, the row of this decision | ../architecture/21-decisions.md |
| Issue 84 of dokimi, a derived input of an array field whose length is a constant of another package | <https://github.com/dokimasia/dokimi/issues/84> |
