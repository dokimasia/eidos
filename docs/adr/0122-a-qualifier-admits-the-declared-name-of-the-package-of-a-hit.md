---
adr: 0122
title: A qualifier admits the declared name of the package of a hit
status: Accepted
date: 2026-11-07
supersedes: none
superseded-by: none
rfc: RFC-0090
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0122: A qualifier admits the declared name of the package of a hit

## Status

Accepted

## Context

A Go import without an alias binds the name that the package clause of the imported package
declares. The Go frontend bound such an import to the name that its path assumes, because its parse
cannot read the clause of another unit. `aws/signer/v4` declares `package v4`, and its path assumes
`signer`. No import bound the qualifier of the reference `v4.SignerOptions` in the S3 client of
the AWS SDK.
The probe of every import without an alias found `SignerOptions` in two packages, and the load
reported an ambiguity. A strict run failed every plan, and a run without `strict` could write a type
of the wrong package.

The resolution step has every loaded package with the name that its clause declares. A kept unit and
a unit of the parse memo select their targets from their recorded tiers, without a parse.

## Decision

We will let the resolution step keep a declaration that a candidate names only where the frontend
admits the name that the package of the declaration declares, because the binding of a Go import
depends on the clause of the imported package.

- The optional frontend role `plugin.Qualifier` has the method `Qualifies(spelling, tier, name)`.
  The resolution step and the selection of a kept or restored unit call it for each declaration of
  a tier, with the name of its package.
- Every frontend that the kit builds implements the role. `Builder.Qualifies` declares its
  function, and a declaration without it admits every declaration.
- A warm load selects again the references whose candidates name a declaration of a package that
  an edit renamed.
- The Go frontend returns two tiers: the import whose alias is the qualifier, and each import
  without an alias. `golang.Qualifies` admits a declaration of the second tier only where its
  package declares the qualifier.
- The Go rules admit the declarations of the name of a directive in the same way, with the names
  of the packages of the view.

## Alternatives Considered

### The declared names in the scope of Resolve

The tiers of Resolve would depend on the packages of other units. A kept unit and a unit of the
parse memo would keep tiers that the names of another load decided.

### The declared name in the bindings of the parse

A parse reads the members of its unit and its shared inputs alone, so an edit of the clause of an
imported package would not invalidate the importing unit.

## Consequences

**Positive:**

- A reference through `aws/signer/v4` resolves to that package, and the S3 client of the AWS SDK
  loads without a finding.
- A warm load and the parse memo reuse the recorded tiers of a unit, which no other unit changes.

**Negative:**

- The selection calls the qualifier for each declaration of a hit of every frontend that the kit
  builds.
- Each Go unit parses once after the rise of the version of the Go frontend.
- A qualifier that the path of one import assumes, while the package of another import declares it,
  loads the second package only where another use names it.

## References

| What | Where |
|---|---|
| RFC-0090, the declared name of the package of a reference | ../rfc/0090-the-declared-name-of-the-package-of-a-reference.md |
| D240, the row of this decision | ../architecture/21-decisions.md |
| Issue 121, an import whose package clause differs from its assumed name | <https://github.com/dokimasia/eidos/issues/121> |
