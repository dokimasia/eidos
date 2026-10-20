---
adr: 0082
title: The render orders the declarations of a file
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0080
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0082: The render orders the declarations of a file

## Status

Accepted

## Context

The render wrote the declarations of a file in the order of the flush, which follows the positions
of their sources. A source of another language puts its declarations in any order.

- A Go source that declares `type Admin struct{ User }` before `type User` gave a Python module whose
  class `Admin` refers to `User` before the module defines it. Python 3.15 raises `NameError`,
  pyright 1.1.413 reports `reportUndefinedVariable`, and tsc 7.0.2 reports TS2449 for the
  TypeScript module.
- tsc 7.0.2 reports TS2391 for overload signatures that another declaration separates from their
  implementation. It reports the same error for a signature after the implementation.

## Decision

We will order the declarations of each file in the render by the names that a module of its
language evaluates when it runs because only the render sees every declaration of a file.

- The functions of one name form one block with the overload signatures first, so the signatures
  come right before the implementation.
- A block follows each block of a name that one of its declarations evaluates. Among the blocks
  that can come next, the block at the lowest position of the flush comes first. A cycle keeps the
  order of the flush.
- A backend states the names through `Builder.Evaluated`, which takes a `render.Evaluated`.
  `Builder.Implemented` declares that each overloaded name of a file has one implementation. The
  render then skips a block of signatures without an implementation and each implementation after
  the first, and reports each under `UnimplementedOverload`.
- A TypeScript module evaluates the superclass and the decorators of a class. A Python module
  evaluates the bases and the decorators of a class, the decorators of a callable and the same
  names of a nested class. Both languages are `Implemented`. A Python stub file states neither,
  because a type checker reads it without running it.
- A language that states neither keeps the order of the flush.

## Alternatives Considered

### The order in each backend

Each backend would order its declarations in its lowering. A lowering sees one declaration at a
time. It cannot move a declaration after another declaration of its file, because that declaration
reaches the file through another unit.

### The order by the values and the types of a file

The order would also follow the names in a default, an initializer, the value of a constant and a
type annotation. A value is text, so the render cannot read the names that it evaluates.
Python below 3.14 writes an annotation of a later declaration as a string, Python 3.14 evaluates an
annotation when a reader asks for it, and TypeScript erases its types.

## Consequences

**Positive:**

- A generator from Go writes a Python or a TypeScript class after the class that it extends.
- tsc accepts the overloads that a generator writes, and the render reports a name without an
  implementation at the file where it occurs.

**Negative:**

- The render orders each file of TypeScript and each module of Python. A render of 1,000 files of
  200 declarations each allocated 613,356 to 613,390 times with the order in eight runs. Without
  the order it allocated about 613,285 times.
- A TypeScript class whose decorator is a function of the file follows the function, although
  TypeScript hoists the function.
- A default or an initializer that calls a later declaration of a file still fails when the module
  runs. A cycle of bases keeps the order of the flush, and the backend refuses the base that comes
  too early.

## References

| What | Where |
|---|---|
| RFC-0080, the design of this decision | ../rfc/0080-overload-signatures-and-the-order-of-a-file.md |
| D200, the row of this decision | ../architecture/21-decisions.md |
| Issue 43, the order of the declarations of a file | <https://github.com/dokimasia/eidos/issues/43> |
| The TypeScript handbook, function overloads | <https://www.typescriptlang.org/docs/handbook/2/functions.html> |
