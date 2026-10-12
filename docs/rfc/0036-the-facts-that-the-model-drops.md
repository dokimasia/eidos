---
rfc: 0036
title: The facts that the model drops
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: 0028
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0036: The facts that the model drops

## Summary

The frontends parse these facts, and the symbol model has no field for them:

- A TypeScript method declared with `?`.
- A readonly index signature, a readonly array and a readonly tuple, and the mutability of a Rust
  borrow.
- The `null` and `undefined` members of a TypeScript union that the frontend folds to an optional.
- A computed property key, such as `[Symbol.iterator]`.
- The underlying type of a Go enum, such as `uint8`.
- The hard privacy, the accessor and the index form of a method that a generator mirrors.

A frontend keeps such a fact in the spelling of a reference or in a stamp, or drops it, and no
backend reads the stamps. A generator that mirrors such a declaration writes output without the
fact, and the rules read some facts back from a spelling. We propose a field of the model for
each fact. The frontends fill the fields from the syntax. The rules read them, and the backends
render each fact or refuse it with a positioned finding.

## Motivation

### An optional method

TypeScript lets an interface declare `run?(): void`, which a value of the interface may lack. The
frontend stamps `typescript.optional` on the method, and no plugin, rule or backend reads the stamp.
The field and the parameter of the model have `Optional`, and the method has none. So a mock of the
interface declares a required method, and a value without the method no longer conforms to the
mock.

### Read-only types and index signatures

TypeScript states read-only access three ways. A readonly property sets `Field.Mutability`. A
readonly array or tuple keeps its readonly in the spelling alone. A readonly index signature has a
stamp `typescript.readonly`. The backend reads the emit model, and the emit model has no field for
it, so a mirrored readonly index signature becomes writable.

The hub folds a readonly array to a list shape without the mark. Its TypeScript spoke writes `T[]`,
and its other spokes write the writable list of their language. The fixture adapter of the
TypeScript testing package also writes `T[]`. A Rust frontend keeps `&mut T` and `&T` as one borrow
form whose mutability is in the spelling alone.

### The nullish members of a union

The TypeScript frontend folds `T | null`, `T | undefined` and `T | null | undefined` to an optional
of `T`. The optional has one child, so only the spelling keeps the members that the source wrote.
The TypeScript rules need them, because the zero value of `T | null` is `null` and the zero value of
`T | undefined` is `undefined`. So the rules scan the spelling for a member at its top level,
outside brackets and quoted literals. The model states that the projections parse nothing, and this
scan is a parse.

### A computed property key

A class member named `[Symbol.iterator]` keeps its brackets in its name, because the frontend
records the text of the name node. The backend then quotes the name as a string key,
`'[Symbol.iterator]'`, which declares another member. No field says that the name is an
expression.

### The underlying type of an enum

The Go frontend promotes `type Level uint8` and its constants to an enum and drops the target of the
defined type. The Go backend lowers every emitted enum to `type Level int`, so a regenerated enum
changes its width. The Go rules compute the value outside the set as the largest value plus one,
which overflows a saturated `uint8` set. A Rust enum with `#[repr(u8)]` loses its representation in
the same way.

### The facts of a mirrored method

