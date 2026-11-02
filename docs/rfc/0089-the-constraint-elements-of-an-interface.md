---
rfc: 0089
title: The constraint elements of an interface
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-11-02
updated: 2026-11-02
discussion: https://github.com/dokimasia/eidos/issues/112
supersedes: none
superseded-by: none
produces-adr: ADR-0120
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0089: The constraint elements of an interface

## Summary

We propose that an interface state its constraint elements as type references. `Interface` gains
`Constraints` on both sides of the model, under the fact `Constraints`. The Go frontend fills it
from each union, approximation and type term of an interface. The Go rules and the Go backend read
it, and every other backend refuses it. The stamp `golang.typeSet`, which records the elements as
text, is removed.

## Motivation

- The Go frontend records the constraint elements of an interface as their spellings under the stamp
  `golang.typeSet`. A spelling uses the import names of the file of the interface and has no
  target, so a generator in another package cannot resolve a term such as `protocol.Ack`.
- A generator that needs the instantiations of a type whose type parameter has a union constraint,
  such as one case of a type switch for each instantiation, can read the terms of the union only as
  text.
- The Go backend does not write constraint elements. A generator that copies a constraint interface
  into Go writes an interface without its type set.

RFC-0056 kept the stamp, because a declared interface had no reference for its elements. This RFC
gives the interface those references.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/symbol/schema` | `Constraints` on `Interface`, under the fact `Constraints`, and the regenerated model |
| Kernel | `eidos-core/toolchain` | the surface of a package lists the constraint elements of an interface |
| SDK | `eidos-sdk` | the regenerated facade |
| Go | `eidos-lang-go` | the frontend fills `Constraints` and stamps no type set, the rules read the terms, and the backend writes each element |
| TypeScript, Java, Rust, Python, protobuf, GraphQL | the backends | the verdict `Refuses` on the fact |

### The model

```go
package schema

type Interface struct {
    // The other fields are unchanged.
    Constraints []*TypeRef `eidos:"both,walk,fact=Constraints"` // the unions and type terms of the type set
}
```

Each element is one type reference. A union is a reference of the form `Union` with a child for
each term in the order of the source, and an approximation outside a union is a union of one term,
as RFC-0056 lowers both. The spelling of a union keeps the approximation marks. Every other element,
such as `int` or `[]byte`, is the reference of its type. The resolution step resolves each term, so
a term that refers to a declaration has its target, and a term of another package has the path of
its package.

`Constraints` is nil for an interface of a language without type sets. The field belongs to a
declared interface. An inline interface, such as the bound `interface{ ~int }` of a type parameter,
keeps its elements as the children of its reference.

### The Go frontend

`lowerInterfaceBody` appends the reference of each constraint element to `Constraints`, in the
order of the source, and does not stamp `golang.typeSet`. It stamps `golang.constraintInterface` on
an interface with constraint elements, as before, because a gate, a fact read and the rules read
that classification.

### The Go rules

The witness of a type parameter reads the terms of `Constraints`. An interface without methods,
embeds and supertypes admits `int` where each of its elements is `int` or a union with the term
`int`. A view without facts reads the terms as well, because they are a field of the model. The Go
rules do not keep a handle for a stamp of the type set.

### The backends

| Target | Verdict |
|---|---|
| Go | renders each element on a line of its own after the embeds and the supertypes of the interface. The shared walk binds the import of each term, and it keeps the spelling of a union where it writes each term as the source wrote it |
| TypeScript, Java, Rust, Python | refuses, because an interface, a trait or a protocol of the language has no type set |
| protobuf, GraphQL | refuses, because the schema language has no interface with a type set |

### The surface

The surface of a package lists the line `<interface> constrains <spelling>` for each constraint
element of an interface.

### Failures

| Failure | Effect |
|---|---|
| A constraint element in a target without type sets | the render reports it under `RefusedFact` and writes the interface without it |
| A union that the Go backend cannot write as the source wrote it | the shared walk writes the terms again joined by `\|`, or refuses the union, as RFC-0056 states |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| `Constraints` on an interface | in | Go states the type set of an interface, and a generator reads and writes it | none known |
| the stamp `golang.typeSet` | out | the field contains the same elements with their targets, so the stamp would record them twice | a consumer that needs the text of an element apart from the spelling of its reference |
| the stamp `golang.constraintInterface` | in | a gate, a fact read and the rules select a constraint interface by the stamp, and a gate does not read a field | none known |

## Alternatives

### The terms in the stamp

The stamp could record a reference for each term. A stamp is a fact of one language, so the
resolution step, the walk of each backend and the surface would not see the references.

### The elements as embeds

Each element could be an embed of the interface. An embed promotes the members of its type, and a
union has no members, so the member walk would meet a contributor that it cannot follow.

### One union for the whole type set

The interface could state its type set as one reference. Go intersects the type sets of the
elements of an interface, and a union states a union, so one reference cannot state two elements.

## Drawbacks

- `Interface` gains one field, so each backend gains one fact to state a verdict on.
- The removal of `golang.typeSet` breaks a plugin that reads the stamp.
- The Go backend writes the constraint elements after the embeds and the supertypes. A copy of a
  source that interleaves them has another order and the same type set.

## Open questions

None.

## References

| What | Where |
|---|---|
| Issue 112, the type set of a constraint is text | <https://github.com/dokimasia/eidos/issues/112> |
| RFC-0056, the references that a generated file writes | 0056-the-references-that-a-generated-file-writes.md |
| ADR-0022, the named handles of the rules | ../adr/0022-a-rules-implementation-reads-a-named-key-through-fact.md |
| The Go specification, interface types | <https://go.dev/ref/spec#Interface_types> |
