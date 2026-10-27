---
adr: 0104
title: The member walk follows an alias and an unexported embed of a dependency
status: Accepted
date: 2026-10-25
supersedes: none
superseded-by: ADR-0109
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0104: The member walk follows an alias and an unexported embed of a dependency

## Status

Accepted

## Context

`Bound.MembersOf` walks the effective member set of a type. It reads the member lists of a
contributor from a struct, an interface, an enum or a sum, and it records a gap for a contributor
that it cannot follow. A generator that needs the whole member set, such as a generator of test
doubles, declines a type whose set has a gap.

- The Go frontend lowers a type spec whose type is not a struct or an interface literal to an
  alias. `type Key [16]byte`, `type Count int` and `net.IP` are defined types, aliases with
  `Defined` set. The alias kind has no member lists. The fold of the frontend attaches a method to
  a struct or an enum of its package, and every other method remains a declaration of the file
  whose identity names its receiver as the owner.
- The walk reported the gap `not-membered` for every contributor whose declaration is an alias.
  Issue 78 reports a struct that embeds `Key`, `Count` or `net.IP`, which lost the methods that Go
  promotes from the embedded type.
- Go gives a defined type the methods that its package declares for it. A defined type has none of
  the methods of the type under it, and it keeps the fields and the embeds of a struct under it and
  the method set of an interface under it.
- At the signature depth the Go frontend left out every unexported declaration and kept every
  embed of an exported struct. `testing.T` embeds the unexported struct `common`, and
  `assert.Recorder` embeds the unexported alias `calls`. Issue 79 reports the gap `unresolved` for
  both embeds, because the embedded type had no declaration.
- The `MethodSet` of the Go rules already read the methods of a defined type from the declarations
  of its package.

## Decision

We will make the member walk follow a contributor whose declaration is an alias, and make a
signature-only Go parse keep each unexported type whose members an exported type promotes.

- A contributor whose declaration is an alias contributes these members at the depth of the alias:
  - the methods that the package of the alias declares outside every type with the name of the
    alias as their owner
  - for a transparent alias, what its target contributes, through each alias of the chain
  - for a defined type, the fields and the embeds of a struct under it, or every member of an
    interface under it
- A defined type contributes none of the methods that the type under it declares. Under a defined
  type, the walk does not read members from a builtin that the language classifies as a scalar, a
  boolean, a text, bytes or the top type. It does not read members from a structural type other
  than an inline body either.
- A target that the walk cannot follow is a gap. A target without a declaration in the view is
  `unresolved`. An inline body, and a structural type under a transparent alias, are
  `not-membered`. A declaration on the path, and a chain of more than eight aliases, are `cyclic`.
  Arguments that differ in number from the parameters of an alias are `generic`.
- Under `ShadowLinearise` the declaration under an alias is the one base of the alias, so the
  linearization of the walked type contains the owner of every member that the alias contributes.
- `MembersOf` of an alias reports false, as before.
- A signature-only parse of the Go frontend keeps an unexported type that an exported type of its
  package embeds or names as the target of an alias. It also keeps each unexported type that a kept
  type reaches the same way. A kept type loads its exported fields and methods alone.
- The parse reads every file of a unit before it lowers the first, because an embed and its type
  can be in two files. The Go frontend has the version 0.11.0.

## Alternatives Considered

### A list of methods on the alias kind

The schema generates the node model of every language from one symbol schema. A list that only the
Go frontend fills would enter the model of every language, the codec of the graph and each frontend
that lowers an alias. The declarations of the package already contain the methods of a defined
type.

### A capability of the Go rules for the members of a defined type

The walk belongs to the kernel, with its depth, its provenance and its gaps. A capability would
restate each of them for one language.

### A transparent alias of a builtin without a gap

The walk does not classify a builtin that a contributor refers to directly, and a builtin of Python
or TypeScript has members. If a transparent alias of `str` did not contribute members, a Python
class would lose the members of `str` without a gap.

### Every unexported type of a dependency

A dependency unit would load most of its unexported declarations. Generated code cannot refer to an
unexported type of another package, and the walk reads only the types that an exported type
promotes.

## Consequences

**Positive:**

- A struct that embeds a defined type has the methods that Go promotes from it. `MethodSet`,
  `ErrorContract`, `Satisfies` and the settable fields of the Go rules read the whole set.
- A struct that embeds `*testing.T` or `*assert.Recorder` has a complete member set.
- A TypeScript interface that extends an alias of an interface has the members of the interface.

**Negative:**

- A walk through an alias reads the package of the alias and scans its declarations for the
  methods of the alias. The read set of the walk records the package.
- A dependency unit loads each unexported type that an exported type of its package promotes, with
  the exported members of that type.
- A transparent alias of a builtin is `unresolved`, as an embed of the builtin is.

## References

| What | Where |
|---|---|
| D222, the row of this decision | ../architecture/21-decisions.md |
| D83, the walks of the kernel | ../architecture/21-decisions.md |
| The member walk | ../architecture/03-projection.md |
| The signature-only parse of the Go frontend | ../architecture/11-languages.md |
| RFC-0014, the projection vocabulary | ../rfc/0014-the-projection-vocabulary.md |
| The Go specification, method sets | <https://go.dev/ref/spec#Method_sets> |
| The Go specification, type definitions | <https://go.dev/ref/spec#Type_definitions> |
| Issue 78, an embedded defined type leaves a gap in the member walk | <https://github.com/dokimasia/eidos/issues/78> |
| Issue 79, an unexported embed in a dependency unit has no target | <https://github.com/dokimasia/eidos/issues/79> |