`eidos.Mirror` copies the name, the visibility, the level, the asynchrony, the type parameters, the
parameters, the returns and the throws of a method. It leaves out the hard privacy of a `#` name,
the accessor of a getter or a setter, the index form of an index signature and the construct form of
a construct signature. So a mirrored getter renders as a method, and a mirrored index signature as
a method named `[]`. A mirrored `#run()` keeps its `#` in the name and loses its hard
privacy, so the backend quotes the name.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/symbol/schema` | `Method` gains `Optional`, `Mutability` and `Computed`. `Field` gains `Computed`. `TypeRef` gains `Mutability` and `Absent`. `Enum` gains `Underlying`. The facts `Computed` and `Underlying` are new |
| `eidos-core/node`, `emit`, `symbol` | generated again from the schema |
| `eidos-core/rules` | `TypeShape` gains `Mutability`, and the fold copies it from a structural reference. The member walk reports a computed member under the gap reason `GapComputed` |
| `eidos-core/backend/render` | the verdict `Rejects` and the code `RejectedFact` |
| `eidos-core` | `Mirror` copies every fact of a method's signature |
| TypeScript satellite | the frontend fills the new fields and drops the stamps `typescript.optional` and `typescript.readonly`. The rules read `Absent` and `Computed`. The backend renders an optional method, a readonly index signature, a computed key and a hard name. The spoke and the fixture adapter write a readonly list and tuple |
| Go satellite | the frontend sets `Enum.Underlying`. The rules bound the value outside the set by it. The backend lowers an enum to its underlying type. The spoke refuses a read-only list and a read-only map |
| Rust satellite | the frontend sets the mutability of a borrow and the underlying type of an enum with a `repr` attribute. The backend writes the `repr` attribute, and the spoke spells a borrow by its mutability and refuses a read-only list and a read-only map |
| Java satellite | the spoke refuses a read-only list, a read-only tuple and a read-only map |
| every backend | its coverage declares a verdict for `Computed` and `Underlying`, and for `Optional` and `Mutability` on a method |

### Invariants

1. A fact that a frontend parses has a field of the model, or the frontend reports a positioned
   finding where it drops the fact.
2. A rule reads every fact from a field and never from a spelling.
3. A backend renders each stated fact, refuses it with `RefusedFact`, or rejects its declaration with
   `RejectedFact`, as its coverage declares.
4. A spoke spells a shape with every fact of the shape or refuses the shape. It never writes a
   spelling with fewer constraints than the shape states.
5. The zero value of each new field states nothing, so a language that does not have the fact
   leaves the field zero and states nothing.

### The schema

    // Method gains three fields.
    type Method struct {
        // The existing fields are unchanged.
        Optional   bool              `eidos:"both,fact=Optional"`   // a value of the type may lack it: TypeScript's ?
        Mutability symbol.Mutability `eidos:"both,fact=Mutability"` // an indexer: whether a holder can assign its entries
        Computed   bool              `eidos:"both,fact=Computed"`   // the name is an expression: TypeScript's [expr]
    }

    // Field gains one field.
    type Field struct {
        // The existing fields are unchanged.
        Computed bool `eidos:"both,fact=Computed"` // the name is an expression: TypeScript's [expr]
    }

    // TypeRef gains two fields.
    type TypeRef struct {
        // The existing fields are unchanged.
        Mutability symbol.Mutability `eidos:"both"` // List, Array, Tuple, Map and Borrow: whether a holder can write through it
        Absent     []*TypeRef        `eidos:"node"` // Optional: the members of the source union that denote absence
    }

    // Enum gains one field.
    type Enum struct {
        // The existing fields are unchanged.
        Underlying *TypeRef `eidos:"both,walk,fact=Underlying"` // the declared representation: Go's uint8, Rust's repr(u8)
    }

`Mutability` on a method states nothing for a method that is not an indexer. `Absent` is empty for
an optional that the source states with a mark, such as Go's `*T` or TypeScript's `T?`, and it lists
the `null` and `undefined` members of a folded union in source order. `Underlying` is nil where the
language does not declare a representation, such as a TypeScript, a protobuf or a Java enum.

The rules fold `TypeRef.Mutability` into the shape:

    type TypeShape struct {
        // The existing fields are unchanged.
        Mutability symbol.Mutability // List, Array, Tuple, Map and Borrow
    }

### The TypeScript satellite

The frontend sets each field from the syntax:

| Syntax | Field |
|---|---|
| `run?(): void` | `Method.Optional` |
| `readonly [key: string]: V` | `Method.Mutability` is Immutable, and a writable index signature has Mutable |
| `readonly T[]`, `readonly [A, B]` | `TypeRef.Mutability` is Immutable, and `T[]` and `[A, B]` have Mutable |
| `T \| null`, `T \| undefined`, `T \| null \| undefined` | the optional of `T` with the written nullish members in `Absent` |
| `[Symbol.iterator]()`, `[key]: V` as a member | `Computed` on the method or the field, with the source text as the name |

The frontend drops the keys `typescript.optional` and `typescript.readonly`. A rest parameter keeps
the type of one argument, so the readonly of its array has no field, and the frontend reports a
Warning at the parameter.

The zero value of an optional is `null` where `Absent` lists `null` and no `undefined`, and
`undefined` otherwise. A literal check accepts `null` where
`Absent` lists `null`, and `undefined` where `Absent` lists `undefined` or is empty. The builtin
table maps `ReadonlyArray<T>` to an Immutable list and `ReadonlyMap<K, V>` to an Immutable map, and
`Array<T>`, `Map<K, V>` and `Record<K, V>` to Mutable ones. `Readonly<T>` and `ReadonlySet<T>` stay
opaque shapes, which every spoke without them refuses.

The kernel's member walk reports a computed member as a gap with the new reason `GapComputed`,
because no other language can call a member by an expression:

    // GapComputed is a member whose name is an expression of its
    // language, such as TypeScript's [Symbol.iterator].
    GapComputed GapReason = 6

The backend renders the facts:

    interface Store {
        run?(): void;
        readonly [key: string]: Row;
        [Symbol.iterator](): Iterator<Row>;
        #flush(): void;
    }

It writes `?` after the key of an optional method, `readonly` before a readonly index signature, a
computed name as written, and a hard name as written with its `#`. A hard name has private
visibility beside its hard privacy, and the backend accepts both.

