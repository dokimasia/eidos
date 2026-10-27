---
adr: 0109
title: An unexported method is a member of its package
status: Accepted
date: 2026-10-27
supersedes: ADR-0104
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0109: An unexported method is a member of its package

## Status

Accepted

## Context

A generator of test doubles reads the method set of an interface through `Bound.MembersOf`. It checks
a candidate type with `Bound.Satisfies`, or lists the candidates with `Bound.Implementers`. Both
compare the member set of a type with the members of the interface.

- At the signature depth the Go frontend left out every unexported method of an interface. Issue 90
  reports `testing.TB`, whose walk listed its 23 exported methods and not `private`. A plugin could
  not tell that code outside the package `testing` cannot implement `testing.TB`.
- The frontend stamped `golang.emptyInterface` on an exported interface whose methods are all
  unexported, because it lowered none of them.
- The walk and `Satisfies` matched members by name alone. The Go specification makes two unexported
  identifiers of two packages different identifiers. A struct of another package with the methods
  `Area` and `area` satisfied an interface `Shape` with the same methods.
- At the signature depth the frontend also left out `func (c *common) private()`. A walk that listed
  `private` of `testing.TB` would not have found an implementation of it in the package `testing`.

## Decision

We will make the member walk scope an unexported name to its package. A signature-only Go parse will
load each unexported method that an implementation of a loaded interface needs.

- `rules.MemberPolicy.PackageScopedNames` makes the name of a field or a method of package
  visibility unique to the package of its owner. The walk settles such a name apart from the same
  name of another package. `Satisfies` matches such a member of an interface only to a member whose
  owner is in the package of the owner of the interface member. The name of an embedded field is
  not scoped, because an embed has no visibility.
- The Go rules set `PackageScopedNames`. A struct that embeds `testing.TB` implements `testing.TB`,
  because the owner of the promoted `private` is `testing.TB`.
- A signature-only parse of the Go frontend lowers each unexported method of an interface that it
  loads. The method has package visibility. The parse also loads the method of that name of each
  type of the same package, also of a type that the parse does not load. `testing.common` keeps
  `private`, so `testing.T` still implements `testing.TB`.
- This decision supersedes the rule of ADR-0104 that a kept type loads its exported fields and
  methods alone. A kept type also loads each unexported method of a name that the previous bullet
  keeps.
- The Go frontend has the version 0.12.0.

## Alternatives Considered

### A stamp on an interface with a hidden method

A stamp in the manner of `golang.hiddenFields` would let a plugin refuse such an interface. The
member set would still lack the method. `Satisfies` would then report a type with the exported
methods alone as an implementation.

### A method of `SatisfactionRules` for the identity of a name

The Go rules implement the interface. A new method of it is a breaking change of the SPI. Go keeps
two promoted methods of one name from two packages apart, and the walk would still cancel both.

### A rule of the kernel for every name of package visibility

The scope of a name is a rule of the language, and the walk takes its other rules from the policy
of each language. A rule of the kernel would also change the member sets of Java classes. No spec of
the Java rules states such a change.

### Every unexported method of a dependency

A comparison of member sets reads only the unexported methods whose names an interface of the
package declares. The other unexported methods of a dependency would add declarations that no
projection reads.

## Consequences

**Positive:**

- The walk of an interface of a dependency lists its whole method set.
- `Satisfies` and `Implementers` refuse a type of another package for an interface with an
  unexported method, as the Go compiler does.
- An exported interface whose methods are all unexported no longer has `golang.emptyInterface`.
- The walk and `Satisfies` allocate as before. In interleaved runs of the benchmarks of the rules,
  `MembersOf` measured 8.8% faster and `Satisfies` 4.2% faster.

**Negative:**

- A signature that an unexported method of an interface names can refer to an unexported type that
  the signature depth does not load, such as `uncommon() *uncommonType` of `reflect.Type`. The
  reference has no target, and the comparison reads its spelling.
- A dependency unit loads more methods, and the packages that their signatures name.

## References

| What | Where |
|---|---|
| D227, the row of this decision | ../architecture/21-decisions.md |
| ADR-0104, the walk through an alias and an unexported embed | 0104-the-member-walk-follows-an-alias-and-an-unexported-embed-of-a-dependency.md |
| The member walk | ../architecture/03-projection.md |
| The signature-only parse of the Go frontend | ../architecture/11-languages.md |
| The Go specification, uniqueness of identifiers | <https://go.dev/ref/spec#Uniqueness_of_identifiers> |
| The Go specification, method sets | <https://go.dev/ref/spec#Method_sets> |
| Issue 90, a dependency unit lists an exported interface without its unexported methods | <https://github.com/dokimasia/eidos/issues/90> |
