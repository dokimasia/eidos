---
rfc: 0081
title: A type parameter inside a structural type
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-20
updated: 2026-10-20
discussion: https://github.com/dokimasia/eidos/issues/41
supersedes: none
superseded-by: none
produces-adr: ADR-0083
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0081: A type parameter inside a structural type

## Summary

We propose that the fold of a reference to a type parameter keeps the identity of the parameter in
its opaque shape. The spoke of each language with type parameters spells the shape as the name of
the parameter at any depth of a structural type. The spokes of protobuf and GraphQL refuse it.

## Motivation

- The fold gives a reference to a type parameter the opaque shape, because the argument of the
  parameter decides its shape. No spoke spells an opaque shape. A generator that translates a
  generic declaration refuses every member whose type contains a parameter inside a structural
  type, such as `tuple[T_co, T_co]`, `[]T` and `Optional[T]`.
- `Emitter.Type` spells a reference to a type parameter by its name before the fold. A member of
  the type `T` translates, and a member of the type `list[T]` does not.
- The TypeScript client of the shop fixture of the Python satellite leaves out the alias
  `Pair: TypeAlias = tuple[T_co, T_co]`. Its translation reports EID-0069: "the typescript target
  cannot spell the python type tuple[T_co,T_co]: typescript: T_co: a type that the rules of its
  language do not classify has no TypeScript spelling".
- tsc 7.0.2 reports TS2637 for a variance annotation on an alias of a reference, a union, a list
  or a tuple, and TS1274 for one on a function and a method. The TypeScript backend writes the
  declared variance of every type parameter, so the client would not type-check once `Pair`
  translates.
- The declarative plugin translates the bounds of type parameters and the interface of an
  implementation through `Emitter.Type`. Rust spells a value of a trait as `Box<dyn Store>` and a
  supertrait as `Store`, so a Rust plan of the plugin would write a value where a bound or the
  trait of an impl belongs.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/rules` | the fold keeps the identity of a type parameter in the `Ref` of its opaque shape |
| Kernel | `eidos-core` | `Emitter.Type` passes a reference to a type parameter to the spoke, as it passes every other reference |
| Kernel | `eidos-core` | `Emitter.TypeParams` translates the type parameters of a declaration |
| Declarative | `eidos-declarative` | the mirrors translate their type parameters through `Emitter.TypeParams`, and an implementation translates its interface as a supertype |
| SDK | `eidos-sdk` | the regenerated facade |
| Lang | `eidos-lang/spoke` | `Param` returns the reference of a type parameter, and `Describe` names the shape |
| Go, TypeScript, Java, Rust, Python | `spell` | the spoke spells a type parameter by its name |
| protobuf, GraphQL | `spell` | the spoke refuses a type parameter |
| TypeScript | `backend` | the lowering leaves out the variance where TypeScript admits no annotation |
| Conformance | `lang/python`, `matrix` | the TypeScript client writes a generic alias, and the hub probes a type parameter |

### The fold

```go
package rules

type TypeShape struct {
    // The other fields are unchanged.
    Ref symbol.Identity // Reference and Sum, and the type parameter of Opaque
}
```

A reference to a type parameter folds to `Opaque`, and `Ref` is the identity of the parameter. A
reader of the fold that switches on the form reads an unknown type as before. The samples, the
zero values and the satisfaction checks are such readers. The argument of the parameter still
decides its shape.

### The translation

`Emitter.Type` and `Emitter.Supertype` fold every reference of another language and pass the shape
to the spoke of the target. The emitter does not spell a reference to a type parameter before the
fold, so the spoke decides at every depth of a reference. A plan without a spoke refuses a type
parameter as it refuses every other reference of another language.

```go
package eidos

// TypeParams translates the type parameters tps of the declaration owner
// into the plan's target. Each emit parameter keeps the name, the variance,
// the const mark and the spelling of the default value of its parameter.
// Its bounds translate as supertypes of owner, and its default and the type
// of its value as types. It returns nil and true for no parameter. It
// reports false after a refusal, and then returns no parameters.
func (e *Emitter) TypeParams(owner symbol.Identity, tps []*node.TypeParam) ([]*emit.TypeParam, bool)
```

The declarative plugin calls `Emitter.TypeParams`, and so does the client generator of the shop.
The declarative plugin translates the interface of an implementation through `Emitter.Supertype`. A bound and the interface of an implementation then take the supertype
spelling of the target, such as `Store` in Rust.

### The spokes

```go
package spoke

