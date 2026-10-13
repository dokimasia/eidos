---
rfc: 0054
title: TypeScript declarations that the model loses
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0054: TypeScript declarations that the model loses

## Summary

The TypeScript frontend drops the `abstract` mark of a property, the comments of an import and of an
export, and the conditional type of an alias, which the projection document routes to the key
`typescript.conditionalType`. The rules have no zero value for a map. We propose a key for an
abstract property, the comments of imports and exports in the fields that the model already has, the
key of a conditional type, and the empty object as the zero of a map. Five capabilities of the output
side and of the import graph get the verdict out, because no generator needs them.

## Motivation

- `abstract name: string;` in an abstract class loads as an ordinary property, so a reader of the
  graph cannot tell that a subclass must declare it.
- `node.Import` and `node.Export` have `Doc`, and `node.Import` has `Comment`. The TypeScript
  frontend fills neither.
- The projection document states that a conditional type goes to `typescript.conditionalType`. The
  satellite does not register the key.
- `ZeroValue` returns the empty array for a list and no value for a `Record` or an object with one
  index signature.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-lang-typescript` | The keys `typescript.abstract` and `typescript.conditionalType` |
| `eidos-lang-typescript/frontend` | The stamps of both keys, and the comments of imports and exports |
| `eidos-lang-typescript/rules` | The zero of a map |

### Invariants

1. A fact that the source of a declaration states is in the model, in a field or under a key.
2. `ZeroValue` returns a value of the type, or reports false.

### Abstract properties

The frontend stamps `typescript.abstract` as true on a property that the source declares with
`abstract`. The key admits fields. The model keeps the abstract mark of a method in `Method.Abstract`,
and no other language in scope has abstract properties, so the mark of a property is a key of the
satellite.

### Comments of imports and exports

The frontend reads the doc comment of an import statement into `Import.Doc`, and its trailing
comment into `Import.Comment`. It reads the doc comment of an export statement into `Export.Doc`. A
carrier in such a comment remains refused, because a rule takes no import as its subject.

### Conditional types

The satellite registers `typescript.conditionalType`, which admits aliases. The frontend stamps the
spelling of the target of a type alias on the alias where the target is a conditional type, such as
`T extends string ? A : B`. The model keeps the target as a named reference with that spelling.

### The zero of a map

`ZeroValue` returns the empty object for an object type with one index signature and for a
`Record`. A `Map` and a `ReadonlyMap` have no zero, because their empty value is the construction
`new Map()`, which the value tree does not have.

### Verdicts out

| Capability | Reason | Reverses the verdict |
|---|---|---|
| An abstract property in the emit model | No generator writes an abstract class | A generator that writes an abstract property |
| Function overloads in the emit model | No generator writes two signatures of one function | A generator that writes an overloaded function |
| Ambient declarations in the emit model | No generator writes a declaration file | A generator that writes a declaration file |
| The type-only mark of an import and an export | No projection reads it. The rules resolve a type through every import, and the backend states its own type-only imports | A rule that resolves a value through an import |
| An explicit `public` kept apart from no modifier | No generator keeps the spelling of the visibility of its source | A generator that copies a class member with its modifiers |

### Failures

| Condition | Report |
|---|---|
| A carrier in the comment of an import or an export | TYPESCRIPT-0003, as before |
| The zero of a `Map` | `ZeroValue` reports false, as before |

### Migration

1. Register the two keys.
2. Stamp them in the frontend, and read the comments of imports and exports.
3. Return the empty object as the zero of a map in the rules.

## Alternatives

### An abstract mark in the field schema

`node.Field` could carry `Abstract`, as `node.Method` does. Every backend would then render or refuse
it, and no other language in scope has an abstract property.

## Drawbacks

- A generator of another language that reads `typescript.abstract` depends on a key of the
  TypeScript satellite.

## Open questions

None.

## Unresolved and future work

None.

## References

- [03](../architecture/03-projection.md), the projection vocabulary and its metadata.
