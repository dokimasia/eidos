---
rfc: 0072
title: Translated supertypes
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-10-19
updated: 2026-10-19
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0072: Translated supertypes

## Summary

A generator translates every type of another language through
`Emitter.Type`. The spoke of the target spells the type without knowing
where the declaration uses it. Rust needs that difference. The type of a
value that is a trait is the boxed trait object `Box<dyn Node>`, and a
trait that another trait extends is the bare name `Node`. This RFC adds
`Emitter.Supertype`. It translates a type that a declaration extends,
implements or embeds, or that bounds a type parameter. A target can
register a second spelling for that position. Every other target spells
both positions alike.

## Motivation

The Rust spoke, `spell.Type` in `eidos-lang-rust/spell/type.go`, spells a
reference to a declaration of another language as its flat name. The Rust
backend renders an interface of another language as a trait, so a field
of such a type renders as `node: Node`. Rust 2024, the edition of
the Rust toolchain adapter, refuses a bare trait in a type position with
E0782, "Trait objects must include the `dyn` keyword". `dyn Node` alone is
unsized, so a field, a parameter, a result and a type argument take the
owned trait object `Box<dyn Node>`.

The spoke cannot spell `Box<dyn Node>` for every reference, because the
same reference reaches it as a supertype. The GraphQL model plugin of the
conformance fixture translates the interfaces that an interface implements
through `Emitter.Type`, in `eidos-conformance/lang/graphql/model.go`. A
Rust plan of the same schema needs `trait Named: Node`, and rustc refuses
`trait Named: Box<dyn Node>`, because `Box` is no trait.

Both positions spell alike in every other target:

| Target | A value of an interface `Node` | A supertype `Node` |
|---|---|---|
| Go | `Node` | `Node`, embedded |
| TypeScript, Java | `Node` | `Node` after `extends` |
| GraphQL | `Node` | `Node` after `implements` |
| Rust | `Box<dyn Node>` | `Node` after the colon of the trait |

The difference is a fact of the position, which the generator knows and
the spoke does not. So the position belongs to the call of the generator,
and its spelling belongs to the spoke of the target.

## Detailed design

### The emitter

```go
// Supertype translates a type that the declaration owner extends,
// implements or embeds, or that bounds one of its type parameters. It
// translates as [Emitter.Type] does, and spells the shape through the
// supertype spelling of the plan's backend, which a backend without one
// spells as a value. Its refusals and its reads are those of Type.
func (e *Emitter) Supertype(owner symbol.Identity, ref *node.TypeRef) (*emit.TypeRef, bool)
```

`Emitter.Type` keeps its signature and translates a type in a value
position: the type of a field, a parameter, a result, an alias, a constant
or a variant payload.

### The backend

The spelling of the supertype position is an optional role of a backend,
so no implementation of `plugin.TypeSpeller` changes:

```go
// SupertypeSpeller is the role of a backend whose target spells a
// supertype of another language otherwise than a value. The emitter
// asserts it on the plan's TypeSpeller, and spells a supertype through
// SpellType where the backend does not implement it.
type SupertypeSpeller interface {
	// SpellSupertype returns the target's reference for s where a
	// declaration extends, implements or embeds it, or where it bounds a
	// type parameter. s is a reference or a sum.
	SpellSupertype(s rules.TypeShape, p Policy) (*emit.TypeRef, error)
}
```

A target declares its supertype spelling through the backend kit. A built
backend implements `SupertypeSpeller` through that declaration, or through
the spoke of `Types` where the target declares none:

```go
// Supertypes declares the target's spelling of a supertype of another
// language, which the built backend serves as
// [plugin.SupertypeSpeller]. A target without it spells a supertype as
// the spoke of [Builder.Types] spells a value.
func (b *Builder) Supertypes(spell func(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error)) *Builder
```

### Rust

The Rust value spoke spells a reference whose referent is an interface as
`Box<dyn Name>`. The argument of `Box` is an Intersection with the
spelling `dyn Name` and the reference as its one element. The Rust
frontend reads `dyn Trait` in the same form. The settle and the layout
resolve the element as they resolve any reference. Where the name of the
trait changes, the settle rewrites the spelling of the Intersection. An
optional of a trait is `Option<Box<dyn Name>>`, and a list is
`Vec<Box<dyn Name>>`. The spoke spells every child in a value position.

The Rust supertype spelling is the reference spelling of the spoke: the
flat name with the translated arguments. Each argument is a value. A trait
that extends `Page<Node>` writes `Page<Box<dyn Node>>`.

### The callers

- The model and client plugins of the GraphQL conformance fixture translate
  `Extends` through `Emitter.Supertype`.
- The hub of the conformance matrix gains the probe of a reference to an
  interface, so the table shows `Box<dyn Session>` for Rust.

## Alternatives considered

### A position in the shape

`rules.TypeShape` could contain the position for every spoke to read.

**Why not:** a shape is the type, and the same type takes both positions.
Five of the six spokes would ignore the field.

### A rewrite in the Rust lowering

The Rust backend could rewrite the translated trait references of fields,
parameters and results into `Box<dyn …>` after the settle.

**Why not:** the lowering would have to rebuild the written spellings of
arrays, tuples and function types around a rewritten child. The spoke
builds those spellings already, and it spells each child in the position
that the child takes.

### A supertype that bypasses the spoke

`Emitter.Supertype` could build the flat name and the arguments itself.

**Why not:** a well-known supertype takes the spelling that the spoke of
the target gives it, such as `error` for the well-known error in Go, and
only the spoke knows that spelling.

## Drawbacks

- One emitter method, one optional role and one builder method in the
  kernel, and their facades in the SDK.
- A generator chooses the method by position. A generator that translates
  a supertype through `Emitter.Type` writes `Box<dyn …>` in a Rust
  supertrait, and rustc refuses it.
- A trait that is not dyn-compatible, such as one with a generic method,
  cannot be a `Box<dyn …>`. rustc refuses such a field in the toolchain
  check, and the spoke does not see the methods of the trait.

## References

| What | Where |
|---|---|
| Rust error E0782, a trait object without `dyn` | <https://doc.rust-lang.org/error_codes/E0782.html> |
| The Rust reference, dyn compatibility | <https://doc.rust-lang.org/reference/items/traits.html#dyn-compatibility> |