// Param returns the reference that a spoke writes for the shape s of a
// type parameter: the spelling of the parameter without a target, because
// the declaration that the generator writes declares the parameter under
// its name. It reports false for every other shape. It allocates the
// reference.
func Param(s rules.TypeShape) (*emit.TypeRef, bool)
```

`Describe` returns "a type parameter" for the shape, so the refusal of a list of `T` reads
"protobuf: T: a type parameter has no protobuf spelling".

| Target | A type parameter |
|---|---|
| Go | its name |
| TypeScript | its name |
| Java | its name |
| Rust | its name |
| Python | its name |
| protobuf | refuses, because a message has no type parameters |
| GraphQL | refuses, because a type has no type parameters |

Each spoke with type parameters checks the shape before its forms, at every depth, so a list of
`T` is `T[]` in TypeScript and `list[T]` in Python.

### The variance in TypeScript

TypeScript admits a variance annotation on a type parameter of a class, of an interface, and of an
alias of an object type, a function type, a constructor type or a mapped type. tsc 7.0.2 reports
TS1274 for the annotation on a function and a method, and TS2637 for one on an alias of a
reference, a union, a list or a tuple. The TypeScript lowering gives each such declaration a copy
of its type parameters without the variance.

- The lowering keeps the variance of an alias whose target is a function type, and of an alias
  whose target opens with `{`. The TypeScript frontend loads an object type as an inline type, an
  object of one index signature as a map and a mapped type as a named type, and each keeps the
  spelling of its source.
- TypeScript infers the variance of every generic type from its structure, and a function and a
  method have no variance. Each declaration keeps its meaning.

### The fixture

The client generator of the shop writes an alias with type parameters, which it translates through
`Emitter.TypeParams`. The alias `Pair` becomes
`export type Pair<T_co extends Line = Line> = readonly [T_co, T_co];`, and tsc type-checks the
client. The hub has a row for a type parameter and a row for a list of a type parameter.

### Failures

| Failure | Effect |
|---|---|
| A type parameter in a target without type parameters | the spoke refuses it, and the emitter reports `RefusedType` at the owner |
| A type parameter in a plan without a spoke | the emitter refuses it, as it refuses every other reference of another language |
| A bound, a default or the type of a value without a spelling | `Emitter.TypeParams` reports false after the refusal, and the generator withholds its declaration |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| A type parameter inside a structural type | in | a generic declaration translates into every language with type parameters | none known |
| The identity of the parameter in its opaque shape | in | a reader that classifies a shape keeps reading an unknown shape, and a spoke reads the identity alone | a reader that needs the bound of a parameter |
| The spelling before the fold at the top of a reference | out | the spoke decides at every depth, and protobuf and GraphQL refuse a parameter at the top as they refuse one inside | none known |
| The variance of a TypeScript callable, and of an alias whose target admits none | out | tsc 7.0.2 reports TS1274 and TS2637, and TypeScript infers the variance from the structure | none known |
| A translation of type parameters in the emitter | in | the declarative plugin and the client of the shop translate type parameters, and the declarative plugin translated a bound as a value | a generator that keeps less of a type parameter than `Emitter.TypeParams` keeps |

## Alternatives

### A form of a type parameter

The kernel would add a leaf form for a type parameter. Every switch on the forms of a shape in the
kernel, the satellites and the plugins would need a branch for it. Each reader that treats a
parameter as an unknown type would read the new form as opaque.

### The parameters in the emitter

The emitter would walk a reference and replace each parameter with its name before the fold. The
fold folds a whole reference, so the emitter would rebuild every structural form around a
parameter, such as the list of `list[T]`.

### A refusal of the variance in TypeScript

The TypeScript backend would refuse a declaration whose type parameter states a variance where
TypeScript does not accept an annotation. A Python alias such as `tuple[T_co, T_co]` would not translate,
and neither would a Python function over a covariant type variable.

## Drawbacks

- The fold keeps one more identity in a shape, and the spokes of five languages check the shape
  before their forms.
- A type parameter at the top of a reference refuses in protobuf, in GraphQL and in a plan without
  a spoke, where the emitter spelled its name.
- A TypeScript alias of a tuple or a union, a TypeScript function and a TypeScript method lose the
  variance of their source. tsc checks the variance that it infers.
- The output of the backends of all seven languages changes. Each backend bumps its version, so
  the fingerprint of each composition that renders through one of them changes.

## Open questions

None.

## Unresolved and future work

- The bound of a type parameter in its shape, for a reader that derives a sample from the bound.

## References

| What | Where |
|---|---|
| Issue 41, a type parameter inside a structural type | <https://github.com/dokimasia/eidos/issues/41> |
| The TypeScript handbook, variance annotations | <https://www.typescriptlang.org/docs/handbook/2/generics.html> |