The spoke writes `readonly T[]` and `readonly [A, B]` for an Immutable list and tuple. The fixture
adapter writes the same spellings.

### The Go satellite

The frontend sets `Enum.Underlying` to the target of the defined type that it promotes, such as
`uint8`. The rules compute the value outside the set as the largest value plus one where the
underlying type can hold it. Otherwise they compute the smallest value minus one. A set that fills
its underlying type has no value outside it, so `EnumInfo.OutOfRange` is zero. The backend lowers an
enum to a defined type of its underlying type. An enum without one lowers to `int`.

The spoke refuses an Immutable list and an Immutable map, because a Go slice and a Go map are
writable. The refusal reads "a read-only list has no Go spelling". The spoke refuses every tuple
already.

### The Rust satellite

The frontend sets the mutability of a borrow: Mutable for `&mut T` and Immutable for `&T`. It sets
`Enum.Underlying` from a `repr` attribute with an integer type, such as `#[repr(u8)]`. The backend
writes the attribute for an enum with an underlying type, and the spoke spells a borrow as `&mut T`
or `&T` by its mutability. The spoke refuses an Immutable list and an Immutable map, because a Rust
collection in a field of a struct is writable through a mutable binding.

### The Java satellite

The spoke refuses an Immutable list, an Immutable tuple and an Immutable map, because a Java
collection type does not state that a holder cannot write through it.

### The mirror

`Mirror` copies every fact of the signature of a method: the name, the visibility, the level, the
asynchrony, the optional mark, the accessor, the index form, the construct form, the hard privacy,
the mutability of an indexer, the computed mark, the type parameters, the parameters, the returns
and the throws. It leaves out the facts of an implementation: abstract, final, override and a
default body. A mirror implements the signature, so those facts belong to the declaration that the
generator writes.

### Coverage

A computed member has no name in Go, Java or Rust, so neither the declaration with the fact nor the
declaration without it renders. The render kit gains a fourth verdict for such a fact:

    const (
        // Rejects means the target cannot render the declaration with the
        // fact or without it: the render reports the stated fact as an
        // Error and leaves the declaration out.
        Rejects Verdict = 4
    )

    // RejectedFact reports a stated fact whose declaration the target
    // cannot render. It is an Error at the declaration.
    var RejectedFact = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
        Number: next, Meaning: "the target cannot render a declaration with a stated fact or without it",
    })

