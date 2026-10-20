---
adr: 0069
title: A target that overloads lets the overloads of a function share its name
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0073
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0069: A target that overloads lets the overloads of a function share its name

## Status

Accepted

## Context

The layout checks the file-level names of a plan in the namespace of each routed file. Two names of
one namespace that settle to one name report `CollidingNames`, and the finding fails the plan.

Python declares the overloads of a function as functions of one name in one module. Each overload
has the decorator `overload` of typing, and the implementation follows them under the same name:

```python
@overload
def find(key: int) -> Line: ...
@overload
def find(key: str) -> Line | None: ...
def find(key: int | str) -> Line | None: ...
```

The Python frontend loads each overload and the implementation as a function of its own, with
distinct identities. A generator that restates them writes three functions of one name into one
module, and the check of names failed every such plan. In Go and in Rust, two functions of one name
in one package do not compile, so the check is right for them.

## Decision

We will let a target declare through the role `plugin.Overloader` that its language overloads
functions, and the layout will not count the functions of one emitted name in one routed scope as
a collision of names where the target overloads, because Python declares the overloads of a
function under one name.

- `Overloads() bool` reports whether the language of the target declares the overloads of a
  function. The backend kit declares the role through `Builder.Overloads`, and every kit backend
  implements it.
- The exemption covers a group whose declarations are all functions of the emitted name of the
  first. Two emitted names that settle to one name still collide, and so do a function and a
  declaration of another kind of one name.
- The Python backend declares the role. Its writer of a function refuses a function of a name that
  an earlier function of its file declares without the decorator `overload`, as its lowering refuses
  two methods of one name in a class outside the overloads of one method. The import set records
  each function without the decorator, and `ImportSet.Has` reads the record back.

## Alternatives Considered

### Overloads as one name in every target

The layout would count every group of functions of one emitted name as the overloads of one
function. A Go or a Rust plan that writes two functions of one name would then pass the check and
fail to compile.

### Distinct names for the overloads

A generator would give each overload a name of its own. The decorator `overload` of typing
requires the overloads and the implementation to share one name, so a type checker would not read
them as one function.

### A setting of the plan

A plan would turn the check of functions off. A composition would then state a fact of the language
of its target, and every composition with a Python plan would state it.

## Consequences

**Positive:**

- A Python plan writes the overloads of a function and its implementation into one module.
- The check of names keeps its findings for the targets whose languages do not overload.

**Negative:**

- The layout leaves the check of the overloads of a name to the backend of a target that overloads.
  A backend that declares the role and does not check its overloads writes two functions of one
  name without a finding.
- TypeScript also declares overloads of a function, and its backend does not declare the role. The
  TypeScript backend writes a body for every function, so two functions of one name would not
  compile there either.

## References

| What | Where |
|---|---|
| RFC-0073, the design of this decision | ../rfc/0073-the-python-satellite.md |
| RFC-0070, the check of the names of a routed scope | ../rfc/0070-names-collide-in-the-routed-package.md |
| D181, the row of this decision | ../architecture/21-decisions.md |
| The typing specification, overloads | <https://typing.python.org/en/latest/spec/overload.html> |
