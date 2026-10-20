---
adr: 0068
title: A property has a delete accessor
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

# ADR-0068: A property has a delete accessor

## Status

Accepted

## Context

A Python property can have a deleter beside its getter and its setter. `del cart.coupon` calls the
method that `@coupon.deleter` decorates. The deleter is a method of its own, with a docstring and
decorators, and a body that a generator can write.

`symbol.Accessor` had the values `AccessorNone`, `AccessorGet` and `AccessorSet`, so the model had
no form for a deleter. The getter and the deleter of one property both take no parameter beside
the receiver, so their discriminators were equal and the second method of the name reported a
duplicate identity.

## Decision

We will add `AccessorDelete` to `symbol.Accessor`, and spell the discriminator of a method with the
delete accessor as `del`, because a deleter is a method of its own that only Python declares.

- `del` is a keyword of Python, so no parameter type of a Python callable has that spelling, and the
  getter and the deleter of a property have two identities.
- The Python frontend loads `@x.deleter` as a method with the delete accessor, and the Python
  backend writes it after the getter and the setter of its property.
- The Python lowering refuses a method with the delete accessor without a getter of its name.
- Every other backend refuses a method with the delete accessor, because no other target deletes a
  property.

## Alternatives Considered

### The deleter folded into the getter

A stamp on the getter of a property could record its deleter. The deleter would lose its docstring
and its decorators, and a generator could not write its body.

## Consequences

**Positive:**

- A Python class loads and writes its properties with their deleters.

**Negative:**

- `symbol.Accessor` gains a value that one language sets, and every backend checks for it.
- The rule of the discriminator has a case that no other accessor has.

## References

| What | Where |
|---|---|
| RFC-0073, the design of this decision | ../rfc/0073-the-python-satellite.md |
| D180, the row of this decision | ../architecture/21-decisions.md |
| The Python documentation of `property` | <https://docs.python.org/3/library/functions.html#property> |