`next` is the next free number of the kernel's codes. The Go backend already refuses six facts with
an Error and skips the declaration: async, abstract, override, a default body, a level and the
mutability of a field or a variable. Its coverage declares them as Rejects, so the coverage table and
the guard state one policy.

| Fact | TypeScript | Go | Java | Rust |
|---|---|---|---|---|
| `Optional` on a method | Renders | Refuses | Refuses | Refuses |
| `Mutability` on a method | Renders | Rejects | Refuses | Refuses |
| `Computed` | Renders | Rejects | Rejects | Rejects |
| `Underlying` | Refuses | Renders | Refuses | Renders |

A refused fact renders the declaration without the fact and reports `RefusedFact`, a Warning. A
rejected fact leaves the declaration out and reports `RejectedFact`, an Error.

### Failures

| Case | Finding |
|---|---|
| A stated fact without a spelling in the target | `RefusedFact`, a Warning, at the declaration |
| A stated fact whose declaration the target cannot render | `RejectedFact`, an Error, at the declaration |
| A read-only list or tuple in a spoke that cannot spell it | the refusal of the spoke, which the translation reports at the reference |
| A readonly rest parameter in TypeScript | a Warning of the frontend at the parameter |
| A computed member in the member walk | a gap of the walk with the reason `GapComputed` |

### Migration

1. Add the fields to the schema and generate the models, the codec, the fingerprint and the facade
   again.
2. Fill the fields in the frontends, and delete the two TypeScript keys.
3. Read the fields in the rules, and delete the scan of the TypeScript rules.
4. Render the fields in the backends and the spokes, and declare each verdict.
5. Copy the facts in `Mirror`.

The model fingerprint changes, so the first run of a new binary runs cold and every parse memo entry
misses once. The sealed state does not need a migration.

## Alternatives

### Keep the facts as stamps

A stamp addresses a declaration with an identity, and a backend reads the emit model alone. The
optional method and the readonly index signature have stamps today, and no backend can render them.
A type reference has no identity, so no stamp can address the readonly of an array or the nullish
members of a union.

### Keep the nullish members as children of the optional

The optional would list `T` and then its nullish members. Every reader of an optional reads one
child, the kernel's check of type arguments takes one, and every spoke checks the arity of one. A
separate field leaves the arity of the form intact.

### Fold `T | null` to a union

The frontend would fold only `T | undefined` to an optional, as the docblock of the form names it,
and keep `T | null` as a union. The hub then sees a union, and every spoke without unions refuses a
field of type `T | null`, which most TypeScript code uses to state an optional value.

### Let every spoke spell a read-only list as its writable list

A Go slice, a Java list and a Rust vector would accept a mirrored readonly array, and the output
would drop the constraint without a finding. The ground rule refuses a narrowing without a finding.

### Name the underlying type in a Go key

The Go frontend could stamp the target of a promoted enum, as it stamps the kind of an underlying
type today. The Go backend reads the emit model alone, and a Rust enum has the same fact.

## Drawbacks

- The schema gains seven fields and two facts, and `TypeShape` gains one field.
- The render kit gains a verdict and a code. Each of the four backends declares the verdicts of the
  two new facts, eight entries in all, and the Go backend moves six verdicts to Rejects.
- Every sealed state and every parse memo entry runs cold once, because the model fingerprint
  changes.
- A TypeScript readonly array or map that a generator mirrors into Go, Java or Rust now fails its
  translation, where the output declared a writable list or map before.
- A Go enum that a generator mirrors into TypeScript or Java now reports `RefusedFact` for its
  underlying type.

## Open questions

None.

## Unresolved and future work

None.

## References

- The symbol model: [`docs/architecture/02-symbol-model.md`](../architecture/02-symbol-model.md).
- The cross-language hub: [RFC-0023](0023-the-cross-language-hub.md).
- The improvement plan rows W2.2, W2.4, W2.7, W2.11, W2.13, W6.X21 and W6.X22, and decision D-7.
