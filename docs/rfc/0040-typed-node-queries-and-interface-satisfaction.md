---
rfc: 0040
title: Typed node queries and interface satisfaction
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

# RFC-0040: Typed node queries and interface satisfaction

## Summary

A plugin that looks for the interfaces of a package or the method of a struct by name writes a
loop over untyped symbols. No query states whether a type satisfies an interface, which members it
misses, or which types of the graph implement an interface. We propose three generic queries in the
node package and two queries with one optional capability in the rules.

## Motivation

The symbol model document names a query surface such as `Interfaces()` and `InterfaceByName`, and
the node package has none. `store.Reader.ByKind` returns untyped symbols. handlergen finds the method
`Serve` with a loop over `Struct.Methods`.

The kernel compares method signatures only to fold the member set of an interface. The Go annotator
checks the methods `Error` and `String` alone. A plugin that writes a double, a stub or an adapter
cannot ask whether a type satisfies an interface, or which members a candidate misses, without
repeating the member walk and the signature comparison.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/node` | `DeclsOf`, `DeclNamed` and `MemberNamed`, generic over the declaration type |
| `eidos-core/rules` | `Bound.Satisfies`, `Bound.Implementers`, `Satisfaction`, `Mismatch` and the optional capability `SatisfactionRules` |
| Go satellite | the rules implement `SatisfactionRules` |
| `eidos-reference/handlergen` | finds `Serve` through `node.MemberNamed` |
| `docs/architecture` | the symbol model and projection documents name the queries |

### Invariants

1. A typed query returns the declarations that a loop over the files of a package returns, in the
   same order.
2. A satisfaction compares the member sets that `Bound.MembersOf` returns, so it applies the member
   policy of the language.
3. A query reads the graph through the view, so a run records the reads that it makes.

### Node queries

```go
// DeclsOf returns the top-level declarations of p of the type T, such as
// every *Interface of a package, in the order of the files and of their
// declarations. It allocates nothing beyond the iterator.
func DeclsOf[T Declaration](p *Package) iter.Seq[T]

// DeclNamed returns the first top-level declaration of p of the type T
// whose identity has the name, and false where p declares none. It
// allocates nothing.
func DeclNamed[T Declaration](p *Package, name string) (T, bool)

// MemberNamed returns the first member of a list whose identity has the
// name, such as a method of a struct, and false where the list has none.
// It allocates nothing.
func MemberNamed[T Declaration](members []T, name string) (T, bool)
```

The queries are generic functions and not methods per kind, because a function over the type
parameter covers every kind of the schema without a generator.

### Satisfaction

```go
// Satisfaction is the comparison of the member set of a type with the
// members of an interface.
type Satisfaction struct {
    // Missing are the members of the interface that the type has no
    // member of the name of, in the order of the interface.
    Missing []Member
    // Mismatched are the members of the interface that the type has
    // with another signature or another field type.
    Mismatched []Mismatch
    // Gaps are the gaps of the member walk of either side. A gap
    // leaves the comparison unproven.
    Gaps []Gap
}

// Mismatch pairs a member of an interface with the member of the same
// name that a type has.
type Mismatch struct {
    Want Member
    Got  Member
}

// Satisfied reports whether the type has every member of the interface
// with its signature, and the walk of both sides is complete.
func (s Satisfaction) Satisfied() bool

// Satisfies compares the member set of the type that t names with the
// members of the interface that iface names. It substitutes the type
// arguments of each reference into the members that it compares, and
// compares a method by its parameter and return types and a field by
// its type. Where t is not an optional and the rules implement
// SatisfactionRules, a method that a value of the type does not have
// counts as missing. It reports false where t or iface does not name a
// declaration of the view, where iface names no interface, and where
// the rules report that the interface constrains a type set.
func (b Bound) Satisfies(t, iface *node.TypeRef) (Satisfaction, bool)

// Implementers returns the identities of the declarations of the view
// whose member sets satisfy the interface that iface names, in identity
// order. It reads every declaration of the kinds that have members,
// through ByKind of the reader of the view, so a run that calls it
// reruns where a declaration of those kinds enters or leaves the graph.
func (b Bound) Implementers(iface *node.TypeRef) []symbol.Identity
```

```go
// SatisfactionRules is the optional capability of a language whose
// satisfaction reads more than the member sets: a method that belongs
// to a pointer to its type alone, such as a Go method with a pointer
// receiver, and an interface that constrains a type set, such as a Go
// constraint.
type SatisfactionRules interface {
    // ValueReceiver reports whether a value of the type of m has m. It
    // allocates nothing.
    ValueReceiver(m *node.Method, v View) bool
    // TypeSet reports whether i constrains a type set beside its
    // methods, which a comparison of members cannot prove. It allocates
    // nothing.
    TypeSet(i *node.Interface, v View) bool
}
```

The Go rules implement both methods from the stamps `golang.receiverIsPointer` and
`golang.constraintInterface` of the Go frontend.

A method compares by the parameter and return types that `rules` keys a member signature with
today, so two types that name one declaration in two packages compare equal, and a variadic
parameter differs from a list parameter.

### Failures

The queries add no finding. `Satisfies` returns false for a reference that does not name a
declaration, and `Implementers` returns no identity for such an interface.

### Migration

1. Add the node queries, and find `Serve` in handlergen through `MemberNamed`.
2. Add the satisfaction queries and `SatisfactionRules` to the rules, and implement the capability in
   the Go rules.
3. Name the queries in the symbol model and projection documents.

## Alternatives

### Methods per kind

`Package.Interfaces()` and `Package.InterfaceByName` read well, and they need either a generator or
twenty hand-written methods for the kinds of the schema. A generic function covers every kind.

### A satisfaction per language

Each language would repeat the member walk and the signature comparison. The member walk already
applies the policy of each language, and the receiver of a Go method and the type set of a Go
interface are the two facts that the walk does not read.

## Drawbacks

- `Implementers` reads every declaration that has members, about one member walk per candidate, so a
  plugin that calls it for each interface of a large graph pays for each call.
- The rules gain one optional capability and two queries, and the node package three functions.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.K3, W5.K4 and W5.K5.
- The Go specification on method sets: <https://go.dev/ref/spec#Method_sets>.
