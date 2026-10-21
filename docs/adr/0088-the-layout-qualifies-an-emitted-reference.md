---
adr: 0088
title: The layout qualifies an emitted reference by the file of its referent
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: RFC-0083
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0088: The layout qualifies an emitted reference by the file of its referent

## Status

Accepted

## Context

The layout qualifies a reference between two declarations of the plan that are written in files of
two packages. It found the referent of two kinds of reference:

- A bare reference has neither a target nor a package. The layout finds its referent among the
  declarations of the package of its unit.
- A translated reference has a target in another language than the plan. The layout finds its
  referent by the origin and the settled name.

A generator could not refer to a declaration that another generator emits into another package.

- In a Go reproduction a fixture generator writes `ThingFixture` into the package `modeltest`, and
  a generator for the package `store` refers to it. The layout did not find the referent of the
  bare reference in `store`. The reference by origin took the package of its origin, `model`. The
  Go compiler refused both fields.
- The TypeScript backend imported a bare reference that the layout qualified from its package as
  written, such as `model/thing.maker`. tsc and Node read that path as the name of a package, so
  the import did not resolve.
- A reference by origin in the language of the plan is ambiguous. A generator of data transfer
  objects emits `dto.Thing` from `model.Thing`. A reference to the source `model.Thing` has the
  same target and the same spelling as a reference to `dto.Thing`.

## Decision

We will mark a reference to a declaration that the plan emits with `emit.TypeRef.Emitted`, and
the layout will qualify a marked reference by the package of the file of its referent in every
language, because only the generator knows whether a reference by origin means the source or a
declaration of the plan.

- A generator sets the mark with the origin of the referent as the target and the emitted name of
  the referent as the spelling.
- The layout finds the referent of a marked reference as it finds the referent of a translated
  reference. It sets the package of the reference to the package of the file of the referent. It
  clears the package where both files are in one package.
- The layout sets the mark on each bare reference that it qualifies into another package.
- `spellref.PackageOf` returns the package of a marked reference. The TypeScript backend imports a
  marked reference from the module path of its referent relative to the file, such as
  `./thing.maker`. The protobuf backend qualifies a marked reference to a protobuf declaration
  with the package clause of the file of its referent.
- The layout refuses the referencing declaration under `UnemittedReference`, EID-0081, where the
  plan does not emit the referent of a marked reference.
- `plugin.Emit.Translates` reports a store with a marked reference, so the layout runs its pass of
  references.

## Alternatives Considered

### Matching by origin and name without a mark

The layout would qualify every reference in the language of the plan whose target and spelling
match a declaration of the plan. A reference to the source `model.Thing` would then read
`dto.Thing` in the plan of the data transfer objects.

### A field for the origin beside the target

`TypeRef` would get an `Origin`, and `Target` would keep the source declaration. The settle and the
layout key a reference by origin on `Target` already, so a second identity would split one rule
across two fields.

### The layout sets the target to zero

With a zero target, `PackageOf` would return the package of a qualified reference. The TypeScript
backend chooses between a module path of the plan and a specifier of the source by the target. A
bare reference has no target either. The backend would then read the same fields for a module path
of the plan and for a specifier of the source.

### A relative specifier from the layout

A specifier such as `./thing.maker` is a rule of TypeScript, and the layout serves every language.
With the specifier in the package of the reference, the Go backend would receive a path that is
not an import path.

## Consequences

**Positive:**

- In the Go reproduction both fields `ByOrigin` read `modeltest.ThingFixture`, and each file
  imports `example.com/shop/model/modeltest`.
- In the TypeScript reproduction `model/thing.user.ts` imports `ThingFixture` from
  `./thing.maker`.
- A backend reads one fact for a reference of a generator and for a bare reference that the layout
  qualifies.

**Negative:**

- Every generator can set one more field of `TypeRef`. A mark without a referent refuses the
  referencing declaration. The same reference without the mark rendered with the package of its
  target.
- `plugin.Emit.Translates` keeps its name, and it also reports an emitted reference.
- The layout writes a field that a generator also writes, so the field does not record which of
  the two set it.
- A bare reference still refers to the package of its unit. A generator that refers to a
  declaration of another package has to know the origin of the declaration.

## References

| What | Where |
|---|---|
| RFC-0083, the design of this decision | ../rfc/0083-references-to-declarations-that-the-plan-emits.md |
| D206, the row of this decision | ../architecture/21-decisions.md |
| Issue 53, a reference by origin in the language of the plan | <https://github.com/dokimasia/eidos/issues/53> |
| Issue 54, the import of a reference that the layout qualifies | <https://github.com/dokimasia/eidos/issues/54> |
