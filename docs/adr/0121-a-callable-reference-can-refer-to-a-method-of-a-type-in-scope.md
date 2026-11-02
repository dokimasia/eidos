---
adr: 0121
title: A callable reference can refer to a method of a type in scope
status: Accepted
date: 2026-11-02
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0121: A callable reference can refer to a method of a type in scope

## Status

Accepted

## Context

A reference param under the resolution kind callable-in-scope resolved to a function in scope. A
method subject also resolved a method of its own type. No resolution kind resolved a method of
another type, so a directive on a type could not refer to `store.Writer.Commit`. A generator of the
effects of a sans-IO machine needs that reference. It writes a performer that calls the method on a
double of the type, and it reads the parameters and the results of the method.

Each language writes a reference to a method of a type in its own syntax:

- Go writes a method expression, `T.M`.
- TypeScript and Python join the class and the method with a dot.
- GraphQL writes a schema coordinate, `Type.field`.
- Rust writes a path, `Type::method`.

Go qualifies a function by the name of an import. The Go resolution of a qualifier that no import
binds probes every unaliased import of the file, so the spelling `Store.Get` of a method of a type of
the package resolved to a function `Get` of an unaliased import, such as `net/http.Get`. Go does not
let an import share the name of a declaration of the package, so a qualifier that refers to a type in
scope never refers to an import.

## Decision

We will resolve a method reference in a callable param to the method of a type in scope.

- `rules.Resolver` has the method `MemberSep`, which returns the separator between the spelling of a
  type and the name of one of its members. It is a dot in Go, TypeScript, Python and GraphQL, and `::`
  in Rust.
- A callable resolves among the methods of the type of a method subject first. It then resolves
  through a method reference, and then to a function in scope.
- The walk resolves the spelling before the last separator to a type in scope, through a chain of
  transparent aliases. It resolves the spelling after the separator to a method among the effective
  members of the type that the subject can use. The own type of the subject is the host of a method
  or a field, and the subject itself otherwise.
- Where the type resolves and neither the method nor a function in scope does, the refusal contains
  the type and the method.
- In Go, `T.M` refers to a method with either receiver, and `(*T).M` is not a Go name.
- In Rust, `Type::method` refers to a method at the level of an instance as well as to an associated
  function.

## Alternatives Considered

### A function before a method reference

A function in scope could come first, so every spelling that resolved to a function would keep its
resolution. In Go that order resolves `Store.Get` to a function `Get` of an unaliased import.

### A split of the spelling by the resolver

Each resolver could split the spelling of a method reference itself. Every language splits at its
last separator, so the kernel splits once and each language states its separator alone.

### A resolution kind of its own

A resolution kind for a method of a type would make a schema state whether a param refers to a
function or to a method. A param such as the callable of an effect accepts both, so the schema would
need two params for one reference.

## Consequences

**Positive:**

- A directive on a type refers to a method of another type, such as `store.Writer.Commit`, and
  validation reports a misspelled method at the carrier.
- A Go reference to a method of a type of the package does not resolve to a function of an unaliased
  import.

**Negative:**

- A Rust path to a method at the level of an instance resolves, where the rules refused it.
- An implementation of `rules.Resolver` outside the repository has to add `MemberSep`.
- A Go callable with a qualifier probes the imports of its file twice, the first time for the type
  of a method reference.

## References

| What | Where |
|---|---|
| D239, the row of this decision | ../architecture/21-decisions.md |
| The resolution of a reference | ../architecture/03-projection.md |
| Issue 114, a directive cannot refer to a method of another type | <https://github.com/dokimasia/eidos/issues/114> |
| The Go specification, method expressions | <https://go.dev/ref/spec#Method_expressions> |
| The Go specification, declarations and scope | <https://go.dev/ref/spec#Declarations_and_scope> |
| The GraphQL specification, schema coordinates | <https://spec.graphql.org/draft/#sec-Schema-Coordinates> |
