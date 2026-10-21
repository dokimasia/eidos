---
adr: 0095
title: A TypeScript sample constructs its class and sets every required property of its interface
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0095: A TypeScript sample constructs its class and sets every required property of its interface

## Status

Accepted

## Context

The TypeScript rules derived the sample of a class or an interface as an object literal that set its
first required property. The literal was asserted to the type with `as`, because it did not set the
other properties. RFC-0023 states that rule in its row of `SamplesOf`.

- The assertion made the type checker accept a value that breaks its type at run time. The sample of
  a class was no instance of the class, so it had none of its methods, and `instanceof` was false. The
  sample of an interface left every other required property undefined.
- Issue 68 showed both on Node.js 26.12.0. For a class `Row` with a method `describe`, the sample
  `{ id: 1.5 } as Row` failed `sample.describe()` and `sample.label.length` with a `TypeError`.
- The value tree now has a construction, which the samples of a `Set` and a `Map` use. The row of
  RFC-0023 states that it had none.
- The Python rules construct a class through its constructor, and set each required key of a
  `TypedDict`.

## Decision

We will derive the sample of an interface as a value of the interface, and the sample of a class as a
construction of the class. Neither carries an assertion.

- An interface takes an object literal that sets each required property from the pair of its type.
  The alternate sets the first property with its alternate value. An interface without a required
  property sets its first property whose pair derives.
- An interface with a required method refuses with no literal, because an object literal of its
  properties does not implement the method. An interface whose required property does not derive
  refuses with the refusal of that property.
- A class takes a construction through its first public constructor whose required parameters
  derive. The construction passes the sample of each required parameter. A constructor without a
  required parameter takes the sample of its first parameter where that parameter derives.
- `Object.assign` then sets each public, writable and required field of an instance that has no
  initializer and no parameter of its name, where the pair of the field derives. A field that the
  library declares is left out.
- The alternate of a class differs in its first argument, or else in the first field that it sets.
  A class without an argument and without such a field sets its first such field with an
  initializer. A class without one takes its construction alone and refuses the alternate.
- An abstract class and a class without a public constructor refuse with no literal.
- The type of a property and of a parameter takes the type arguments of the reference.
- The TypeScript backend spells a callee with an owner as the owner, a dot and the name. The owner
  takes the place of the name in the import and in the scope. `Object.assign` is the member `assign`
  of the owner `Object` in the root of the store of the library.

## Alternatives Considered

### The assertion over an object literal of every property

An object literal that sets each required property of a class is still no instance of the class. The
methods remain undefined, and `instanceof` remains false.

### Python's rule for a class without a required parameter

The Python rules refuse a class whose constructor passes no parameter that derives. A TypeScript class
often declares its fields with initializers and no constructor, so that rule leaves such a class
without a sample.

### A value of the kernel that sets fields after a construction

Every target other than TypeScript would refuse such a value. `Object.assign` is the idiom of
TypeScript for it. A call of a member of a global does not need a new value.

### Every writable field after the construction

A field with an initializer or a parameter of its name has its value from the constructor. A sample
that overwrites it can break an invariant of the class that the constructor establishes.

## Consequences

**Positive:**

- A sample of a class or an interface is a value of its type at run time.
- The type checker checks each sample, because no assertion hides a missing property.

**Negative:**

- An interface with a required method has no sample.
- The frontend does not read the body of a constructor, so a sample can set a field that the body
  of the constructor sets already.
- A sample of a class is longer. The projection pass of the rules over their fixture allocates 617
  times instead of 520.

## References

| What | Where |
|---|---|
| D213, the row of this decision | ../architecture/21-decisions.md |
| RFC-0023, whose row of `SamplesOf` this decision replaces for a class and an interface | ../rfc/0023-the-cross-language-hub.md |
| The two values of a type that `DeriveSamples` returns | ../architecture/03-projection.md |
| Issue 68, the sample of a class or an interface breaks its type at run time | <https://github.com/dokimasia/eidos/issues/68> |
