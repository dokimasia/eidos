---
adr: 0070
title: A backend respells a member by its level and its mutability
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

# ADR-0070: A backend respells a member by its level and its mutability

## Status

Accepted

## Context

The settle respells every declared name through the hook `plugin.Respell` of the backend, which
takes the kind of the host, the kind of the declaration, its visibility and the name. PEP 8 spells
a constant in upper case and every other attribute of a class in snake case. A Python class states
its constants as fields at the type level or as immutable fields:

```python
@dataclass(kw_only=True)
class Order:
    CAPACITY: ClassVar[int] = 100
    instances: ClassVar[int] = 0
    channel: Final[str] = "web"
```

The hook saw a field of a class and nothing more, so it turned `CAPACITY` into `capacity`. A
generator that restated the class wrote another class, and a reader of `Order.CAPACITY` broke.
Java has the same convention for a `static final` field, and Kotlin for a `const val`.

## Decision

We will let a backend declare through the role `plugin.MemberRespeller` how it spells the name of a
member by its level and its mutability, because a target's convention for a constant of a class
reads both.

- `plugin.Member` is the facts of a member that a convention reads: the kind of its host, its kind,
  its visibility, its level and its mutability. `RespellMember(m Member, name string) (string,
  error)` spells the name of a field or a method.
- The settle respells each field and each method of a declaration through the role where the
  backend has it, and through `Respell` otherwise. The lowering of the workspace names a member of
  a translated declaration through the role in the same way.
- The backend kit declares the role through `Builder.RespellMember`. A kit backend without a member
  hook serves the role through its `Respell`.
- The Python backend spells a field at the type level or an immutable field in upper case where its
  name has no lower-case letter, and every other member in snake case.

## Alternatives Considered

### A wider hook

`plugin.Respell` would take the level and the mutability of every name. The signature of the hook
of every backend would change, and the declarations that are not members would pass two facts that
no convention reads for them.

### A marker of the generator

A generator would mark a constant of a class with an annotation, and the Python backend would keep
the name of a marked field. Every generator into Python would have to state the marker for every
constant, and a generator into another language would state a fact of Python.

## Consequences

**Positive:**

- A Python class keeps the names of its constants and the snake case of its other members.
- Java and Kotlin can spell a constant of a class in their conventions through the same role.

**Negative:**

- A backend has two hooks of a name, and a member that a backend respells through the role does not
  pass through `Respell`.
- The Java backend does not declare the role, so a `static final` field of Java keeps the camel
  case that its hook writes.

## References

| What | Where |
|---|---|
| RFC-0073, the design of this decision | ../rfc/0073-the-python-satellite.md |
| D182, the row of this decision | ../architecture/21-decisions.md |
| PEP 8, naming conventions | <https://peps.python.org/pep-0008/> |
