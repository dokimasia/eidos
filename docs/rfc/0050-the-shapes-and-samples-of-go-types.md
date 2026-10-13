---
rfc: 0050
title: The shapes and samples of Go types
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

# RFC-0050: The shapes and samples of Go types

## Summary

The Go rules derive no sample for a field of a generic struct at its type arguments. They fold
`interface{}` apart from `any`, report every inline struct as unprovably comparable, and report
`time.Time` as a problem where the load does not place the standard library. A struct of a
dependency loses its unexported fields at the signature depth, so the rules report `big.Int` as
comparable. Complex numbers have neither samples nor a zero. We propose samples at the type
arguments, the empty inline interface as the dynamic type, the comparability of an inline struct
from its recorded fields, a mark on a struct whose lowering left out a field, and a table of the
standard library types that a struct field commonly has. Complex numbers get samples and a zero
from the Go table. The verdicts refuse distinct witnesses for the parameters of one declaration and
a shape for a sequence outside a signature.

## Motivation

- `DeriveSamples` of `Box[int]` derives the field `Item T` against the parameter `T`, which has no
  literal, so a builder check of `Box[int]` has no sample. The reference builder skips every field
  that a parameter types.
- The frontend lowers `interface{}` to an inline reference, which the kernel folds to the inline
  form. `any` folds to the dynamic form, so two spellings of one type have two shapes, and
  `ZeroValue` returns no zero for `interface{}`.
- `Comparable` reports every inline struct as a problem. Its docblock states that the model has no
  declaration for the struct, and the reference records the fields of the struct.
- `Comparable` reports `time.Time` as a problem where the reference has no target, because the
  rules have no table for the standard library.
- The frontend leaves out an unexported field at the signature depth, and a blank field at every
  depth. `Comparable` walks the fields that remain, so `big.Int`, whose one field is a slice, is
  comparable, and so is a struct that declares `_ [0]func()` to forbid `==`.
- `complex64` and `complex128` fold to the opaque form, so they have no sample and no zero.
- The witnesses of `Pair[K comparable, V any]` are `int` and `int`.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/rules` | The fold asks the rules of the language about an inline reference before it folds it to the inline form |
| `eidos-lang-go/frontend` | The stamp `golang.hiddenFields` on a struct whose lowering left out a field |
| `eidos-lang-go` | The key `golang.hiddenFields` |
| `eidos-lang-go/rules` | Samples at the type arguments, the empty inline interface, the comparability of an inline struct, of a struct with hidden fields and of the standard library table, and complex numbers |

### Invariants

1. `Comparable` reports true only for a type that Go compares with `==`.
2. The sample of a field of an instantiated generic type is a value of the field at the type
   arguments.
3. `any` and `interface{}` have one shape and one zero.

### The samples of a generic declaration

`DeriveSamples` of a reference to a generic struct substitutes the type arguments of the reference
for the parameters of the struct in the type of each field, through `rules.Substitute`, before it
derives the sample of the field. For a generic defined type, `DeriveSamples` substitutes them in its
target in the same way. A reference without type arguments substitutes nothing, so a field that a
parameter types does not have a literal.

### The empty inline interface

The kernel's fold asks `SourceRules.Builtin` about an inline reference. Where the language returns
a shape other than the opaque one, the fold takes it. The Go rules return the dynamic form for an
inline interface without elements, so `interface{}` folds as `any` does. Every other inline
reference keeps the inline form. `ZeroValue` returns nil for an inline interface, and the empty
composite for an inline struct.

### Comparability

| Type | Comparable where |
|---|---|
| An inline struct | every field that the reference records is comparable, and the spelling of the struct declares no field that the reference leaves out, such as an embedded or a blank field |
| A struct with the mark `golang.hiddenFields` | the table of the standard library lists it as comparable |
| A named reference without a target | the builtin table or the table of the standard library lists it as comparable |

The Go rules parse the spelling of an inline struct with `go/parser` to find a field that the
reference leaves out.

The frontend stamps `golang.hiddenFields` on a struct where it leaves out a blank field, and at the
signature depth where it leaves out an unexported field. The model does not keep the type of such a
field, so the fields that remain do not decide whether the struct is comparable.

The table of the standard library lists each type by its import path and its name, with the answer
of Go. It contains the struct types that a struct field commonly has:

| Comparable | Not comparable |
|---|---|
| `time.Time`, `net/netip.Addr`, `net/netip.AddrPort`, `net/netip.Prefix`, `net/url.URL`, `net/url.Userinfo`, `sync.Mutex`, `sync.RWMutex`, `sync.Once`, `sync.WaitGroup` | `time.Location`, `math/big.Int`, `math/big.Float`, `math/big.Rat`, `bytes.Buffer`, `strings.Builder`, `regexp.Regexp` |

A test checks each entry against `reflect.Type.Comparable` of the type, which reports what the
compiler decided.

### Complex numbers

`DeriveSamples` of `complex64` and `complex128` returns the float literals 1.5 and 2.5, which Go
converts to a complex number, and `ZeroValue` returns 0. The shape remains opaque, because no other
target has a complex number, and a spoke refuses an opaque shape.

### Verdicts out

| Row | Capability | Reason | Reverses the verdict |
|---|---|---|---|
| W5.G19 | Distinct witnesses for the parameters of one declaration | A witness instantiates a declaration so that its generated checks compile. The sample of each field differs from the sample of every other field, so a field that a method reads through the wrong parameter fails its check at equal witnesses too | A check that only distinct type arguments can tell apart |
| W5.G24 | The shape of `iter.Seq` without a target | The return roles classify `iter.Seq` and `iter.Seq2` in a signature, and no generator reads the shape of a sequence elsewhere | A generator that reads a sequence outside a signature |

### Failures

| Condition | Report |
|---|---|
| A struct with hidden fields outside the table | `Comparable` reports the reference to the struct as a problem |
| An inline struct with an embedded or a blank field | `Comparable` reports the inline reference as a problem |

### Migration

1. Register `golang.hiddenFields`, and stamp it in the frontend.
2. Make the fold read the shape that the language gives an inline reference.
3. Change the samples, the zero values and the comparability of the Go rules.

## Alternatives

### The unexported fields at the signature depth

The frontend could keep the unexported fields of a struct at the signature depth. Every struct of
every dependency would then load its private fields, which no generator can set and few read. The
mark costs one stamp for each such struct, and the table answers for the types that fields commonly
have.

### The empty interface in the frontend

The frontend could lower `interface{}` as a reference to `any`. The model keeps the spelling of a
reference verbatim, and a backend would then write `any` where the source wrote `interface{}`.

## Drawbacks

- A struct with a blank field, such as `_ noCopy`, and a struct of a dependency outside the table
  are no longer comparable for the rules, also where Go compares them. A generator then refuses
  them where it needs `==`, which is the cost of a refusal in place of a compile error.
- A version of Go that changes the fields of a type of the table changes its answer, and the test
  of the table fails until the table follows.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.G10, W5.G16, W5.G17, W5.G18, W5.G19, W5.G21 and W5.G24.
- [RFC-0014](0014-the-projection-vocabulary.md), the projection vocabulary.
