---
adr: 0020
title: The interface rules account for properties
status: Accepted
date: 2026-10-11
supersedes: none
superseded-by: none
rfc: RFC-0025
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0020: The interface rules account for properties

## Status

Accepted

## Context

A declarative rule reads the effective members of its subject through the rules of the subject's
language. The members of an interface are its methods and its properties. A TypeScript interface
can declare properties, and a Go interface cannot.

The accepted design gave both interface rules the methods alone. A mirror of an interface contained
the effective methods, and an implementation contained a method for each effective method. Under
that rule a mirror of an interface with a property leaves the property out, and the run does not
report the gap. An implementation leaves the property out too. The TypeScript compiler then refuses the
class with TS2420, because a class that implements an interface needs each property of the
interface.

Each method of an implementation passes its call on to the field `next`. A property has no call to
pass on.

## Decision

We will make both interface rules account for each property of the subject, because the output of
a rule must contain each member of its subject or a finding that reports the member.

- A mirror of an interface contains a field for each effective property. The field has the name,
  the visibility, the documentation and the type of the property.
- A rule that emits implementations reports `UndelegatedProperty` (DECLARATIVE-0002) at an
  interface with a property. The message lists the properties, and the plugin emits nothing for the
  subject.

## Alternatives Considered

### Methods alone

Both rules read the methods of the subject and skip its properties. The accepted design specified
this rule. Both rules give a Go interface the same output, because a Go interface cannot declare a
property.

It lost because a mirror of a TypeScript interface with a property then differs from its subject,
and the run does not report the difference. An implementation of such an interface does not type-check.
TypeScript 7.0.2 reports TS2420 for a class that leaves out a property of its interface.

## Consequences

**Positive:**

- A mirror contains every member of its subject.
- Each property of a subject is in the output or in a finding at the subject.

**Negative:**

- A rule that emits implementations does not serve an interface with a property. Such an interface
  needs a typed plugin.
- `UndelegatedProperty` is a code of the module. Its number and its meaning are stable once a
  release contains it.

**Neutral:**

- A Go interface cannot declare a property, so the files of the conformance fixture do not change.

## References

| What | Where |
|---|---|
| D65, the effective member set and the reason for each gap | ../architecture/21-decisions.md |
| TypeScript: the implements clause of a class | https://www.typescriptlang.org/docs/handbook/2/classes.html#implements-clauses |
